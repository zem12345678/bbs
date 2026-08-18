package http

import (
	stdhttp "net/http"
	"strconv"

	"api-gateway/pkg/ratelimit"

	"github.com/gin-gonic/gin"
)

const antennaImportRateLimitKeyPrefix = "rate:imports:antennas:user:"

func (h *Handler) SetAntennaImportLimit(limiter ratelimit.Limiter) {
	h.antennaImportLimit = limiter
}

func (h *Handler) allowAntennaImportRateLimit(c *gin.Context, userID int64) bool {
	if h.antennaImportLimit == nil {
		return true
	}
	limited, err := h.antennaImportLimit.Limit(c.Request.Context(), antennaImportRateLimitKey(userID))
	if err != nil {
		writeError(c, stdhttp.StatusServiceUnavailable, "antenna import rate limiter unavailable", "unavailable")
		return false
	}
	if limited {
		writeError(c, stdhttp.StatusTooManyRequests, "antenna import rate limit exceeded", "rate_limited")
		return false
	}
	return true
}

func antennaImportRateLimitKey(userID int64) string {
	return antennaImportRateLimitKeyPrefix + strconv.FormatInt(userID, 10)
}
