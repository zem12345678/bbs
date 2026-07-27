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
)

func TestCreateChatRoomBindsAuthenticatedUserAndHydratesOnce(t *testing.T) {
	gin.SetMode(gin.TestMode)
	const roomID int64 = 9223372036854770000
	const userID int64 = 9223372036854770001
	chatClient := &chatHTTPClient{createRoomResponse: &chatpb.RoomDetailsResponse{
		Details: &chatpb.RoomDetails{
			Room: &chatpb.Room{
				Id: roomID, RoomNo: "ABCD1234", Name: "Lounge", CreatorId: userID,
				AnnouncementVersion: 9223372036854770002, LastMessageSeq: 9223372036854770003,
			},
			Membership: &chatpb.Membership{
				RoomId: roomID, UserId: userID, JoinedAtSeq: 9223372036854770002,
				LastReadSeq: 9223372036854770003,
			},
		},
	}}
	userClient := &chatHTTPUserClient{users: []*userpb.UserInfo{{
		Id: userID, Username: "alice", Nickname: "Alice", Email: "private@example.test", AvatarUrl: "/alice.png",
	}}}
	h := NewHandler(&clients.Clients{Chat: chatClient, User: userClient}, "Authorization", "Bearer", testJWTSecret)
	router := gin.New()
	NewInitControllers(h)(router)

	recorder := httptest.NewRecorder()
	req := httptest.NewRequest(stdhttp.MethodPost, "/api/v1/chat/rooms", strings.NewReader(`{"name":"Lounge","user_id":999}`))
	req.Header.Set("Authorization", "Bearer "+signedAuthToken(t, jwt.MapClaims{"sub": "9223372036854770001"}))
	router.ServeHTTP(recorder, req)

	require.Equal(t, stdhttp.StatusOK, recorder.Code, recorder.Body.String())
	require.Equal(t, userID, chatClient.createRoomRequest.GetCreatorId())
	require.Equal(t, []int64{userID}, userClient.request.GetIds())
	require.Equal(t, 1, userClient.calls)
	require.NotContains(t, recorder.Body.String(), "private@example.test")
	var envelope struct {
		Data chatRoomDetailsResponse `json:"data"`
	}
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &envelope))
	require.Len(t, envelope.Data.Users, 1)
	require.Equal(t, "alice", envelope.Data.Users[0].Username)
	require.Equal(t, "9223372036854770000", envelope.Data.Details.Room.ID)
	require.Equal(t, "9223372036854770001", envelope.Data.Details.Room.CreatorID)
	require.Equal(t, "9223372036854770003", envelope.Data.Details.Room.LastMessageSeq)
	require.Equal(t, "9223372036854770000", envelope.Data.Details.Membership.RoomID)
	require.Equal(t, "9223372036854770003", envelope.Data.Details.Membership.LastReadSeq)
	require.Equal(t, "9223372036854770001", envelope.Data.Users[0].ID)
}

func TestCreateChatRoomReturnsRateLimitedBeforeRPC(t *testing.T) {
	gin.SetMode(gin.TestMode)
	chatClient := &chatHTTPClient{}
	limiter := &chatRateLimiterStub{limited: true}
	h := NewHandler(&clients.Clients{Chat: chatClient}, "Authorization", "Bearer", testJWTSecret)
	h.SetChatCreateRoomLimit(limiter)
	router := gin.New()
	NewInitControllers(h)(router)

	recorder := httptest.NewRecorder()
	req := httptest.NewRequest(stdhttp.MethodPost, "/api/v1/chat/rooms", strings.NewReader(`{"name":"Lounge"}`))
	req.Header.Set("Authorization", "Bearer "+signedAuthToken(t, jwt.MapClaims{"sub": "42"}))
	router.ServeHTTP(recorder, req)

	require.Equal(t, stdhttp.StatusTooManyRequests, recorder.Code, recorder.Body.String())
	require.Contains(t, recorder.Body.String(), `"legacy_code":"rate_limited"`)
	require.Equal(t, []string{"rate:chat:create-room:42"}, limiter.keys)
	require.Nil(t, chatClient.createRoomRequest)
}

func TestChatSidebarHydratesDuplicateUsersWithOneLookup(t *testing.T) {
	gin.SetMode(gin.TestMode)
	chatClient := &chatHTTPClient{sidebarResponse: &chatpb.SidebarResponse{
		Rooms: []*chatpb.SidebarRoom{
			{Room: &chatpb.Room{CreatorId: 42}, LastMessage: &chatpb.ChatMessage{SenderId: 7}},
			{Room: &chatpb.Room{CreatorId: 7}, LastMessage: &chatpb.ChatMessage{SenderId: 42}},
		},
	}}
	userClient := &chatHTTPUserClient{users: []*userpb.UserInfo{
		{Id: 42, Username: "alice"}, {Id: 7, Username: "bob"},
	}}
	h := NewHandler(&clients.Clients{Chat: chatClient, User: userClient}, "Authorization", "Bearer", testJWTSecret)
	router := gin.New()
	NewInitControllers(h)(router)

	recorder := httptest.NewRecorder()
	req := httptest.NewRequest(stdhttp.MethodGet, "/api/v1/chat/sidebar", nil)
	req.Header.Set("Authorization", "Bearer "+signedAuthToken(t, jwt.MapClaims{"sub": "42"}))
	router.ServeHTTP(recorder, req)

	require.Equal(t, stdhttp.StatusOK, recorder.Code, recorder.Body.String())
	require.Equal(t, 1, userClient.calls)
	require.Equal(t, []int64{42, 7}, userClient.request.GetIds())
	var envelope struct {
		Data chatSidebarResponse `json:"data"`
	}
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &envelope))
	require.Len(t, envelope.Data.Users, 2)
}

func TestChatMessagesRejectMutuallyExclusivePaginationBeforeRPC(t *testing.T) {
	gin.SetMode(gin.TestMode)
	chatClient := &chatHTTPClient{}
	h := NewHandler(&clients.Clients{Chat: chatClient}, "Authorization", "Bearer", testJWTSecret)
	router := gin.New()
	NewInitControllers(h)(router)

	recorder := httptest.NewRecorder()
	req := httptest.NewRequest(stdhttp.MethodGet, "/api/v1/chat/rooms/ABCD1234/messages?before_seq=3&after=1", nil)
	req.Header.Set("Authorization", "Bearer "+signedAuthToken(t, jwt.MapClaims{"sub": "42"}))
	router.ServeHTTP(recorder, req)

	require.Equal(t, stdhttp.StatusBadRequest, recorder.Code, recorder.Body.String())
	require.Equal(t, 0, chatClient.listMessagesCalls)
}

func TestChatMessagesForwardAnchorAndHydrateSendersOnce(t *testing.T) {
	gin.SetMode(gin.TestMode)
	const messageID int64 = 9223372036854770000
	const latestSeq int64 = 9223372036854770003
	chatClient := &chatHTTPClient{listMessagesResponse: &chatpb.MessagePageResponse{
		Messages: []*chatpb.ChatMessage{
			{Id: messageID, RoomId: 9223372036854770001, SenderId: 7, Seq: 9223372036854770002},
			{Id: 2, SenderId: 42, Seq: 9},
			{Id: 3, SenderId: 7, Seq: 10},
		},
		LatestSeq: latestSeq,
		AnchorSeq: 8,
	}}
	userClient := &chatHTTPUserClient{users: []*userpb.UserInfo{
		{Id: 7, Username: "bob"}, {Id: 42, Username: "alice"},
	}}
	h := NewHandler(&clients.Clients{Chat: chatClient, User: userClient}, "Authorization", "Bearer", testJWTSecret)
	router := gin.New()
	NewInitControllers(h)(router)

	recorder := httptest.NewRecorder()
	req := httptest.NewRequest(stdhttp.MethodGet, "/api/v1/chat/rooms/ABCD1234/messages?anchor_seq=8&before=10&after=20", nil)
	req.Header.Set("Authorization", "Bearer "+signedAuthToken(t, jwt.MapClaims{"sub": "42"}))
	router.ServeHTTP(recorder, req)

	require.Equal(t, stdhttp.StatusOK, recorder.Code, recorder.Body.String())
	require.Equal(t, int64(42), chatClient.listMessagesRequest.GetUserId())
	require.Equal(t, int64(8), chatClient.listMessagesRequest.GetAnchorSeq())
	require.Equal(t, int32(10), chatClient.listMessagesRequest.GetBefore())
	require.Equal(t, int32(20), chatClient.listMessagesRequest.GetAfter())
	require.Equal(t, 1, userClient.calls)
	require.Equal(t, []int64{7, 42}, userClient.request.GetIds())
	var envelope struct {
		Data chatMessagePageResponse `json:"data"`
	}
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &envelope))
	require.Equal(t, "9223372036854770000", envelope.Data.Messages[0].ID)
	require.Equal(t, "9223372036854770001", envelope.Data.Messages[0].RoomID)
	require.Equal(t, "9223372036854770002", envelope.Data.Messages[0].Seq)
	require.Equal(t, "9223372036854770003", envelope.Data.LatestSeq)
}

func TestChatMessagesPreserveZeroAfterSequence(t *testing.T) {
	gin.SetMode(gin.TestMode)
	chatClient := &chatHTTPClient{listMessagesResponse: &chatpb.MessagePageResponse{}}
	h := NewHandler(&clients.Clients{Chat: chatClient}, "Authorization", "Bearer", testJWTSecret)
	router := gin.New()
	NewInitControllers(h)(router)

	recorder := httptest.NewRecorder()
	req := httptest.NewRequest(stdhttp.MethodGet, "/api/v1/chat/rooms/ABCD1234/messages?after_seq=0&limit=100", nil)
	req.Header.Set("Authorization", "Bearer "+signedAuthToken(t, jwt.MapClaims{"sub": "42"}))
	router.ServeHTTP(recorder, req)

	require.Equal(t, stdhttp.StatusOK, recorder.Code, recorder.Body.String())
	require.NotNil(t, chatClient.listMessagesRequest.AfterSeq)
	require.Equal(t, int64(0), chatClient.listMessagesRequest.GetAfterSeq())
	require.Equal(t, int32(100), chatClient.listMessagesRequest.GetLimit())
}

func TestLookupChatRoomReturnsSafePreview(t *testing.T) {
	gin.SetMode(gin.TestMode)
	chatClient := &chatHTTPClient{lookupRoomResponse: &chatpb.RoomDetailsResponse{Details: &chatpb.RoomDetails{
		Room: &chatpb.Room{
			Id: 9, RoomNo: "ABCD1234", Name: "Lounge", CreatorId: 7,
			Announcement: "members only announcement", Status: 1,
		},
		MemberCount: 3,
	}}}
	h := NewHandler(&clients.Clients{Chat: chatClient}, "Authorization", "Bearer", testJWTSecret)
	router := gin.New()
	NewInitControllers(h)(router)

	recorder := httptest.NewRecorder()
	req := httptest.NewRequest(stdhttp.MethodGet, "/api/v1/chat/rooms/lookup?room_no=abcd1234", nil)
	req.Header.Set("Authorization", "Bearer "+signedAuthToken(t, jwt.MapClaims{"sub": "42"}))
	router.ServeHTTP(recorder, req)

	require.Equal(t, stdhttp.StatusOK, recorder.Code, recorder.Body.String())
	require.Equal(t, int64(42), chatClient.lookupRoomRequest.GetUserId())
	require.NotContains(t, recorder.Body.String(), "members only announcement")
	require.NotContains(t, recorder.Body.String(), "creator_id")
	var envelope struct {
		Data chatRoomPreviewResponse `json:"data"`
	}
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &envelope))
	require.Equal(t, "3", envelope.Data.MemberCount)
	require.False(t, envelope.Data.Joined)
}

func TestChatRoutesRequireAuthentication(t *testing.T) {
	gin.SetMode(gin.TestMode)
	chatClient := &chatHTTPClient{}
	h := NewHandler(&clients.Clients{Chat: chatClient}, "Authorization", "Bearer", testJWTSecret)
	router := gin.New()
	NewInitControllers(h)(router)

	recorder := httptest.NewRecorder()
	req := httptest.NewRequest(stdhttp.MethodGet, "/api/v1/chat/sidebar", nil)
	router.ServeHTTP(recorder, req)

	require.Equal(t, stdhttp.StatusUnauthorized, recorder.Code, recorder.Body.String())
	require.Equal(t, 0, chatClient.sidebarCalls)
}

func TestMoveChatGroupBindsAuthenticatedUserAndDirection(t *testing.T) {
	gin.SetMode(gin.TestMode)
	chatClient := &chatHTTPClient{}
	h := NewHandler(&clients.Clients{Chat: chatClient}, "Authorization", "Bearer", testJWTSecret)
	router := gin.New()
	NewInitControllers(h)(router)

	recorder := httptest.NewRecorder()
	req := httptest.NewRequest(stdhttp.MethodPost, "/api/v1/chat/groups/900/move", strings.NewReader(`{"direction":-1}`))
	req.Header.Set("Authorization", "Bearer "+signedAuthToken(t, jwt.MapClaims{"sub": "42"}))
	router.ServeHTTP(recorder, req)

	require.Equal(t, stdhttp.StatusOK, recorder.Code, recorder.Body.String())
	require.Equal(t, int64(42), chatClient.moveGroupRequest.GetUserId())
	require.Equal(t, int64(900), chatClient.moveGroupRequest.GetGroupId())
	require.Equal(t, int32(-1), chatClient.moveGroupRequest.GetDirection())
}

func TestMoveChatGroupRequiresAuthentication(t *testing.T) {
	gin.SetMode(gin.TestMode)
	chatClient := &chatHTTPClient{}
	h := NewHandler(&clients.Clients{Chat: chatClient}, "Authorization", "Bearer", testJWTSecret)
	router := gin.New()
	NewInitControllers(h)(router)

	recorder := httptest.NewRecorder()
	req := httptest.NewRequest(stdhttp.MethodPost, "/api/v1/chat/groups/900/move", strings.NewReader(`{"direction":1}`))
	router.ServeHTTP(recorder, req)

	require.Equal(t, stdhttp.StatusUnauthorized, recorder.Code, recorder.Body.String())
	require.Nil(t, chatClient.moveGroupRequest)
}

func TestMoveChatGroupRejectsInvalidDirectionBeforeRPC(t *testing.T) {
	gin.SetMode(gin.TestMode)
	chatClient := &chatHTTPClient{}
	h := NewHandler(&clients.Clients{Chat: chatClient}, "Authorization", "Bearer", testJWTSecret)
	router := gin.New()
	NewInitControllers(h)(router)

	recorder := httptest.NewRecorder()
	req := httptest.NewRequest(stdhttp.MethodPost, "/api/v1/chat/groups/900/move", strings.NewReader(`{"direction":0}`))
	req.Header.Set("Authorization", "Bearer "+signedAuthToken(t, jwt.MapClaims{"sub": "42"}))
	router.ServeHTTP(recorder, req)

	require.Equal(t, stdhttp.StatusBadRequest, recorder.Code, recorder.Body.String())
	require.Nil(t, chatClient.moveGroupRequest)
}

func TestSendChatMessageUsesAuthenticatedUserAndClientID(t *testing.T) {
	gin.SetMode(gin.TestMode)
	chatClient := &chatHTTPClient{sendMessageResponse: &chatpb.SendMessageResponse{
		Message: &chatpb.ChatMessage{Id: 1, SenderId: 42, Body: "hello"}, LatestSeq: 1,
	}}
	userClient := &chatHTTPUserClient{users: []*userpb.UserInfo{{Id: 42, Username: "alice"}}}
	h := NewHandler(&clients.Clients{Chat: chatClient, User: userClient}, "Authorization", "Bearer", testJWTSecret)
	router := gin.New()
	NewInitControllers(h)(router)

	recorder := httptest.NewRecorder()
	req := httptest.NewRequest(stdhttp.MethodPost, "/api/v1/chat/rooms/ABCD1234/messages", strings.NewReader(`{"client_message_id":"4c0a3f4b-0d6d-4e3a-8a8b-6b5944d4e3d1","body":"hello","user_id":999}`))
	req.Header.Set("Authorization", "Bearer "+signedAuthToken(t, jwt.MapClaims{"sub": "42"}))
	router.ServeHTTP(recorder, req)

	require.Equal(t, stdhttp.StatusOK, recorder.Code, recorder.Body.String())
	require.Equal(t, int64(42), chatClient.sendMessageRequest.GetUserId())
	require.Equal(t, "4c0a3f4b-0d6d-4e3a-8a8b-6b5944d4e3d1", chatClient.sendMessageRequest.GetClientMessageId())
	require.Equal(t, "hello", chatClient.sendMessageRequest.GetBody())
	require.Equal(t, 1, userClient.calls)
}

func TestDeleteChatMessageUsesAuthenticatedUserAndMessageID(t *testing.T) {
	gin.SetMode(gin.TestMode)
	const messageID int64 = 9223372036854770000
	chatClient := &chatHTTPClient{deleteMessageResponse: &chatpb.DeleteMessageResponse{
		Message: &chatpb.ChatMessage{Id: messageID, RoomId: 9, SenderId: 42, Seq: 7, Status: 2},
	}}
	h := NewHandler(&clients.Clients{Chat: chatClient}, "Authorization", "Bearer", testJWTSecret)
	router := gin.New()
	NewInitControllers(h)(router)

	recorder := httptest.NewRecorder()
	req := httptest.NewRequest(stdhttp.MethodDelete, "/api/v1/chat/rooms/ABCD1234/messages/9223372036854770000", nil)
	req.Header.Set("Authorization", "Bearer "+signedAuthToken(t, jwt.MapClaims{"sub": "42"}))
	router.ServeHTTP(recorder, req)

	require.Equal(t, stdhttp.StatusOK, recorder.Code, recorder.Body.String())
	require.Equal(t, int64(42), chatClient.deleteMessageRequest.GetUserId())
	require.Equal(t, "ABCD1234", chatClient.deleteMessageRequest.GetRoomNo())
	require.Equal(t, messageID, chatClient.deleteMessageRequest.GetMessageId())
	var envelope struct {
		Data chatDeleteMessageResponse `json:"data"`
	}
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &envelope))
	require.Equal(t, "9223372036854770000", envelope.Data.Message.ID)
	require.Equal(t, int32(2), envelope.Data.Message.Status)
}

func TestJoinChatRoomReturnsRateLimitedBeforeRPC(t *testing.T) {
	gin.SetMode(gin.TestMode)
	chatClient := &chatHTTPClient{}
	limiter := &chatRateLimiterStub{limited: true}
	h := NewHandlerWithRealtimeAndRateLimits(
		&clients.Clients{Chat: chatClient}, "Authorization", "Bearer", testJWTSecret,
		nil, nil, limiter, nil,
	)
	router := gin.New()
	NewInitControllers(h)(router)

	recorder := httptest.NewRecorder()
	req := httptest.NewRequest(stdhttp.MethodPost, "/api/v1/chat/rooms/ABCD1234/join", nil)
	req.Header.Set("Authorization", "Bearer "+signedAuthToken(t, jwt.MapClaims{"sub": "42"}))
	router.ServeHTTP(recorder, req)

	require.Equal(t, stdhttp.StatusTooManyRequests, recorder.Code, recorder.Body.String())
	require.Contains(t, recorder.Body.String(), `"legacy_code":"rate_limited"`)
	require.Equal(t, []string{"rate:chat:join:42"}, limiter.keys)
	require.Equal(t, 0, chatClient.joinRoomCalls)
}

func TestSendChatMessageReturnsRateLimitedBeforeRPC(t *testing.T) {
	gin.SetMode(gin.TestMode)
	chatClient := &chatHTTPClient{}
	limiter := &chatRateLimiterStub{limited: true}
	h := NewHandlerWithRealtimeAndRateLimits(
		&clients.Clients{Chat: chatClient}, "Authorization", "Bearer", testJWTSecret,
		nil, nil, nil, limiter,
	)
	router := gin.New()
	NewInitControllers(h)(router)

	recorder := httptest.NewRecorder()
	req := httptest.NewRequest(
		stdhttp.MethodPost,
		"/api/v1/chat/rooms/ABCD1234/messages",
		strings.NewReader(`{"client_message_id":"4c0a3f4b-0d6d-4e3a-8a8b-6b5944d4e3d1","body":"hello"}`),
	)
	req.Header.Set("Authorization", "Bearer "+signedAuthToken(t, jwt.MapClaims{"sub": "42"}))
	router.ServeHTTP(recorder, req)

	require.Equal(t, stdhttp.StatusTooManyRequests, recorder.Code, recorder.Body.String())
	require.Contains(t, recorder.Body.String(), `"legacy_code":"rate_limited"`)
	require.Equal(t, []string{"rate:chat:send:42"}, limiter.keys)
	require.Equal(t, 0, chatClient.sendMessageCalls)
}

func TestAdvanceChatReadReturnsRateLimitedBeforeRPC(t *testing.T) {
	gin.SetMode(gin.TestMode)
	chatClient := &chatHTTPClient{}
	limiter := &chatRateLimiterStub{limited: true}
	h := NewHandler(&clients.Clients{Chat: chatClient}, "Authorization", "Bearer", testJWTSecret)
	h.SetChatReadLimit(limiter)
	router := gin.New()
	NewInitControllers(h)(router)

	recorder := httptest.NewRecorder()
	req := httptest.NewRequest(
		stdhttp.MethodPut,
		"/api/v1/chat/rooms/ABCD1234/read",
		strings.NewReader(`{"read_seq":"1"}`),
	)
	req.Header.Set("Authorization", "Bearer "+signedAuthToken(t, jwt.MapClaims{"sub": "42"}))
	router.ServeHTTP(recorder, req)

	require.Equal(t, stdhttp.StatusTooManyRequests, recorder.Code, recorder.Body.String())
	require.Contains(t, recorder.Body.String(), `"legacy_code":"rate_limited"`)
	require.Equal(t, []string{"rate:chat:read:42"}, limiter.keys)
	require.Equal(t, 0, chatClient.advanceReadCalls)
}

type chatRateLimiterStub struct {
	limited bool
	err     error
	keys    []string
}

func (l *chatRateLimiterStub) Limit(_ context.Context, key string) (bool, error) {
	l.keys = append(l.keys, key)
	return l.limited, l.err
}

type chatHTTPUserClient struct {
	userpb.UserServiceClient
	users   []*userpb.UserInfo
	request *userpb.ListUsersRequest
	calls   int
}

func (c *chatHTTPUserClient) ListUsers(_ context.Context, request *userpb.ListUsersRequest, _ ...grpc.CallOption) (*userpb.UserListResponse, error) {
	c.calls++
	c.request = request
	return &userpb.UserListResponse{Items: c.users}, nil
}

type chatHTTPClient struct {
	chatpb.ChatServiceClient
	createRoomRequest     *chatpb.CreateRoomRequest
	createRoomResponse    *chatpb.RoomDetailsResponse
	lookupRoomRequest     *chatpb.LookupRoomRequest
	lookupRoomResponse    *chatpb.RoomDetailsResponse
	sidebarResponse       *chatpb.SidebarResponse
	sidebarCalls          int
	listMessagesRequest   *chatpb.ListMessagesRequest
	listMessagesResponse  *chatpb.MessagePageResponse
	listMessagesCalls     int
	joinRoomCalls         int
	sendMessageRequest    *chatpb.SendMessageRequest
	sendMessageResponse   *chatpb.SendMessageResponse
	sendMessageCalls      int
	deleteMessageRequest  *chatpb.DeleteMessageRequest
	deleteMessageResponse *chatpb.DeleteMessageResponse
	deleteMessageCalls    int
	advanceReadRequest    *chatpb.AdvanceReadRequest
	advanceReadResponse   *chatpb.AdvanceReadResponse
	advanceReadCalls      int
	moveGroupRequest      *chatpb.MoveGroupRequest
}

func (c *chatHTTPClient) CreateRoom(_ context.Context, request *chatpb.CreateRoomRequest, _ ...grpc.CallOption) (*chatpb.RoomDetailsResponse, error) {
	c.createRoomRequest = request
	return c.createRoomResponse, nil
}

func (c *chatHTTPClient) LookupRoom(_ context.Context, request *chatpb.LookupRoomRequest, _ ...grpc.CallOption) (*chatpb.RoomDetailsResponse, error) {
	c.lookupRoomRequest = request
	return c.lookupRoomResponse, nil
}

func (c *chatHTTPClient) JoinRoom(_ context.Context, _ *chatpb.JoinRoomRequest, _ ...grpc.CallOption) (*chatpb.RoomDetailsResponse, error) {
	c.joinRoomCalls++
	return &chatpb.RoomDetailsResponse{}, nil
}

func (c *chatHTTPClient) ListSidebar(_ context.Context, _ *chatpb.ListSidebarRequest, _ ...grpc.CallOption) (*chatpb.SidebarResponse, error) {
	c.sidebarCalls++
	if c.sidebarResponse == nil {
		return &chatpb.SidebarResponse{}, nil
	}
	return c.sidebarResponse, nil
}

func (c *chatHTTPClient) ListMessages(_ context.Context, request *chatpb.ListMessagesRequest, _ ...grpc.CallOption) (*chatpb.MessagePageResponse, error) {
	c.listMessagesCalls++
	c.listMessagesRequest = request
	if c.listMessagesResponse == nil {
		return &chatpb.MessagePageResponse{}, nil
	}
	return c.listMessagesResponse, nil
}

func (c *chatHTTPClient) SendMessage(_ context.Context, request *chatpb.SendMessageRequest, _ ...grpc.CallOption) (*chatpb.SendMessageResponse, error) {
	c.sendMessageCalls++
	c.sendMessageRequest = request
	if c.sendMessageResponse == nil {
		return &chatpb.SendMessageResponse{}, nil
	}
	return c.sendMessageResponse, nil
}

func (c *chatHTTPClient) DeleteMessage(_ context.Context, request *chatpb.DeleteMessageRequest, _ ...grpc.CallOption) (*chatpb.DeleteMessageResponse, error) {
	c.deleteMessageCalls++
	c.deleteMessageRequest = request
	if c.deleteMessageResponse == nil {
		return &chatpb.DeleteMessageResponse{}, nil
	}
	return c.deleteMessageResponse, nil
}

func (c *chatHTTPClient) AdvanceRead(_ context.Context, request *chatpb.AdvanceReadRequest, _ ...grpc.CallOption) (*chatpb.AdvanceReadResponse, error) {
	c.advanceReadCalls++
	c.advanceReadRequest = request
	if c.advanceReadResponse == nil {
		return &chatpb.AdvanceReadResponse{}, nil
	}
	return c.advanceReadResponse, nil
}

func (c *chatHTTPClient) MoveGroup(_ context.Context, request *chatpb.MoveGroupRequest, _ ...grpc.CallOption) (*chatpb.SimpleResponse, error) {
	c.moveGroupRequest = request
	return &chatpb.SimpleResponse{Success: true}, nil
}
