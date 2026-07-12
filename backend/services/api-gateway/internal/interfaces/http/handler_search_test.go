package http

import (
	"encoding/json"
	stdhttp "net/http"
	"net/http/httptest"
	"testing"

	"api-gateway/api/proto/userpb"
	"api-gateway/internal/clients"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestSearchUsersUsesPublicActiveUserQuery(t *testing.T) {
	gin.SetMode(gin.TestMode)
	userClient := &fakeUserClient{
		users: []*userpb.UserInfo{
			{Id: 101, Username: "alice", Nickname: "Alice"},
		},
	}
	h := NewHandler(&clients.Clients{User: userClient}, "Authorization", "Bearer", testJWTSecret)
	router := gin.New()
	NewInitControllers(h)(router)

	recorder := httptest.NewRecorder()
	req := httptest.NewRequest(stdhttp.MethodGet, "/api/v1/search/users?q=ali&page=2&page_size=7", nil)
	router.ServeHTTP(recorder, req)

	require.Equal(t, stdhttp.StatusOK, recorder.Code, recorder.Body.String())
	require.Equal(t, 1, userClient.listUsersCalls)
	require.Equal(t, "ali", userClient.listUsersReq.GetQuery())
	require.Equal(t, userStatusActive, userClient.listUsersReq.GetStatus())
	require.Equal(t, int32(2), userClient.listUsersReq.GetPage())
	require.Equal(t, int32(7), userClient.listUsersReq.GetPageSize())

	var envelope struct {
		Data userpb.UserListResponse `json:"data"`
	}
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &envelope))
	require.Len(t, envelope.Data.Items, 1)
	require.Equal(t, "alice", envelope.Data.Items[0].GetUsername())
}

func TestSearchUsersRequiresKeyword(t *testing.T) {
	gin.SetMode(gin.TestMode)
	userClient := &fakeUserClient{}
	h := NewHandler(&clients.Clients{User: userClient}, "Authorization", "Bearer", testJWTSecret)
	router := gin.New()
	NewInitControllers(h)(router)

	recorder := httptest.NewRecorder()
	req := httptest.NewRequest(stdhttp.MethodGet, "/api/v1/search/users?q=+%20", nil)
	router.ServeHTTP(recorder, req)

	require.Equal(t, stdhttp.StatusBadRequest, recorder.Code, recorder.Body.String())
	require.Zero(t, userClient.listUsersCalls)
}
