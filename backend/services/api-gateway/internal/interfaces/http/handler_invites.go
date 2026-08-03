package http

import (
	"net/http"
	"strconv"
	"strings"

	"api-gateway/api/proto/userpb"
	"api-gateway/pkg/http/response"

	"github.com/gin-gonic/gin"
)

const maxInviteCodeBatchSize int32 = 100

type inviteCodeView struct {
	ID               string `json:"id"`
	Code             string `json:"code"`
	CreatedByAdminID string `json:"created_by_admin_id,omitempty"`
	UsedByUserID     string `json:"used_by_user_id,omitempty"`
	ExpiresAt        int64  `json:"expires_at,omitempty"`
	UsedAt           int64  `json:"used_at,omitempty"`
	RevokedAt        int64  `json:"revoked_at,omitempty"`
	RevokedByAdminID string `json:"revoked_by_admin_id,omitempty"`
	CreatedAt        int64  `json:"created_at,omitempty"`
	Status           string `json:"status"`
}

func (h *Handler) createAdminInviteCodes(c *gin.Context) {
	if !h.inviteClientAvailable(c) {
		return
	}
	var req createInviteCodesRequest
	if !bindJSON(c, &req) {
		return
	}
	if req.Count <= 0 || req.Count > maxInviteCodeBatchSize {
		writeError(c, http.StatusBadRequest, "count must be between 1 and 100", "bad_request")
		return
	}
	ctx, cancel := rpcContext(c)
	defer cancel()
	resp, err := h.clients.UserInvites.CreateInviteCodes(ctx, &userpb.CreateInviteCodesRequest{
		ActorId:   currentActor(c).GetId(),
		Count:     req.Count,
		ExpiresAt: req.ExpiresAt.Int64(),
	})
	if err != nil {
		writeRPCError(c, err)
		return
	}
	response.Success(c, gin.H{"items": toInviteCodeViews(resp.GetItems()), "total": resp.GetTotal()})
}

func (h *Handler) listAdminInviteCodes(c *gin.Context) {
	if !h.inviteClientAvailable(c) {
		return
	}
	page, pageSize := systemPage(c)
	ctx, cancel := rpcContext(c)
	defer cancel()
	resp, err := h.clients.UserInvites.ListInviteCodes(ctx, &userpb.ListInviteCodesRequest{
		Status:   strings.ToLower(strings.TrimSpace(c.Query("status"))),
		Page:     page,
		PageSize: pageSize,
	})
	if err != nil {
		writeRPCError(c, err)
		return
	}
	response.Success(c, systemTablePayload(toInviteCodeViews(resp.GetItems()), resp.GetTotal(), page, pageSize))
}

func (h *Handler) revokeAdminInviteCode(c *gin.Context) {
	if !h.inviteClientAvailable(c) {
		return
	}
	id, ok := pathInt64(c, "id")
	if !ok {
		return
	}
	ctx, cancel := rpcContext(c)
	defer cancel()
	resp, err := h.clients.UserInvites.RevokeInviteCode(ctx, &userpb.RevokeInviteCodeRequest{
		ActorId: currentActor(c).GetId(),
		Id:      id,
	})
	if err != nil {
		writeRPCError(c, err)
		return
	}
	response.Success(c, resp)
}

func (h *Handler) inviteClientAvailable(c *gin.Context) bool {
	if h != nil && h.clients != nil && h.clients.UserInvites != nil {
		return true
	}
	writeError(c, http.StatusServiceUnavailable, "invite service unavailable", "service_unavailable")
	return false
}

func toInviteCodeViews(items []*userpb.InviteCodeInfo) []inviteCodeView {
	out := make([]inviteCodeView, 0, len(items))
	for _, item := range items {
		if item == nil {
			continue
		}
		out = append(out, inviteCodeView{
			ID:               inviteIDString(item.GetId()),
			Code:             item.GetCode(),
			CreatedByAdminID: inviteIDString(item.GetCreatedByAdminId()),
			UsedByUserID:     inviteIDString(item.GetUsedByUserId()),
			ExpiresAt:        item.GetExpiresAt(),
			UsedAt:           item.GetUsedAt(),
			RevokedAt:        item.GetRevokedAt(),
			RevokedByAdminID: inviteIDString(item.GetRevokedByAdminId()),
			CreatedAt:        item.GetCreatedAt(),
			Status:           item.GetStatus(),
		})
	}
	return out
}

func inviteIDString(value int64) string {
	if value <= 0 {
		return ""
	}
	return strconv.FormatInt(value, 10)
}
