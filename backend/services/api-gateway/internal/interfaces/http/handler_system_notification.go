package http

import (
	"api-gateway/api/proto/adminpb"
	"api-gateway/pkg/http/response"

	"github.com/gin-gonic/gin"
)

type sendSystemNotificationRequest struct {
	RecipientIDs   []jsonInt64 `json:"recipient_ids"`
	Title          string      `json:"title"`
	Content        string      `json:"content"`
	IdempotencyKey string      `json:"idempotency_key"`
}

func (h *Handler) sendSystemNotification(c *gin.Context) {
	var req sendSystemNotificationRequest
	if !bindJSON(c, &req) {
		return
	}
	recipientIDs := make([]int64, len(req.RecipientIDs))
	for i, id := range req.RecipientIDs {
		recipientIDs[i] = id.Int64()
	}
	ctx, cancel := rpcContext(c)
	defer cancel()
	resp, err := h.clients.Admin.SendSystemNotification(ctx, &adminpb.SendSystemNotificationRequest{
		Actor:          currentActor(c),
		RecipientIds:   recipientIDs,
		Title:          req.Title,
		Content:        req.Content,
		IdempotencyKey: req.IdempotencyKey,
	})
	if err != nil {
		writeRPCError(c, err)
		return
	}
	response.Success(c, resp)
}
