package http

import (
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"strconv"
	"strings"

	"api-gateway/pkg/ratelimit"

	"github.com/gin-gonic/gin"
)

const (
	authRateLimitRegister             = "register"
	authRateLimitLogin                = "login"
	authRateLimitPasswordReset        = "password_reset"
	authRateLimitPasswordResetConfirm = "password_reset_confirm"
	authRateLimitEmailVerification    = "email_verification"
	authRateLimitAdminLogin           = "admin_login"
)

type AuthRateLimits struct {
	Register             ratelimit.Limiter
	Login                ratelimit.Limiter
	PasswordReset        ratelimit.Limiter
	PasswordResetConfirm ratelimit.Limiter
	EmailVerification    ratelimit.Limiter
	AdminLogin           ratelimit.Limiter
}

func (h *Handler) SetAuthRateLimits(limits AuthRateLimits) {
	h.authRateLimits = limits
}

func (h *Handler) allowAuthRateLimit(c *gin.Context, limiter ratelimit.Limiter, action string, subject string) bool {
	if limiter == nil {
		return true
	}
	for _, key := range authRateLimitKeys(action, c.ClientIP(), subject) {
		limited, err := limiter.Limit(c.Request.Context(), key)
		if err != nil {
			writeError(c, http.StatusServiceUnavailable, "auth rate limiter unavailable", "unavailable")
			return false
		}
		if limited {
			writeError(c, http.StatusTooManyRequests, "auth rate limit exceeded", "rate_limited")
			return false
		}
	}
	return true
}

func authRateLimitKeys(action string, clientIP string, subject string) []string {
	keys := make([]string, 0, 2)
	if key := authRateLimitKey(action, "ip", clientIP); key != "" {
		keys = append(keys, key)
	}
	if key := authRateLimitKey(action, "subject", subject); key != "" {
		keys = append(keys, key)
	}
	return keys
}

func authRateLimitKey(action string, dimension string, value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	if value == "" {
		return ""
	}
	hash := sha256.Sum256([]byte(value))
	return "rate:auth:" + action + ":" + dimension + ":" + hex.EncodeToString(hash[:])
}

func authUserRateLimitSubject(userID int64) string {
	if userID <= 0 {
		return ""
	}
	return strconv.FormatInt(userID, 10)
}
