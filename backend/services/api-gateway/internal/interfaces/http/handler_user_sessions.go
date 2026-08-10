package http

import (
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"api-gateway/api/proto/userpb"
	"api-gateway/pkg/http/response"

	"github.com/gin-gonic/gin"
)

// maxSessionListLimit mirrors the user-service domain cap so the gateway
// rejects oversized page sizes without a round trip.
const (
	defaultSessionListLimit = 20
	maxSessionListLimit     = 100
)

func (h *Handler) listCurrentUserSessions(c *gin.Context) {
	if !h.sessionClientAvailable(c) {
		return
	}
	ctx, cancel := rpcContext(c)
	defer cancel()
	result, err := h.clients.UserSessions.ListSessions(ctx, &userpb.ListSessionsRequest{
		UserId: currentUserID(c),
		Limit:  sessionListLimit(c),
	})
	if err != nil {
		writeRPCError(c, err)
		return
	}
	response.Success(c, gin.H{"items": sessionPayloads(result.GetItems(), currentSessionID(c)), "total": result.GetTotal()})
}

func (h *Handler) getCurrentUserSession(c *gin.Context) {
	if !h.sessionClientAvailable(c) {
		return
	}
	sessionID, ok := sessionIDParam(c)
	if !ok {
		return
	}
	ctx, cancel := rpcContext(c)
	defer cancel()
	result, err := h.clients.UserSessions.GetSession(ctx, &userpb.GetSessionRequest{
		UserId:    currentUserID(c),
		SessionId: sessionID,
	})
	if err != nil {
		writeRPCError(c, err)
		return
	}
	response.Success(c, gin.H{"session": sessionPayload(result.GetSession(), currentSessionID(c))})
}

func (h *Handler) revokeCurrentUserSession(c *gin.Context) {
	if !h.sessionClientAvailable(c) {
		return
	}
	sessionID, ok := sessionIDParam(c)
	if !ok {
		return
	}
	ctx, cancel := rpcContext(c)
	defer cancel()
	result, err := h.clients.UserSessions.RevokeSession(ctx, &userpb.RevokeSessionRequest{
		UserId:    currentUserID(c),
		SessionId: sessionID,
	})
	if err != nil {
		writeRPCError(c, err)
		return
	}
	// The database row alone does not stop an issued token, so mirror the
	// revocation into Redis where requireAuth and the chat validator look.
	if err := h.revokeSessionToken(c, result.GetSession()); err != nil {
		writeError(c, http.StatusServiceUnavailable, err.Error(), "service_unavailable")
		return
	}
	response.Success(c, gin.H{"session": sessionPayload(result.GetSession(), currentSessionID(c))})
}

func (h *Handler) listCurrentUserLoginEvents(c *gin.Context) {
	if !h.sessionClientAvailable(c) {
		return
	}
	ctx, cancel := rpcContext(c)
	defer cancel()
	result, err := h.clients.UserSessions.ListLoginEvents(ctx, &userpb.ListLoginEventsRequest{
		UserId: currentUserID(c),
		Limit:  sessionListLimit(c),
	})
	if err != nil {
		writeRPCError(c, err)
		return
	}
	response.Success(c, gin.H{"items": loginEventPayloads(result.GetItems()), "total": result.GetTotal()})
}

func (h *Handler) sessionClientAvailable(c *gin.Context) bool {
	if h != nil && h.clients != nil && h.clients.UserSessions != nil {
		return true
	}
	writeError(c, http.StatusServiceUnavailable, "user session service unavailable", "service_unavailable")
	return false
}

// sessionClientInfo captures the caller network fingerprint recorded against
// sessions and login events. Values come from the request, never the client
// payload, so callers cannot spoof their own audit trail.
func sessionClientInfo(c *gin.Context) *userpb.SessionClientInfo {
	if c == nil {
		return nil
	}
	return &userpb.SessionClientInfo{
		IpAddress: c.ClientIP(),
		UserAgent: c.GetHeader("User-Agent"),
	}
}

func sessionIDParam(c *gin.Context) (string, bool) {
	sessionID := strings.TrimSpace(c.Param("sessionId"))
	if sessionID == "" {
		writeError(c, http.StatusBadRequest, "invalid sessionId", "bad_request")
		return "", false
	}
	return sessionID, true
}

func sessionListLimit(c *gin.Context) int32 {
	limit := queryInt32(c, "limit", defaultSessionListLimit)
	if limit <= 0 {
		return defaultSessionListLimit
	}
	if limit > maxSessionListLimit {
		return maxSessionListLimit
	}
	return limit
}

// revokeSessionToken blocks the revoked session id until the session would
// have expired anyway. An already expired session needs no Redis entry.
func (h *Handler) revokeSessionToken(c *gin.Context, session *userpb.SessionInfo) error {
	if session == nil {
		return nil
	}
	return h.revokeSessionID(c, session.GetSessionId(), session.GetExpiresAt())
}

func (h *Handler) revokeSessionID(c *gin.Context, sessionID string, expiresAt int64) error {
	sessionID = strings.TrimSpace(sessionID)
	if sessionID == "" || h.tokenRevocations == nil {
		return nil
	}
	if expiresAt <= 0 {
		return nil
	}
	expiry := time.Unix(expiresAt, 0)
	if !expiry.After(time.Now()) {
		return nil
	}
	ctx, cancel := rpcContext(c)
	defer cancel()
	if err := h.tokenRevocations.RevokeSession(ctx, sessionID, expiry); err != nil {
		return errors.New("session revocation is unavailable")
	}
	return nil
}

func sessionPayloads(items []*userpb.SessionInfo, currentSession string) []gin.H {
	payloads := make([]gin.H, 0, len(items))
	for _, item := range items {
		if item == nil {
			continue
		}
		payloads = append(payloads, sessionPayload(item, currentSession))
	}
	return payloads
}

// sessionPayload reports current for the session that issued this request and
// active for a session that is neither revoked nor past its expiry, so the
// client does not have to recompute either from raw timestamps.
func sessionPayload(session *userpb.SessionInfo, currentSession string) gin.H {
	if session == nil {
		return gin.H{}
	}
	sessionID := session.GetSessionId()
	expiresAt := session.GetExpiresAt()
	active := session.GetRevokedAt() <= 0 && (expiresAt <= 0 || time.Unix(expiresAt, 0).After(time.Now()))
	return gin.H{
		"session_id":   sessionID,
		"user_id":      strconv.FormatInt(session.GetUserId(), 10),
		"ip_address":   session.GetIpAddress(),
		"user_agent":   session.GetUserAgent(),
		"login_method": session.GetLoginMethod(),
		"created_at":   session.GetCreatedAt(),
		"expires_at":   expiresAt,
		"revoked_at":   session.GetRevokedAt(),
		"current":      sessionID != "" && sessionID == currentSession,
		"active":       active,
	}
}

func loginEventPayloads(items []*userpb.LoginEventInfo) []gin.H {
	payloads := make([]gin.H, 0, len(items))
	for _, item := range items {
		if item == nil {
			continue
		}
		payloads = append(payloads, loginEventPayload(item))
	}
	return payloads
}

func loginEventPayload(event *userpb.LoginEventInfo) gin.H {
	if event == nil {
		return gin.H{}
	}
	return gin.H{
		"id":             event.GetId(),
		"user_id":        strconv.FormatInt(event.GetUserId(), 10),
		"session_id":     event.GetSessionId(),
		"ip_address":     event.GetIpAddress(),
		"user_agent":     event.GetUserAgent(),
		"success":        event.GetSuccess(),
		"failure_reason": event.GetFailureReason(),
		"created_at":     event.GetCreatedAt(),
	}
}
