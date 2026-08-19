package http

import (
	stdhttp "net/http"
	"strconv"

	"api-gateway/pkg/ratelimit"

	"github.com/gin-gonic/gin"
)

const (
	notificationRateLimitList    = "list"
	notificationRateLimitGrouped = "grouped"
	notificationRateLimitPrefix  = "rate:notifications:"
)

type NotificationRateLimits struct {
	List    ratelimit.Limiter
	Grouped ratelimit.Limiter
}

func (h *Handler) SetNotificationRateLimits(limits NotificationRateLimits) {
	h.notificationRateLimits = limits
}

func (h *Handler) allowNotificationRateLimit(c *gin.Context, limiter ratelimit.Limiter, action string) bool {
	if limiter == nil {
		return true
	}
	limited, err := limiter.Limit(c.Request.Context(), notificationRateLimitKey(action, currentUserID(c)))
	if err != nil {
		writeError(c, stdhttp.StatusServiceUnavailable, "notification rate limiter unavailable", "unavailable")
		return false
	}
	if limited {
		writeError(c, stdhttp.StatusTooManyRequests, "notification rate limit exceeded", "rate_limited")
		return false
	}
	return true
}

func notificationRateLimitKey(action string, userID int64) string {
	return notificationRateLimitPrefix + action + ":user:" + strconv.FormatInt(userID, 10)
}
