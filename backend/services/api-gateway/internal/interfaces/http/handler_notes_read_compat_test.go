package http

import (
	"context"
	"encoding/json"
	stdhttp "net/http"
	"testing"

	"api-gateway/api/proto/commentpb"
	"api-gateway/api/proto/contentpb"
	"api-gateway/api/proto/userpb"
	"api-gateway/internal/clients"

	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func TestNotesReadCompatListsGlobalFeaturedAndUserNotes(t *testing.T) {
	content := &notesCompatContentClient{
		topics: []*contentpb.TopicInfo{
			{Id: 201, Type: "tweet", Status: contentStatusPublished, AuthorId: 42, Body: "recent", CreatedAt: 1700000002000},
			{Id: 200, Type: "tweet", Status: contentStatusPublished, AuthorId: 43, Body: "older", CreatedAt: 1700000001000},
		},
	}
	users := &readCompatUserClient{users: map[int64]*userpb.UserInfo{
		42: {Id: 42, Username: "alice", Nickname: "Alice"},
		43: {Id: 43, Username: "bob", Nickname: "Bob"},
	}}
	h := NewHandler(&clients.Clients{Content: content, User: users}, "Authorization", "Bearer", testJWTSecret)
	router := newCompatRouter(h)

	global := performNotesCompatRequest(router, stdhttp.MethodPost, "/api/v1/notes/global-timeline", `{"limit":1}`, "")
	require.Equal(t, stdhttp.StatusOK, global.Code, global.Body.String())
	var globalItems []map[string]any
	require.NoError(t, json.Unmarshal(global.Body.Bytes(), &globalItems))
	require.Len(t, globalItems, 1)
	require.Equal(t, "201", globalItems[0]["id"])

	featured := performNotesCompatRequest(router, stdhttp.MethodPost, "/notes/featured", `{"limit":1}`, "")
	require.Equal(t, stdhttp.StatusOK, featured.Code, featured.Body.String())

	userFeatured := performNotesCompatRequest(router, stdhttp.MethodPost, "/users/featured-notes", `{"userId":"43","limit":1}`, "")
	require.Equal(t, stdhttp.StatusOK, userFeatured.Code, userFeatured.Body.String())
	var userItems []map[string]any
	require.NoError(t, json.Unmarshal(userFeatured.Body.Bytes(), &userItems))
	require.Len(t, userItems, 1)
	require.Equal(t, "200", userItems[0]["id"])
}

func TestNotesReadCompatMapsTopicCommentsToReplies(t *testing.T) {
	content := &notesCompatContentClient{topic: &contentpb.TopicInfo{Id: 202, Type: "tweet", Status: contentStatusPublished, AuthorId: 42}}
	users := &readCompatUserClient{users: map[int64]*userpb.UserInfo{43: {Id: 43, Username: "bob", Nickname: "Bob"}}}
	comments := &readCompatCommentClient{response: &commentpb.CommentListResponse{Items: []*commentpb.CommentInfo{{Id: 601, EntityType: "topic", EntityId: 202, AuthorId: 43, Content: "reply", Status: 1, CreatedAt: 1700000003000}}}}
	h := NewHandler(&clients.Clients{Content: content, User: users, Comment: comments}, "Authorization", "Bearer", testJWTSecret)
	router := newCompatRouter(h)

	recorder := performNotesCompatRequest(router, stdhttp.MethodPost, "/api/v1/notes/replies", `{"noteId":"202","limit":10}`, "")
	require.Equal(t, stdhttp.StatusOK, recorder.Code, recorder.Body.String())
	var items []map[string]any
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &items))
	require.Len(t, items, 1)
	require.Equal(t, "601", items[0]["id"])
	require.Equal(t, "reply", items[0]["text"])
}

type readCompatUserClient struct {
	userpb.UserServiceClient
	users map[int64]*userpb.UserInfo
}

func (c *readCompatUserClient) GetUser(_ context.Context, request *userpb.UserIDRequest, _ ...grpc.CallOption) (*userpb.UserResponse, error) {
	user := c.users[request.GetId()]
	if user == nil {
		return nil, status.Error(codes.NotFound, "user not found")
	}
	return &userpb.UserResponse{Success: true, User: user}, nil
}

func (c *readCompatUserClient) ListUsers(_ context.Context, request *userpb.ListUsersRequest, _ ...grpc.CallOption) (*userpb.UserListResponse, error) {
	items := make([]*userpb.UserInfo, 0, len(request.GetIds()))
	for _, id := range request.GetIds() {
		if user := c.users[id]; user != nil {
			items = append(items, user)
		}
	}
	return &userpb.UserListResponse{Items: items, Total: int64(len(items))}, nil
}

type readCompatCommentClient struct {
	commentpb.CommentServiceClient
	response *commentpb.CommentListResponse
}

func (c *readCompatCommentClient) ListComments(context.Context, *commentpb.ListCommentsRequest, ...grpc.CallOption) (*commentpb.CommentListResponse, error) {
	return c.response, nil
}
