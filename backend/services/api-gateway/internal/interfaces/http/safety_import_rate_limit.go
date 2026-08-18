package http

import (
	stdhttp "net/http"
	"strconv"

	"api-gateway/pkg/ratelimit"

	"github.com/gin-gonic/gin"
)

const (
	blockingImportRateLimitKeyPrefix  = "rate:imports:blocking:user:"
	mutingImportRateLimitKeyPrefix    = "rate:imports:muting:user:"
	followingImportRateLimitKeyPrefix = "rate:imports:following:user:"
)

func (h *Handler) SetBlockingImportLimit(limiter ratelimit.Limiter) {
	h.blockingImportLimit = limiter
}

func (h *Handler) SetMutingImportLimit(limiter ratelimit.Limiter) {
	h.mutingImportLimit = limiter
}

func (h *Handler) SetFollowingImportLimit(limiter ratelimit.Limiter) {
	h.followingImportLimit = limiter
}

func (h *Handler) allowFollowingImportRateLimit(c *gin.Context, userID int64) bool {
	if h.followingImportLimit == nil {
		return true
	}
	limited, err := h.followingImportLimit.Limit(c.Request.Context(), followingImportRateLimitKey(userID))
	if err != nil {
		writeError(c, stdhttp.StatusServiceUnavailable, "following import rate limiter unavailable", "unavailable")
		return false
	}
	if limited {
		writeError(c, stdhttp.StatusTooManyRequests, "following import rate limit exceeded", "rate_limited")
		return false
	}
	return true
}

func followingImportRateLimitKey(userID int64) string {
	return followingImportRateLimitKeyPrefix + strconv.FormatInt(userID, 10)
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
