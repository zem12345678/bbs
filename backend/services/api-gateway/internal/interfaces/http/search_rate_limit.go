package http

import (
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"strings"

	"api-gateway/pkg/ratelimit"

	"github.com/gin-gonic/gin"
)

const (
	searchRateLimitContent = "content"
	searchRateLimitUser    = "user"
)

type SearchRateLimits struct {
	Content ratelimit.Limiter
	User    ratelimit.Limiter
}

func (h *Handler) SetSearchRateLimits(limits SearchRateLimits) {
	h.searchRateLimits = limits
}

func (h *Handler) allowSearchRateLimit(c *gin.Context, limiter ratelimit.Limiter, action string) bool {
	if limiter == nil {
		return true
	}
	key := searchRateLimitKey(action, c.ClientIP())
	if key == "" {
		return true
	}
	limited, err := limiter.Limit(c.Request.Context(), key)
	if err != nil {
		writeError(c, http.StatusServiceUnavailable, "search rate limiter unavailable", "unavailable")
		return false
	}
	if limited {
		writeError(c, http.StatusTooManyRequests, "search rate limit exceeded", "rate_limited")
		return false
	}
	return true
}

func searchRateLimitKey(action string, clientIP string) string {
	clientIP = strings.TrimSpace(clientIP)
	if clientIP == "" {
		return ""
	}
	hash := sha256.Sum256([]byte(clientIP))
	return "rate:search:" + action + ":ip:" + hex.EncodeToString(hash[:])
}
