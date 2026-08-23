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

func TestUsersShowCompatSupportsSingleBatchAndUsernameAliases(t *testing.T) {
	gin.SetMode(gin.TestMode)
	client := newUsersShowCompatClient()
	h := NewHandler(&clients.Clients{User: client}, "Authorization", "Bearer", testJWTSecret)
	router := gin.New()
	NewInitControllers(h)(router)

	for _, path := range []string{"/users/show", "/api/users/show", "/api/v1/users/show"} {
		t.Run(path+"-single", func(t *testing.T) {
			response := performUsersShowRequest(router, path, `{"userId":"77"}`, "")
			require.Equal(t, stdhttp.StatusOK, response.Code, response.Body.String())
			var payload map[string]any
			require.NoError(t, json.Unmarshal(response.Body.Bytes(), &payload))
			require.Equal(t, "77", payload["id"])
			require.Equal(t, "alice", payload["username"])
		})
	}

	batch := performUsersShowRequest(router, "/api/users/show", `{"userIds":["99","77"]}`, "")
	require.Equal(t, stdhttp.StatusOK, batch.Code, batch.Body.String())
	var batchPayload []map[string]any
	require.NoError(t, json.Unmarshal(batch.Body.Bytes(), &batchPayload))
	require.Len(t, batchPayload, 2)
	require.Equal(t, "99", batchPayload[0]["id"])
	require.Equal(t, "77", batchPayload[1]["id"])

	username := performUsersShowRequest(router, "/users/show", `{"username":"ALICE","host":null}`, "")
	require.Equal(t, stdhttp.StatusOK, username.Code, username.Body.String())
	require.Equal(t, "alice", client.usernameRequest.GetUsername())
	usernameWithoutHost := performUsersShowRequest(router, "/users/show", `{"username":"alice"}`, "")
	require.Equal(t, stdhttp.StatusOK, usernameWithoutHost.Code, usernameWithoutHost.Body.String())
}

func TestUsersListCompatReturnsActiveLocalUsers(t *testing.T) {
	gin.SetMode(gin.TestMode)
	client := newUsersShowCompatClient()
	h := NewHandler(&clients.Clients{User: client}, "Authorization", "Bearer", testJWTSecret)
	router := gin.New()
	NewInitControllers(h)(router)

	for _, path := range []string{"/users", "/api/users", "/api/v1/users"} {
		t.Run(path, func(t *testing.T) {
			response := performUsersShowRequest(router, path, `{"limit":2,"offset":0,"sort":"-createdAt","state":"alive","origin":"local","hostname":null}`, "")
			require.Equal(t, stdhttp.StatusOK, response.Code, response.Body.String())
			var payload []map[string]any
			require.NoError(t, json.Unmarshal(response.Body.Bytes(), &payload))
			require.Len(t, payload, 1)
			require.Equal(t, "42", payload[0]["id"])
			require.Equal(t, int32(1), client.listRequest.GetPage())
			require.Equal(t, int32(2), client.listRequest.GetPageSize())
			require.Equal(t, "-createdAt", client.listRequest.GetSort())
			require.Equal(t, userStatusActive, client.listRequest.GetStatus())
		})
	}

	offset := performUsersShowRequest(router, "/users", `{"limit":5,"offset":10}`, "")
	require.Equal(t, stdhttp.StatusOK, offset.Code, offset.Body.String())
	require.Equal(t, int32(3), client.listRequest.GetPage())

	pageOffset := performUsersShowRequest(router, "/users", `{"limit":2,"offset":1}`, "")
	require.Equal(t, stdhttp.StatusOK, pageOffset.Code, pageOffset.Body.String())
	var pageOffsetPayload []map[string]any
	require.NoError(t, json.Unmarshal(pageOffset.Body.Bytes(), &pageOffsetPayload))
	require.Empty(t, pageOffsetPayload)
}

func TestUsersListCompatRejectsRemoteAndUnknownSort(t *testing.T) {
	client := newUsersShowCompatClient()
	h := NewHandler(&clients.Clients{User: client}, "Authorization", "Bearer", testJWTSecret)
	router := gin.New()
	NewInitControllers(h)(router)
	for _, body := range []string{`{"origin":"remote"}`, `{"hostname":"example.com"}`, `{"sort":"bad"}`} {
		response := performUsersShowRequest(router, "/users", body, "")
		require.Equal(t, stdhttp.StatusBadRequest, response.Code, response.Body.String())
		require.Contains(t, response.Body.String(), "INVALID_PARAM")
	}
}

func TestCurrentUserCompatRequiresReadScopeAndReturnsBareUser(t *testing.T) {
	client := newUsersShowCompatClient()
	h := NewHandler(&clients.Clients{User: client}, "Authorization", "Bearer", testJWTSecret)
	router := gin.New()
	NewInitControllers(h)(router)
	token := signedAuthToken(t, jwt.MapClaims{
		"sub": "42", "jti": "user-show-read", "exp": time.Now().Add(time.Hour).Unix(),
		credentialVersionClaim: credentialVersionInitial, "token_type": apiTokenType, "scopes": []string{"read"},
	})
	response := performUsersShowRequest(router, "/api/v1/i", `{}`, token)
	require.Equal(t, stdhttp.StatusOK, response.Code, response.Body.String())
	var payload map[string]any
	require.NoError(t, json.Unmarshal(response.Body.Bytes(), &payload))
	require.Equal(t, "42", payload["id"])
	require.NotContains(t, payload, "data")
}

func TestUsersShowCompatRejectsUnknownFieldsAndMissingUser(t *testing.T) {
	client := newUsersShowCompatClient()
	h := NewHandler(&clients.Clients{User: client}, "Authorization", "Bearer", testJWTSecret)
	router := gin.New()
	NewInitControllers(h)(router)

	unknown := performUsersShowRequest(router, "/users/show", `{"userId":"77","unexpected":true}`, "")
	require.Equal(t, stdhttp.StatusBadRequest, unknown.Code, unknown.Body.String())
	require.Contains(t, unknown.Body.String(), "INVALID_PARAM")

	missing := performUsersShowRequest(router, "/users/show", `{"userId":"123"}`, "")
	require.Equal(t, stdhttp.StatusBadRequest, missing.Code, missing.Body.String())
	require.Contains(t, missing.Body.String(), "NO_SUCH_USER")
	require.Contains(t, missing.Body.String(), usersShowNoSuchUserID)
}

func performUsersShowRequest(router stdhttp.Handler, path, body, token string) *httptest.ResponseRecorder {
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(stdhttp.MethodPost, path, bytes.NewBufferString(body))
	request.Header.Set("Content-Type", "application/json")
	if token != "" {
		request.Header.Set("Authorization", "Bearer "+token)
	}
	router.ServeHTTP(recorder, request)
	return recorder
}

type usersShowCompatClient struct {
	userpb.UserServiceClient
	listRequest     *userpb.ListUsersRequest
	users           map[int64]*userpb.UserInfo
	usernameRequest *userpb.UsernameRequest
}

func newUsersShowCompatClient() *usersShowCompatClient {
	return &usersShowCompatClient{users: map[int64]*userpb.UserInfo{
		42: {Id: 42, Username: "viewer", Nickname: "Viewer", Status: userStatusActive, AccountState: "active", CreatedAt: 1690000000000},
		77: {Id: 77, Username: "alice", Nickname: "Alice", CreatedAt: 1700000000000},
		99: {Id: 99, Username: "bob", Nickname: "Bob", CreatedAt: 1710000000000},
	}}
}

func (client *usersShowCompatClient) GetUser(_ context.Context, request *userpb.UserIDRequest, _ ...grpc.CallOption) (*userpb.UserResponse, error) {
	user := client.users[request.GetId()]
	if user == nil {
		return nil, status.Error(codes.NotFound, "user not found")
	}
	return &userpb.UserResponse{Success: true, User: user}, nil
}

func (client *usersShowCompatClient) GetUserByUsername(_ context.Context, request *userpb.UsernameRequest, _ ...grpc.CallOption) (*userpb.UserResponse, error) {
	client.usernameRequest = request
	for _, user := range client.users {
		if user.GetUsername() == request.GetUsername() {
			return &userpb.UserResponse{Success: true, User: user}, nil
		}
	}
	return nil, status.Error(codes.NotFound, "user not found")
}

func (client *usersShowCompatClient) ListUsers(_ context.Context, request *userpb.ListUsersRequest, _ ...grpc.CallOption) (*userpb.UserListResponse, error) {
	client.listRequest = request
	items := make([]*userpb.UserInfo, 0, len(request.GetIds()))
	if len(request.GetIds()) > 0 {
		for _, id := range request.GetIds() {
			if user := client.users[id]; user != nil {
				items = append(items, user)
			}
		}
	}
	if request.GetStatus() == userStatusActive && len(request.GetIds()) == 0 {
		items = append(items, client.users[42])
	}
	return &userpb.UserListResponse{Items: items, Total: int64(len(items))}, nil
}

var _ clients.UserClient = (*usersShowCompatClient)(nil)
