package http

import (
	"bytes"
	"context"
	stdhttp "net/http"
	"net/http/httptest"
	"testing"
	"time"

	"api-gateway/api/proto/userpb"
	"api-gateway/internal/clients"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func TestChangePasswordCompatAliasesForwardMFAAndReturnNoContent(t *testing.T) {
	gin.SetMode(gin.TestMode)
	for _, path := range []string{"/i/change-password", "/api/i/change-password", "/api/v1/i/change-password"} {
		t.Run(path, func(t *testing.T) {
			userClient := &sensitiveAccountUserClient{}
			router := sensitiveAccountCompatRouter(&clients.Clients{User: userClient})

			recorder := performSensitiveAccountCompatRequest(t, router, path, `{"currentPassword":"old-secret","newPassword":"new-secret","token":"  recovery-code  "}`, interactiveAccountToken(t))

			require.Equal(t, stdhttp.StatusNoContent, recorder.Code, recorder.Body.String())
			require.Empty(t, recorder.Body.String())
			require.NotNil(t, userClient.request)
			require.EqualValues(t, 42, userClient.request.GetId())
			require.Equal(t, "old-secret", userClient.request.GetOldPassword())
			require.Equal(t, "new-secret", userClient.request.GetNewPassword())
			require.Equal(t, "recovery-code", userClient.request.GetMfaCode())
		})
	}
}

func TestDeleteAccountCompatAliasesForwardMFAAndReturnNoContent(t *testing.T) {
	gin.SetMode(gin.TestMode)
	for _, path := range []string{"/i/delete-account", "/api/i/delete-account", "/api/v1/i/delete-account"} {
		t.Run(path, func(t *testing.T) {
			lifecycleClient := &sensitiveAccountLifecycleClient{}
			router := sensitiveAccountCompatRouter(&clients.Clients{UserAccountLifecycle: lifecycleClient})

			recorder := performSensitiveAccountCompatRequest(t, router, path, `{"password":"account-secret","token":"123456"}`, interactiveAccountToken(t))

			require.Equal(t, stdhttp.StatusNoContent, recorder.Code, recorder.Body.String())
			require.Empty(t, recorder.Body.String())
			require.NotNil(t, lifecycleClient.request)
			require.EqualValues(t, 42, lifecycleClient.request.GetUserId())
			require.Equal(t, "account-secret", lifecycleClient.request.GetPassword())
			require.Equal(t, "123456", lifecycleClient.request.GetCode())
		})
	}
}

func TestSensitiveAccountCompatRejectsInvalidPayloadAndAPITokens(t *testing.T) {
	gin.SetMode(gin.TestMode)
	userClient := &sensitiveAccountUserClient{}
	lifecycleClient := &sensitiveAccountLifecycleClient{}
	router := sensitiveAccountCompatRouter(&clients.Clients{User: userClient, UserAccountLifecycle: lifecycleClient})

	invalid := performSensitiveAccountCompatRequest(t, router, "/api/v1/i/change-password", `{"currentPassword":"old","newPassword":"new","unexpected":true}`, interactiveAccountToken(t))
	require.Equal(t, stdhttp.StatusBadRequest, invalid.Code, invalid.Body.String())
	require.Contains(t, invalid.Body.String(), "INVALID_PARAM")
	require.Nil(t, userClient.request)

	apiToken := signedAuthToken(t, jwt.MapClaims{
		"sub": "42", "jti": "api-account-compat-token", "exp": time.Now().Add(time.Hour).Unix(),
		credentialVersionClaim: credentialVersionInitial, "token_type": apiTokenType, "scopes": []string{"write"},
	})
	forbidden := performSensitiveAccountCompatRequest(t, router, "/api/v1/i/delete-account", `{"password":"secret"}`, apiToken)
	require.Equal(t, stdhttp.StatusForbidden, forbidden.Code, forbidden.Body.String())
	require.Nil(t, lifecycleClient.request)
}

func TestSensitiveAccountCompatMapsCredentialErrors(t *testing.T) {
	gin.SetMode(gin.TestMode)
	tests := []struct {
		name       string
		path       string
		body       string
		clients    *clients.Clients
		legacyCode string
	}{
		{
			name: "incorrect password", path: "/api/v1/i/change-password", body: `{"currentPassword":"wrong","newPassword":"new-secret"}`,
			clients: &clients.Clients{User: &sensitiveAccountUserClient{err: status.Error(codes.InvalidArgument, "invalid password")}}, legacyCode: "INCORRECT_PASSWORD",
		},
		{
			name: "invalid new password", path: "/api/v1/i/change-password", body: `{"currentPassword":"secret","newPassword":"short"}`,
			clients: &clients.Clients{User: &sensitiveAccountUserClient{err: status.Error(codes.InvalidArgument, "password too short")}}, legacyCode: "INVALID_PARAM",
		},
		{
			name: "incorrect mfa", path: "/api/v1/i/delete-account", body: `{"password":"secret","token":"bad-code"}`,
			clients: &clients.Clients{UserAccountLifecycle: &sensitiveAccountLifecycleClient{err: status.Error(codes.Unauthenticated, "mfa code invalid")}}, legacyCode: "INCORRECT_TOTP",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			recorder := performSensitiveAccountCompatRequest(t, sensitiveAccountCompatRouter(tt.clients), tt.path, tt.body, interactiveAccountToken(t))
			require.Equal(t, stdhttp.StatusBadRequest, recorder.Code, recorder.Body.String())
			require.Contains(t, recorder.Body.String(), tt.legacyCode)
		})
	}
}

func sensitiveAccountCompatRouter(clientSet *clients.Clients) *gin.Engine {
	handler := NewHandler(clientSet, "Authorization", "Bearer", testJWTSecret)
	router := gin.New()
	NewInitControllers(handler)(router)
	return router
}

func performSensitiveAccountCompatRequest(t *testing.T, router stdhttp.Handler, path, body, token string) *httptest.ResponseRecorder {
	t.Helper()
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(stdhttp.MethodPost, path, bytes.NewBufferString(body))
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Authorization", "Bearer "+token)
	router.ServeHTTP(recorder, request)
	return recorder
}

func interactiveAccountToken(t *testing.T) string {
	t.Helper()
	return signedAuthToken(t, jwt.MapClaims{"sub": "42"})
}

type sensitiveAccountUserClient struct {
	userpb.UserServiceClient
	request *userpb.ChangePasswordRequest
	err     error
}

func (c *sensitiveAccountUserClient) ChangePassword(_ context.Context, request *userpb.ChangePasswordRequest, _ ...grpc.CallOption) (*userpb.SimpleResponse, error) {
	c.request = request
	return &userpb.SimpleResponse{Success: true}, c.err
}

type sensitiveAccountLifecycleClient struct {
	request *userpb.RequestAccountDeletionRequest
	err     error
}

func (c *sensitiveAccountLifecycleClient) GetAccountLifecycle(context.Context, *userpb.UserIDRequest, ...grpc.CallOption) (*userpb.AccountLifecycleResponse, error) {
	return &userpb.AccountLifecycleResponse{}, nil
}

func (c *sensitiveAccountLifecycleClient) RequestAccountDeletion(_ context.Context, request *userpb.RequestAccountDeletionRequest, _ ...grpc.CallOption) (*userpb.AccountLifecycleResponse, error) {
	c.request = request
	return &userpb.AccountLifecycleResponse{}, c.err
}
