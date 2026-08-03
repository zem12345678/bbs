package http

import (
	"context"
	stdhttp "net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"api-gateway/api/proto/adminpb"
	"api-gateway/api/proto/userpb"
	"api-gateway/internal/clients"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func TestRegisterForwardsInviteCodeAndRequirement(t *testing.T) {
	gin.SetMode(gin.TestMode)
	userClient := &registerHTTPClient{resp: &userpb.AuthResponse{Success: true}}
	h := NewHandler(&clients.Clients{
		Admin: fakeAuthSettingsAdminClient{items: []*adminpb.SettingInfo{
			authSetting("auth.password.enabled", "true"),
			authSetting("auth.register.enabled", "true"),
			authSetting("auth.register.mode", "invite_only"),
		}},
		User: userClient,
	}, "Authorization", "Bearer", testJWTSecret)
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(stdhttp.MethodPost, "/api/v1/auth/register", strings.NewReader(`{"username":"alice","email":"alice@example.com","password":"secret123","invite_code":"  code-1  "}`))
	c.Request.Header.Set("Content-Type", "application/json")

	h.register(c)

	require.Equal(t, stdhttp.StatusOK, recorder.Code, recorder.Body.String())
	require.NotNil(t, userClient.req)
	require.Equal(t, "code-1", userClient.req.GetInviteCode())
	require.True(t, userClient.req.GetRequireInvite())
}

func TestRegisterOpenModeDoesNotRequireInvite(t *testing.T) {
	gin.SetMode(gin.TestMode)
	userClient := &registerHTTPClient{resp: &userpb.AuthResponse{Success: true}}
	h := NewHandler(&clients.Clients{
		Admin: fakeAuthSettingsAdminClient{items: []*adminpb.SettingInfo{
			authSetting("auth.password.enabled", "true"),
			authSetting("auth.register.enabled", "true"),
		}},
		User: userClient,
	}, "Authorization", "Bearer", testJWTSecret)
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(stdhttp.MethodPost, "/api/v1/auth/register", strings.NewReader(`{"username":"alice","email":"alice@example.com","password":"secret123","invite_code":"optional"}`))
	c.Request.Header.Set("Content-Type", "application/json")

	h.register(c)

	require.Equal(t, stdhttp.StatusOK, recorder.Code, recorder.Body.String())
	require.NotNil(t, userClient.req)
	require.Empty(t, userClient.req.GetInviteCode())
	require.False(t, userClient.req.GetRequireInvite())
}

func TestRegisterClosedModeDoesNotCallUserService(t *testing.T) {
	gin.SetMode(gin.TestMode)
	userClient := &registerHTTPClient{}
	h := NewHandler(&clients.Clients{
		Admin: fakeAuthSettingsAdminClient{items: []*adminpb.SettingInfo{
			authSetting("auth.password.enabled", "true"),
			authSetting("auth.register.enabled", "true"),
			authSetting("auth.register.mode", "closed"),
		}},
		User: userClient,
	}, "Authorization", "Bearer", testJWTSecret)
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(stdhttp.MethodPost, "/api/v1/auth/register", strings.NewReader(`{"username":"alice","email":"alice@example.com","password":"secret123"}`))
	c.Request.Header.Set("Content-Type", "application/json")

	h.register(c)

	require.Equal(t, stdhttp.StatusForbidden, recorder.Code)
	require.Nil(t, userClient.req)
}

func TestRegisterSettingsFailureFailsClosed(t *testing.T) {
	gin.SetMode(gin.TestMode)
	userClient := &registerHTTPClient{}
	h := NewHandler(&clients.Clients{
		Admin: failingAuthSettingsAdminClient{err: status.Error(codes.Unavailable, "settings unavailable")},
		User:  userClient,
	}, "Authorization", "Bearer", testJWTSecret)
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(stdhttp.MethodPost, "/api/v1/auth/register", strings.NewReader(`{"username":"alice","email":"alice@example.com","password":"secret123"}`))
	c.Request.Header.Set("Content-Type", "application/json")

	h.register(c)

	require.Equal(t, stdhttp.StatusServiceUnavailable, recorder.Code, recorder.Body.String())
	require.Nil(t, userClient.req)
}

func TestRegisterMissingSettingsResponseFailsClosed(t *testing.T) {
	gin.SetMode(gin.TestMode)
	userClient := &registerHTTPClient{}
	h := NewHandler(&clients.Clients{
		Admin: failingAuthSettingsAdminClient{},
		User:  userClient,
	}, "Authorization", "Bearer", testJWTSecret)
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(stdhttp.MethodPost, "/api/v1/auth/register", strings.NewReader(`{"username":"alice","email":"alice@example.com","password":"secret123"}`))
	c.Request.Header.Set("Content-Type", "application/json")

	h.register(c)

	require.Equal(t, stdhttp.StatusServiceUnavailable, recorder.Code, recorder.Body.String())
	require.Nil(t, userClient.req)
}

func TestRegistrationModeExplicitInvalidValueClosesRegistration(t *testing.T) {
	require.Equal(t, registrationModeClosed, registrationModeFromSettings(authSettings{
		"auth.register.mode":    "unknown",
		"auth.register.enabled": "true",
	}))
	require.Equal(t, registrationModeClosed, registrationModeFromSettings(authSettings{
		"auth.register.enabled": "not-a-bool",
	}))
}

type registerHTTPClient struct {
	userpb.UserServiceClient
	req  *userpb.RegisterRequest
	resp *userpb.AuthResponse
	err  error
}

func (c *registerHTTPClient) Register(_ context.Context, req *userpb.RegisterRequest, _ ...grpc.CallOption) (*userpb.AuthResponse, error) {
	c.req = req
	if c.err != nil {
		return nil, c.err
	}
	if c.resp != nil {
		return c.resp, nil
	}
	return &userpb.AuthResponse{Success: true}, nil
}

type failingAuthSettingsAdminClient struct {
	adminpb.AdminServiceClient
	err error
}

func (f failingAuthSettingsAdminClient) ListAuthSettings(context.Context, *adminpb.ListAuthSettingsRequest, ...grpc.CallOption) (*adminpb.SettingListResponse, error) {
	return nil, f.err
}

var _ clients.UserClient = (*registerHTTPClient)(nil)
