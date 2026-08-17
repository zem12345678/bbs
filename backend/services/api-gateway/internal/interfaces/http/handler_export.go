package http

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	stdhttp "net/http"
	"strconv"
	"time"

	"api-gateway/api/proto/filepb"
	"api-gateway/api/proto/notificationpb"

	"github.com/gin-gonic/gin"
)

const userExportTimeout = 2 * time.Minute

type userExportSpec struct {
	label          string
	filenamePrefix string
	exportedEntity string
	extension      string
	contentType    string
	gate           ExportGate
	build          func(context.Context, int64) ([]byte, error)
}

func (h *Handler) deliverUserExport(c *gin.Context, spec userExportSpec) {
	if !h.hasFileClient(c) || !h.hasAttachmentStore(c) {
		return
	}
	userID := currentUserID(c)
	permit, ok := h.beginUserExport(c, userID, spec.label, spec.gate)
	if !ok {
		return
	}
	releasePermit := true
	if permit != nil {
		defer func() {
			if releasePermit {
				releaseCtx, cancel := context.WithTimeout(context.Background(), requestTimeout)
				_ = permit.Release(releaseCtx)
				cancel()
			}
		}()
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), userExportTimeout)
	defer cancel()
	payload, err := spec.build(ctx, userID)
	if err != nil {
		writeRPCError(c, err)
		return
	}
	objectName, err := uploadedAvatarName(spec.extension)
	if err != nil {
		writeError(c, stdhttp.StatusInternalServerError, "create export file name failed", "internal_error")
		return
	}
	filename := spec.filenamePrefix + "-" + time.Now().Format("2006-01-02-15-04-05") + spec.extension
	objectKey := "files/" + strconv.FormatInt(userID, 10) + "/exports/" + objectName
	if err := h.attachments.Upload(ctx, objectKey, bytes.NewReader(payload), int64(len(payload)), spec.contentType); err != nil {
		writeError(c, stdhttp.StatusBadGateway, "store "+spec.label+" export failed", "storage_unavailable")
		return
	}
	created, err := h.clients.File.CreateFile(ctx, &filepb.CreateFileRequest{
		OwnerId: userID, BizType: "exports", ObjectKey: objectKey, OriginalName: filename,
		ContentType: spec.contentType, SizeBytes: int64(len(payload)),
	})
	if err != nil {
		if canDeleteUploadedAttachmentAfterCreateError(err) {
			cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), requestTimeout)
			_ = h.cleanupUploadedObject(cleanupCtx, objectKey)
			cleanupCancel()
		}
		writeRPCError(c, err)
		return
	}
	if created.GetFile() == nil {
		cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), requestTimeout)
		_ = h.cleanupUploadedObject(cleanupCtx, objectKey)
		cleanupCancel()
		writeError(c, stdhttp.StatusBadGateway, "file metadata unavailable", "service_unavailable")
		return
	}

	releasePermit = false
	if permit != nil {
		commitCtx, commitCancel := context.WithTimeout(context.Background(), requestTimeout)
		if err := permit.Commit(commitCtx); err != nil {
			_ = c.Error(fmt.Errorf("commit %s export rate limit: %w", spec.label, err))
		}
		commitCancel()
	}
	notifyCtx, notifyCancel := context.WithTimeout(context.Background(), requestTimeout)
	if err := h.notifyExportCompleted(notifyCtx, userID, created.GetFile().GetId(), spec.exportedEntity); err != nil {
		_ = c.Error(fmt.Errorf("notify %s export completion: %w", spec.label, err))
	}
	notifyCancel()
	c.Status(stdhttp.StatusNoContent)
}

func (h *Handler) beginUserExport(c *gin.Context, userID int64, label string, gate ExportGate) (ExportPermit, bool) {
	if gate == nil {
		return nil, true
	}
	permit, err := gate.Begin(c.Request.Context(), userID)
	switch {
	case err == nil:
		return permit, true
	case errors.Is(err, errExportRateLimited):
		writeError(c, stdhttp.StatusTooManyRequests, label+" export rate limit exceeded", "rate_limited")
	case errors.Is(err, errExportInProgress):
		writeError(c, stdhttp.StatusTooManyRequests, label+" export already in progress", "rate_limited")
	default:
		writeError(c, stdhttp.StatusServiceUnavailable, label+" export rate limiter unavailable", "service_unavailable")
	}
	return nil, false
}

func (h *Handler) notifyExportCompleted(ctx context.Context, userID, fileID int64, exportedEntity string) error {
	if h == nil || h.clients == nil || h.clients.NotificationInternal == nil {
		return nil
	}
	_, err := h.clients.NotificationInternal.CreateExportCompletedNotification(ctx, &notificationpb.CreateExportCompletedNotificationRequest{
		RecipientId: userID, FileId: fileID, ExportedEntity: exportedEntity,
		IdempotencyKey: exportedEntity + "-" + strconv.FormatInt(userID, 10) + "-" + strconv.FormatInt(fileID, 10),
	})
	return err
}
