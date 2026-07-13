package http

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"api-gateway/api/proto/mallpb"
	"api-gateway/api/proto/userpb"
	"api-gateway/internal/clients"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
)

func TestUpdateMeForwardsBackgroundURL(t *testing.T) {
	gin.SetMode(gin.TestMode)
	userClient := &captureUpdateProfileUserClient{}
	h := NewHandler(&clients.Clients{User: userClient}, "Authorization", "Bearer", testJWTSecret)

	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Set("user_id", int64(42))
	c.Request = httptest.NewRequest(
		http.MethodPut,
		"/api/v1/users/me",
		strings.NewReader(`{"nickname":"alice","avatar_url":"http://example.test/avatar.png","background_url":"http://example.test/bg.webp","profile_theme":"default","bio":"hello"}`),
	)
	c.Request.Header.Set("Content-Type", "application/json")

	h.updateMe(c)

	require.Equal(t, http.StatusOK, recorder.Code, recorder.Body.String())
	require.NotNil(t, userClient.req)
	require.Equal(t, int64(42), userClient.req.GetId())
	require.Equal(t, "alice", userClient.req.GetNickname())
	require.Equal(t, "http://example.test/avatar.png", userClient.req.GetAvatarUrl())
	require.Equal(t, "http://example.test/bg.webp", userClient.req.GetBackgroundUrl())
	require.Equal(t, "default", userClient.req.GetProfileTheme())
	require.Equal(t, "hello", userClient.req.GetBio())
}

func TestUpdateMeAllowsPurchasedProfileTheme(t *testing.T) {
	gin.SetMode(gin.TestMode)
	userClient := &captureUpdateProfileUserClient{}
	mallClient := &captureThemeMallClient{
		entitlements: []*mallpb.DigitalEntitlement{
			{GrantType: "theme", GrantKey: "theme-pro", Status: "ACTIVE"},
		},
	}
	h := NewHandler(&clients.Clients{User: userClient, Mall: mallClient}, "Authorization", "Bearer", testJWTSecret)

	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Set("user_id", int64(42))
	c.Request = httptest.NewRequest(
		http.MethodPut,
		"/api/v1/users/me",
		strings.NewReader(`{"nickname":"alice","profile_theme":"theme-pro"}`),
	)
	c.Request.Header.Set("Content-Type", "application/json")

	h.updateMe(c)

	require.Equal(t, http.StatusOK, recorder.Code, recorder.Body.String())
	require.NotNil(t, mallClient.req)
	require.Equal(t, int64(42), mallClient.req.GetUserId())
	require.Equal(t, digitalEntitlementStatusActive, mallClient.req.GetStatus())
	require.Equal(t, "theme", mallClient.req.GetGrantType())
	require.Equal(t, "theme-pro", mallClient.req.GetGrantKey())
	require.Equal(t, int32(1), mallClient.req.GetLimit())
	require.NotNil(t, userClient.req)
	require.Equal(t, "theme-pro", userClient.req.GetProfileTheme())
}

func TestUpdateMeRejectsUnownedProfileTheme(t *testing.T) {
	gin.SetMode(gin.TestMode)
	userClient := &captureUpdateProfileUserClient{}
	mallClient := &captureThemeMallClient{}
	h := NewHandler(&clients.Clients{User: userClient, Mall: mallClient}, "Authorization", "Bearer", testJWTSecret)

	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Set("user_id", int64(42))
	c.Request = httptest.NewRequest(
		http.MethodPut,
		"/api/v1/users/me",
		strings.NewReader(`{"nickname":"alice","profile_theme":"theme-pro"}`),
	)
	c.Request.Header.Set("Content-Type", "application/json")

	h.updateMe(c)

	require.Equal(t, http.StatusForbidden, recorder.Code, recorder.Body.String())
	require.Nil(t, userClient.req)
}

func TestGetUserFallsBackToDefaultThemeWithoutEntitlement(t *testing.T) {
	gin.SetMode(gin.TestMode)
	userClient := &captureProfileUserClient{
		user: &userpb.UserInfo{Id: 42, Username: "alice", ProfileTheme: "theme-pro"},
	}
	h := NewHandler(&clients.Clients{User: userClient, Mall: &captureThemeMallClient{}}, "Authorization", "Bearer", testJWTSecret)

	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Params = gin.Params{{Key: "id", Value: "42"}}
	c.Request = httptest.NewRequest(http.MethodGet, "/api/v1/users/42", nil)

	h.getUser(c)

	require.Equal(t, http.StatusOK, recorder.Code, recorder.Body.String())
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

type captureUpdateProfileUserClient struct {
	userpb.UserServiceClient
	req *userpb.UpdateProfileRequest
}

func (c *captureUpdateProfileUserClient) UpdateProfile(_ context.Context, req *userpb.UpdateProfileRequest, _ ...grpc.CallOption) (*userpb.UserResponse, error) {
	c.req = req
	return &userpb.UserResponse{
		User: &userpb.UserInfo{
			Id:            req.GetId(),
			Nickname:      req.GetNickname(),
			AvatarUrl:     req.GetAvatarUrl(),
			BackgroundUrl: req.GetBackgroundUrl(),
			ProfileTheme:  req.GetProfileTheme(),
			Bio:           req.GetBio(),
		},
	}, nil
}

type captureProfileUserClient struct {
	userpb.UserServiceClient
	user *userpb.UserInfo
}

func (c *captureProfileUserClient) GetUser(_ context.Context, _ *userpb.UserIDRequest, _ ...grpc.CallOption) (*userpb.UserResponse, error) {
	return &userpb.UserResponse{User: c.user}, nil
}

type captureThemeMallClient struct {
	mallpb.MallServiceClient
	req          *mallpb.ListUserDigitalEntitlementsRequest
	entitlements []*mallpb.DigitalEntitlement
}

func (c *captureThemeMallClient) ListUserDigitalEntitlements(_ context.Context, req *mallpb.ListUserDigitalEntitlementsRequest, _ ...grpc.CallOption) (*mallpb.ListDigitalEntitlementsResponse, error) {
	c.req = req
	return &mallpb.ListDigitalEntitlementsResponse{Items: c.entitlements, Total: int64(len(c.entitlements))}, nil
}
