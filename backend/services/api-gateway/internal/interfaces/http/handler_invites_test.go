package http

import (
	"context"
	"encoding/json"
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

func TestCreateAdminInviteCodesForwardsActorAndExpiry(t *testing.T) {
	gin.SetMode(gin.TestMode)
	const largeInviteID int64 = 9007199254740993
	const largeAdminID int64 = 9007199254740995
	client := &inviteHTTPClient{createResponse: &userpb.InviteCodeListResponse{
		Total: 2,
		Items: []*userpb.InviteCodeInfo{{Id: largeInviteID, Code: "ABC", CreatedByAdminId: largeAdminID}, {Id: 12, Code: "DEF"}},
	}}
	h := NewHandler(&clients.Clients{UserInvites: client}, "Authorization", "Bearer", testJWTSecret)
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Set("admin_id", int64(42))
	c.Set("admin_username", "operator")
	c.Request = httptest.NewRequest(stdhttp.MethodPost, "/api/v1/admin/invites", strings.NewReader(`{"count":2,"expires_at":"1800000000"}`))
	c.Request.Header.Set("Content-Type", "application/json")

	h.createAdminInviteCodes(c)

	require.Equal(t, stdhttp.StatusOK, recorder.Code, recorder.Body.String())
	require.NotNil(t, client.createReq)
	require.Equal(t, int64(42), client.createReq.GetActorId())
	require.Equal(t, int32(2), client.createReq.GetCount())
	require.Equal(t, int64(1800000000), client.createReq.GetExpiresAt())
	var envelope struct {
		Data struct {
			Items []struct {
				ID               string `json:"id"`
				CreatedByAdminID string `json:"created_by_admin_id"`
			} `json:"items"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &envelope))
	require.Equal(t, "9007199254740993", envelope.Data.Items[0].ID)
	require.Equal(t, "9007199254740995", envelope.Data.Items[0].CreatedByAdminID)
}

func TestCreateAdminInviteCodesRejectsOversizedBatch(t *testing.T) {
	gin.SetMode(gin.TestMode)
	client := &inviteHTTPClient{}
	h := NewHandler(&clients.Clients{UserInvites: client}, "Authorization", "Bearer", testJWTSecret)
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(stdhttp.MethodPost, "/api/v1/admin/invites", strings.NewReader(`{"count":101}`))
	c.Request.Header.Set("Content-Type", "application/json")

	h.createAdminInviteCodes(c)

	require.Equal(t, stdhttp.StatusBadRequest, recorder.Code)
	require.Nil(t, client.createReq)
}

func TestListAdminInviteCodesUsesSystemPagination(t *testing.T) {
	gin.SetMode(gin.TestMode)
	const largeInviteID int64 = 9007199254740993
	const largeUserID int64 = 9007199254740997
	client := &inviteHTTPClient{listResponse: &userpb.InviteCodeListResponse{
		Total: 7,
		Items: []*userpb.InviteCodeInfo{{Id: largeInviteID, Code: "USED", UsedByUserId: largeUserID, Status: "used"}},
	}}
	h := NewHandler(&clients.Clients{UserInvites: client}, "Authorization", "Bearer", testJWTSecret)
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(stdhttp.MethodGet, "/api/v1/admin/invites?status=USED&page=3&page_size=7", nil)

	h.listAdminInviteCodes(c)

	require.Equal(t, stdhttp.StatusOK, recorder.Code, recorder.Body.String())
	require.Equal(t, "used", client.listReq.GetStatus())
	require.Equal(t, int32(3), client.listReq.GetPage())
	require.Equal(t, int32(7), client.listReq.GetPageSize())
	var envelope struct {
		Data struct {
			Items       []map[string]any `json:"items"`
			Total       int64            `json:"total"`
			CurrentPage int32            `json:"currentPage"`
			PageSize    int32            `json:"pageSize"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &envelope))
	require.Equal(t, int64(7), envelope.Data.Total)
	require.Equal(t, int32(3), envelope.Data.CurrentPage)
	require.Len(t, envelope.Data.Items, 1)
	require.Equal(t, "9007199254740993", envelope.Data.Items[0]["id"])
	require.Equal(t, "9007199254740997", envelope.Data.Items[0]["used_by_user_id"])
}

func TestRevokeAdminInviteCodeForwardsActorAndID(t *testing.T) {
	gin.SetMode(gin.TestMode)
	client := &inviteHTTPClient{}
	h := NewHandler(&clients.Clients{UserInvites: client}, "Authorization", "Bearer", testJWTSecret)
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Params = gin.Params{{Key: "id", Value: "99"}}
	c.Set("admin_id", int64(42))
	c.Request = httptest.NewRequest(stdhttp.MethodDelete, "/api/v1/admin/invites/99", nil)

	h.revokeAdminInviteCode(c)

	require.Equal(t, stdhttp.StatusOK, recorder.Code, recorder.Body.String())
	require.Equal(t, int64(42), client.revokeReq.GetActorId())
	require.Equal(t, int64(99), client.revokeReq.GetId())
}

func TestInviteHandlersFailClosedWhenClientUnavailable(t *testing.T) {
	gin.SetMode(gin.TestMode)
	h := NewHandler(nil, "Authorization", "Bearer", testJWTSecret)
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(stdhttp.MethodGet, "/api/v1/admin/invites", nil)

	h.listAdminInviteCodes(c)

	require.Equal(t, stdhttp.StatusServiceUnavailable, recorder.Code, recorder.Body.String())
}

func TestAdminInviteRoutesUseExactPermissions(t *testing.T) {
	gin.SetMode(gin.TestMode)
	tests := []struct {
		name       string
		permission string
		method     string
		path       string
		body       string
		called     func(*inviteHTTPClient) bool
	}{
		{
			name: "list", permission: "governance:list_invite_codes",
			method: stdhttp.MethodGet, path: "/api/v1/admin/invites",
			called: func(client *inviteHTTPClient) bool { return client.listReq != nil },
		},
		{
			name: "create", permission: "governance:create_invite_codes",
			method: stdhttp.MethodPost, path: "/api/v1/admin/invites", body: `{"count":1}`,
			called: func(client *inviteHTTPClient) bool { return client.createReq != nil },
		},
		{
			name: "revoke", permission: "governance:revoke_invite_code",
			method: stdhttp.MethodDelete, path: "/api/v1/admin/invites/99",
			called: func(client *inviteHTTPClient) bool { return client.revokeReq != nil },
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			inviteClient := &inviteHTTPClient{}
			adminClient := &inviteRouteAdminClient{permissions: []string{tt.permission}}
			h := NewHandler(&clients.Clients{Admin: adminClient, UserInvites: inviteClient}, "Authorization", "Bearer", testJWTSecret)
			router := gin.New()
			NewInitControllers(h)(router)
			recorder := httptest.NewRecorder()
			request := httptest.NewRequest(tt.method, tt.path, strings.NewReader(tt.body))
			request.Header.Set("Authorization", "Bearer admin-token")
			if tt.body != "" {
				request.Header.Set("Content-Type", "application/json")
			}

			router.ServeHTTP(recorder, request)

			require.Equal(t, stdhttp.StatusOK, recorder.Code, recorder.Body.String())
			require.True(t, tt.called(inviteClient))
		})
	}
}

type inviteHTTPClient struct {
	userpb.UserServiceClient
	createReq      *userpb.CreateInviteCodesRequest
	listReq        *userpb.ListInviteCodesRequest
	revokeReq      *userpb.RevokeInviteCodeRequest
	createResponse *userpb.InviteCodeListResponse
	listResponse   *userpb.InviteCodeListResponse
	err            error
}

func (c *inviteHTTPClient) CreateInviteCodes(_ context.Context, req *userpb.CreateInviteCodesRequest, _ ...grpc.CallOption) (*userpb.InviteCodeListResponse, error) {
	c.createReq = req
	if c.err != nil {
		return nil, c.err
	}
	if c.createResponse != nil {
		return c.createResponse, nil
	}
	return &userpb.InviteCodeListResponse{}, nil
}

func (c *inviteHTTPClient) ListInviteCodes(_ context.Context, req *userpb.ListInviteCodesRequest, _ ...grpc.CallOption) (*userpb.InviteCodeListResponse, error) {
	c.listReq = req
	if c.err != nil {
		return nil, c.err
	}
	if c.listResponse != nil {
		return c.listResponse, nil
	}
	return &userpb.InviteCodeListResponse{}, nil
}

func (c *inviteHTTPClient) RevokeInviteCode(_ context.Context, req *userpb.RevokeInviteCodeRequest, _ ...grpc.CallOption) (*userpb.SimpleResponse, error) {
	c.revokeReq = req
	if c.err != nil {
		return nil, c.err
	}
	return &userpb.SimpleResponse{Success: true, Message: "ok"}, nil
}

var _ clients.UserInviteClient = (*inviteHTTPClient)(nil)

type inviteRouteAdminClient struct {
	adminpb.AdminServiceClient
	permissions []string
}

func (c *inviteRouteAdminClient) GetProfile(context.Context, *adminpb.ProfileRequest, ...grpc.CallOption) (*adminpb.ProfileResponse, error) {
	return &adminpb.ProfileResponse{
		User:        &adminpb.AdminUserInfo{Id: 42, Username: "operator"},
		Permissions: c.permissions,
	}, nil
}

func (*inviteRouteAdminClient) RecordOperationLog(context.Context, *adminpb.RecordOperationLogRequest, ...grpc.CallOption) (*adminpb.SimpleResponse, error) {
	return &adminpb.SimpleResponse{Success: true}, nil
}

func TestOAuthLoginErrorMessageForInvitePolicy(t *testing.T) {
	err := status.Error(codes.PermissionDenied, "oauth signup disabled")
	require.Equal(t, "oauth registration requires an invite code", oauthLoginErrorMessage(err, registrationModeInviteOnly))
	require.Equal(t, "registration closed", oauthLoginErrorMessage(err, registrationModeClosed))
	require.Equal(t, "community login failed", oauthLoginErrorMessage(err, registrationModeOpen))
}
