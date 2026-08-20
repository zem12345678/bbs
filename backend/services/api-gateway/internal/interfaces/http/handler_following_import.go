package http

import (
	"context"
	stdhttp "net/http"
	"sync"

	"api-gateway/api/proto/userpb"

	"github.com/gin-gonic/gin"
)

type followingImportRequest struct {
	FileID      string `json:"fileId"`
	WithReplies bool   `json:"withReplies"`
}

func (h *Handler) importFollowing(c *gin.Context) {
	if h == nil || h.clients == nil || h.clients.File == nil || h.clients.User == nil || h.attachments == nil {
		writeError(c, stdhttp.StatusServiceUnavailable, "following import dependencies unavailable", "service_unavailable")
		return
	}
	var request followingImportRequest
	if !bindJSON(c, &request) {
		return
	}
	fileID, ok := parseImportFileID(c, request.FileID)
	if !ok {
		return
	}
	ownerID := currentUserID(c)
	if !h.allowFollowingImportRateLimit(c, ownerID) {
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
	// BBS has local accounts only, so remote federation rows are ignored by the shared parser.
	usernames, err := safetyImportUsernames(payload, localHost)
	if err != nil {
		writeError(c, stdhttp.StatusBadRequest, "invalid following import file", "bad_request")
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
	if err := h.applyFollowingImport(ctx, ownerID, targetIDs, request.WithReplies); err != nil {
		writeRPCError(c, err)
		return
	}
	c.Status(stdhttp.StatusNoContent)
}

func (h *Handler) applyFollowingImport(ctx context.Context, ownerID int64, targetIDs []int64, withReplies ...bool) error {
	if len(targetIDs) == 0 {
		return nil
	}
	workerCtx, cancel := context.WithCancel(ctx)
	defer cancel()
	jobs := make(chan int64)
	errorsFound := make(chan error, 1)
	workers := min(safetyImportWorkerCount, len(targetIDs))
	includeReplies := len(withReplies) > 0 && withReplies[0]
	var wait sync.WaitGroup
	wait.Add(workers)
	for range workers {
		go func() {
			defer wait.Done()
			for targetID := range jobs {
				preference := includeReplies
				_, err := h.clients.User.Follow(workerCtx, &userpb.FollowRequest{
					FollowerId: ownerID, FolloweeId: targetID, WithReplies: &preference,
				})
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
