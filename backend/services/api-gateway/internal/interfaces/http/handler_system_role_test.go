package http

import (
	"context"
	"encoding/json"
	stdhttp "net/http"
	"net/http/httptest"
	"testing"

	"api-gateway/api/proto/adminpb"
	"api-gateway/internal/clients"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
)

func TestSystemRoleDetailRoutesUseDirectRoleLookup(t *testing.T) {
	gin.SetMode(gin.TestMode)
	adminClient := &fakeSystemRoleDetailAdminClient{
		role: &adminpb.SystemRoleInfo{
			Id:          731,
			MenuIds:     []int64{11, 12},
			Permissions: []string{"system:list_system_roles"},
		},
	}
	handler := NewHandler(&clients.Clients{Admin: adminClient}, "Authorization", "Bearer", testJWTSecret)

	t.Run("menu ids", func(t *testing.T) {
		recorder := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(recorder)
		c.Request = httptest.NewRequest(stdhttp.MethodGet, "/api/v1/admin/system/roles/731/menu-ids", nil)
		c.Params = gin.Params{{Key: "id", Value: "731"}}

		handler.getSystemRoleMenuIDs(c)

		require.Equal(t, stdhttp.StatusOK, recorder.Code)
		var envelope struct {
			Data []int64 `json:"data"`
		}
		require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &envelope))
		require.Equal(t, []int64{11, 12}, envelope.Data)
	})

	t.Run("permissions", func(t *testing.T) {
		recorder := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(recorder)
		c.Request = httptest.NewRequest(stdhttp.MethodGet, "/api/v1/admin/system/roles/731/permissions", nil)
		c.Params = gin.Params{{Key: "id", Value: "731"}}

		handler.getSystemRolePermissions(c)

		require.Equal(t, stdhttp.StatusOK, recorder.Code)
		var envelope struct {
			Data struct {
				RoleID      int64    `json:"role_id"`
				MenuIDs     []int64  `json:"menu_ids"`
				Permissions []string `json:"permissions"`
			} `json:"data"`
		}
		require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &envelope))
		require.Equal(t, int64(731), envelope.Data.RoleID)
		require.Equal(t, []int64{11, 12}, envelope.Data.MenuIDs)
		require.Equal(t, []string{"system:list_system_roles"}, envelope.Data.Permissions)
	})

	require.Len(t, adminClient.requests, 2)
	for _, request := range adminClient.requests {
		require.Equal(t, int64(731), request.GetId())
	}
	require.Zero(t, adminClient.listCalls)
}

type fakeSystemRoleDetailAdminClient struct {
	adminpb.AdminServiceClient
	role      *adminpb.SystemRoleInfo
	requests  []*adminpb.SystemRoleIDRequest
	listCalls int
}

func (f *fakeSystemRoleDetailAdminClient) GetSystemRole(_ context.Context, request *adminpb.SystemRoleIDRequest, _ ...grpc.CallOption) (*adminpb.SystemRoleResponse, error) {
	f.requests = append(f.requests, request)
	return &adminpb.SystemRoleResponse{Success: true, Role: f.role}, nil
}

func (f *fakeSystemRoleDetailAdminClient) ListSystemRoles(context.Context, *adminpb.ListSystemRolesRequest, ...grpc.CallOption) (*adminpb.SystemRoleListResponse, error) {
	f.listCalls++
	return &adminpb.SystemRoleListResponse{}, nil
}
