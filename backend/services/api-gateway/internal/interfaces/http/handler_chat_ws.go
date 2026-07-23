package http

import (
	"errors"
	"net/http"
	"strconv"
	"strings"

	realtimechat "api-gateway/internal/realtime/chat"
	"api-gateway/pkg/http/response"

	"github.com/gin-gonic/gin"
)

func (h *Handler) createChatWebSocketTicket(c *gin.Context) {
	if h.chatRealtime == nil {
		writeError(c, http.StatusServiceUnavailable, "chat realtime service unavailable", "unavailable")
		return
	}
	ctx, cancel := rpcContext(c)
	defer cancel()
	ticket, expiresAt, err := h.chatRealtime.IssueTicket(ctx, currentUserID(c))
	if err != nil {
		writeError(c, http.StatusServiceUnavailable, "chat realtime service unavailable", "unavailable")
		return
	}
	response.Success(c, gin.H{
		"ticket": ticket, "expires_at": strconv.FormatInt(expiresAt.UnixMilli(), 10),
	})
}

func (h *Handler) serveChatWebSocket(c *gin.Context) {
	if h.chatRealtime == nil {
		writeError(c, http.StatusServiceUnavailable, "chat realtime service unavailable", "unavailable")
		return
	}
	ticket := strings.TrimSpace(c.Query("ticket"))
	if ticket == "" {
		writeError(c, http.StatusUnauthorized, "websocket ticket is required", "unauthorized")
		return
	}
	err := h.chatRealtime.ServeWebSocket(c.Writer, c.Request, ticket)
	if err == nil || c.Writer.Written() {
		return
	}
	switch {
	case errors.Is(err, realtimechat.ErrInvalidTicket):
		writeError(c, http.StatusUnauthorized, "invalid websocket ticket", "unauthorized")
	case errors.Is(err, realtimechat.ErrOriginRejected):
		writeError(c, http.StatusForbidden, "websocket origin is not allowed", "permission_denied")
	default:
		writeError(c, http.StatusServiceUnavailable, "chat realtime service unavailable", "unavailable")
	}
}
