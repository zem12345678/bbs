package http

import (
	"crypto/sha256"
	"encoding/hex"
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
	userID := currentUserID(c)
	if !h.allowChatTicketRateLimit(c, userID) {
		return
	}
	if !h.allowChatWebSocketCapacity(c, userID) {
		return
	}
	accessToken, err := h.authTokenFromRequest(c)
	if err != nil {
		writeAuthenticationError(c, err)
		return
	}
	claims, err := h.parseAuthToken(accessToken)
	if err != nil {
		writeAuthenticationError(c, errors.New("invalid authorization token"))
		return
	}
	sessionTicket := realtimechat.Ticket{
		UserID:           userID,
		TokenFingerprint: tokenRevocationFingerprint(accessToken),
		SessionID:        currentSessionID(c),
	}
	if expiresAt, expiresErr := claims.GetExpirationTime(); expiresErr != nil {
		writeAuthenticationError(c, errors.New("invalid authorization token"))
		return
	} else if expiresAt != nil {
		expires := expiresAt.Time.UTC()
		sessionTicket.TokenExpiresAt = &expires
	}
	sessionTicket.CredentialVersion, sessionTicket.CredentialVersionClaim = credentialVersionClaimValue(claims)
	ctx, cancel := rpcContext(c)
	defer cancel()
	ticket, expiresAt, err := h.chatRealtime.IssueAuthenticatedTicket(ctx, sessionTicket)
	if err != nil {
		writeError(c, http.StatusServiceUnavailable, "chat realtime service unavailable", "unavailable")
		return
	}
	response.Success(c, gin.H{
		"ticket": ticket, "expires_at": strconv.FormatInt(expiresAt.UnixMilli(), 10),
	})
}

func (h *Handler) allowChatWebSocketCapacity(c *gin.Context, userID int64) bool {
	err := h.chatRealtime.CheckConnectionCapacity(c.Request.Context(), userID, c.ClientIP())
	if err == nil {
		return true
	}
	if errors.Is(err, realtimechat.ErrUserConnectionLimit) || errors.Is(err, realtimechat.ErrIPConnectionLimit) {
		c.Header("Retry-After", "30")
		writeError(c, http.StatusTooManyRequests, "chat websocket connection limit reached", "rate_limited")
		return false
	}
	writeError(c, http.StatusServiceUnavailable, "chat realtime service unavailable", "unavailable")
	return false
}

func (h *Handler) allowChatTicketRateLimit(c *gin.Context, userID int64) bool {
	if h.chatTicketLimit == nil {
		return true
	}
	for _, key := range chatTicketRateLimitKeys(c.ClientIP(), userID) {
		limited, err := h.chatTicketLimit.Limit(c.Request.Context(), key)
		if err != nil {
			writeError(c, http.StatusServiceUnavailable, "chat ticket rate limiter unavailable", "unavailable")
			return false
		}
		if limited {
			if h.chatTicketRetryAfterSeconds > 0 {
				c.Header("Retry-After", strconv.Itoa(h.chatTicketRetryAfterSeconds))
			}
			writeError(c, http.StatusTooManyRequests, "chat ticket rate limit exceeded", "rate_limited")
			return false
		}
	}
	return true
}

func chatTicketRateLimitKeys(clientIP string, userID int64) []string {
	keys := make([]string, 0, 2)
	if key := chatTicketRateLimitKey("ip", clientIP); key != "" {
		keys = append(keys, key)
	}
	if userID > 0 {
		keys = append(keys, "rate:chat:ticket:user:"+strconv.FormatInt(userID, 10))
	}
	return keys
}

func chatTicketRateLimitKey(dimension string, value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	hash := sha256.Sum256([]byte(value))
	return "rate:chat:ticket:" + dimension + ":" + hex.EncodeToString(hash[:])
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
	err := h.chatRealtime.ServeWebSocketWithClientIP(c.Writer, c.Request, ticket, c.ClientIP())
	if err == nil || c.Writer.Written() {
		return
	}
	switch {
	case errors.Is(err, realtimechat.ErrInvalidTicket):
		writeError(c, http.StatusUnauthorized, "invalid websocket ticket", "unauthorized")
	case errors.Is(err, realtimechat.ErrUserConnectionLimit), errors.Is(err, realtimechat.ErrIPConnectionLimit):
		writeError(c, http.StatusTooManyRequests, "chat websocket connection limit reached", "rate_limited")
	case errors.Is(err, realtimechat.ErrOriginRejected):
		writeError(c, http.StatusForbidden, "websocket origin is not allowed", "permission_denied")
	default:
		writeError(c, http.StatusServiceUnavailable, "chat realtime service unavailable", "unavailable")
	}
}
