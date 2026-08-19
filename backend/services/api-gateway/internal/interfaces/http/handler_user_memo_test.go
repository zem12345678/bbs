package http

import (
	"context"
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
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type userMemoClient struct {
	userpb.UserServiceClient
	updateReq       *userpb.UpdateUserMemoRequest
	getReq          *userpb.GetUserMemoRequest
	followReqs      []*userpb.FollowRequest
	safetyReqs      []*userpb.UserRelationRequest
	memo            string
	memos           map[int64]string
	followResponses map[[2]int64]*userpb.IsFollowingResponse
	safetyResponses map[int64]*userpb.SafetyRelationResponse
	updateErr       error
	getErr          error
}

func (c *userMemoClient) UpdateUserMemo(_ context.Context, req *userpb.UpdateUserMemoRequest, _ ...grpc.CallOption) (*userpb.SimpleResponse, error) {
	c.updateReq = req
	return &userpb.SimpleResponse{Success: true}, c.updateErr
}

func (c *userMemoClient) GetUserMemo(_ context.Context, req *userpb.GetUserMemoRequest, _ ...grpc.CallOption) (*userpb.UserMemoResponse, error) {
	c.getReq = req
	memo := c.memo
	if c.memos != nil {
		memo = c.memos[req.GetTargetUserId()]
	}
	return &userpb.UserMemoResponse{Success: true, Memo: memo}, c.getErr
}

func (c *userMemoClient) IsFollowing(_ context.Context, req *userpb.FollowRequest, _ ...grpc.CallOption) (*userpb.IsFollowingResponse, error) {
	c.followReqs = append(c.followReqs, req)
	if response := c.followResponses[[2]int64{req.GetFollowerId(), req.GetFolloweeId()}]; response != nil {
		return response, nil
	}
	return &userpb.IsFollowingResponse{}, nil
}

func (c *userMemoClient) GetSafetyRelation(_ context.Context, req *userpb.UserRelationRequest, _ ...grpc.CallOption) (*userpb.SafetyRelationResponse, error) {
	c.safetyReqs = append(c.safetyReqs, req)
	if response := c.safetyResponses[req.GetTargetId()]; response != nil {
		return response, nil
	}
	return &userpb.SafetyRelationResponse{}, nil
}

func TestUpdateUserMemoRouteAliasesPreserveSnowflakeIDAndNullDeletes(t *testing.T) {
	client := &userMemoClient{}
	router := newUserMemoTestRouter(client)
	token := signedAuthToken(t, jwt.MapClaims{"sub": "42"})

	for _, path := range []string{"/users/update-memo", "/api/users/update-memo", "/api/v1/users/update-memo"} {
		response := performUserMemoRequest(router, stdhttp.MethodPost, path, `{"userId":"9223372036854775807","memo":" teammate "}`, token)
		require.Equal(t, stdhttp.StatusNoContent, response.Code, "%s: %s", path, response.Body.String())
		require.Equal(t, int64(42), client.updateReq.GetUserId())
		require.Equal(t, int64(9223372036854775807), client.updateReq.GetTargetUserId())
		require.Equal(t, " teammate ", client.updateReq.GetMemo())
	}

	response := performUserMemoRequest(router, stdhttp.MethodPost, "/api/v1/users/update-memo", `{"userId":"77","memo":null}`, token)
	require.Equal(t, stdhttp.StatusNoContent, response.Code, response.Body.String())
	require.Empty(t, client.updateReq.GetMemo())
}

func TestGetUserMemoReturnsCurrentViewerMemo(t *testing.T) {
	client := &userMemoClient{memo: "project lead"}
	router := newUserMemoTestRouter(client)
	response := performUserMemoRequest(router, stdhttp.MethodGet, "/api/v1/users/77/memo", "", signedAuthToken(t, jwt.MapClaims{"sub": "42"}))

	require.Equal(t, stdhttp.StatusOK, response.Code, response.Body.String())
	require.Equal(t, int64(42), client.getReq.GetUserId())
	require.Equal(t, int64(77), client.getReq.GetTargetUserId())
	require.Contains(t, response.Body.String(), `"memo":"project lead"`)
}

func TestUserMemoRoutesRequireMatchingAPITokenScopesAndMapMissingTarget(t *testing.T) {
	client := &userMemoClient{}
	router := newUserMemoTestRouter(client)

	readOnly := performUserMemoRequest(router, stdhttp.MethodPost, "/api/v1/users/update-memo", `{"userId":"77","memo":"x"}`, userMemoScopedToken(t, "read"))
	require.Equal(t, stdhttp.StatusForbidden, readOnly.Code, readOnly.Body.String())
	writeOnly := performUserMemoRequest(router, stdhttp.MethodGet, "/api/v1/users/77/memo", "", userMemoScopedToken(t, "write"))
	require.Equal(t, stdhttp.StatusForbidden, writeOnly.Code, writeOnly.Body.String())

	client.updateErr = status.Error(codes.NotFound, "user not found")
	missing := performUserMemoRequest(router, stdhttp.MethodPost, "/api/v1/users/update-memo", `{"userId":"77","memo":"x"}`, userMemoScopedToken(t, "write"))
	require.Equal(t, stdhttp.StatusBadRequest, missing.Code, missing.Body.String())
	require.Contains(t, missing.Body.String(), `"legacy_code":"NO_SUCH_USER"`)
}

func TestUserRelationCompatibilityReturnsViewerRelationAndMemo(t *testing.T) {
	client := &userMemoClient{
		memos: map[int64]string{77: "project lead"},
		followResponses: map[[2]int64]*userpb.IsFollowingResponse{
			{42, 77}: {Following: true},
			{77, 42}: {Pending: true},
		},
		safetyResponses: map[int64]*userpb.SafetyRelationResponse{
			77: {Blocked: true, BlockedBy: true, Muted: true},
		},
	}
	router := newUserRelationTestRouter(client)
	token := signedAuthToken(t, jwt.MapClaims{"sub": "42"})

	for _, path := range []string{"/users/relation", "/api/users/relation", "/api/v1/users/relation"} {
		response := performUserMemoRequest(router, stdhttp.MethodPost, path, `{"userId":"77"}`, token)
		require.Equal(t, stdhttp.StatusOK, response.Code, "%s: %s", path, response.Body.String())
		require.JSONEq(t, `{
			"id":"77","isFollowing":true,"hasPendingFollowRequestFromYou":false,
			"hasPendingFollowRequestToYou":true,"isFollowed":false,"isBlocking":true,
			"isBlocked":true,"isMuted":true,"isRenoteMuted":false,
			"isInstanceMuted":false,"memo":"project lead"
		}`, response.Body.String())
	}
	require.Equal(t, int64(42), client.getReq.GetUserId())
	require.Equal(t, int64(77), client.getReq.GetTargetUserId())
}

func TestUserRelationCompatibilityPreservesBatchShapeAndNullMemo(t *testing.T) {
	client := &userMemoClient{memos: map[int64]string{77: "known"}}
	router := newUserRelationTestRouter(client)
	token := signedAuthToken(t, jwt.MapClaims{"sub": "42"})

	response := performUserMemoRequest(router, stdhttp.MethodPost, "/api/v1/users/relation", `{"userId":["77",88]}`, token)
	require.Equal(t, stdhttp.StatusOK, response.Code, response.Body.String())
	require.JSONEq(t, `[
		{"id":"77","isFollowing":false,"hasPendingFollowRequestFromYou":false,"hasPendingFollowRequestToYou":false,"isFollowed":false,"isBlocking":false,"isBlocked":false,"isMuted":false,"isRenoteMuted":false,"isInstanceMuted":false,"memo":"known"},
		{"id":"88","isFollowing":false,"hasPendingFollowRequestFromYou":false,"hasPendingFollowRequestToYou":false,"isFollowed":false,"isBlocking":false,"isBlocked":false,"isMuted":false,"isRenoteMuted":false,"isInstanceMuted":false,"memo":null}
	]`, response.Body.String())

	for _, body := range []string{`{}`, `{"userId":[]}`, `{"userId":[0]}`} {
		invalid := performUserMemoRequest(router, stdhttp.MethodPost, "/api/v1/users/relation", body, token)
		require.Equal(t, stdhttp.StatusBadRequest, invalid.Code, invalid.Body.String())
	}
	denied := performUserMemoRequest(router, stdhttp.MethodPost, "/api/v1/users/relation", `{"userId":"77"}`, userMemoScopedToken(t, "write"))
	require.Equal(t, stdhttp.StatusForbidden, denied.Code, denied.Body.String())
}

func TestUpdateUserMemoRequiresNullableMemoField(t *testing.T) {
	client := &userMemoClient{}
	router := newUserMemoTestRouter(client)
	token := signedAuthToken(t, jwt.MapClaims{"sub": "42"})

	missing := performUserMemoRequest(router, stdhttp.MethodPost, "/api/v1/users/update-memo", `{"userId":"77"}`, token)
	require.Equal(t, stdhttp.StatusBadRequest, missing.Code, missing.Body.String())
	require.Nil(t, client.updateReq)

	wrongType := performUserMemoRequest(router, stdhttp.MethodPost, "/api/v1/users/update-memo", `{"userId":"77","memo":true}`, token)
	require.Equal(t, stdhttp.StatusBadRequest, wrongType.Code, wrongType.Body.String())
	require.Nil(t, client.updateReq)
}

func newUserMemoTestRouter(client *userMemoClient) *gin.Engine {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	NewInitControllers(NewHandler(&clients.Clients{UserMemos: client}, "Authorization", "Bearer", testJWTSecret))(router)
	return router
}

func newUserRelationTestRouter(client *userMemoClient) *gin.Engine {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	NewInitControllers(NewHandler(&clients.Clients{User: client, UserMemos: client, UserSafety: client}, "Authorization", "Bearer", testJWTSecret))(router)
	return router
}

func performUserMemoRequest(router *gin.Engine, method, path, body, token string) *httptest.ResponseRecorder {
	request := httptest.NewRequest(method, path, strings.NewReader(body))
	if body != "" {
		request.Header.Set("Content-Type", "application/json")
	}
	request.Header.Set("Authorization", "Bearer "+token)
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, request)
	return recorder
}

func userMemoScopedToken(t *testing.T, scope string) string {
	t.Helper()
	return signedAuthToken(t, jwt.MapClaims{
		"sub": "42", "jti": "user-memo-" + scope, "exp": time.Now().Add(time.Hour).Unix(),
		credentialVersionClaim: credentialVersionInitial, "token_type": apiTokenType, "scopes": []string{scope},
	})
}
