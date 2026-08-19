package http

import (
	stdhttp "net/http"
	"strconv"

	"api-gateway/pkg/ratelimit"

	"github.com/gin-gonic/gin"
)

const registryRateLimitPrefix = "rate:registry:"

type RegistryRateLimits struct {
	Set              ratelimit.Limiter
	Get              ratelimit.Limiter
	GetAll           ratelimit.Limiter
	GetDetail        ratelimit.Limiter
	GetUnsecure      ratelimit.Limiter
	Keys             ratelimit.Limiter
	KeysWithType     ratelimit.Limiter
	Remove           ratelimit.Limiter
	ScopesWithDomain ratelimit.Limiter
}

func (h *Handler) SetRegistryRateLimits(limits RegistryRateLimits) {
	h.registryRateLimits = limits
}

func (h *Handler) allowRegistryRateLimit(c *gin.Context, limiter ratelimit.Limiter, action string) bool {
	if limiter == nil {
		return true
	}
	limited, err := limiter.Limit(c.Request.Context(), registryRateLimitKey(action, currentUserID(c)))
	if err != nil {
		writeError(c, stdhttp.StatusServiceUnavailable, "registry rate limiter unavailable", "unavailable")
		return false
	}
	if limited {
		writeError(c, stdhttp.StatusTooManyRequests, "Rate limit exceeded. Please try again later.", "RATE_LIMIT_EXCEEDED")
		return false
	}
	return true
}

func registryRateLimitKey(action string, userID int64) string {
	return registryRateLimitPrefix + action + ":user:" + strconv.FormatInt(userID, 10)
}
