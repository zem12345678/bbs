package http

import (
	"context"
	"encoding/json"
	stdhttp "net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"api-gateway/api/proto/chatpb"
	"api-gateway/api/proto/userpb"
	"api-gateway/internal/clients"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func TestListChatRoomMembersBindsRequesterAndHydratesMembers(t *testing.T) {
	gin.SetMode(gin.TestMode)
	chatClient := &chatMemberHTTPClient{listResponse: &chatpb.RoomMemberListResponse{
		Items: []*chatpb.Membership{
			{RoomId: 9, UserId: 42, Role: 3, Status: 1, MutedUntil: chatPermanentMuteUntilMilli},
			{RoomId: 9, UserId: 99, Role: 2, Status: 1},
		},
		Total: 2,
	}}
	userClient := &chatMemberUserClient{users: []*userpb.UserInfo{
		{Id: 42, Username: "alice", Nickname: "Alice Cooper", Email: "private@example.test", AvatarUrl: "/alice.png", Bio: "hello", FollowerCount: 8},
		{Id: 99, Username: "bob", Nickname: "Bob"},
		{Id: 777, Username: "cooper-outsider", Nickname: "Not a member"},
	}}
	router := newChatMemberTestRouter(chatClient, userClient)

	recorder := httptest.NewRecorder()
	req := httptest.NewRequest(stdhttp.MethodGet, "/api/v1/chat/rooms/ABCD1234/members?limit=25&offset=5&role=manager&user_id=42", nil)
	req.Header.Set("Authorization", "Bearer "+signedAuthToken(t, jwt.MapClaims{"sub": "7"}))
	router.ServeHTTP(recorder, req)

	require.Equal(t, stdhttp.StatusOK, recorder.Code, recorder.Body.String())
	require.Equal(t, "ABCD1234", chatClient.listRequest.GetRoomNo())
	require.EqualValues(t, 7, chatClient.listRequest.GetRequesterId())
	require.EqualValues(t, 25, chatClient.listRequest.GetLimit())
	require.EqualValues(t, 5, chatClient.listRequest.GetOffset())
	require.EqualValues(t, 3, chatClient.listRequest.GetRole())
	require.EqualValues(t, 42, chatClient.listRequest.GetUserId())
	require.Equal(t, []int64{42, 99}, userClient.request.GetIds())
	require.Empty(t, userClient.request.GetQuery())
	require.NotContains(t, recorder.Body.String(), "private@example.test")
	require.NotContains(t, recorder.Body.String(), "cooper-outsider")

	var envelope struct {
		Data chatRoomMemberListResponse `json:"data"`
	}
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &envelope))
	require.EqualValues(t, 2, envelope.Data.Total)
	require.EqualValues(t, 25, envelope.Data.Limit)
	require.EqualValues(t, 5, envelope.Data.Offset)
	require.Len(t, envelope.Data.Items, 2)
	item := envelope.Data.Items[0]
	require.Equal(t, "42", item.UserID)
	require.EqualValues(t, 3, item.Role)
	require.Equal(t, "manager", item.RoleName)
	require.NotNil(t, item.MutedUntil)
	require.Equal(t, "253402300799000", *item.MutedUntil)
	require.NotNil(t, item.User)
	require.Equal(t, "42", item.User.ID)
	require.Equal(t, "alice", item.User.Username)
	require.Equal(t, "Alice Cooper", item.User.Nickname)
}

func TestChatMembershipViewKeepsNumericRoleAndAddsGovernanceFields(t *testing.T) {
	view := chatMembershipViewFromProto(&chatpb.Membership{Role: 1, MutedUntil: chatPermanentMuteUntilMilli})
	require.EqualValues(t, 1, view.Role)
	require.Equal(t, "owner", view.RoleName)
	require.NotNil(t, view.MutedUntil)
	require.Equal(t, "253402300799000", *view.MutedUntil)

	unmuted := chatMembershipViewFromProto(&chatpb.Membership{Role: 2})
	require.Equal(t, "member", unmuted.RoleName)
	require.Nil(t, unmuted.MutedUntil)
}

func TestChatRoomMemberRoutesRequireAuthentication(t *testing.T) {
	gin.SetMode(gin.TestMode)
	chatClient := &chatMemberHTTPClient{}
	router := newChatMemberTestRouter(chatClient, &chatMemberUserClient{})

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, httptest.NewRequest(stdhttp.MethodGet, "/api/v1/chat/rooms/ABCD1234/members", nil))

	require.Equal(t, stdhttp.StatusUnauthorized, recorder.Code, recorder.Body.String())
	require.Zero(t, chatClient.totalCalls())
}

func TestChatRoomMemberRoutesRejectInvalidInputsBeforeRPC(t *testing.T) {
	gin.SetMode(gin.TestMode)
	tests := []struct {
		name   string
		method string
		path   string
		body   string
	}{
		{name: "limit", method: stdhttp.MethodGet, path: "/api/v1/chat/rooms/ABCD1234/members?limit=101"},
		{name: "offset", method: stdhttp.MethodGet, path: "/api/v1/chat/rooms/ABCD1234/members?offset=-1"},
		{name: "role filter", method: stdhttp.MethodGet, path: "/api/v1/chat/rooms/ABCD1234/members?role=moderator"},
		{name: "numeric role filter", method: stdhttp.MethodGet, path: "/api/v1/chat/rooms/ABCD1234/members?role=3"},
		{name: "user filter", method: stdhttp.MethodGet, path: "/api/v1/chat/rooms/ABCD1234/members?user_id=0"},
		{name: "target", method: stdhttp.MethodPut, path: "/api/v1/chat/rooms/ABCD1234/members/not-a-number/role", body: `{"role":"manager"}`},
		{name: "owner role", method: stdhttp.MethodPut, path: "/api/v1/chat/rooms/ABCD1234/members/9/role", body: `{"role":"owner"}`},
		{name: "expired mute", method: stdhttp.MethodPut, path: "/api/v1/chat/rooms/ABCD1234/members/9/mute", body: `{"expires_at":1}`},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			chatClient := &chatMemberHTTPClient{}
			router := newChatMemberTestRouter(chatClient, &chatMemberUserClient{})
			recorder := httptest.NewRecorder()
			req := httptest.NewRequest(test.method, test.path, strings.NewReader(test.body))
			req.Header.Set("Authorization", "Bearer "+signedAuthToken(t, jwt.MapClaims{"sub": "7"}))
			router.ServeHTTP(recorder, req)

			require.Equal(t, stdhttp.StatusBadRequest, recorder.Code, recorder.Body.String())
			require.Zero(t, chatClient.totalCalls())
		})
	}
}

func TestChatRoomMemberMutationsBindActorAndTarget(t *testing.T) {
	gin.SetMode(gin.TestMode)
	const targetID int64 = 9007199254740993
	chatClient := &chatMemberHTTPClient{}
	router := newChatMemberTestRouter(chatClient, &chatMemberUserClient{})
	token := "Bearer " + signedAuthToken(t, jwt.MapClaims{"sub": "7"})

	roleRecorder := httptest.NewRecorder()
	roleRequest := httptest.NewRequest(stdhttp.MethodPut, "/api/v1/chat/rooms/ABCD1234/members/9007199254740993/role", strings.NewReader(`{"role":"manager","actor_id":999}`))
	roleRequest.Header.Set("Authorization", token)
	router.ServeHTTP(roleRecorder, roleRequest)
	require.Equal(t, stdhttp.StatusOK, roleRecorder.Code, roleRecorder.Body.String())
	require.EqualValues(t, 7, chatClient.roleRequest.GetActorId())
	require.Equal(t, targetID, chatClient.roleRequest.GetUserId())
	require.EqualValues(t, 3, chatClient.roleRequest.GetRole())

	muteRecorder := httptest.NewRecorder()
	muteRequest := httptest.NewRequest(stdhttp.MethodPut, "/api/v1/chat/rooms/ABCD1234/members/9007199254740993/mute", strings.NewReader(`{"expires_at":null,"actor_id":999}`))
	muteRequest.Header.Set("Authorization", token)
	router.ServeHTTP(muteRecorder, muteRequest)
	require.Equal(t, stdhttp.StatusOK, muteRecorder.Code, muteRecorder.Body.String())
	require.EqualValues(t, 7, chatClient.muteRequest.GetActorId())
	require.Equal(t, targetID, chatClient.muteRequest.GetUserId())
	require.Equal(t, chatPermanentMuteUntilMilli, chatClient.muteRequest.GetMutedUntil())

	omittedMuteRecorder := httptest.NewRecorder()
	omittedMuteRequest := httptest.NewRequest(stdhttp.MethodPut, "/api/v1/chat/rooms/ABCD1234/members/9007199254740993/mute", strings.NewReader(`{}`))
	omittedMuteRequest.Header.Set("Authorization", token)
	router.ServeHTTP(omittedMuteRecorder, omittedMuteRequest)
	require.Equal(t, stdhttp.StatusOK, omittedMuteRecorder.Code, omittedMuteRecorder.Body.String())
	require.Equal(t, chatPermanentMuteUntilMilli, chatClient.muteRequest.GetMutedUntil())

	unmuteRecorder := httptest.NewRecorder()
	unmuteRequest := httptest.NewRequest(stdhttp.MethodDelete, "/api/v1/chat/rooms/ABCD1234/members/9007199254740993/mute", nil)
	unmuteRequest.Header.Set("Authorization", token)
	router.ServeHTTP(unmuteRecorder, unmuteRequest)
	require.Equal(t, stdhttp.StatusOK, unmuteRecorder.Code, unmuteRecorder.Body.String())
	require.EqualValues(t, 7, chatClient.unmuteRequest.GetActorId())
	require.Equal(t, targetID, chatClient.unmuteRequest.GetUserId())
	require.Contains(t, unmuteRecorder.Body.String(), `"user_id":"9007199254740993"`)
	require.Contains(t, unmuteRecorder.Body.String(), `"role_name":"member"`)
	require.Contains(t, unmuteRecorder.Body.String(), `"muted_until":null`)
}

func TestUpdateChatRoomMemberRoleMapsPermissionDenied(t *testing.T) {
	gin.SetMode(gin.TestMode)
	chatClient := &chatMemberHTTPClient{roleErr: status.Error(codes.PermissionDenied, "only the room owner can manage roles")}
	router := newChatMemberTestRouter(chatClient, &chatMemberUserClient{})

	recorder := httptest.NewRecorder()
	req := httptest.NewRequest(stdhttp.MethodPut, "/api/v1/chat/rooms/ABCD1234/members/9/role", strings.NewReader(`{"role":"manager"}`))
	req.Header.Set("Authorization", "Bearer "+signedAuthToken(t, jwt.MapClaims{"sub": "7"}))
	router.ServeHTTP(recorder, req)

	require.Equal(t, stdhttp.StatusForbidden, recorder.Code, recorder.Body.String())
	require.Contains(t, recorder.Body.String(), "only the room owner can manage roles")
	require.EqualValues(t, 7, chatClient.roleRequest.GetActorId())
}

func newChatMemberTestRouter(chatClient chatpb.ChatServiceClient, userClient userpb.UserServiceClient) *gin.Engine {
	h := NewHandler(&clients.Clients{Chat: chatClient, User: userClient}, "Authorization", "Bearer", testJWTSecret)
	router := gin.New()
	NewInitControllers(h)(router)
	return router
}

type chatMemberHTTPClient struct {
	chatpb.ChatServiceClient
	listRequest   *chatpb.ListRoomMembersRequest
	listResponse  *chatpb.RoomMemberListResponse
	listErr       error
	roleRequest   *chatpb.UpdateRoomMemberRoleRequest
	roleErr       error
	muteRequest   *chatpb.MuteRoomMemberRequest
	muteErr       error
	unmuteRequest *chatpb.UnmuteRoomMemberRequest
	unmuteErr     error
}

func (c *chatMemberHTTPClient) ListRoomMembers(_ context.Context, request *chatpb.ListRoomMembersRequest, _ ...grpc.CallOption) (*chatpb.RoomMemberListResponse, error) {
	c.listRequest = request
	if c.listErr != nil {
		return nil, c.listErr
	}
	if c.listResponse == nil {
		return &chatpb.RoomMemberListResponse{}, nil
	}
	return c.listResponse, nil
}

func (c *chatMemberHTTPClient) UpdateRoomMemberRole(_ context.Context, request *chatpb.UpdateRoomMemberRoleRequest, _ ...grpc.CallOption) (*chatpb.MembershipResponse, error) {
	c.roleRequest = request
	if c.roleErr != nil {
		return nil, c.roleErr
	}
	return &chatpb.MembershipResponse{Membership: &chatpb.Membership{RoomId: 9, UserId: request.GetUserId(), Role: request.GetRole(), Status: 1}}, nil
}

func (c *chatMemberHTTPClient) MuteRoomMember(_ context.Context, request *chatpb.MuteRoomMemberRequest, _ ...grpc.CallOption) (*chatpb.MembershipResponse, error) {
	c.muteRequest = request
	if c.muteErr != nil {
		return nil, c.muteErr
	}
	return &chatpb.MembershipResponse{Membership: &chatpb.Membership{RoomId: 9, UserId: request.GetUserId(), Role: 2, Status: 1, MutedUntil: request.GetMutedUntil()}}, nil
}

func (c *chatMemberHTTPClient) UnmuteRoomMember(_ context.Context, request *chatpb.UnmuteRoomMemberRequest, _ ...grpc.CallOption) (*chatpb.MembershipResponse, error) {
	c.unmuteRequest = request
	if c.unmuteErr != nil {
		return nil, c.unmuteErr
	}
	return &chatpb.MembershipResponse{Membership: &chatpb.Membership{RoomId: 9, UserId: request.GetUserId(), Role: 2, Status: 1}}, nil
}

func (c *chatMemberHTTPClient) totalCalls() int {
	count := 0
	if c.listRequest != nil {
		count++
	}
	if c.roleRequest != nil {
		count++
	}
	if c.muteRequest != nil {
		count++
	}
	if c.unmuteRequest != nil {
		count++
	}
	return count
}

type chatMemberUserClient struct {
	userpb.UserServiceClient
	request *userpb.ListUsersRequest
	users   []*userpb.UserInfo
	err     error
}

func (c *chatMemberUserClient) ListUsers(_ context.Context, request *userpb.ListUsersRequest, _ ...grpc.CallOption) (*userpb.UserListResponse, error) {
	c.request = request
	if c.err != nil {
		return nil, c.err
	}
	return &userpb.UserListResponse{Items: c.users, Total: int64(len(c.users))}, nil
}
