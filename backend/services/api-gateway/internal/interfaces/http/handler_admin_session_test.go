package http

import (
	"context"
	"encoding/json"
	stdhttp "net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"api-gateway/api/proto/adminpb"
	"api-gateway/internal/clients"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
)

func TestAdminRefreshForwardsRefreshToken(t *testing.T) {
	gin.SetMode(gin.TestMode)
	adminClient := &captureAdminRefreshClient{resp: &adminpb.AuthResponse{
		Success:          true,
		AccessToken:      "new-access-token",
		RefreshToken:     "new-refresh-token",
		ExpiresAt:        1_800_000_000,
		RefreshExpiresAt: 1_900_000_000,
	}}
	h := NewHandler(&clients.Clients{Admin: adminClient}, "Authorization", "Bearer", testJWTSecret)
	router := gin.New()
	NewInitControllers(h)(router)

	recorder := httptest.NewRecorder()
	req := httptest.NewRequest(stdhttp.MethodPost, "/api/v1/admin/auth/refresh", strings.NewReader(`{"refresh_token":"old-refresh-token"}`))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(recorder, req)

	require.Equal(t, stdhttp.StatusOK, recorder.Code, recorder.Body.String())
	require.NotNil(t, adminClient.req)
	require.Equal(t, "old-refresh-token", adminClient.req.GetRefreshToken())
	var envelope struct {
		Code int `json:"code"`
		Data struct {
			AccessToken      string `json:"access_token"`
			RefreshToken     string `json:"refresh_token"`
			RefreshExpiresAt int64  `json:"refresh_expires_at"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &envelope))
	require.Equal(t, 0, envelope.Code)
	require.Equal(t, "new-access-token", envelope.Data.AccessToken)
	require.Equal(t, "new-refresh-token", envelope.Data.RefreshToken)
	require.Equal(t, int64(1_900_000_000), envelope.Data.RefreshExpiresAt)
}

func TestAdminLogoutForwardsAccessTokenAfterAdminAuthentication(t *testing.T) {
	gin.SetMode(gin.TestMode)
	adminClient := &captureAdminRefreshClient{}
	h := NewHandler(&clients.Clients{Admin: adminClient}, "Authorization", "Bearer", testJWTSecret)
	router := gin.New()
	NewInitControllers(h)(router)

	recorder := httptest.NewRecorder()
	req := httptest.NewRequest(stdhttp.MethodPost, "/api/v1/admin/auth/logout", nil)
	req.Header.Set("Authorization", "Bearer current-access-token")
	router.ServeHTTP(recorder, req)

	require.Equal(t, stdhttp.StatusOK, recorder.Code, recorder.Body.String())
	require.NotNil(t, adminClient.logoutReq)
	require.Equal(t, "current-access-token", adminClient.logoutReq.GetAccessToken())
}

type captureAdminRefreshClient struct {
	adminpb.AdminServiceClient
	req       *adminpb.RefreshTokenRequest
	resp      *adminpb.AuthResponse
	logoutReq *adminpb.LogoutRequest
}

func (c *captureAdminRefreshClient) Refresh(_ context.Context, req *adminpb.RefreshTokenRequest, _ ...grpc.CallOption) (*adminpb.AuthResponse, error) {
	c.req = req
	return c.resp, nil
}

func (c *captureAdminRefreshClient) GetProfile(context.Context, *adminpb.ProfileRequest, ...grpc.CallOption) (*adminpb.ProfileResponse, error) {
	return &adminpb.ProfileResponse{User: &adminpb.AdminUserInfo{Id: 1, Username: "admin"}}, nil
}

func (c *captureAdminRefreshClient) Logout(_ context.Context, req *adminpb.LogoutRequest, _ ...grpc.CallOption) (*adminpb.SimpleResponse, error) {
	c.logoutReq = req
	return &adminpb.SimpleResponse{Success: true}, nil
}

func (*captureAdminRefreshClient) RecordOperationLog(context.Context, *adminpb.RecordOperationLogRequest, ...grpc.CallOption) (*adminpb.SimpleResponse, error) {
	return &adminpb.SimpleResponse{Success: true}, nil
}
