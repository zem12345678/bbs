package http

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	stdhttp "net/http"
	"strings"
	"time"

	"api-gateway/api/proto/userpb"

	"github.com/gin-gonic/gin"
)

const relationshipExportPageSize = int32(100)

type followingExportRequest struct {
	ExcludeMuting   bool `json:"excludeMuting"`
	ExcludeInactive bool `json:"excludeInactive"`
}

func (h *Handler) exportFollowing(c *gin.Context) {
	var request followingExportRequest
	if c.Request.Body != nil {
		c.Request.Body = stdhttp.MaxBytesReader(c.Writer, c.Request.Body, 1024)
		decoder := json.NewDecoder(c.Request.Body)
		decoder.DisallowUnknownFields()
		if err := decoder.Decode(&request); err != nil && !errors.Is(err, io.EOF) {
			writeError(c, stdhttp.StatusBadRequest, "invalid request body", "bad_request")
			return
		}
		if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
			writeError(c, stdhttp.StatusBadRequest, "invalid request body", "bad_request")
			return
		}
	}
	if h == nil || h.clients == nil || h.clients.User == nil || (request.ExcludeMuting && h.clients.UserSafety == nil) {
		writeError(c, stdhttp.StatusServiceUnavailable, "following export dependencies unavailable", "service_unavailable")
		return
	}
	h.deliverUserExport(c, userExportSpec{
		label: "following", filenamePrefix: "following", exportedEntity: "following",
		extension: ".csv", contentType: "text/csv; charset=utf-8",
		gate: h.followingExportGate,
		build: func(ctx context.Context, userID int64) ([]byte, error) {
			return h.buildFollowingExport(ctx, userID, request)
		},
	})
}

func (h *Handler) buildFollowingExport(ctx context.Context, userID int64, request followingExportRequest) ([]byte, error) {
	muted := map[int64]struct{}{}
	if request.ExcludeMuting {
		var afterID int64
		for {
			response, err := h.clients.UserSafety.ListMutedUsers(ctx, &userpb.ListUserRelationsRequest{
				ActorId: userID, PageSize: relationshipExportPageSize,
				AfterTargetId: afterID, AscendingByTargetId: true,
			})
			if err != nil {
				return nil, err
			}
			items := response.GetItems()
			if len(items) == 0 {
				break
			}
			for _, user := range items {
				if user == nil || user.GetId() <= afterID {
					return nil, errors.New("invalid muted export user")
				}
				afterID = user.GetId()
				muted[afterID] = struct{}{}
			}
			if len(items) < int(relationshipExportPageSize) {
				break
			}
		}
	}

	host, err := h.exportAccountHost()
	if err != nil {
		return nil, err
	}
	inactiveBefore := time.Now().Add(-90 * 24 * time.Hour).UnixMilli()
	var result strings.Builder
	var afterID int64
	for {
		response, err := h.clients.User.ListFollowing(ctx, &userpb.ListFollowsRequest{
			UserId: userID, PageSize: relationshipExportPageSize,
			AfterUserId: afterID, AscendingByUserId: true,
		})
		if err != nil {
			return nil, err
		}
		items := response.GetItems()
		if len(items) == 0 {
			break
		}
		for _, user := range items {
			if user == nil || user.GetId() <= afterID || strings.TrimSpace(user.GetUsername()) == "" {
				return nil, errors.New("invalid following export user")
			}
			afterID = user.GetId()
			if _, excluded := muted[user.GetId()]; excluded {
				continue
			}
			if request.ExcludeInactive && user.GetUpdatedAt() > 0 && user.GetUpdatedAt() < inactiveBefore {
				continue
			}
			result.WriteString(user.GetUsername())
			result.WriteByte('@')
			result.WriteString(host)
			result.WriteByte('\n')
		}
		if len(items) < int(relationshipExportPageSize) {
			break
		}
	}
	return []byte(result.String()), nil
}

func (h *Handler) exportUserLists(c *gin.Context) {
	if h == nil || h.clients == nil || h.clients.UserLists == nil {
		writeError(c, stdhttp.StatusServiceUnavailable, "user list export dependencies unavailable", "service_unavailable")
		return
	}
	h.deliverUserExport(c, userExportSpec{
		label: "user list", filenamePrefix: "user-lists", exportedEntity: "userList",
		extension: ".csv", contentType: "text/csv; charset=utf-8",
		gate: h.userListExportGate, build: h.buildUserListsExport,
	})
}

func (h *Handler) buildUserListsExport(ctx context.Context, userID int64) ([]byte, error) {
	host, err := h.exportAccountHost()
	if err != nil {
		return nil, err
	}
	var result strings.Builder
	var listsRead int64
	for page := int32(1); ; page++ {
		response, err := h.clients.UserLists.ListUserLists(ctx, &userpb.ListUserListsRequest{
			ViewerId: userID, OwnerId: userID, Page: page, PageSize: relationshipExportPageSize,
		})
		if err != nil {
			return nil, err
		}
		lists := response.GetItems()
		for _, list := range lists {
			if list == nil || list.GetId() <= 0 {
				return nil, errors.New("invalid user list export record")
			}
			if err := h.writeUserListExportMembers(ctx, &result, userID, list, host); err != nil {
				return nil, err
			}
		}
		listsRead += int64(len(lists))
		if len(lists) < int(relationshipExportPageSize) || listsRead >= response.GetTotal() {
			break
		}
	}
	return []byte(result.String()), nil
}

func (h *Handler) writeUserListExportMembers(ctx context.Context, result *strings.Builder, userID int64, list *userpb.UserListInfo, host string) error {
	var membersRead int64
	for page := int32(1); ; page++ {
		response, err := h.clients.UserLists.ListUserListMembers(ctx, &userpb.ListUserListMembersRequest{
			ViewerId: userID, ListId: list.GetId(), Page: page, PageSize: relationshipExportPageSize,
		})
		if err != nil {
			return err
		}
		members := response.GetItems()
		for _, member := range members {
			if member == nil || strings.TrimSpace(member.GetUsername()) == "" {
				return errors.New("invalid user list export member")
			}
			result.WriteString(list.GetName())
			result.WriteByte(',')
			result.WriteString(member.GetUsername())
			result.WriteByte('@')
			result.WriteString(host)
			result.WriteByte('\n')
		}
		membersRead += int64(len(members))
		if len(members) < int(relationshipExportPageSize) || membersRead >= response.GetTotal() {
			return nil
		}
	}
}
