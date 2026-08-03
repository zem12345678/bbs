package http

import (
	"context"
	"encoding/json"
	stdhttp "net/http"
	"testing"

	"api-gateway/api/proto/userpb"
	"api-gateway/internal/clients"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
)

func TestPasskeyRegistrationMapsCurrentUserAndStructuredCredential(t *testing.T) {
	gin.SetMode(gin.TestMode)
	client := &captureUserPasskeyClient{
		optionsResponse: &userpb.PasskeyOptionsResponse{Challenge: "registration-token", OptionsJson: `{"publicKey":{"challenge":"AQID","rp":{"id":"example.com"}}}`, ExpiresAt: 1_800_000_000_000},
		infoResponse:    &userpb.PasskeyInfoResponse{Success: true, Passkey: &userpb.PasskeyInfo{CredentialId: "credential-id", Name: "Laptop"}},
	}
	h := NewHandler(&clients.Clients{UserPasskeys: client}, "Authorization", "Bearer", testJWTSecret)

	c, recorder := newMFAHandlerContext(stdhttp.MethodPost, "/api/v1/users/me/passkeys/registration/options", `{"name":"Laptop","password":"secret","code":"123456"}`, 77)
	h.beginPasskeyRegistration(c)
	require.Equal(t, stdhttp.StatusOK, recorder.Code, recorder.Body.String())
	require.Equal(t, int64(77), client.beginRegistrationRequest.GetUserId())
	require.Equal(t, "Laptop", client.beginRegistrationRequest.GetName())
	var envelope struct {
		Data map[string]any `json:"data"`
	}
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &envelope))
	require.Equal(t, "registration-token", envelope.Data["challenge"])
	require.NotContains(t, envelope.Data, "options_json")
	require.Equal(t, "AQID", envelope.Data["options"].(map[string]any)["publicKey"].(map[string]any)["challenge"])

	c, recorder = newMFAHandlerContext(stdhttp.MethodPost, "/api/v1/users/me/passkeys/registration/verify", `{"challenge":"registration-token","credential":{"id":"credential-id","response":{"clientDataJSON":"AQID"}}}`, 77)
	h.finishPasskeyRegistration(c)
	require.Equal(t, stdhttp.StatusOK, recorder.Code, recorder.Body.String())
	require.Equal(t, int64(77), client.finishRegistrationRequest.GetUserId())
	var credential map[string]any
	require.NoError(t, json.Unmarshal([]byte(client.finishRegistrationRequest.GetCredentialJson()), &credential))
	require.Equal(t, "credential-id", credential["id"])
}

func TestPasskeyLoginMapsMFAAndPasswordlessCeremonies(t *testing.T) {
	gin.SetMode(gin.TestMode)
	client := &captureUserPasskeyClient{
		optionsResponse: &userpb.PasskeyOptionsResponse{Challenge: "passkey-token", OptionsJson: `{"publicKey":{"challenge":"AQID"}}`, ExpiresAt: 1_800_000_000_000},
		authResponse:    &userpb.AuthResponse{Success: true, AccessToken: "access-token", User: &userpb.UserInfo{Id: 77, Username: "alice", ProfileTheme: "default"}},
	}
	h := NewHandler(&clients.Clients{UserPasskeys: client}, "Authorization", "Bearer", testJWTSecret)

	c, recorder := newMFAHandlerContext(stdhttp.MethodPost, "/api/v1/auth/login/mfa/passkey/options", `{"mfa_challenge":"mfa-token"}`, 0)
	h.beginPasskeyMFALogin(c)
	require.Equal(t, stdhttp.StatusOK, recorder.Code, recorder.Body.String())
	require.Equal(t, "mfa-token", client.beginMFARequest.GetMfaChallenge())

	c, recorder = newMFAHandlerContext(stdhttp.MethodPost, "/api/v1/auth/login/mfa/passkey", `{"challenge":"passkey-token","credential":{"id":"credential-id"}}`, 0)
	h.completePasskeyMFALogin(c)
	require.Equal(t, stdhttp.StatusOK, recorder.Code, recorder.Body.String())
	require.Equal(t, "passkey-token", client.completeMFARequest.GetChallenge())
	require.Contains(t, recorder.Body.String(), "access-token")

	c, recorder = newMFAHandlerContext(stdhttp.MethodPost, "/api/v1/auth/passkeys/options", `{}`, 0)
	h.beginPasswordlessPasskeyLogin(c)
	require.Equal(t, stdhttp.StatusOK, recorder.Code, recorder.Body.String())
	require.NotNil(t, client.beginPasswordlessRequest)

	c, recorder = newMFAHandlerContext(stdhttp.MethodPost, "/api/v1/auth/passkeys/login", `{"challenge":"passkey-token","credential":{"id":"credential-id"}}`, 0)
	h.completePasswordlessPasskeyLogin(c)
	require.Equal(t, stdhttp.StatusOK, recorder.Code, recorder.Body.String())
	require.Equal(t, "passkey-token", client.completePasswordlessRequest.GetChallenge())
}

func TestPasswordlessPasskeyOptionsRateLimitsByClientOnly(t *testing.T) {
	gin.SetMode(gin.TestMode)
	client := &captureUserPasskeyClient{
		optionsResponse: &userpb.PasskeyOptionsResponse{Challenge: "passkey-token", OptionsJson: `{"publicKey":{"challenge":"AQID"}}`},
	}
	limiter := &authRateLimitStub{}
	h := NewHandler(&clients.Clients{UserPasskeys: client}, "Authorization", "Bearer", testJWTSecret)
	h.SetAuthRateLimits(AuthRateLimits{Login: limiter})

	c, recorder := newMFAHandlerContext(stdhttp.MethodPost, "/api/v1/auth/passkeys/options", `{}`, 0)
	c.Request.RemoteAddr = "203.0.113.20:4321"
	h.beginPasswordlessPasskeyLogin(c)

	require.Equal(t, stdhttp.StatusOK, recorder.Code, recorder.Body.String())
	require.Equal(t, []string{authRateLimitKey(authRateLimitLogin, "ip", "203.0.113.20")}, limiter.keys)
}

type captureUserPasskeyClient struct {
	userpb.UserServiceClient
	optionsResponse             *userpb.PasskeyOptionsResponse
	infoResponse                *userpb.PasskeyInfoResponse
	authResponse                *userpb.AuthResponse
	beginRegistrationRequest    *userpb.BeginPasskeyRegistrationRequest
	finishRegistrationRequest   *userpb.FinishPasskeyRegistrationRequest
	beginMFARequest             *userpb.BeginPasskeyMFALoginRequest
	completeMFARequest          *userpb.CompletePasskeyLoginRequest
	beginPasswordlessRequest    *userpb.PasswordlessPasskeyOptionsRequest
	completePasswordlessRequest *userpb.CompletePasskeyLoginRequest
}

func (c *captureUserPasskeyClient) BeginPasskeyRegistration(_ context.Context, req *userpb.BeginPasskeyRegistrationRequest, _ ...grpc.CallOption) (*userpb.PasskeyOptionsResponse, error) {
	c.beginRegistrationRequest = req
	return c.optionsResponse, nil
}

func (c *captureUserPasskeyClient) FinishPasskeyRegistration(_ context.Context, req *userpb.FinishPasskeyRegistrationRequest, _ ...grpc.CallOption) (*userpb.PasskeyInfoResponse, error) {
	c.finishRegistrationRequest = req
	return c.infoResponse, nil
}

func (c *captureUserPasskeyClient) BeginPasskeyMFALogin(_ context.Context, req *userpb.BeginPasskeyMFALoginRequest, _ ...grpc.CallOption) (*userpb.PasskeyOptionsResponse, error) {
	c.beginMFARequest = req
	return c.optionsResponse, nil
}

func (c *captureUserPasskeyClient) CompletePasskeyMFALogin(_ context.Context, req *userpb.CompletePasskeyLoginRequest, _ ...grpc.CallOption) (*userpb.AuthResponse, error) {
	c.completeMFARequest = req
	return c.authResponse, nil
}

func (c *captureUserPasskeyClient) BeginPasswordlessPasskeyLogin(_ context.Context, req *userpb.PasswordlessPasskeyOptionsRequest, _ ...grpc.CallOption) (*userpb.PasskeyOptionsResponse, error) {
	c.beginPasswordlessRequest = req
	return c.optionsResponse, nil
}

func (c *captureUserPasskeyClient) CompletePasswordlessPasskeyLogin(_ context.Context, req *userpb.CompletePasskeyLoginRequest, _ ...grpc.CallOption) (*userpb.AuthResponse, error) {
	c.completePasswordlessRequest = req
	return c.authResponse, nil
}
