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
)

func TestListUsersByIDsUsesOneUserQuery(t *testing.T) {
	gin.SetMode(gin.TestMode)
	userClient := &batchUserClient{
		response: &userpb.UserListResponse{Items: []*userpb.UserInfo{
			{Id: 42, Username: "alice", ProfileTheme: "default"},
			{Id: 7, Username: "bob", ProfileTheme: "default"},
		}},
	}
	h := NewHandler(&clients.Clients{User: userClient}, "Authorization", "Bearer", testJWTSecret)
	router := gin.New()
	NewInitControllers(h)(router)

	recorder := httptest.NewRecorder()
	req := httptest.NewRequest(stdhttp.MethodGet, "/api/v1/users/batch?ids=42,7,42", nil)
	router.ServeHTTP(recorder, req)

	require.Equal(t, stdhttp.StatusOK, recorder.Code, recorder.Body.String())
	require.Equal(t, 1, userClient.calls)
	require.Equal(t, []int64{42, 7}, userClient.request.GetIds())
	require.Zero(t, userClient.request.GetStatus())
	require.Equal(t, int32(1), userClient.request.GetPage())
	require.Equal(t, int32(2), userClient.request.GetPageSize())

	var envelope struct {
		Data userpb.UserListResponse `json:"data"`
	}
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &envelope))
	require.Len(t, envelope.Data.GetItems(), 2)
}

func TestListUsersByIDsRejectsInvalidInput(t *testing.T) {
	gin.SetMode(gin.TestMode)
	userClient := &batchUserClient{}
	h := NewHandler(&clients.Clients{User: userClient}, "Authorization", "Bearer", testJWTSecret)
	router := gin.New()
	NewInitControllers(h)(router)

	recorder := httptest.NewRecorder()
	req := httptest.NewRequest(stdhttp.MethodGet, "/api/v1/users/batch?ids=1,invalid", nil)
	router.ServeHTTP(recorder, req)

	require.Equal(t, stdhttp.StatusBadRequest, recorder.Code, recorder.Body.String())
	require.Zero(t, userClient.calls)
}

type batchUserClient struct {
	userpb.UserServiceClient
	calls    int
	request  *userpb.ListUsersRequest
	response *userpb.UserListResponse
}

func (c *batchUserClient) ListUsers(_ context.Context, req *userpb.ListUsersRequest, _ ...grpc.CallOption) (*userpb.UserListResponse, error) {
	c.calls++
	c.request = req
	if c.response == nil {
		return &userpb.UserListResponse{}, nil
	}
	return c.response, nil
}
