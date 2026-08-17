package http

import (
	"context"
	"errors"
	stdhttp "net/http"
	"strings"

	"api-gateway/api/proto/userpb"

	"github.com/gin-gonic/gin"
)

const safetyExportPageSize = int32(100)

func (h *Handler) exportBlocking(c *gin.Context) {
	if !h.hasSafetyExportDependencies(c) {
		return
	}
	h.deliverUserExport(c, userExportSpec{
		label: "blocking", filenamePrefix: "blocking", exportedEntity: "blocking",
		extension: ".csv", contentType: "text/csv; charset=utf-8",
		gate: h.blockingExportGate,
		build: func(ctx context.Context, userID int64) ([]byte, error) {
			return h.buildSafetyRelationExport(ctx, userID, true)
		},
	})
}

func (h *Handler) exportMute(c *gin.Context) {
	if !h.hasSafetyExportDependencies(c) {
		return
	}
	h.deliverUserExport(c, userExportSpec{
		label: "mute", filenamePrefix: "mute", exportedEntity: "muting",
		extension: ".csv", contentType: "text/csv; charset=utf-8",
		gate: h.muteExportGate,
		build: func(ctx context.Context, userID int64) ([]byte, error) {
			return h.buildSafetyRelationExport(ctx, userID, false)
		},
	})
}

func (h *Handler) hasSafetyExportDependencies(c *gin.Context) bool {
	if h == nil || h.clients == nil || h.clients.UserSafety == nil {
		writeError(c, stdhttp.StatusServiceUnavailable, "safety export dependencies unavailable", "service_unavailable")
		return false
	}
	return true
}

func (h *Handler) buildSafetyRelationExport(ctx context.Context, userID int64, blocking bool) ([]byte, error) {
	host, err := h.exportAccountHost()
	if err != nil {
		return nil, err
	}
	var result strings.Builder
	var afterID int64
	for {
		request := &userpb.ListUserRelationsRequest{
			ActorId: userID, PageSize: safetyExportPageSize,
			AfterTargetId: afterID, AscendingByTargetId: true,
		}
		var response *userpb.UserListResponse
		if blocking {
			response, err = h.clients.UserSafety.ListBlockedUsers(ctx, request)
		} else {
			response, err = h.clients.UserSafety.ListMutedUsers(ctx, request)
		}
		if err != nil {
			return nil, err
		}
		items := response.GetItems()
		if len(items) == 0 {
			break
		}
		for _, user := range items {
			if user == nil || user.GetId() <= afterID || strings.TrimSpace(user.GetUsername()) == "" {
				return nil, errors.New("invalid safety export user")
			}
			afterID = user.GetId()
			result.WriteString(user.GetUsername())
			result.WriteByte('@')
			result.WriteString(host)
			result.WriteByte('\n')
		}
		if len(items) < int(safetyExportPageSize) {
			break
		}
	}
	return []byte(result.String()), nil
}
