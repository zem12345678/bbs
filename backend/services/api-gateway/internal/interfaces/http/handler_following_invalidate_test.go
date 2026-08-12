package http

import (
	"context"
	"encoding/json"
	stdhttp "net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"api-gateway/api/proto/userpb"
	"api-gateway/internal/clients"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
)

func TestRemoveFollowerRoutesReverseTheFollowDirection(t *testing.T) {
	gin.SetMode(gin.TestMode)
	for _, testCase := range []struct {
		name          string
		method        string
		path          string
		body          string
		compatibility bool
	}{
		{name: "canonical", method: stdhttp.MethodDelete, path: "/api/v1/users/me/followers/77"},
		{name: "api compatibility", method: stdhttp.MethodPost, path: "/api/following/invalidate", body: `{"userId":"77"}`, compatibility: true},
		{name: "root compatibility", method: stdhttp.MethodPost, path: "/following/invalidate", body: `{"userId":"77"}`, compatibility: true},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			client := &followingInvalidateUserClient{user: &userpb.UserInfo{
				Id: 77, Username: "alice", Nickname: "Alice", Email: "private@example.test", Bio: "member",
				AvatarUrl: "https://cdn.example.test/alice.png", Status: userStatusActive, FollowerCount: 8, FollowingCount: 3,
				CreatedAt: 1700000000000, UpdatedAt: 1700000000100,
			}}
			router := followingInvalidateRouter(client)
			recorder := performFollowingInvalidateRequest(router, testCase.method, testCase.path, testCase.body, signedAuthToken(t, jwt.MapClaims{"sub": "42"}))

			require.Equal(t, stdhttp.StatusOK, recorder.Code, recorder.Body.String())
			require.NotNil(t, client.unfollowRequest)
			require.Equal(t, int64(77), client.unfollowRequest.GetFollowerId())
			require.Equal(t, int64(42), client.unfollowRequest.GetFolloweeId())
			require.NotNil(t, client.getUserRequest)
			require.Equal(t, int64(77), client.getUserRequest.GetId())
			require.NotContains(t, recorder.Body.String(), "private@example.test")

			var payload map[string]json.RawMessage
			require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &payload))
			if testCase.compatibility {
				require.NotContains(t, payload, "data")
				require.JSONEq(t, `"77"`, string(payload["id"]))
				require.JSONEq(t, `"alice"`, string(payload["username"]))
				require.JSONEq(t, `null`, string(payload["host"]))
				require.JSONEq(t, `[]`, string(payload["avatarDecorations"]))
				return
			}
			require.Contains(t, payload, "data")
			require.Contains(t, string(payload["data"]), `"user"`)
		})
	}
}

func TestFollowingInvalidateRequiresWriteScopeAndValidTarget(t *testing.T) {
	client := &followingInvalidateUserClient{user: &userpb.UserInfo{Id: 77, Username: "alice", CreatedAt: 1700000000000}}
	router := followingInvalidateRouter(client)

	missingAuth := performFollowingInvalidateRequest(router, stdhttp.MethodPost, "/api/following/invalidate", `{"userId":"77"}`, "")
	require.Equal(t, stdhttp.StatusUnauthorized, missingAuth.Code, missingAuth.Body.String())

	readOnly := performFollowingInvalidateRequest(router, stdhttp.MethodPost, "/api/following/invalidate", `{"userId":"77"}`, followingInvalidateScopedToken(t, "read"))
	require.Equal(t, stdhttp.StatusForbidden, readOnly.Code, readOnly.Body.String())
	readOnlyCanonical := performFollowingInvalidateRequest(router, stdhttp.MethodDelete, "/api/v1/users/me/followers/77", "", followingInvalidateScopedToken(t, "read"))
	require.Equal(t, stdhttp.StatusForbidden, readOnlyCanonical.Code, readOnlyCanonical.Body.String())

	invalid := performFollowingInvalidateRequest(router, stdhttp.MethodPost, "/api/following/invalidate", `{"userId":"0"}`, followingInvalidateScopedToken(t, "write"))
	require.Equal(t, stdhttp.StatusBadRequest, invalid.Code, invalid.Body.String())

	self := performFollowingInvalidateRequest(router, stdhttp.MethodPost, "/api/following/invalidate", `{"userId":"42"}`, followingInvalidateScopedToken(t, "write"))
	require.Equal(t, stdhttp.StatusBadRequest, self.Code, self.Body.String())
	require.Nil(t, client.unfollowRequest)
}

func followingInvalidateRouter(client clients.UserClient) *gin.Engine {
	h := NewHandler(&clients.Clients{User: client}, "Authorization", "Bearer", testJWTSecret)
	router := gin.New()
	NewInitControllers(h)(router)
	return router
}

func performFollowingInvalidateRequest(router stdhttp.Handler, method, path, body, token string) *httptest.ResponseRecorder {
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(method, path, strings.NewReader(body))
	if body != "" {
		request.Header.Set("Content-Type", "application/json")
	}
	if token != "" {
		request.Header.Set("Authorization", "Bearer "+token)
	}
	router.ServeHTTP(recorder, request)
	return recorder
}

func followingInvalidateScopedToken(t *testing.T, scope string) string {
	t.Helper()
	return signedAuthToken(t, jwt.MapClaims{
		"sub": "42", "jti": "following-invalidate-" + scope, "exp": time.Now().Add(time.Hour).Unix(),
		credentialVersionClaim: "0", "token_type": apiTokenType, "scopes": []string{scope},
	})
}

type followingInvalidateUserClient struct {
	userpb.UserServiceClient
	user            *userpb.UserInfo
	unfollowRequest *userpb.FollowRequest
	getUserRequest  *userpb.UserIDRequest
}

func (c *followingInvalidateUserClient) Unfollow(_ context.Context, request *userpb.FollowRequest, _ ...grpc.CallOption) (*userpb.SimpleResponse, error) {
	c.unfollowRequest = request
	return &userpb.SimpleResponse{Success: true}, nil
}

func (c *followingInvalidateUserClient) GetUser(_ context.Context, request *userpb.UserIDRequest, _ ...grpc.CallOption) (*userpb.UserResponse, error) {
	c.getUserRequest = request
	return &userpb.UserResponse{Success: true, User: c.user}, nil
}

var _ clients.UserClient = (*followingInvalidateUserClient)(nil)
