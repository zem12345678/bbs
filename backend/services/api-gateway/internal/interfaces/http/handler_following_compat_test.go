package http

import (
	"bytes"
	"context"
	"encoding/json"
	stdhttp "net/http"
	"net/http/httptest"
	"testing"
	"time"

	"api-gateway/api/proto/userpb"
	"api-gateway/internal/clients"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func TestFollowingCreateCompatAliasesForwardPreferenceAndReturnBareUser(t *testing.T) {
	gin.SetMode(gin.TestMode)
	for _, path := range []string{"/following/create", "/api/following/create", "/api/v1/following/create"} {
		t.Run(path, func(t *testing.T) {
			userClient := newFollowingCompatUserClient()
			router := followingCompatRouter(userClient, &followingCompatPreferenceClient{})

			response := performFollowingCompatRequest(router, path, `{"userId":"77","withReplies":true}`, followingCompatWriteToken(t))

			require.Equal(t, stdhttp.StatusOK, response.Code, response.Body.String())
			require.NotNil(t, userClient.followRequest)
			require.EqualValues(t, 42, userClient.followRequest.GetFollowerId())
			require.EqualValues(t, 77, userClient.followRequest.GetFolloweeId())
			require.NotNil(t, userClient.followRequest.WithReplies)
			require.True(t, userClient.followRequest.GetWithReplies())
			var payload map[string]json.RawMessage
			require.NoError(t, json.Unmarshal(response.Body.Bytes(), &payload))
			require.JSONEq(t, `"77"`, string(payload["id"]))
			require.NotContains(t, payload, "data")
		})
	}
}

func TestCanonicalFollowAcceptsOptionalWithReplies(t *testing.T) {
	userClient := newFollowingCompatUserClient()
	router := followingCompatRouter(userClient, &followingCompatPreferenceClient{})

	response := performFollowingCompatRequest(router, "/api/v1/users/77/follow", `{"withReplies":true}`, followingCompatWriteToken(t))

	require.Equal(t, stdhttp.StatusOK, response.Code, response.Body.String())
	require.NotNil(t, userClient.followRequest)
	require.NotNil(t, userClient.followRequest.WithReplies)
	require.True(t, userClient.followRequest.GetWithReplies())
}

func TestFollowingUpdateAndUpdateAllPreserveOptionalFields(t *testing.T) {
	userClient := newFollowingCompatUserClient()
	preferenceClient := &followingCompatPreferenceClient{}
	router := followingCompatRouter(userClient, preferenceClient)
	token := followingCompatWriteToken(t)

	updated := performFollowingCompatRequest(router, "/api/v1/following/update", `{"userId":"77","notify":"normal"}`, token)
	require.Equal(t, stdhttp.StatusOK, updated.Code, updated.Body.String())
	require.NotNil(t, preferenceClient.updateRequest)
	require.Nil(t, preferenceClient.updateRequest.WithReplies)
	require.NotNil(t, preferenceClient.updateRequest.Notify)
	require.Equal(t, "normal", preferenceClient.updateRequest.GetNotify())
	var updatedUser map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(updated.Body.Bytes(), &updatedUser))
	require.JSONEq(t, `"42"`, string(updatedUser["id"]), updated.Body.String())

	all := performFollowingCompatRequest(router, "/following/update-all", `{}`, token)
	require.Equal(t, stdhttp.StatusNoContent, all.Code, all.Body.String())
	require.Empty(t, all.Body.String())
	require.NotNil(t, preferenceClient.updateAllRequest)
	require.Nil(t, preferenceClient.updateAllRequest.WithReplies)
	require.Nil(t, preferenceClient.updateAllRequest.Notify)
}

func TestFollowingListCompatUsesEdgeCursorsAndBareArrays(t *testing.T) {
	userClient := newFollowingCompatUserClient()
	preferenceClient := &followingCompatPreferenceClient{items: []*userpb.FollowingInfo{{
		Id: 901, FollowerId: 42, FolloweeId: 77, WithReplies: true, Notify: "normal", CreatedAt: 1700000000000,
		Followee: userClient.users[77],
	}}}
	router := followingCompatRouter(userClient, preferenceClient)

	response := performFollowingCompatRequest(router, "/api/users/following", `{"userId":"42","sinceId":"800","untilId":"950","limit":25}`, "")

	require.Equal(t, stdhttp.StatusOK, response.Code, response.Body.String())
	require.NotNil(t, preferenceClient.listFollowingRequest)
	require.EqualValues(t, 42, preferenceClient.listFollowingRequest.GetUserId())
	require.EqualValues(t, 800, preferenceClient.listFollowingRequest.GetSinceId())
	require.EqualValues(t, 950, preferenceClient.listFollowingRequest.GetUntilId())
	require.EqualValues(t, 25, preferenceClient.listFollowingRequest.GetLimit())
	var payload []map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(response.Body.Bytes(), &payload))
	require.Len(t, payload, 1)
	require.JSONEq(t, `"901"`, string(payload[0]["id"]))
	require.JSONEq(t, `true`, string(payload[0]["withReplies"]))
	require.JSONEq(t, `"normal"`, string(payload[0]["notify"]))
	require.Contains(t, payload[0], "followee")
	require.NotContains(t, payload[0], "follower")
}

func TestFollowingListCompatResolvesLocalUsernameAndValidatesBirthday(t *testing.T) {
	userClient := newFollowingCompatUserClient()
	preferenceClient := &followingCompatPreferenceClient{}
	router := followingCompatRouter(userClient, preferenceClient)

	local := performFollowingCompatRequest(router, "/users/followers", `{"username":"ALICE","host":null}`, "")
	require.Equal(t, stdhttp.StatusOK, local.Code, local.Body.String())
	require.Equal(t, "alice", userClient.usernameRequest.GetUsername())
	require.NotNil(t, preferenceClient.listFollowerRequest)
	require.EqualValues(t, 77, preferenceClient.listFollowerRequest.GetUserId())

	invalidBirthday := performFollowingCompatRequest(router, "/users/following", `{"userId":"42","birthday":"2026-02-30"}`, "")
	require.Equal(t, stdhttp.StatusBadRequest, invalidBirthday.Code, invalidBirthday.Body.String())
	require.Contains(t, invalidBirthday.Body.String(), "BIRTHDAY_DATE_FORMAT_INVALID")
	require.Contains(t, invalidBirthday.Body.String(), followingListBirthdayInvalidID)

	validBirthday := performFollowingCompatRequest(router, "/users/following", `{"userId":"42","birthday":"2000-08-19"}`, "")
	require.Equal(t, stdhttp.StatusOK, validBirthday.Code, validBirthday.Body.String())
	require.JSONEq(t, `[]`, validBirthday.Body.String())
}

func TestFollowingCompatRejectsUnknownFieldsAndMapsNotFollowing(t *testing.T) {
	userClient := newFollowingCompatUserClient()
	preferenceClient := &followingCompatPreferenceClient{updateErr: status.Error(codes.FailedPrecondition, "not following user")}
	router := followingCompatRouter(userClient, preferenceClient)
	token := followingCompatWriteToken(t)

	unknown := performFollowingCompatRequest(router, "/following/create", `{"userId":"77","unknown":true}`, token)
	require.Equal(t, stdhttp.StatusBadRequest, unknown.Code, unknown.Body.String())
	require.Contains(t, unknown.Body.String(), "INVALID_PARAM")
	require.Nil(t, userClient.followRequest)

	notFollowing := performFollowingCompatRequest(router, "/following/update", `{"userId":"77","withReplies":false}`, token)
	require.Equal(t, stdhttp.StatusBadRequest, notFollowing.Code, notFollowing.Body.String())
	require.Contains(t, notFollowing.Body.String(), "NOT_FOLLOWING")
	require.Contains(t, notFollowing.Body.String(), followingUpdateNotFollowingID)
}

func TestFollowingCreateRejectsUpdateOnlyNotifyField(t *testing.T) {
	userClient := newFollowingCompatUserClient()
	router := followingCompatRouter(userClient, &followingCompatPreferenceClient{})

	response := performFollowingCompatRequest(router, "/following/create", `{"userId":"77","notify":"normal"}`, followingCompatWriteToken(t))

	require.Equal(t, stdhttp.StatusBadRequest, response.Code, response.Body.String())
	require.Contains(t, response.Body.String(), "INVALID_PARAM")
	require.Nil(t, userClient.followRequest)
}

func followingCompatRouter(userClient clients.UserClient, followingClient clients.UserFollowingClient) *gin.Engine {
	handler := NewHandler(&clients.Clients{User: userClient, UserFollowing: followingClient}, "Authorization", "Bearer", testJWTSecret)
	router := gin.New()
	NewInitControllers(handler)(router)
	return router
}

func performFollowingCompatRequest(router stdhttp.Handler, path, body, token string) *httptest.ResponseRecorder {
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(stdhttp.MethodPost, path, bytes.NewBufferString(body))
	request.Header.Set("Content-Type", "application/json")
	if token != "" {
		request.Header.Set("Authorization", "Bearer "+token)
	}
	router.ServeHTTP(recorder, request)
	return recorder
}

func followingCompatWriteToken(t *testing.T) string {
	t.Helper()
	return signedAuthToken(t, jwt.MapClaims{
		"sub": "42", "jti": "following-compat-write", "exp": time.Now().Add(time.Hour).Unix(),
		credentialVersionClaim: credentialVersionInitial, "token_type": apiTokenType, "scopes": []string{"write"},
	})
}

type followingCompatUserClient struct {
	userpb.UserServiceClient
	users           map[int64]*userpb.UserInfo
	followRequest   *userpb.FollowRequest
	unfollowRequest *userpb.FollowRequest
	usernameRequest *userpb.UsernameRequest
}

func newFollowingCompatUserClient() *followingCompatUserClient {
	return &followingCompatUserClient{users: map[int64]*userpb.UserInfo{
		42: {Id: 42, Username: "viewer", Nickname: "Viewer", CreatedAt: 1690000000000},
		77: {Id: 77, Username: "alice", Nickname: "Alice", CreatedAt: 1700000000000},
	}}
}

func (client *followingCompatUserClient) GetUser(_ context.Context, request *userpb.UserIDRequest, _ ...grpc.CallOption) (*userpb.UserResponse, error) {
	user := client.users[request.GetId()]
	if user == nil {
		return nil, status.Error(codes.NotFound, "user not found")
	}
	return &userpb.UserResponse{Success: true, User: user}, nil
}

func (client *followingCompatUserClient) GetUserByUsername(_ context.Context, request *userpb.UsernameRequest, _ ...grpc.CallOption) (*userpb.UserResponse, error) {
	client.usernameRequest = request
	for _, user := range client.users {
		if user.GetUsername() == request.GetUsername() {
			return &userpb.UserResponse{Success: true, User: user}, nil
		}
	}
	return nil, status.Error(codes.NotFound, "user not found")
}

func (client *followingCompatUserClient) Follow(_ context.Context, request *userpb.FollowRequest, _ ...grpc.CallOption) (*userpb.FollowResponse, error) {
	client.followRequest = request
	return &userpb.FollowResponse{Success: true}, nil
}

func (client *followingCompatUserClient) Unfollow(_ context.Context, request *userpb.FollowRequest, _ ...grpc.CallOption) (*userpb.SimpleResponse, error) {
	client.unfollowRequest = request
	return &userpb.SimpleResponse{Success: true}, nil
}

type followingCompatPreferenceClient struct {
	userpb.UserServiceClient
	items                []*userpb.FollowingInfo
	updateRequest        *userpb.UpdateFollowingRequest
	updateAllRequest     *userpb.UpdateAllFollowingsRequest
	listFollowingRequest *userpb.ListFollowingEdgesRequest
	listFollowerRequest  *userpb.ListFollowingEdgesRequest
	updateErr            error
}

func (client *followingCompatPreferenceClient) UpdateFollowing(_ context.Context, request *userpb.UpdateFollowingRequest, _ ...grpc.CallOption) (*userpb.FollowingResponse, error) {
	client.updateRequest = request
	return &userpb.FollowingResponse{}, client.updateErr
}

func (client *followingCompatPreferenceClient) UpdateAllFollowings(_ context.Context, request *userpb.UpdateAllFollowingsRequest, _ ...grpc.CallOption) (*userpb.SimpleResponse, error) {
	client.updateAllRequest = request
	return &userpb.SimpleResponse{Success: true}, nil
}

func (client *followingCompatPreferenceClient) ListFollowingEdges(_ context.Context, request *userpb.ListFollowingEdgesRequest, _ ...grpc.CallOption) (*userpb.FollowingListResponse, error) {
	client.listFollowingRequest = request
	return &userpb.FollowingListResponse{Items: client.items}, nil
}

func (client *followingCompatPreferenceClient) ListFollowerEdges(_ context.Context, request *userpb.ListFollowingEdgesRequest, _ ...grpc.CallOption) (*userpb.FollowingListResponse, error) {
	client.listFollowerRequest = request
	return &userpb.FollowingListResponse{Items: client.items}, nil
}

var _ clients.UserClient = (*followingCompatUserClient)(nil)
var _ clients.UserFollowingClient = (*followingCompatPreferenceClient)(nil)
