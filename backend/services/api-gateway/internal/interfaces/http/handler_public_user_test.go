package http

import (
	"context"
	"encoding/json"
	stdhttp "net/http"
	"net/http/httptest"
	"testing"

	"api-gateway/api/proto/searchpb"
	"api-gateway/api/proto/userpb"
	"api-gateway/internal/clients"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
)

func TestPublicUserEndpointsRedactPrivateFields(t *testing.T) {
	gin.SetMode(gin.TestMode)
	user := privateUserFixture()
	h := NewHandler(&clients.Clients{
		User:   &publicUserClient{user: user},
		Search: &fakeSearchVisibilityClient{userResponse: &searchpb.SearchUsersResponse{Items: []*searchpb.UserHit{{User: &searchpb.UserDocument{Id: user.GetId()}}}}},
	}, "Authorization", "Bearer", testJWTSecret)
	router := gin.New()
	NewInitControllers(h)(router)

	for _, path := range []string{
		"/api/v1/users/42",
		"/api/v1/users/by-username/alice",
		"/api/v1/users/batch?ids=42",
		"/api/v1/users/42/followers",
		"/api/v1/users/42/following",
		"/api/v1/search/users?q=alice",
	} {
		t.Run(path, func(t *testing.T) {
			recorder := httptest.NewRecorder()
			req := httptest.NewRequest(stdhttp.MethodGet, path, nil)
			router.ServeHTTP(recorder, req)

			require.Equal(t, stdhttp.StatusOK, recorder.Code, recorder.Body.String())
			requirePublicUserPayload(t, recorder.Body.Bytes())
		})
	}
}

func TestGetMeRetainsPrivateUserFields(t *testing.T) {
	gin.SetMode(gin.TestMode)
	h := NewHandler(&clients.Clients{User: &publicUserClient{user: privateUserFixture()}}, "Authorization", "Bearer", testJWTSecret)

	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Set("user_id", int64(42))
	c.Request = httptest.NewRequest(stdhttp.MethodGet, "/api/v1/users/me", nil)
	h.getMe(c)

	require.Equal(t, stdhttp.StatusOK, recorder.Code, recorder.Body.String())
	require.Contains(t, recorder.Body.String(), `"email":"alice@example.test"`)
	require.Contains(t, recorder.Body.String(), `"status":1`)
	require.Contains(t, recorder.Body.String(), `"last_login_at":1710000000000`)
	require.Contains(t, recorder.Body.String(), `"email_verified":true`)
	require.Contains(t, recorder.Body.String(), `"email_verified_at":1710000000100`)
}

func TestPublicUserEndpointsReturnTombstoneForDeletedAccount(t *testing.T) {
	gin.SetMode(gin.TestMode)
	user := privateUserFixture()
	user.AccountState = "deletion_pending"
	h := NewHandler(&clients.Clients{User: &publicUserClient{user: user}}, "Authorization", "Bearer", testJWTSecret)
	router := gin.New()
	NewInitControllers(h)(router)

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, httptest.NewRequest(stdhttp.MethodGet, "/api/v1/users/42", nil))

	require.Equal(t, stdhttp.StatusOK, recorder.Code, recorder.Body.String())
	var envelope struct {
		Data publicUserResponse `json:"data"`
	}
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &envelope))
	require.NotNil(t, envelope.Data.User)
	require.EqualValues(t, 42, envelope.Data.User.ID)
	require.Equal(t, "deleted", envelope.Data.User.Username)
	require.Equal(t, "已注销用户", envelope.Data.User.Nickname)
	require.Equal(t, profileThemeDefault, envelope.Data.User.ProfileTheme)
	require.Empty(t, envelope.Data.User.AvatarURL)
	require.Empty(t, envelope.Data.User.Bio)
	require.Empty(t, envelope.Data.User.BackgroundURL)
	require.Zero(t, envelope.Data.User.FollowerCount)
	require.Zero(t, envelope.Data.User.FollowingCount)
}

func requirePublicUserPayload(t *testing.T, body []byte) {
	t.Helper()
	for _, privateField := range []string{
		`"email":`,
		`"status":`,
		`"created_at":`,
		`"updated_at":`,
		`"last_login_at":`,
		`"email_verified":`,
		`"email_verified_at":`,
	} {
		require.NotContains(t, string(body), privateField)
	}

	var envelope struct {
		Data json.RawMessage `json:"data"`
	}
	require.NoError(t, json.Unmarshal(body, &envelope))
	var data map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(envelope.Data, &data))

	userJSON, hasUser := data["user"]
	if !hasUser {
		var items []json.RawMessage
		require.NoError(t, json.Unmarshal(data["items"], &items))
		require.Len(t, items, 1)
		userJSON = items[0]
	}
	var user map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(userJSON, &user))
	for _, publicField := range []string{
		"id",
		"username",
		"nickname",
		"avatar_url",
		"bio",
		"follower_count",
		"following_count",
		"profile_theme",
	} {
		require.Contains(t, user, publicField)
	}
}

func privateUserFixture() *userpb.UserInfo {
	return &userpb.UserInfo{
		Id:              42,
		Username:        "alice",
		Email:           "alice@example.test",
		Nickname:        "Alice",
		AvatarUrl:       "https://cdn.example.test/alice.png",
		Bio:             "community member",
		Status:          userStatusActive,
		FollowerCount:   8,
		FollowingCount:  3,
		CreatedAt:       1700000000000,
		UpdatedAt:       1700000000100,
		LastLoginAt:     1710000000000,
		EmailVerified:   true,
		EmailVerifiedAt: 1710000000100,
		ProfileTheme:    profileThemeDefault,
	}
}

type publicUserClient struct {
	userpb.UserServiceClient
	user *userpb.UserInfo
}

func (c *publicUserClient) GetUser(context.Context, *userpb.UserIDRequest, ...grpc.CallOption) (*userpb.UserResponse, error) {
	return &userpb.UserResponse{User: c.user}, nil
}

func (c *publicUserClient) GetUserByUsername(context.Context, *userpb.UsernameRequest, ...grpc.CallOption) (*userpb.UserResponse, error) {
	return &userpb.UserResponse{User: c.user}, nil
}

func (c *publicUserClient) ListUsers(context.Context, *userpb.ListUsersRequest, ...grpc.CallOption) (*userpb.UserListResponse, error) {
	return &userpb.UserListResponse{Items: []*userpb.UserInfo{c.user}, Total: 1}, nil
}

func (c *publicUserClient) ListFollowers(context.Context, *userpb.ListFollowsRequest, ...grpc.CallOption) (*userpb.UserListResponse, error) {
	return &userpb.UserListResponse{Items: []*userpb.UserInfo{c.user}, Total: 1}, nil
}

func (c *publicUserClient) ListFollowing(context.Context, *userpb.ListFollowsRequest, ...grpc.CallOption) (*userpb.UserListResponse, error) {
	return &userpb.UserListResponse{Items: []*userpb.UserInfo{c.user}, Total: 1}, nil
}
