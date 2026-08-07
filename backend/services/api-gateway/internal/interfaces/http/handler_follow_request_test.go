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

func TestFollowRequestRoutesRequireAuthentication(t *testing.T) {
	gin.SetMode(gin.TestMode)
	h := NewHandler(&clients.Clients{User: &followRequestHTTPClient{}}, "Authorization", "Bearer", testJWTSecret)
	router := gin.New()
	NewInitControllers(h)(router)

	tests := []struct {
		method string
		path   string
		body   string
	}{
		{method: stdhttp.MethodGet, path: "/api/v1/users/me/follow-requests"},
		{method: stdhttp.MethodGet, path: "/api/v1/users/me/follow-requests/sent"},
		{method: stdhttp.MethodPost, path: "/api/v1/users/me/follow-requests/77/accept"},
		{method: stdhttp.MethodPost, path: "/api/v1/users/me/follow-requests/77/reject"},
		{method: stdhttp.MethodPost, path: "/api/v1/users/77/follow/cancel"},
		{method: stdhttp.MethodPut, path: "/api/v1/users/me/settings/follow-approval", body: `{"required":true}`},
	}
	for _, tt := range tests {
		t.Run(tt.method+" "+tt.path, func(t *testing.T) {
			recorder := httptest.NewRecorder()
			req := httptest.NewRequest(tt.method, tt.path, strings.NewReader(tt.body))
			req.Header.Set("Content-Type", "application/json")

			router.ServeHTTP(recorder, req)

			require.Equal(t, stdhttp.StatusUnauthorized, recorder.Code, recorder.Body.String())
		})
	}
}

func TestListFollowRequestsForwardsActorAndPagination(t *testing.T) {
	gin.SetMode(gin.TestMode)
	client := &followRequestHTTPClient{listResponse: &userpb.FollowRequestListResponse{
		Total: 1,
		Items: []*userpb.FollowRequestInfo{{
			Id: 9, RequesterId: 77, TargetId: 42, CreatedAt: 1234,
			Counterpart: &userpb.UserInfo{Id: 77, Username: "alice", Email: "private@example.test", FollowApprovalRequired: true},
		}},
	}}
	h := NewHandler(&clients.Clients{User: client}, "Authorization", "Bearer", testJWTSecret)
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Set("user_id", int64(42))
	c.Request = httptest.NewRequest(stdhttp.MethodGet, "/api/v1/users/me/follow-requests?page=3&page_size=7", nil)

	h.listReceivedFollowRequests(c)

	require.Equal(t, stdhttp.StatusOK, recorder.Code, recorder.Body.String())
	require.Equal(t, "received", client.listKind)
	require.Equal(t, int64(42), client.lastList.GetActorId())
	require.Equal(t, int32(3), client.lastList.GetPage())
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
	counterpart := envelope.Data.Items[0]["counterpart"].(map[string]any)
	require.Equal(t, "alice", counterpart["username"])
	require.Equal(t, true, counterpart["follow_approval_required"])
	require.NotContains(t, counterpart, "email")
}

func TestFollowRequestMutationsForwardActorAndCounterpart(t *testing.T) {
	gin.SetMode(gin.TestMode)
	tests := []struct {
		name     string
		paramKey string
		method   func(*Handler, *gin.Context)
	}{
		{name: "accept", paramKey: "requesterId", method: (*Handler).acceptFollowRequest},
		{name: "reject", paramKey: "requesterId", method: (*Handler).rejectFollowRequest},
		{name: "cancel", paramKey: "id", method: (*Handler).cancelFollowRequest},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := &followRequestHTTPClient{}
			h := NewHandler(&clients.Clients{User: client}, "Authorization", "Bearer", testJWTSecret)
			recorder := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(recorder)
			c.Set("user_id", int64(42))
			c.Params = gin.Params{{Key: tt.paramKey, Value: "77"}}
			c.Request = httptest.NewRequest(stdhttp.MethodPost, "/", nil)

			tt.method(h, c)

			require.Equal(t, stdhttp.StatusOK, recorder.Code, recorder.Body.String())
			require.Equal(t, tt.name, client.lastAction)
			require.Equal(t, int64(42), client.lastActionRequest.GetActorId())
			require.Equal(t, int64(77), client.lastActionRequest.GetCounterpartId())
		})
	}
}

func TestSetFollowApprovalRequiredForwardsCurrentUser(t *testing.T) {
	gin.SetMode(gin.TestMode)
	client := &followRequestHTTPClient{}
	h := NewHandler(&clients.Clients{User: client}, "Authorization", "Bearer", testJWTSecret)
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Set("user_id", int64(42))
	c.Request = httptest.NewRequest(stdhttp.MethodPut, "/api/v1/users/me/settings/follow-approval", strings.NewReader(`{"required":true}`))
	c.Request.Header.Set("Content-Type", "application/json")

	h.setFollowApprovalRequired(c)

	require.Equal(t, stdhttp.StatusOK, recorder.Code, recorder.Body.String())
	require.Equal(t, int64(42), client.lastApproval.GetUserId())
	require.True(t, client.lastApproval.GetRequired())
}

func TestFollowReturnsPendingState(t *testing.T) {
	gin.SetMode(gin.TestMode)
	client := &followRequestHTTPClient{followResponse: &userpb.FollowResponse{Success: true, Message: "follow request sent", Pending: true}}
	h := NewHandler(&clients.Clients{User: client}, "Authorization", "Bearer", testJWTSecret)
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Set("user_id", int64(42))
	c.Params = gin.Params{{Key: "id", Value: "77"}}
	c.Request = httptest.NewRequest(stdhttp.MethodPost, "/api/v1/users/77/follow", nil)

	h.follow(c)

	require.Equal(t, stdhttp.StatusOK, recorder.Code, recorder.Body.String())
	require.Equal(t, int64(42), client.lastFollow.GetFollowerId())
	require.Equal(t, int64(77), client.lastFollow.GetFolloweeId())
	var envelope struct {
		Data struct {
			Pending bool `json:"pending"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &envelope))
	require.True(t, envelope.Data.Pending)
}

type followRequestHTTPClient struct {
	userpb.UserServiceClient
	listResponse      *userpb.FollowRequestListResponse
	followResponse    *userpb.FollowResponse
	listKind          string
	lastList          *userpb.ListFollowRequestsRequest
	lastAction        string
	lastActionRequest *userpb.FollowRequestActionRequest
	lastApproval      *userpb.SetFollowApprovalRequest
	lastFollow        *userpb.FollowRequest
}

func (c *followRequestHTTPClient) Follow(_ context.Context, req *userpb.FollowRequest, _ ...grpc.CallOption) (*userpb.FollowResponse, error) {
	c.lastFollow = req
	if c.followResponse != nil {
		return c.followResponse, nil
	}
	return &userpb.FollowResponse{Success: true}, nil
}

func (c *followRequestHTTPClient) ListReceivedFollowRequests(_ context.Context, req *userpb.ListFollowRequestsRequest, _ ...grpc.CallOption) (*userpb.FollowRequestListResponse, error) {
	c.listKind = "received"
	c.lastList = req
	return c.followRequests(), nil
}

func (c *followRequestHTTPClient) ListSentFollowRequests(_ context.Context, req *userpb.ListFollowRequestsRequest, _ ...grpc.CallOption) (*userpb.FollowRequestListResponse, error) {
	c.listKind = "sent"
	c.lastList = req
	return c.followRequests(), nil
}

func (c *followRequestHTTPClient) followRequests() *userpb.FollowRequestListResponse {
	if c.listResponse != nil {
		return c.listResponse
	}
	return &userpb.FollowRequestListResponse{}
}

func (c *followRequestHTTPClient) AcceptFollowRequest(_ context.Context, req *userpb.FollowRequestActionRequest, _ ...grpc.CallOption) (*userpb.SimpleResponse, error) {
	return c.mutate("accept", req)
}

func (c *followRequestHTTPClient) RejectFollowRequest(_ context.Context, req *userpb.FollowRequestActionRequest, _ ...grpc.CallOption) (*userpb.SimpleResponse, error) {
	return c.mutate("reject", req)
}

func (c *followRequestHTTPClient) CancelFollowRequest(_ context.Context, req *userpb.FollowRequestActionRequest, _ ...grpc.CallOption) (*userpb.SimpleResponse, error) {
	return c.mutate("cancel", req)
}

func (c *followRequestHTTPClient) mutate(action string, req *userpb.FollowRequestActionRequest) (*userpb.SimpleResponse, error) {
	c.lastAction = action
	c.lastActionRequest = req
	return &userpb.SimpleResponse{Success: true}, nil
}

func (c *followRequestHTTPClient) SetFollowApprovalRequired(_ context.Context, req *userpb.SetFollowApprovalRequest, _ ...grpc.CallOption) (*userpb.SimpleResponse, error) {
	c.lastApproval = req
	return &userpb.SimpleResponse{Success: true}, nil
}

var _ clients.UserClient = (*followRequestHTTPClient)(nil)
