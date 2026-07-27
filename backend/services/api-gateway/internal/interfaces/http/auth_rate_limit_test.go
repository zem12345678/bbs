package http

import (
	"context"
	"errors"
	stdhttp "net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"api-gateway/pkg/ratelimit"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestAuthRateLimitsBlockEveryPublicIdentityActionBeforeRPC(t *testing.T) {
	gin.SetMode(gin.TestMode)
	tests := []struct {
		name      string
		action    string
		subject   string
		body      string
		configure func(*AuthRateLimits, ratelimit.Limiter)
		handle    func(*Handler, *gin.Context)
		setUserID bool
	}{
		{
			name: "register", action: authRateLimitRegister, subject: "member@example.com", body: `{"email":"member@example.com"}`,
			configure: func(limits *AuthRateLimits, limiter ratelimit.Limiter) { limits.Register = limiter },
			handle:    func(h *Handler, c *gin.Context) { h.register(c) },
		},
		{
			name: "login", action: authRateLimitLogin, subject: "member@example.com", body: `{"account":"member@example.com"}`,
			configure: func(limits *AuthRateLimits, limiter ratelimit.Limiter) { limits.Login = limiter },
			handle:    func(h *Handler, c *gin.Context) { h.login(c) },
		},
		{
			name: "password reset", action: authRateLimitPasswordReset, subject: "member@example.com", body: `{"email":"member@example.com"}`,
			configure: func(limits *AuthRateLimits, limiter ratelimit.Limiter) { limits.PasswordReset = limiter },
			handle:    func(h *Handler, c *gin.Context) { h.requestPasswordReset(c) },
		},
		{
			name: "password reset confirm", action: authRateLimitPasswordResetConfirm, subject: "reset-token", body: `{"token":"reset-token","new_password":"Password123!"}`,
			configure: func(limits *AuthRateLimits, limiter ratelimit.Limiter) { limits.PasswordResetConfirm = limiter },
			handle:    func(h *Handler, c *gin.Context) { h.resetPassword(c) },
		},
		{
			name: "email verification", action: authRateLimitEmailVerification, subject: "42",
			configure: func(limits *AuthRateLimits, limiter ratelimit.Limiter) { limits.EmailVerification = limiter },
			handle:    func(h *Handler, c *gin.Context) { h.requestEmailVerification(c) }, setUserID: true,
		},
		{
			name: "admin login", action: authRateLimitAdminLogin, subject: "admin", body: `{"account":"admin"}`,
			configure: func(limits *AuthRateLimits, limiter ratelimit.Limiter) { limits.AdminLogin = limiter },
			handle:    func(h *Handler, c *gin.Context) { h.adminLogin(c) },
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			limiter := &authRateLimitStub{limitedOnCall: 2}
			limits := AuthRateLimits{}
			tt.configure(&limits, limiter)
			h := NewHandler(nil, "Authorization", "Bearer", testJWTSecret)
			h.SetAuthRateLimits(limits)
			recorder := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(recorder)
			c.Request = httptest.NewRequest(stdhttp.MethodPost, "/api/v1/auth/test", strings.NewReader(tt.body))
			c.Request.Header.Set("Content-Type", "application/json")
			c.Request.RemoteAddr = "203.0.113.20:12345"
			if tt.setUserID {
				c.Set("user_id", int64(42))
			}

			tt.handle(h, c)

			require.Equal(t, stdhttp.StatusTooManyRequests, recorder.Code, recorder.Body.String())
			require.Equal(t, []string{
				authRateLimitKey(tt.action, "ip", "203.0.113.20"),
				authRateLimitKey(tt.action, "subject", tt.subject),
			}, limiter.keys)
		})
	}
}

func TestAuthRateLimitReturnsServiceUnavailableWhenRedisFails(t *testing.T) {
	gin.SetMode(gin.TestMode)
	h := NewHandler(nil, "Authorization", "Bearer", testJWTSecret)
	h.SetAuthRateLimits(AuthRateLimits{Login: &authRateLimitStub{err: errors.New("redis unavailable")}})
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(stdhttp.MethodPost, "/api/v1/auth/login", strings.NewReader(`{"account":"member"}`))
	c.Request.Header.Set("Content-Type", "application/json")
	c.Request.RemoteAddr = "203.0.113.20:12345"

	h.login(c)

	require.Equal(t, stdhttp.StatusServiceUnavailable, recorder.Code, recorder.Body.String())
	require.Contains(t, recorder.Body.String(), "auth rate limiter unavailable")
}

func TestAuthRateLimitKeysAreNormalizedAndDoNotExposeSubjects(t *testing.T) {
	keys := authRateLimitKeys(authRateLimitLogin, "203.0.113.20", " Member@Example.com ")
	require.Equal(t, []string{
		authRateLimitKey(authRateLimitLogin, "ip", "203.0.113.20"),
		authRateLimitKey(authRateLimitLogin, "subject", "member@example.com"),
	}, keys)
	for _, key := range keys {
		require.NotContains(t, key, "member@example.com")
		require.True(t, strings.HasPrefix(key, "rate:auth:login:"))
	}
}

type authRateLimitStub struct {
	err           error
	keys          []string
	limitedOnCall int
}

func (l *authRateLimitStub) Limit(_ context.Context, key string) (bool, error) {
	l.keys = append(l.keys, key)
	if l.err != nil {
		return false, l.err
	}
	return l.limitedOnCall > 0 && len(l.keys) >= l.limitedOnCall, nil
}
