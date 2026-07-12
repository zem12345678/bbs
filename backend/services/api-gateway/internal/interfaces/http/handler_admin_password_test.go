package http

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"api-gateway/api/proto/adminpb"
	"api-gateway/internal/clients"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
)

func TestChangeAdminPasswordForwardsCurrentActor(t *testing.T) {
	gin.SetMode(gin.TestMode)
	adminClient := &captureAdminPasswordClient{}
	h := NewHandler(&clients.Clients{Admin: adminClient}, "Authorization", "Bearer", testJWTSecret)

	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Set("admin_id", int64(7))
	c.Set("admin_username", "admin")
	c.Request = httptest.NewRequest(
		http.MethodPut,
		"/api/v1/admin/auth/password",
		strings.NewReader(`{"old_password":"Old123!","new_password":"New123!"}`),
	)
	c.Request.Header.Set("Content-Type", "application/json")

	h.changeAdminPassword(c)

	require.Equal(t, http.StatusOK, recorder.Code, recorder.Body.String())
	require.NotNil(t, adminClient.req)
	require.Equal(t, int64(7), adminClient.req.GetActor().GetId())
	require.Equal(t, "admin", adminClient.req.GetActor().GetUsername())
	require.Equal(t, "Old123!", adminClient.req.GetOldPassword())
	require.Equal(t, "New123!", adminClient.req.GetNewPassword())
}

type captureAdminPasswordClient struct {
	adminpb.AdminServiceClient
	req *adminpb.ChangePasswordRequest
}

func (c *captureAdminPasswordClient) ChangePassword(_ context.Context, req *adminpb.ChangePasswordRequest, _ ...grpc.CallOption) (*adminpb.ProfileResponse, error) {
	c.req = req
	return &adminpb.ProfileResponse{
		User: &adminpb.AdminUserInfo{Id: req.GetActor().GetId(), Username: req.GetActor().GetUsername()},
	}, nil
}
