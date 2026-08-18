package http

import (
	"bufio"
	"bytes"
	"context"
	"encoding/csv"
	stdhttp "net/http"
	"strings"
	"time"

	"api-gateway/api/proto/userpb"

	"github.com/gin-gonic/gin"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

const (
	userListImportMaxBytes    = int64(64 << 10)
	userListImportLookupBatch = 100
	userListImportPageSize    = int32(100)
	userListImportTimeout     = 2 * time.Minute
)

type userListImportRecord struct {
	Name     string
	Username string
}

func (h *Handler) importUserLists(c *gin.Context) {
	if h == nil || h.clients == nil || h.clients.File == nil || h.clients.User == nil || h.clients.UserLists == nil || h.attachments == nil {
		writeError(c, stdhttp.StatusServiceUnavailable, "user list import dependencies unavailable", "service_unavailable")
		return
	}
	fileID, ok := bindImportFileID(c)
	if !ok {
		return
	}
	ownerID := currentUserID(c)
	if !h.allowUserListImportRateLimit(c, ownerID) {
		return
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), userListImportTimeout)
	defer cancel()
	payload, ok := h.readOwnedImportFile(c, ctx, ownerID, fileID, userListImportMaxBytes)
	if !ok {
		return
	}
	localHost, err := h.exportAccountHost()
	if err != nil {
		writeError(c, stdhttp.StatusServiceUnavailable, "public account host unavailable", "service_unavailable")
		return
	}
	records, err := parseUserListImportRecords(payload, localHost)
	if err != nil {
		writeError(c, stdhttp.StatusBadRequest, "invalid user list import file", "bad_request")
		return
	}
	if len(records) == 0 {
		c.Status(stdhttp.StatusNoContent)
		return
	}

	usernames := uniqueUserListImportUsernames(records)
	users, err := h.resolveUserListImportUsers(ctx, usernames)
	if err != nil {
		writeRPCError(c, err)
		return
	}
	lists, err := h.loadOwnedUserListsForImport(ctx, ownerID)
	if err != nil {
		writeRPCError(c, err)
		return
	}
	for _, record := range records {
		targetID, found := users[strings.ToLower(record.Username)]
		if !found {
			continue
		}
		listKey := strings.ToLower(record.Name)
		list := lists[listKey]
		if list == nil {
			response, createErr := h.clients.UserLists.CreateUserList(ctx, &userpb.CreateUserListRequest{
				OwnerId: ownerID, Name: record.Name, IsPublic: false,
			})
			if createErr != nil {
				if status.Code(createErr) == codes.AlreadyExists {
					lists, err = h.loadOwnedUserListsForImport(ctx, ownerID)
					if err != nil {
						writeRPCError(c, err)
						return
					}
					list = lists[listKey]
				}
				if list == nil {
					if ignorableUserListImportError(createErr) {
						continue
					}
					writeRPCError(c, createErr)
					return
				}
			} else {
				list = response.GetUserList()
				if list == nil || list.GetId() <= 0 {
					writeError(c, stdhttp.StatusBadGateway, "created user list unavailable", "upstream_invalid_response")
					return
				}
				lists[listKey] = list
			}
		}
		_, addErr := h.clients.UserLists.AddUserListMember(ctx, &userpb.UserListMemberRequest{
			OwnerId: ownerID, ListId: list.GetId(), UserId: targetID,
		})
		if addErr == nil || ignorableUserListImportError(addErr) {
			continue
		}
		writeRPCError(c, addErr)
		return
	}
	c.Status(stdhttp.StatusNoContent)
}

func parseUserListImportRecords(payload []byte, localHost string) ([]userListImportRecord, error) {
	result := make([]userListImportRecord, 0)
	seen := make(map[string]struct{})
	scanner := bufio.NewScanner(bytes.NewReader(payload))
	scanner.Buffer(make([]byte, 1024), len(payload)+1)
	for scanner.Scan() {
		line := strings.TrimSpace(strings.TrimPrefix(scanner.Text(), "\ufeff"))
		if line == "" {
			continue
		}
		reader := csv.NewReader(strings.NewReader(line))
		reader.FieldsPerRecord = -1
		fields, err := reader.Read()
		if err != nil || len(fields) < 2 {
			continue
		}
		name := strings.TrimSpace(strings.Join(fields[:len(fields)-1], ","))
		account := strings.TrimPrefix(strings.TrimSpace(fields[len(fields)-1]), "@")
		separator := strings.LastIndex(account, "@")
		if name == "" || len([]rune(name)) > userListNameMaxRunes || separator <= 0 || separator == len(account)-1 {
			continue
		}
		username := strings.TrimSpace(account[:separator])
		host := strings.TrimSpace(account[separator+1:])
		if username == "" || strings.Contains(username, "@") || !strings.EqualFold(host, localHost) {
			continue
		}
		key := strings.ToLower(name) + "\x00" + strings.ToLower(username)
		if _, duplicate := seen[key]; duplicate {
			continue
		}
		seen[key] = struct{}{}
		result = append(result, userListImportRecord{Name: name, Username: username})
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	return result, nil
}

func uniqueUserListImportUsernames(records []userListImportRecord) []string {
	result := make([]string, 0, len(records))
	seen := make(map[string]struct{}, len(records))
	for _, record := range records {
		key := strings.ToLower(record.Username)
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		result = append(result, record.Username)
	}
	return result
}

func (h *Handler) resolveUserListImportUsers(ctx context.Context, usernames []string) (map[string]int64, error) {
	users := make(map[string]int64, len(usernames))
	for start := 0; start < len(usernames); start += userListImportLookupBatch {
		end := min(len(usernames), start+userListImportLookupBatch)
		batch := usernames[start:end]
		response, err := h.clients.User.ListUsers(ctx, &userpb.ListUsersRequest{
			Usernames: batch, Status: userStatusActive, Page: 1, PageSize: int32(len(batch)),
		})
		if err != nil {
			return nil, err
		}
		for _, user := range response.GetItems() {
			if user == nil || user.GetId() <= 0 || user.GetStatus() != userStatusActive || !publicAccountStateActive(user.GetAccountState()) {
				continue
			}
			username := strings.ToLower(strings.TrimSpace(user.GetUsername()))
			if username != "" {
				users[username] = user.GetId()
			}
		}
	}
	return users, nil
}

func (h *Handler) loadOwnedUserListsForImport(ctx context.Context, ownerID int64) (map[string]*userpb.UserListInfo, error) {
	result := make(map[string]*userpb.UserListInfo)
	var listsRead int64
	for page := int32(1); ; page++ {
		response, err := h.clients.UserLists.ListUserLists(ctx, &userpb.ListUserListsRequest{
			ViewerId: ownerID, OwnerId: ownerID, Page: page, PageSize: userListImportPageSize,
		})
		if err != nil {
			return nil, err
		}
		items := response.GetItems()
		for _, list := range items {
			if list == nil || list.GetId() <= 0 || list.GetOwnerId() != ownerID {
				continue
			}
			name := strings.ToLower(strings.TrimSpace(list.GetName()))
			if name != "" {
				result[name] = list
			}
		}
		listsRead += int64(len(items))
		if len(items) < int(userListImportPageSize) || listsRead >= response.GetTotal() {
			break
		}
	}
	return result, nil
}

func ignorableUserListImportError(err error) bool {
	switch status.Code(err) {
	case codes.AlreadyExists, codes.NotFound, codes.FailedPrecondition:
		return true
	default:
		return false
	}
}
