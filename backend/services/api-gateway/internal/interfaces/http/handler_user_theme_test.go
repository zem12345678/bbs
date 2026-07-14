package http

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"api-gateway/api/proto/mallpb"
	"api-gateway/api/proto/userpb"
	"api-gateway/internal/clients"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
)

func TestSearchUsersSanitizesProfileThemes(t *testing.T) {
	gin.SetMode(gin.TestMode)
	userClient := &themeListUserClient{
		listUsersResponse: &userpb.UserListResponse{
			Items: []*userpb.UserInfo{
				{Id: 11, Username: "alice", ProfileTheme: "theme-pro"},
				{Id: 12, Username: "bob", ProfileTheme: "theme-pro"},
			},
		},
	}
	mallClient := &themeListMallClient{
		entitlementsByUser: map[int64][]*mallpb.DigitalEntitlement{
			11: {{GrantType: "theme", GrantKey: "theme-pro", Status: "ACTIVE"}},
		},
	}
	h := NewHandler(&clients.Clients{User: userClient, Mall: mallClient}, "Authorization", "Bearer", testJWTSecret)

	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodGet, "/api/v1/search/users?q=alice", nil)

	h.searchUsers(c)

	require.Equal(t, http.StatusOK, recorder.Code, recorder.Body.String())
	require.Equal(t, 1, userClient.listUsersCalls)
	require.Len(t, mallClient.requestUserIDs, 2)
	require.Equal(t, int64(11), mallClient.requestUserIDs[0])
	require.Equal(t, int64(12), mallClient.requestUserIDs[1])

	var envelope struct {
		Data struct {
			Items []struct {
				Username     string `json:"username"`
				ProfileTheme string `json:"profile_theme"`
			} `json:"items"`
		} `json:"data"`
	}
	require.NoError(t, unmarshalBody(recorder.Body.Bytes(), &envelope))
	require.Len(t, envelope.Data.Items, 2)
	require.Equal(t, "theme-pro", envelope.Data.Items[0].ProfileTheme)
	require.Equal(t, "default", envelope.Data.Items[1].ProfileTheme)
}

func TestListFollowersSanitizesProfileThemes(t *testing.T) {
	gin.SetMode(gin.TestMode)
	userClient := &themeListUserClient{
		listFollowersResponse: &userpb.UserListResponse{
			Items: []*userpb.UserInfo{
				{Id: 21, Username: "charlie", ProfileTheme: "theme-pro"},
			},
		},
	}
	mallClient := &themeListMallClient{
		entitlementsByUser: map[int64][]*mallpb.DigitalEntitlement{
			21: {{GrantType: "theme", GrantKey: "theme-pro", Status: "ACTIVE"}},
		},
	}
	h := NewHandler(&clients.Clients{User: userClient, Mall: mallClient}, "Authorization", "Bearer", testJWTSecret)

	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Params = gin.Params{{Key: "id", Value: "42"}}
	c.Request = httptest.NewRequest(http.MethodGet, "/api/v1/users/42/followers", nil)

	h.listFollowers(c)

	require.Equal(t, http.StatusOK, recorder.Code, recorder.Body.String())
	require.Len(t, mallClient.requestUserIDs, 1)
	require.Equal(t, int64(21), mallClient.requestUserIDs[0])

	var envelope struct {
		Data struct {
			Items []struct {
				ProfileTheme string `json:"profile_theme"`
			} `json:"items"`
		} `json:"data"`
	}
	require.NoError(t, unmarshalBody(recorder.Body.Bytes(), &envelope))
	require.Len(t, envelope.Data.Items, 1)
	require.Equal(t, "theme-pro", envelope.Data.Items[0].ProfileTheme)
}

type themeListUserClient struct {
	userpb.UserServiceClient
	listUsersCalls        int
	listUsersResponse     *userpb.UserListResponse
	listFollowersResponse *userpb.UserListResponse
}

func (c *themeListUserClient) ListUsers(_ context.Context, _ *userpb.ListUsersRequest, _ ...grpc.CallOption) (*userpb.UserListResponse, error) {
	c.listUsersCalls++
	return c.listUsersResponse, nil
}

func (c *themeListUserClient) ListFollowers(_ context.Context, _ *userpb.ListFollowsRequest, _ ...grpc.CallOption) (*userpb.UserListResponse, error) {
	return c.listFollowersResponse, nil
}

func (c *themeListUserClient) GetUser(context.Context, *userpb.UserIDRequest, ...grpc.CallOption) (*userpb.UserResponse, error) {
	return nil, nil
}

func (c *themeListUserClient) GetUserByUsername(context.Context, *userpb.UsernameRequest, ...grpc.CallOption) (*userpb.UserResponse, error) {
	return nil, nil
}

type themeListMallClient struct {
	mallpb.MallServiceClient
	entitlementsByUser map[int64][]*mallpb.DigitalEntitlement
	requestUserIDs     []int64
}

func (c *themeListMallClient) ListUserDigitalEntitlements(_ context.Context, req *mallpb.ListUserDigitalEntitlementsRequest, _ ...grpc.CallOption) (*mallpb.ListDigitalEntitlementsResponse, error) {
	c.requestUserIDs = append(c.requestUserIDs, req.GetUserId())
	return &mallpb.ListDigitalEntitlementsResponse{Items: c.entitlementsByUser[req.GetUserId()], Total: int64(len(c.entitlementsByUser[req.GetUserId()]))}, nil
}

func unmarshalBody(body []byte, dst any) error {
	return json.Unmarshal(body, dst)
}
