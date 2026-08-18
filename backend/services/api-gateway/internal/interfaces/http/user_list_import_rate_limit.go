package http

import (
	stdhttp "net/http"
	"strconv"

	"api-gateway/pkg/ratelimit"

	"github.com/gin-gonic/gin"
)

const userListImportRateLimitKeyPrefix = "rate:imports:user-lists:user:"

func (h *Handler) SetUserListImportLimit(limiter ratelimit.Limiter) {
	h.userListImportLimit = limiter
}

func (h *Handler) allowUserListImportRateLimit(c *gin.Context, userID int64) bool {
	if h.userListImportLimit == nil {
		return true
	}
	limited, err := h.userListImportLimit.Limit(c.Request.Context(), userListImportRateLimitKey(userID))
	if err != nil {
		writeError(c, stdhttp.StatusServiceUnavailable, "user list import rate limiter unavailable", "unavailable")
		return false
	}
	if limited {
		writeError(c, stdhttp.StatusTooManyRequests, "user list import rate limit exceeded", "rate_limited")
		return false
	}
	return true
}

func userListImportRateLimitKey(userID int64) string {
	return userListImportRateLimitKeyPrefix + strconv.FormatInt(userID, 10)
}
