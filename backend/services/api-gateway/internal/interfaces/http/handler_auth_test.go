package http

import (
	stdhttp "net/http"
	"net/http/httptest"
	"strconv"
	"testing"

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
