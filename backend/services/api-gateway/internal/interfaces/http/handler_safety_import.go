package http

import (
	"bufio"
	"bytes"
	"context"
	"encoding/csv"
	stdhttp "net/http"
	"strings"
	"sync"
	"time"

	"api-gateway/api/proto/userpb"

	"github.com/gin-gonic/gin"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

const (
	safetyImportMaxBytes        = int64(64 << 10)
	safetyImportLookupBatchSize = 100
	safetyImportWorkerCount     = 8
	safetyImportTimeout         = 2 * time.Minute
)

func (h *Handler) importBlocking(c *gin.Context) {
	h.importSafetyRelations(c, true)
}

func (h *Handler) importMuting(c *gin.Context) {
	h.importSafetyRelations(c, false)
}

func (h *Handler) importSafetyRelations(c *gin.Context, blocking bool) {
	if h == nil || h.clients == nil || h.clients.File == nil || h.clients.User == nil || h.clients.UserSafety == nil || h.attachments == nil {
		writeError(c, stdhttp.StatusServiceUnavailable, "safety import dependencies unavailable", "service_unavailable")
		return
	}
	fileID, ok := bindImportFileID(c)
	if !ok {
		return
	}
	ownerID := currentUserID(c)
	if !h.allowSafetyImportRateLimit(c, ownerID, blocking) {
		return
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), safetyImportTimeout)
	defer cancel()
	payload, ok := h.readOwnedImportFile(c, ctx, ownerID, fileID, safetyImportMaxBytes)
	if !ok {
		return
	}
	localHost, err := h.exportAccountHost()
	if err != nil {
		writeError(c, stdhttp.StatusServiceUnavailable, "public account host unavailable", "service_unavailable")
		return
	}
	usernames, err := safetyImportUsernames(payload, localHost)
	if err != nil {
		writeError(c, stdhttp.StatusBadRequest, "invalid safety import file", "bad_request")
		return
	}
	if len(usernames) == 0 {
		c.Status(stdhttp.StatusNoContent)
		return
	}
	targetIDs, err := h.resolveSafetyImportTargetIDs(ctx, ownerID, usernames)
	if err != nil {
		writeRPCError(c, err)
		return
	}
	if err := h.applySafetyImport(ctx, ownerID, targetIDs, blocking); err != nil {
		writeRPCError(c, err)
		return
	}
	c.Status(stdhttp.StatusNoContent)
}

func safetyImportUsernames(payload []byte, localHost string) ([]string, error) {
	result := make([]string, 0)
	seen := make(map[string]struct{})
	scanner := bufio.NewScanner(bytes.NewReader(payload))
	scanner.Buffer(make([]byte, 1024), len(payload)+1)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		line = strings.TrimSpace(strings.TrimPrefix(line, "\ufeff"))
		if line == "" {
			continue
		}
		reader := csv.NewReader(strings.NewReader(line))
		reader.FieldsPerRecord = -1
		reader.TrimLeadingSpace = true
		record, err := reader.Read()
		if err != nil || len(record) == 0 {
			continue
		}
		account := strings.TrimSpace(record[0])
		account = strings.TrimPrefix(account, "@")
		separator := strings.LastIndex(account, "@")
		if separator <= 0 || separator == len(account)-1 {
			continue
		}
		username := strings.TrimSpace(account[:separator])
		host := strings.TrimSpace(account[separator+1:])
		if username == "" || strings.Contains(username, "@") || !strings.EqualFold(host, localHost) {
			continue
		}
		key := strings.ToLower(username)
		if _, duplicate := seen[key]; duplicate {
			continue
		}
		seen[key] = struct{}{}
		result = append(result, username)
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	return result, nil
}

func (h *Handler) resolveSafetyImportTargetIDs(ctx context.Context, ownerID int64, usernames []string) ([]int64, error) {
	usersByName := make(map[string]*userpb.UserInfo, len(usernames))
	for start := 0; start < len(usernames); start += safetyImportLookupBatchSize {
		end := min(len(usernames), start+safetyImportLookupBatchSize)
		batch := usernames[start:end]
		response, err := h.clients.User.ListUsers(ctx, &userpb.ListUsersRequest{
			Usernames: batch, Status: userStatusActive, Page: 1, PageSize: int32(len(batch)),
		})
		if err != nil {
			return nil, err
		}
		requested := make(map[string]struct{}, len(batch))
		for _, username := range batch {
			requested[strings.ToLower(username)] = struct{}{}
		}
		for _, user := range response.GetItems() {
			if user == nil || user.GetId() <= 0 || user.GetStatus() != userStatusActive || !publicAccountStateActive(user.GetAccountState()) {
				continue
			}
			key := strings.ToLower(strings.TrimSpace(user.GetUsername()))
			if _, expected := requested[key]; expected {
				usersByName[key] = user
			}
		}
	}

	targetIDs := make([]int64, 0, len(usersByName))
	seenIDs := make(map[int64]struct{}, len(usersByName))
	for _, username := range usernames {
		user := usersByName[strings.ToLower(username)]
		if user == nil || user.GetId() == ownerID {
			continue
		}
		if _, duplicate := seenIDs[user.GetId()]; duplicate {
			continue
		}
		seenIDs[user.GetId()] = struct{}{}
		targetIDs = append(targetIDs, user.GetId())
	}
	return targetIDs, nil
}

func (h *Handler) applySafetyImport(ctx context.Context, ownerID int64, targetIDs []int64, blocking bool) error {
	if len(targetIDs) == 0 {
		return nil
	}
	workerCtx, cancel := context.WithCancel(ctx)
	defer cancel()
	jobs := make(chan int64)
	errorsFound := make(chan error, 1)
	workers := min(safetyImportWorkerCount, len(targetIDs))
	var wait sync.WaitGroup
	wait.Add(workers)
	for range workers {
		go func() {
			defer wait.Done()
			for targetID := range jobs {
				request := &userpb.UserRelationRequest{ActorId: ownerID, TargetId: targetID}
				var err error
				if blocking {
					_, err = h.clients.UserSafety.Block(workerCtx, request)
				} else {
					_, err = h.clients.UserSafety.Mute(workerCtx, request)
				}
				if err == nil || ignorableSafetyImportError(err) {
					continue
				}
				select {
				case errorsFound <- err:
					cancel()
				default:
				}
				return
			}
		}()
	}

sendJobs:
	for _, targetID := range targetIDs {
		select {
		case jobs <- targetID:
		case <-workerCtx.Done():
			break sendJobs
		}
	}
	close(jobs)
	wait.Wait()
	select {
	case err := <-errorsFound:
		return err
	default:
		return workerCtx.Err()
	}
}

func ignorableSafetyImportError(err error) bool {
	switch status.Code(err) {
	case codes.AlreadyExists, codes.NotFound, codes.FailedPrecondition:
		return true
	default:
		return false
	}
}
