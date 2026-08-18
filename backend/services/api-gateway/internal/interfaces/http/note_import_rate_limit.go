package http

import (
	stdhttp "net/http"
	"strconv"

	"api-gateway/pkg/ratelimit"

	"github.com/gin-gonic/gin"
)

const noteImportRateLimitKeyPrefix = "rate:imports:notes:user:"

func (h *Handler) SetNoteImportLimit(limiter ratelimit.Limiter) {
	h.noteImportLimit = limiter
}

func (h *Handler) allowNoteImportRateLimit(c *gin.Context, userID int64) bool {
	if h.noteImportLimit == nil {
		return true
	}
	limited, err := h.noteImportLimit.Limit(c.Request.Context(), noteImportRateLimitKey(userID))
	if err != nil {
		writeError(c, stdhttp.StatusServiceUnavailable, "note import rate limiter unavailable", "unavailable")
		return false
	}
	if limited {
		writeError(c, stdhttp.StatusTooManyRequests, "note import rate limit exceeded", "rate_limited")
		return false
	}
	return true
}

func noteImportRateLimitKey(userID int64) string {
	return noteImportRateLimitKeyPrefix + strconv.FormatInt(userID, 10)
}
