package http

import (
	"bytes"
	"context"
	"encoding/json"
	stdhttp "net/http"

	"api-gateway/api/proto/filepb"
	"api-gateway/api/proto/userpb"
	"api-gateway/pkg/http/response"

	"github.com/gin-gonic/gin"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

const (
	bytesPerMiB                = int64(1 << 20)
	maxFileCapacityOverrideMiB = int64(10_485_760)
)

type adminFileCapacityRequest struct {
	OverrideMB json.RawMessage `json:"override_mb"`
}

func (h *Handler) getAdminUserFileCapacity(c *gin.Context) {
	ownerID, ok := pathInt64(c, "id")
	if !ok || !h.adminFileCapacityClientsAvailable(c) {
		return
	}
	ctx, cancel := rpcContext(c)
	defer cancel()
	if !h.adminFileCapacityUserExists(c, ctx, ownerID) {
		return
	}
	usage, err := h.clients.File.GetFileUsage(ctx, &filepb.GetFileUsageRequest{OwnerId: ownerID})
	if err != nil {
		writeRPCError(c, err)
		return
	}
	if usage == nil {
		writeError(c, stdhttp.StatusBadGateway, "file capacity service returned an empty response", "service_unavailable")
		return
	}
	response.Success(c, adminFileCapacityPayload(usage))
}

func (h *Handler) updateAdminUserFileCapacity(c *gin.Context) {
	ownerID, ok := pathInt64(c, "id")
	if !ok || !h.adminFileCapacityClientsAvailable(c) {
		return
	}
	var request adminFileCapacityRequest
	if !bindJSON(c, &request) {
		return
	}
	overrideBytes, clearOverride, ok := parseAdminFileCapacityOverride(c, request.OverrideMB)
	if !ok {
		return
	}
	ctx, cancel := rpcContext(c)
	defer cancel()
	if !h.adminFileCapacityUserExists(c, ctx, ownerID) {
		return
	}
	usage, err := h.clients.File.SetFileCapacity(ctx, &filepb.SetFileCapacityRequest{
		OwnerId:               ownerID,
		OverrideCapacityBytes: overrideBytes,
		ClearOverride:         clearOverride,
	})
	if err != nil {
		writeRPCError(c, err)
		return
	}
	if usage == nil {
		writeError(c, stdhttp.StatusBadGateway, "file capacity service returned an empty response", "service_unavailable")
		return
	}
	response.Success(c, adminFileCapacityPayload(usage))
}

func (h *Handler) adminFileCapacityClientsAvailable(c *gin.Context) bool {
	if h == nil || h.clients == nil || h.clients.User == nil {
		writeError(c, stdhttp.StatusServiceUnavailable, "user service unavailable", "service_unavailable")
		return false
	}
	return h.hasFileClient(c)
}

func (h *Handler) adminFileCapacityUserExists(c *gin.Context, ctx context.Context, ownerID int64) bool {
	user, err := h.clients.User.GetUser(ctx, &userpb.UserIDRequest{Id: ownerID})
	if err != nil {
		writeRPCError(c, err)
		return false
	}
	if user.GetUser() == nil {
		writeRPCError(c, status.Error(codes.NotFound, "user not found"))
		return false
	}
	return true
}

func parseAdminFileCapacityOverride(c *gin.Context, raw json.RawMessage) (int64, bool, bool) {
	if len(raw) == 0 {
		writeError(c, stdhttp.StatusBadRequest, "override_mb is required", "bad_request")
		return 0, false, false
	}
	if bytes.Equal(bytes.TrimSpace(raw), []byte("null")) {
		return 0, true, true
	}
	var overrideMB int64
	if err := json.Unmarshal(raw, &overrideMB); err != nil || overrideMB < 0 || overrideMB > maxFileCapacityOverrideMiB {
		writeError(c, stdhttp.StatusBadRequest, "override_mb must be null or an integer between 0 and 10485760", "bad_request")
		return 0, false, false
	}
	return overrideMB * bytesPerMiB, false, true
}

func adminFileCapacityPayload(usage *filepb.FileUsageResponse) gin.H {
	var overrideMB any
	if usage.GetHasOverride() {
		overrideMB = usage.GetOverrideCapacityBytes() / bytesPerMiB
	}
	return gin.H{
		"used_bytes":            usage.GetUsedBytes(),
		"file_count":            usage.GetFileCount(),
		"policy_capacity_mb":    usage.GetPolicyCapacityBytes() / bytesPerMiB,
		"max_file_size_mb":      usage.GetMaxFileSizeBytes() / bytesPerMiB,
		"override_mb":           overrideMB,
		"effective_capacity_mb": usage.GetCapacityBytes() / bytesPerMiB,
	}
}
