package http

import (
	"context"
	"encoding/json"
	stdhttp "net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"api-gateway/api/proto/userpb"
	"api-gateway/internal/clients"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
)

func TestLoginReturnsMFAChallengeWithoutAccessToken(t *testing.T) {
	gin.SetMode(gin.TestMode)
	userClient := &authLoginUserClient{resp: &userpb.AuthResponse{
		Success:      true,
		Message:      "ok",
		User:         &userpb.UserInfo{Id: 42, Username: "alice", ProfileTheme: "default"},
		MfaRequired:  true,
		MfaChallenge: "mfa-challenge",
		MfaExpiresAt: 1_800_000_000_000,
	}}
	h := NewHandler(&clients.Clients{Admin: fakeAuthSettingsAdminClient{}, User: userClient}, "Authorization", "Bearer", testJWTSecret)
	c, recorder := newMFAHandlerContext(stdhttp.MethodPost, "/api/v1/auth/login", `{"account":"alice","password":"secret"}`, 0)

	h.login(c)

	require.Equal(t, stdhttp.StatusOK, recorder.Code, recorder.Body.String())
	var envelope struct {
		Data map[string]any `json:"data"`
	}
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &envelope))
	require.Equal(t, true, envelope.Data["mfa_required"])
	require.Equal(t, "mfa-challenge", envelope.Data["mfa_challenge"])
	require.NotContains(t, envelope.Data, "access_token")
}

func TestCompleteMFALoginMapsChallengeAndReturnsToken(t *testing.T) {
	gin.SetMode(gin.TestMode)
	mfaClient := &captureUserMFAClient{completeResponse: &userpb.AuthResponse{
		Success:     true,
		Message:     "ok",
		AccessToken: "access-token",
		ExpiresAt:   1_800_000_000_000,
		User:        &userpb.UserInfo{Id: 42, Username: "alice", ProfileTheme: "default"},
	}}
	h := NewHandler(&clients.Clients{UserMFA: mfaClient}, "Authorization", "Bearer", testJWTSecret)
	c, recorder := newMFAHandlerContext(stdhttp.MethodPost, "/api/v1/auth/login/mfa", `{"challenge":"challenge-token","code":"123456"}`, 0)

	h.completeMFALogin(c)

	require.Equal(t, stdhttp.StatusOK, recorder.Code, recorder.Body.String())
	require.NotNil(t, mfaClient.completeRequest)
	require.Equal(t, "challenge-token", mfaClient.completeRequest.GetChallenge())
	require.Equal(t, "123456", mfaClient.completeRequest.GetCode())
	var envelope struct {
		Data map[string]any `json:"data"`
	}
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &envelope))
	require.Equal(t, "access-token", envelope.Data["access_token"])
	require.NotContains(t, envelope.Data, "mfa_challenge")
}

func TestMFAStatusRouteUsesAuthenticatedUserID(t *testing.T) {
	gin.SetMode(gin.TestMode)
	mfaClient := &captureUserMFAClient{statusResponse: &userpb.MFAStatusResponse{
		Enabled: true, RecoveryCodesRemaining: 7, EnabledAt: 1_800_000_000_000,
	}}
	h := NewHandler(&clients.Clients{UserMFA: mfaClient}, "Authorization", "Bearer", testJWTSecret)
	router := gin.New()
	NewInitControllers(h)(router)
	token := signedAuthToken(t, mapClaimsForMFAUser("9007199254740993"))
	recorder := httptest.NewRecorder()
	req := httptest.NewRequest(stdhttp.MethodGet, "/api/v1/users/me/mfa", nil)
	req.Header.Set("Authorization", "Bearer "+token)

	router.ServeHTTP(recorder, req)

	require.Equal(t, stdhttp.StatusOK, recorder.Code, recorder.Body.String())
	require.NotNil(t, mfaClient.statusRequest)
	require.Equal(t, int64(9_007_199_254_740_993), mfaClient.statusRequest.GetId())
	require.Contains(t, recorder.Body.String(), `"recovery_codes_remaining":7`)
}

func TestMFAManagementHandlersMapCurrentUserAndCredentials(t *testing.T) {
	gin.SetMode(gin.TestMode)
	mfaClient := &captureUserMFAClient{
		beginResponse:    &userpb.TOTPEnrollmentResponse{Secret: "secret", OtpauthUrl: "otpauth://totp/test", QrDataUrl: "data:image/png;base64,abc"},
		confirmResponse:  &userpb.MFARecoveryCodesResponse{RecoveryCodes: []string{"first-code"}},
		regenerateResult: &userpb.MFARecoveryCodesResponse{RecoveryCodes: []string{"replacement-code"}},
		disableResponse:  &userpb.SimpleResponse{Success: true, Message: "ok"},
	}
	h := NewHandler(&clients.Clients{UserMFA: mfaClient}, "Authorization", "Bearer", testJWTSecret)

	c, recorder := newMFAHandlerContext(stdhttp.MethodPost, "/api/v1/users/me/mfa/totp/enrollment", `{"password":"secret-password","current_code":"old-code"}`, 77)
	h.beginTOTPEnrollment(c)
	require.Equal(t, stdhttp.StatusOK, recorder.Code, recorder.Body.String())
	require.Equal(t, int64(77), mfaClient.beginRequest.GetUserId())
	require.Equal(t, "secret-password", mfaClient.beginRequest.GetPassword())
	require.Equal(t, "old-code", mfaClient.beginRequest.GetCurrentCode())

	c, recorder = newMFAHandlerContext(stdhttp.MethodPost, "/api/v1/users/me/mfa/totp/confirm", `{"code":"123456"}`, 77)
	h.confirmTOTPEnrollment(c)
	require.Equal(t, stdhttp.StatusOK, recorder.Code, recorder.Body.String())
	require.Equal(t, int64(77), mfaClient.confirmRequest.GetUserId())
	require.Equal(t, "123456", mfaClient.confirmRequest.GetCode())
	require.Contains(t, recorder.Body.String(), "first-code")

	c, recorder = newMFAHandlerContext(stdhttp.MethodPost, "/api/v1/users/me/mfa/recovery-codes", `{"password":"secret-password","code":"backup-code"}`, 77)
	h.regenerateMFARecoveryCodes(c)
	require.Equal(t, stdhttp.StatusOK, recorder.Code, recorder.Body.String())
	require.Equal(t, int64(77), mfaClient.regenerateRequest.GetUserId())
	require.Equal(t, "secret-password", mfaClient.regenerateRequest.GetPassword())
	require.Equal(t, "backup-code", mfaClient.regenerateRequest.GetCode())

	c, recorder = newMFAHandlerContext(stdhttp.MethodDelete, "/api/v1/users/me/mfa/totp", `{"password":"secret-password","code":"backup-code"}`, 77)
	h.disableTOTP(c)
	require.Equal(t, stdhttp.StatusOK, recorder.Code, recorder.Body.String())
	require.Equal(t, int64(77), mfaClient.disableRequest.GetUserId())
	require.Equal(t, "secret-password", mfaClient.disableRequest.GetPassword())
	require.Equal(t, "backup-code", mfaClient.disableRequest.GetCode())
}

func newMFAHandlerContext(method string, path string, body string, userID int64) (*gin.Context, *httptest.ResponseRecorder) {
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(method, path, strings.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")
	if userID > 0 {
		c.Set("user_id", userID)
	}
	return c, recorder
}

func mapClaimsForMFAUser(userID string) map[string]any {
	return map[string]any{"sub": userID, "username": "mfa-user"}
}

type captureUserMFAClient struct {
	userpb.UserServiceClient
	statusRequest     *userpb.UserIDRequest
	statusResponse    *userpb.MFAStatusResponse
	beginRequest      *userpb.BeginTOTPEnrollmentRequest
	beginResponse     *userpb.TOTPEnrollmentResponse
	confirmRequest    *userpb.ConfirmTOTPEnrollmentRequest
	confirmResponse   *userpb.MFARecoveryCodesResponse
	regenerateRequest *userpb.MFAReauthenticateRequest
	regenerateResult  *userpb.MFARecoveryCodesResponse
	disableRequest    *userpb.MFAReauthenticateRequest
	disableResponse   *userpb.SimpleResponse
	completeRequest   *userpb.CompleteMFALoginRequest
	completeResponse  *userpb.AuthResponse
}

func (c *captureUserMFAClient) GetMFAStatus(_ context.Context, req *userpb.UserIDRequest, _ ...grpc.CallOption) (*userpb.MFAStatusResponse, error) {
	c.statusRequest = req
	return c.statusResponse, nil
}

func (c *captureUserMFAClient) BeginTOTPEnrollment(_ context.Context, req *userpb.BeginTOTPEnrollmentRequest, _ ...grpc.CallOption) (*userpb.TOTPEnrollmentResponse, error) {
	c.beginRequest = req
	return c.beginResponse, nil
}

func (c *captureUserMFAClient) ConfirmTOTPEnrollment(_ context.Context, req *userpb.ConfirmTOTPEnrollmentRequest, _ ...grpc.CallOption) (*userpb.MFARecoveryCodesResponse, error) {
	c.confirmRequest = req
	return c.confirmResponse, nil
}

func (c *captureUserMFAClient) RegenerateMFARecoveryCodes(_ context.Context, req *userpb.MFAReauthenticateRequest, _ ...grpc.CallOption) (*userpb.MFARecoveryCodesResponse, error) {
	c.regenerateRequest = req
	return c.regenerateResult, nil
}

func (c *captureUserMFAClient) DisableTOTP(_ context.Context, req *userpb.MFAReauthenticateRequest, _ ...grpc.CallOption) (*userpb.SimpleResponse, error) {
	c.disableRequest = req
	return c.disableResponse, nil
}

func (c *captureUserMFAClient) CompleteMFALogin(_ context.Context, req *userpb.CompleteMFALoginRequest, _ ...grpc.CallOption) (*userpb.AuthResponse, error) {
	c.completeRequest = req
	return c.completeResponse, nil
}
