package http

import (
	"context"
	"encoding/json"
	stdhttp "net/http"
	"net/http/httptest"
	"testing"

	"api-gateway/api/proto/userpb"
	"api-gateway/internal/clients"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func TestUserSafetyMutationHandlersForwardActorTarget(t *testing.T) {
	gin.SetMode(gin.TestMode)
	client := &safetyHTTPClient{}
	h := NewHandler(&clients.Clients{UserSafety: client}, "Authorization", "Bearer", testJWTSecret)

	tests := []struct {
		name   string
		method func(*Handler, *gin.Context)
		want   string
	}{
		{name: "block", method: (*Handler).blockUser, want: "block"},
		{name: "unblock", method: (*Handler).unblockUser, want: "unblock"},
		{name: "mute", method: (*Handler).muteUserRelation, want: "mute"},
		{name: "unmute", method: (*Handler).unmuteUserRelation, want: "unmute"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			recorder := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(recorder)
			c.Params = gin.Params{{Key: "id", Value: "77"}}
			c.Set("user_id", int64(42))
			c.Request = httptest.NewRequest(stdhttp.MethodPost, "/api/v1/users/77/"+tt.name, nil)

			tt.method(h, c)

			require.Equal(t, stdhttp.StatusOK, recorder.Code, recorder.Body.String())
			require.Equal(t, tt.want, client.lastAction)
			require.Equal(t, int64(42), client.lastRelation.GetActorId())
			require.Equal(t, int64(77), client.lastRelation.GetTargetId())
		})
	}
}

func TestUserSafetyRoutesRequireAuthentication(t *testing.T) {
	gin.SetMode(gin.TestMode)
	client := &safetyHTTPClient{}
	h := NewHandler(&clients.Clients{UserSafety: client}, "Authorization", "Bearer", testJWTSecret)
	router := gin.New()
	NewInitControllers(h)(router)
	recorder := httptest.NewRecorder()

	router.ServeHTTP(recorder, httptest.NewRequest(stdhttp.MethodPost, "/api/v1/users/77/block", nil))

	require.Equal(t, stdhttp.StatusUnauthorized, recorder.Code, recorder.Body.String())
	require.Empty(t, client.lastAction)
}

func TestGetUserSafetyStateReturnsDirectionalRelation(t *testing.T) {
	gin.SetMode(gin.TestMode)
	client := &safetyHTTPClient{relationResponse: &userpb.SafetyRelationResponse{
		Blocked: true, BlockedBy: true, Muted: true,
	}}
	h := NewHandler(&clients.Clients{UserSafety: client}, "Authorization", "Bearer", testJWTSecret)
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Params = gin.Params{{Key: "id", Value: "77"}}
	c.Set("user_id", int64(42))
	c.Request = httptest.NewRequest(stdhttp.MethodGet, "/api/v1/users/77/safety-state", nil)

	h.getUserSafetyState(c)

	require.Equal(t, stdhttp.StatusOK, recorder.Code, recorder.Body.String())
	var envelope struct {
		Data userSafetyStateResponse `json:"data"`
	}
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &envelope))
	require.Equal(t, userSafetyStateResponse{Blocked: true, BlockedBy: true, Muted: true}, envelope.Data)
	require.Equal(t, int64(42), client.lastRelation.GetActorId())
	require.Equal(t, int64(77), client.lastRelation.GetTargetId())
}

func TestListSafetyUsersForwardsPaginationAndUsesPublicProjection(t *testing.T) {
	gin.SetMode(gin.TestMode)
	client := &safetyHTTPClient{blockedResponse: &userpb.UserListResponse{
		Total: 1,
		Items: []*userpb.UserInfo{{
			Id: 77, Username: "bob", Email: "private@example.test", Status: userStatusActive,
		}},
	}}
	h := NewHandler(&clients.Clients{UserSafety: client}, "Authorization", "Bearer", testJWTSecret)
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Set("user_id", int64(42))
	c.Request = httptest.NewRequest(stdhttp.MethodGet, "/api/v1/users/me/blocked?page=2&page_size=7", nil)

	h.listBlockedUsers(c)

	require.Equal(t, stdhttp.StatusOK, recorder.Code, recorder.Body.String())
	require.Equal(t, int64(42), client.lastList.GetActorId())
	require.Equal(t, int32(2), client.lastList.GetPage())
	require.Equal(t, int32(7), client.lastList.GetPageSize())
	var envelope struct {
		Data struct {
			Items []map[string]any `json:"items"`
			Total int64            `json:"total"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &envelope))
	require.Equal(t, int64(1), envelope.Data.Total)
	require.Len(t, envelope.Data.Items, 1)
	require.Equal(t, "bob", envelope.Data.Items[0]["username"])
	require.NotContains(t, envelope.Data.Items[0], "email")
	require.NotContains(t, envelope.Data.Items[0], "status")
}

func TestUserSafetyHandlersFailClosedWhenClientUnavailable(t *testing.T) {
	gin.SetMode(gin.TestMode)
	h := NewHandler(nil, "Authorization", "Bearer", testJWTSecret)
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Params = gin.Params{{Key: "id", Value: "77"}}
	c.Set("user_id", int64(42))
	c.Request = httptest.NewRequest(stdhttp.MethodGet, "/api/v1/users/77/safety-state", nil)

	h.getUserSafetyState(c)

	require.Equal(t, stdhttp.StatusServiceUnavailable, recorder.Code, recorder.Body.String())
}

func TestUserSafetyMutationMapsDuplicateRelationToConflict(t *testing.T) {
	gin.SetMode(gin.TestMode)
	client := &safetyHTTPClient{err: status.Error(codes.AlreadyExists, "user already blocked")}
	h := NewHandler(&clients.Clients{UserSafety: client}, "Authorization", "Bearer", testJWTSecret)
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Params = gin.Params{{Key: "id", Value: "77"}}
	c.Set("user_id", int64(42))
	c.Request = httptest.NewRequest(stdhttp.MethodPost, "/api/v1/users/77/block", nil)

	h.blockUser(c)

	require.Equal(t, stdhttp.StatusConflict, recorder.Code, recorder.Body.String())
	require.Contains(t, recorder.Body.String(), "user already blocked")
}

type safetyHTTPClient struct {
	userpb.UserServiceClient
	lastAction       string
	lastRelation     *userpb.UserRelationRequest
	lastList         *userpb.ListUserRelationsRequest
	relationResponse *userpb.SafetyRelationResponse
	blockedResponse  *userpb.UserListResponse
	err              error
}

func (c *safetyHTTPClient) Block(_ context.Context, req *userpb.UserRelationRequest, _ ...grpc.CallOption) (*userpb.SimpleResponse, error) {
	return c.mutate("block", req)
}

func (c *safetyHTTPClient) Unblock(_ context.Context, req *userpb.UserRelationRequest, _ ...grpc.CallOption) (*userpb.SimpleResponse, error) {
	return c.mutate("unblock", req)
}

func (c *safetyHTTPClient) Mute(_ context.Context, req *userpb.UserRelationRequest, _ ...grpc.CallOption) (*userpb.SimpleResponse, error) {
	return c.mutate("mute", req)
}

func (c *safetyHTTPClient) Unmute(_ context.Context, req *userpb.UserRelationRequest, _ ...grpc.CallOption) (*userpb.SimpleResponse, error) {
	return c.mutate("unmute", req)
}

func (c *safetyHTTPClient) mutate(action string, req *userpb.UserRelationRequest) (*userpb.SimpleResponse, error) {
	c.lastAction = action
	c.lastRelation = req
	if c.err != nil {
		return nil, c.err
	}
	return &userpb.SimpleResponse{Success: true, Message: "ok"}, nil
}

func (c *safetyHTTPClient) GetSafetyRelation(_ context.Context, req *userpb.UserRelationRequest, _ ...grpc.CallOption) (*userpb.SafetyRelationResponse, error) {
	c.lastRelation = req
	if c.err != nil {
		return nil, c.err
	}
	if c.relationResponse == nil {
		return &userpb.SafetyRelationResponse{}, nil
	}
	return c.relationResponse, nil
}

func (c *safetyHTTPClient) ListBlockedUsers(_ context.Context, req *userpb.ListUserRelationsRequest, _ ...grpc.CallOption) (*userpb.UserListResponse, error) {
	c.lastList = req
	if c.err != nil {
		return nil, c.err
	}
	if c.blockedResponse == nil {
		return &userpb.UserListResponse{}, nil
	}
	return c.blockedResponse, nil
}

func (c *safetyHTTPClient) ListMutedUsers(_ context.Context, req *userpb.ListUserRelationsRequest, _ ...grpc.CallOption) (*userpb.UserListResponse, error) {
	c.lastList = req
	if c.err != nil {
		return nil, c.err
	}
	return &userpb.UserListResponse{}, nil
}

var _ clients.UserSafetyClient = (*safetyHTTPClient)(nil)
