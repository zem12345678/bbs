package http

import (
	"context"
	"encoding/json"
	stdhttp "net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"

	"api-gateway/api/proto/adminpb"
	"api-gateway/api/proto/userpb"
	"api-gateway/internal/clients"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
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
	require.Nil(t, (&userpb.PasswordResetResponse{}).ProtoReflect().Descriptor().Fields().ByName("reset_token"))
	h := NewHandler(&clients.Clients{
		Admin: fakeAuthSettingsAdminClient{},
		User: &fakeUserClient{passwordResetResponse: &userpb.PasswordResetResponse{
			Accepted: true, ExpiresAt: 123,
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

func TestRequestEmailVerificationNeverExposesToken(t *testing.T) {
	gin.SetMode(gin.TestMode)
	h := NewHandler(&clients.Clients{
		User: &fakeUserClient{emailVerificationResponse: &userpb.EmailVerificationResponse{
			Accepted: true, VerificationToken: "sensitive-verification-token", ExpiresAt: 123,
		}},
	}, "Authorization", "Bearer", testJWTSecret)
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Set("user_id", int64(42))
	c.Request = httptest.NewRequest(stdhttp.MethodPost, "/api/v1/auth/email/verification", nil)

	h.requestEmailVerification(c)

	require.Equal(t, stdhttp.StatusOK, recorder.Code)
	var envelope struct {
		Data map[string]any `json:"data"`
	}
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &envelope))
	require.Equal(t, true, envelope.Data["accepted"])
	require.NotContains(t, envelope.Data, "verification_token")
	require.NotContains(t, envelope.Data, "verify_url")
}

func TestRequestPasswordResetMapsSecurityEmailDeliveryFailure(t *testing.T) {
	gin.SetMode(gin.TestMode)
	h := NewHandler(&clients.Clients{
		Admin: fakeAuthSettingsAdminClient{},
		User:  &fakeUserClient{passwordResetErr: status.Error(codes.Unavailable, "security email delivery unavailable")},
	}, "Authorization", "Bearer", testJWTSecret)
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(stdhttp.MethodPost, "/api/v1/auth/password/forgot", strings.NewReader(`{"email":"member@example.com"}`))
	c.Request.Header.Set("Content-Type", "application/json")

	h.requestPasswordReset(c)

	require.Equal(t, stdhttp.StatusServiceUnavailable, recorder.Code, recorder.Body.String())
	require.Contains(t, recorder.Body.String(), "security email delivery unavailable")
}

func TestLoginSanitizesProfileThemeWithoutEntitlement(t *testing.T) {
	gin.SetMode(gin.TestMode)
	userClient := &authLoginUserClient{
		resp: &userpb.AuthResponse{
			Success:     true,
			Message:     "ok",
			AccessToken: "token",
			ExpiresAt:   123,
			User:        &userpb.UserInfo{Id: 42, Username: "alice", ProfileTheme: "theme-pro"},
		},
	}
	mallClient := &captureThemeMallClient{}
	h := NewHandler(&clients.Clients{
		Admin: fakeAuthSettingsAdminClient{},
		User:  userClient,
		Mall:  mallClient,
	}, "Authorization", "Bearer", testJWTSecret)
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(stdhttp.MethodPost, "/api/v1/auth/login", strings.NewReader(`{"account":"alice","password":"secret"}`))
	c.Request.Header.Set("Content-Type", "application/json")

	h.login(c)

	require.Equal(t, stdhttp.StatusOK, recorder.Code, recorder.Body.String())
	require.NotNil(t, userClient.req)
	require.Equal(t, "alice", userClient.req.GetAccount())
	require.NotNil(t, mallClient.req)
	require.Equal(t, int64(42), mallClient.req.GetUserId())
	require.Equal(t, "theme", mallClient.req.GetGrantType())
	require.Equal(t, "theme-pro", mallClient.req.GetGrantKey())

	var envelope struct {
		Data struct {
			User struct {
				ProfileTheme string `json:"profile_theme"`
			} `json:"user"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &envelope))
	require.Equal(t, "default", envelope.Data.User.ProfileTheme)
}

func TestRequireAdminPermissionRejectsMissingPermission(t *testing.T) {
	gin.SetMode(gin.TestMode)
	h := NewHandler(nil, "Authorization", "Bearer", testJWTSecret)
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Set("admin_profile", &adminpb.ProfileResponse{User: &adminpb.AdminUserInfo{Id: 1}, Permissions: []string{"governance:list_categories"}})

	h.requireAdminPermission("governance:delete_category")(c)

	require.True(t, c.IsAborted())
	require.Equal(t, stdhttp.StatusForbidden, recorder.Code)
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

type authLoginUserClient struct {
	userpb.UserServiceClient
	req  *userpb.LoginRequest
	resp *userpb.AuthResponse
}

func (c *authLoginUserClient) Login(_ context.Context, req *userpb.LoginRequest, _ ...grpc.CallOption) (*userpb.AuthResponse, error) {
	c.req = req
	return c.resp, nil
}
