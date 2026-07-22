package http

import (
	"context"
	"encoding/json"
	stdhttp "net/http"
	"net/http/httptest"
	"testing"

	"api-gateway/api/proto/creditpb"
	"api-gateway/api/proto/mallpb"
	"api-gateway/api/proto/userpb"
	"api-gateway/internal/clients"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
)

func TestListCreditLeaderboardBatchesActiveUserProfiles(t *testing.T) {
	gin.SetMode(gin.TestMode)
	creditClient := &leaderboardCreditClient{
		response: &creditpb.ListLeaderboardResponse{Items: []*creditpb.LeaderboardEntry{
			{UserId: 42, Total: 120, Rank: 1},
			{UserId: 7, Total: 95, Rank: 2},
		}},
	}
	userClient := &leaderboardUserClient{
		response: &userpb.UserListResponse{Items: []*userpb.UserInfo{
			{Id: 7, Username: "bob", Nickname: "Bob", Email: "bob@example.test", ProfileTheme: "default"},
			{Id: 42, Username: "alice", Nickname: "Alice", Email: "alice@example.test", ProfileTheme: "default"},
		}},
	}
	h := NewHandler(&clients.Clients{Credit: creditClient, User: userClient}, "Authorization", "Bearer", testJWTSecret)
	router := gin.New()
	NewInitControllers(h)(router)

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, httptest.NewRequest(stdhttp.MethodGet, "/api/v1/credits/leaderboard?limit=20", nil))

	require.Equal(t, stdhttp.StatusOK, recorder.Code, recorder.Body.String())
	require.Equal(t, 1, creditClient.calls)
	require.NotNil(t, creditClient.request)
	require.Equal(t, int32(20), creditClient.request.GetLimit())
	require.Equal(t, 1, userClient.calls)
	require.NotNil(t, userClient.request)
	require.Equal(t, []int64{42, 7}, userClient.request.GetIds())
	require.Equal(t, userStatusActive, userClient.request.GetStatus())
	require.Equal(t, int32(1), userClient.request.GetPage())
	require.Equal(t, int32(2), userClient.request.GetPageSize())
	require.NotContains(t, recorder.Body.String(), "@example.test")

	var envelope struct {
		Data struct {
			Items []struct {
				Rank   int32 `json:"rank"`
				UserID int64 `json:"user_id"`
				Total  int64 `json:"total"`
				User   struct {
					Nickname string `json:"nickname"`
					Username string `json:"username"`
				} `json:"user"`
			} `json:"items"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &envelope))
	require.Len(t, envelope.Data.Items, 2)
	require.Equal(t, int32(1), envelope.Data.Items[0].Rank)
	require.Equal(t, int64(42), envelope.Data.Items[0].UserID)
	require.Equal(t, int64(120), envelope.Data.Items[0].Total)
	require.Equal(t, "Alice", envelope.Data.Items[0].User.Nickname)
	require.Equal(t, "alice", envelope.Data.Items[0].User.Username)
}

func TestListCreditLeaderboardBatchesEntitlementChecks(t *testing.T) {
	gin.SetMode(gin.TestMode)
	entries := make([]*creditpb.LeaderboardEntry, 0, 50)
	users := make([]*userpb.UserInfo, 0, 50)
	for userID := int64(1); userID <= 50; userID++ {
		entries = append(entries, &creditpb.LeaderboardEntry{UserId: userID, Total: 1_000 - userID, Rank: int32(userID)})
		users = append(users, &userpb.UserInfo{
			Id:            userID,
			Username:      "member",
			ProfileTheme:  "theme-pro",
			BackgroundUrl: "https://cdn.example/profile.jpg",
		})
	}
	creditClient := &leaderboardCreditClient{response: &creditpb.ListLeaderboardResponse{Items: entries}}
	userClient := &leaderboardUserClient{response: &userpb.UserListResponse{Items: users}}
	mallClient := &leaderboardMallClient{}
	h := NewHandler(&clients.Clients{Credit: creditClient, User: userClient, Mall: mallClient}, "Authorization", "Bearer", testJWTSecret)
	router := gin.New()
	NewInitControllers(h)(router)

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, httptest.NewRequest(stdhttp.MethodGet, "/api/v1/credits/leaderboard?limit=50", nil))

	require.Equal(t, stdhttp.StatusOK, recorder.Code, recorder.Body.String())
	require.Equal(t, 1, creditClient.calls)
	require.Equal(t, 1, userClient.calls)
	require.Len(t, mallClient.requests, 2)
	requestsByGrant := make(map[string]*mallpb.ListActiveEntitlementUserIDsRequest, len(mallClient.requests))
	for _, req := range mallClient.requests {
		requestsByGrant[req.GetGrantType()+":"+req.GetGrantKey()] = req
	}
	require.Len(t, requestsByGrant["membership:"].GetUserIds(), 50)
	require.Len(t, requestsByGrant["theme:theme-pro"].GetUserIds(), 50)
}

type leaderboardCreditClient struct {
	creditpb.CreditServiceClient
	calls    int
	request  *creditpb.ListLeaderboardRequest
	response *creditpb.ListLeaderboardResponse
}

func (c *leaderboardCreditClient) ListLeaderboard(_ context.Context, req *creditpb.ListLeaderboardRequest, _ ...grpc.CallOption) (*creditpb.ListLeaderboardResponse, error) {
	c.calls++
	c.request = req
	return c.response, nil
}

type leaderboardUserClient struct {
	userpb.UserServiceClient
	calls    int
	request  *userpb.ListUsersRequest
	response *userpb.UserListResponse
}

func (c *leaderboardUserClient) ListUsers(_ context.Context, req *userpb.ListUsersRequest, _ ...grpc.CallOption) (*userpb.UserListResponse, error) {
	c.calls++
	c.request = req
	return c.response, nil
}

type leaderboardMallClient struct {
	mallpb.MallServiceClient
	requests []*mallpb.ListActiveEntitlementUserIDsRequest
}

func (c *leaderboardMallClient) ListActiveEntitlementUserIDs(_ context.Context, req *mallpb.ListActiveEntitlementUserIDsRequest, _ ...grpc.CallOption) (*mallpb.ListActiveEntitlementUserIDsResponse, error) {
	c.requests = append(c.requests, req)
	return &mallpb.ListActiveEntitlementUserIDsResponse{UserIds: req.GetUserIds()}, nil
}
