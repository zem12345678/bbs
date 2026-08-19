package http

import (
	stdhttp "net/http"
	"strconv"

	"api-gateway/pkg/ratelimit"

	"github.com/gin-gonic/gin"
)

const pinActionRateLimitKeyPrefix = "rate:pins:user:"

func (h *Handler) SetPinActionLimit(limiter ratelimit.Limiter) {
	h.pinActionLimit = limiter
}

func (h *Handler) allowPinActionRateLimit(c *gin.Context, userID int64) bool {
	if h.pinActionLimit == nil {
		return true
	}
	limited, err := h.pinActionLimit.Limit(c.Request.Context(), pinActionRateLimitKey(userID))
	if err != nil {
		writeError(c, stdhttp.StatusServiceUnavailable, "pin rate limiter unavailable", "unavailable")
		return false
	}
	if limited {
		writeError(c, stdhttp.StatusTooManyRequests, "pin rate limit exceeded", "rate_limited")
		return false
	}
	return true
}

func pinActionRateLimitKey(userID int64) string {
	return pinActionRateLimitKeyPrefix + strconv.FormatInt(userID, 10)
}
