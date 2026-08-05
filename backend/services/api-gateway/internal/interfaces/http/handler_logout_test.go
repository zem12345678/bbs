package http

import (
	"context"
	"errors"
	stdhttp "net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"github.com/stretchr/testify/require"
)

func TestLogoutRevokesOnlyTheCurrentAccessToken(t *testing.T) {
	gin.SetMode(gin.TestMode)
	store := &fakeTokenRevocationStore{}
	handler := NewHandlerWithTokenRevocationStore(nil, "Authorization", "Bearer", testJWTSecret, store)
	router := newLogoutTestRouter(handler)
	firstToken := signedAuthToken(t, jwt.MapClaims{
		"sub": "42", "exp": time.Now().Add(time.Hour).Unix(), "jti": "first",
	})
	secondToken := signedAuthToken(t, jwt.MapClaims{
		"sub": "42", "exp": time.Now().Add(time.Hour).Unix(), "jti": "second",
	})

	require.Equal(t, stdhttp.StatusNoContent, performAuthenticatedRequest(router, stdhttp.MethodGet, "/protected", firstToken).Code)
	require.Equal(t, stdhttp.StatusOK, performAuthenticatedRequest(router, stdhttp.MethodPost, "/api/v1/auth/logout", firstToken).Code)
	require.Equal(t, firstToken, store.revokedToken)
	require.False(t, store.revokedExpiry.IsZero())
	require.Equal(t, stdhttp.StatusUnauthorized, performAuthenticatedRequest(router, stdhttp.MethodGet, "/protected", firstToken).Code)
	require.Equal(t, stdhttp.StatusNoContent, performAuthenticatedRequest(router, stdhttp.MethodGet, "/protected", secondToken).Code)
}

func TestRequireAuthFailsClosedWhenTokenRevocationStoreIsUnavailable(t *testing.T) {
	gin.SetMode(gin.TestMode)
	handler := NewHandlerWithTokenRevocationStore(nil, "Authorization", "Bearer", testJWTSecret, &fakeTokenRevocationStore{
		isRevokedErr: errors.New("redis is unavailable"),
	})
	router := newLogoutTestRouter(handler)
	token := signedAuthToken(t, jwt.MapClaims{"sub": "42", "exp": time.Now().Add(time.Hour).Unix()})

	recorder := performAuthenticatedRequest(router, stdhttp.MethodGet, "/protected", token)

	require.Equal(t, stdhttp.StatusServiceUnavailable, recorder.Code)
	require.NotContains(t, recorder.Body.String(), "redis is unavailable")
}

func TestLogoutRequiresAnExpiringAccessToken(t *testing.T) {
	gin.SetMode(gin.TestMode)
	store := &fakeTokenRevocationStore{}
	handler := NewHandlerWithTokenRevocationStore(nil, "Authorization", "Bearer", testJWTSecret, store)
	router := newLogoutTestRouter(handler)
	token := signedAuthToken(t, jwt.MapClaims{"sub": "42"})

	recorder := performAuthenticatedRequest(router, stdhttp.MethodPost, "/api/v1/auth/logout", token)

	require.Equal(t, stdhttp.StatusUnauthorized, recorder.Code)
	require.Empty(t, store.revokedToken)
}

func TestLogoutFailsClosedWhenTokenRevocationCannotBeWritten(t *testing.T) {
	gin.SetMode(gin.TestMode)
	handler := NewHandlerWithTokenRevocationStore(nil, "Authorization", "Bearer", testJWTSecret, &fakeTokenRevocationStore{
		revokeErr: errors.New("redis is unavailable"),
	})
	router := newLogoutTestRouter(handler)
	token := signedAuthToken(t, jwt.MapClaims{"sub": "42", "exp": time.Now().Add(time.Hour).Unix()})

	recorder := performAuthenticatedRequest(router, stdhttp.MethodPost, "/api/v1/auth/logout", token)

	require.Equal(t, stdhttp.StatusServiceUnavailable, recorder.Code)
	require.NotContains(t, recorder.Body.String(), "redis is unavailable")
}

func newLogoutTestRouter(handler *Handler) *gin.Engine {
	router := gin.New()
	NewInitControllers(handler)(router)
	router.GET("/protected", handler.requireAuth(), func(c *gin.Context) {
		c.Status(stdhttp.StatusNoContent)
	})
	return router
}

func performAuthenticatedRequest(router stdhttp.Handler, method string, path string, token string) *httptest.ResponseRecorder {
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(method, path, nil)
	request.Header.Set("Authorization", "Bearer "+token)
	router.ServeHTTP(recorder, request)
	return recorder
}

type fakeTokenRevocationStore struct {
	revokedToken    string
	revokedExpiry   time.Time
	isRevokedErr    error
	revokeErr       error
	revokedSession  string
	sessionExpiry   time.Time
	sessionErr      error
	revokeSessionEr error
}

func (s *fakeTokenRevocationStore) Revoke(_ context.Context, token string, expiresAt time.Time) error {
	if s.revokeErr != nil {
		return s.revokeErr
	}
	s.revokedToken = token
	s.revokedExpiry = expiresAt
	return nil
}

func (s *fakeTokenRevocationStore) IsRevoked(_ context.Context, token string) (bool, error) {
	if s.isRevokedErr != nil {
		return false, s.isRevokedErr
	}
	return token == s.revokedToken && token != "", nil
}

func (s *fakeTokenRevocationStore) IsRevokedFingerprint(_ context.Context, fingerprint string) (bool, error) {
	if s.isRevokedErr != nil {
		return false, s.isRevokedErr
	}
	return s.revokedToken != "" && fingerprint == tokenRevocationFingerprint(s.revokedToken), nil
}

func (s *fakeTokenRevocationStore) RevokeSession(_ context.Context, sessionID string, expiresAt time.Time) error {
	if s.revokeSessionEr != nil {
		return s.revokeSessionEr
	}
	s.revokedSession = sessionID
	s.sessionExpiry = expiresAt
	return nil
}

func (s *fakeTokenRevocationStore) IsSessionRevoked(_ context.Context, sessionID string) (bool, error) {
	if s.sessionErr != nil {
		return false, s.sessionErr
	}
	return sessionID != "" && sessionID == s.revokedSession, nil
}
