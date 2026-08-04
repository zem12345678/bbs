package http

import (
	"net/http"
	"strconv"

	"api-gateway/pkg/ratelimit"

	"github.com/gin-gonic/gin"
)

func (h *Handler) SetFileUploadLimit(limiter ratelimit.Limiter) {
	h.fileUploadLimit = limiter
}

func (h *Handler) allowFileUploadRateLimit(c *gin.Context, userID int64) bool {
	if h.fileUploadLimit == nil {
		return true
	}
	limited, err := h.fileUploadLimit.Limit(c.Request.Context(), fileUploadRateLimitKey(userID))
	if err != nil {
		writeError(c, http.StatusServiceUnavailable, "file upload rate limiter unavailable", "unavailable")
		return false
	}
	if limited {
		writeError(c, http.StatusTooManyRequests, "file upload rate limit exceeded", "rate_limited")
		return false
	}
	return true
}

func fileUploadRateLimitKey(userID int64) string {
	return "rate:files:upload:user:" + strconv.FormatInt(userID, 10)
}
