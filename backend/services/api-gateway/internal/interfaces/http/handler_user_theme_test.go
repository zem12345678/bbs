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
	items := make([]*userpb.UserInfo, 0, 20)
	for id := int64(11); id < 31; id++ {
		items = append(items, &userpb.UserInfo{Id: id, Username: "member", ProfileTheme: "theme-pro", BackgroundUrl: "https://cdn.example/profile.jpg"})
	}
	items[0].ProfileTheme = "THEME-PRO"
	userClient := &themeListUserClient{
		listUsersResponse: &userpb.UserListResponse{
			Items: items,
		},
	}
	mallClient := &themeListMallClient{
		activeUserIDsByGrant: map[string][]int64{
			"membership:":     {11, 13},
			"theme:theme-pro": {11, 12},
		},
	}
	h := NewHandler(&clients.Clients{User: userClient, Mall: mallClient}, "Authorization", "Bearer", testJWTSecret)

	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodGet, "/api/v1/search/users?q=alice", nil)

	h.searchUsers(c)

	require.Equal(t, http.StatusOK, recorder.Code, recorder.Body.String())
	require.Equal(t, 1, userClient.listUsersCalls)
	require.Len(t, mallClient.batchRequests, 2)
	batchRequestUserIDs := map[string][]int64{}
	for _, req := range mallClient.batchRequests {
		batchRequestUserIDs[req.GetGrantType()+":"+req.GetGrantKey()] = req.GetUserIds()
	}
	require.Len(t, batchRequestUserIDs["membership:"], 20)
	require.Len(t, batchRequestUserIDs["theme:theme-pro"], 20)

	var envelope struct {
		Data struct {
			Items []struct {
				Username      string `json:"username"`
				ProfileTheme  string `json:"profile_theme"`
				BackgroundURL string `json:"background_url"`
			} `json:"items"`
		} `json:"data"`
	}
	require.NoError(t, unmarshalBody(recorder.Body.Bytes(), &envelope))
	require.Len(t, envelope.Data.Items, 20)
	require.Equal(t, "theme-pro", envelope.Data.Items[0].ProfileTheme)
	require.Equal(t, "https://cdn.example/profile.jpg", envelope.Data.Items[0].BackgroundURL)
	require.Equal(t, "theme-pro", envelope.Data.Items[1].ProfileTheme)
	require.Empty(t, envelope.Data.Items[1].BackgroundURL)
	require.Equal(t, "default", envelope.Data.Items[2].ProfileTheme)
	require.Equal(t, "https://cdn.example/profile.jpg", envelope.Data.Items[2].BackgroundURL)
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
		activeUserIDsByGrant: map[string][]int64{
			"theme:theme-pro": {21},
		},
	}
	h := NewHandler(&clients.Clients{User: userClient, Mall: mallClient}, "Authorization", "Bearer", testJWTSecret)

	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Params = gin.Params{{Key: "id", Value: "42"}}
	c.Request = httptest.NewRequest(http.MethodGet, "/api/v1/users/42/followers", nil)

	h.listFollowers(c)

	require.Equal(t, http.StatusOK, recorder.Code, recorder.Body.String())
	require.Len(t, mallClient.batchRequests, 1)
	require.Equal(t, "theme", mallClient.batchRequests[0].GetGrantType())
	require.Equal(t, "theme-pro", mallClient.batchRequests[0].GetGrantKey())
	require.Equal(t, []int64{21}, mallClient.batchRequests[0].GetUserIds())

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
	activeUserIDsByGrant map[string][]int64
	batchRequests        []*mallpb.ListActiveEntitlementUserIDsRequest
}

func (c *themeListMallClient) ListActiveEntitlementUserIDs(_ context.Context, req *mallpb.ListActiveEntitlementUserIDsRequest, _ ...grpc.CallOption) (*mallpb.ListActiveEntitlementUserIDsResponse, error) {
	c.batchRequests = append(c.batchRequests, req)
	userIDs := c.activeUserIDsByGrant[req.GetGrantType()+":"+req.GetGrantKey()]
	return &mallpb.ListActiveEntitlementUserIDsResponse{UserIds: userIDs}, nil
}

func unmarshalBody(body []byte, dst any) error {
	return json.Unmarshal(body, dst)
}
