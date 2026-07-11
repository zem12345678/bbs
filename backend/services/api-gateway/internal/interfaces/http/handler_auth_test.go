package http

import (
	"encoding/json"
	stdhttp "net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"

	"api-gateway/api/proto/userpb"
	"api-gateway/internal/clients"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"github.com/stretchr/testify/require"
)

const testJWTSecret = "test-secret"

func TestAuthIdentityFromRequestPrefersSubjectForLargeID(t *testing.T) {
	gin.SetMode(gin.TestMode)
	const userID int64 = 9007199254740993
	token := signedAuthToken(t, jwt.MapClaims{
		"sub":      strconv.FormatInt(userID, 10),
		"user_id":  userID,
		"username": "Alice",
	})
	h := NewHandler(nil, "Authorization", "Bearer", testJWTSecret)

	identity, err := h.authIdentityFromRequest(newAuthContext(token))

	require.NoError(t, err)
	require.Equal(t, userID, identity.userID)
	require.Equal(t, "alice", identity.username)
}

func TestAuthIdentityFromRequestAcceptsStringUserID(t *testing.T) {
	gin.SetMode(gin.TestMode)
	const userID int64 = 332578385639251969
	token := signedAuthToken(t, jwt.MapClaims{
		"user_id":  strconv.FormatInt(userID, 10),
		"username": "bob",
	})
	h := NewHandler(nil, "Authorization", "Bearer", testJWTSecret)

	identity, err := h.authIdentityFromRequest(newAuthContext(token))

	require.NoError(t, err)
	require.Equal(t, userID, identity.userID)
	require.Equal(t, "bob", identity.username)
}

func TestAuthIdentityFromRequestAcceptsSafeNumericUserID(t *testing.T) {
	gin.SetMode(gin.TestMode)
	const userID int64 = 12345
	token := signedAuthToken(t, jwt.MapClaims{"user_id": userID})
	h := NewHandler(nil, "Authorization", "Bearer", testJWTSecret)

	identity, err := h.authIdentityFromRequest(newAuthContext(token))

	require.NoError(t, err)
	require.Equal(t, userID, identity.userID)
}

func TestAuthIdentityFromRequestRejectsUnsafeNumericUserIDWithoutSubject(t *testing.T) {
	gin.SetMode(gin.TestMode)
	const userID int64 = 9007199254740993
	token := signedAuthToken(t, jwt.MapClaims{"user_id": userID})
	h := NewHandler(nil, "Authorization", "Bearer", testJWTSecret)

	_, err := h.authIdentityFromRequest(newAuthContext(token))

	require.ErrorContains(t, err, "missing user id claim")
}

func TestRequestPasswordResetNeverExposesToken(t *testing.T) {
	gin.SetMode(gin.TestMode)
	h := NewHandler(&clients.Clients{
		Admin: fakeAuthSettingsAdminClient{},
		User: &fakeUserClient{passwordResetResponse: &userpb.PasswordResetResponse{
			Accepted: true, ResetToken: "sensitive-reset-token", ExpiresAt: 123,
		}},
	}, "Authorization", "Bearer", testJWTSecret)
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(stdhttp.MethodPost, "/api/v1/auth/password-reset", strings.NewReader(`{"email":"member@example.com"}`))
	c.Request.Header.Set("Content-Type", "application/json")
	c.Request.Host = "localhost:18080"
	c.Request.Header.Set("Origin", "http://localhost:8850")

	h.requestPasswordReset(c)

	require.Equal(t, stdhttp.StatusOK, recorder.Code)
	var envelope struct {
		Data map[string]any `json:"data"`
	}
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &envelope))
	require.Equal(t, true, envelope.Data["accepted"])
	require.NotContains(t, envelope.Data, "reset_token")
	require.NotContains(t, envelope.Data, "reset_url")
}

func signedAuthToken(t *testing.T, claims jwt.MapClaims) string {
	t.Helper()
	token, err := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString([]byte(testJWTSecret))
	require.NoError(t, err)
	return token
}

func newAuthContext(token string) *gin.Context {
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	req := httptest.NewRequest(stdhttp.MethodGet, "/api/v1/users/me", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	c.Request = req
	return c
}
