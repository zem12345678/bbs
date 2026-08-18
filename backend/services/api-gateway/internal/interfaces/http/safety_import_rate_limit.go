package http

import (
	stdhttp "net/http"
	"strconv"

	"api-gateway/pkg/ratelimit"

	"github.com/gin-gonic/gin"
)

const (
	blockingImportRateLimitKeyPrefix = "rate:imports:blocking:user:"
	mutingImportRateLimitKeyPrefix   = "rate:imports:muting:user:"
)

func (h *Handler) SetBlockingImportLimit(limiter ratelimit.Limiter) {
	h.blockingImportLimit = limiter
}

func (h *Handler) SetMutingImportLimit(limiter ratelimit.Limiter) {
	h.mutingImportLimit = limiter
}

func (h *Handler) allowSafetyImportRateLimit(c *gin.Context, userID int64, blocking bool) bool {
	limiter := h.mutingImportLimit
	label := "muting"
	if blocking {
		limiter = h.blockingImportLimit
		label = "blocking"
	}
	if limiter == nil {
		return true
	}
	limited, err := limiter.Limit(c.Request.Context(), safetyImportRateLimitKey(userID, blocking))
	if err != nil {
		writeError(c, stdhttp.StatusServiceUnavailable, label+" import rate limiter unavailable", "unavailable")
		return false
	}
	if limited {
		writeError(c, stdhttp.StatusTooManyRequests, label+" import rate limit exceeded", "rate_limited")
		return false
	}
	return true
}

func safetyImportRateLimitKey(userID int64, blocking bool) string {
	prefix := mutingImportRateLimitKeyPrefix
	if blocking {
		prefix = blockingImportRateLimitKeyPrefix
	}
	return prefix + strconv.FormatInt(userID, 10)
}
