package http

import (
	"context"
	"encoding/json"
	stdhttp "net/http"
	"testing"

	"api-gateway/api/proto/contentpb"
	"api-gateway/api/proto/reactionpb"
	"api-gateway/api/proto/userpb"
	"api-gateway/internal/clients"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func TestNoteReactionCompatCreatesAndDeletesRaw204(t *testing.T) {
	content := &notesCompatContentClient{topic: &contentpb.TopicInfo{Id: 100, Type: "tweet", Status: contentStatusPublished, AuthorId: 42, Body: "hello", CreatedAt: 1700000000000}}
	reaction := &compatReactionClient{}
	h := NewHandler(&clients.Clients{Content: content, Reaction: reaction, User: &notesCompatUserClient{}}, "Authorization", "Bearer", testJWTSecret)
	router := newCompatRouter(h)
	token := signedAuthToken(t, mapClaimsForNotesTest())

	created := performNotesCompatRequest(router, stdhttp.MethodPost, "/api/v1/notes/reactions/create", `{"noteId":"100","reaction":"👍"}`, token)
	require.Equal(t, stdhttp.StatusNoContent, created.Code, created.Body.String())
	require.Equal(t, int64(100), reaction.createRequest.GetEntity().GetEntityId())
	require.Equal(t, "👍", reaction.createRequest.GetReaction())

	deleted := performNotesCompatRequest(router, stdhttp.MethodPost, "/notes/reactions/delete", `{"noteId":"100"}`, token)
	require.Equal(t, stdhttp.StatusNoContent, deleted.Code, deleted.Body.String())
	require.Equal(t, int64(100), reaction.deleteRequest.GetEntity().GetEntityId())
}

func TestNoteReactionCompatMapsErrorsAndRequiresAuth(t *testing.T) {
	content := &notesCompatContentClient{topic: &contentpb.TopicInfo{Id: 101, Type: "tweet", Status: contentStatusPublished, AuthorId: 42}}
	reaction := &compatReactionClient{createErr: status.Error(codes.AlreadyExists, "already")}
	h := NewHandler(&clients.Clients{Content: content, Reaction: reaction, User: &notesCompatUserClient{}}, "Authorization", "Bearer", testJWTSecret)
	router := newCompatRouter(h)
	token := signedAuthToken(t, mapClaimsForNotesTest())

	already := performNotesCompatRequest(router, stdhttp.MethodPost, "/api/v1/notes/reactions/create", `{"noteId":"101","reaction":"like"}`, token)
	require.Equal(t, stdhttp.StatusBadRequest, already.Code)
	require.Contains(t, already.Body.String(), `"legacy_code":"ALREADY_REACTED"`)
	require.Contains(t, already.Body.String(), notesReactionAlreadyReactedID)

	invalid := performNotesCompatRequest(router, stdhttp.MethodPost, "/api/v1/notes/reactions/create", `{"noteId":"101","reaction":""}`, token)
	require.Equal(t, stdhttp.StatusBadRequest, invalid.Code)
	require.Contains(t, invalid.Body.String(), `"legacy_code":"INVALID_PARAM"`)

	anonymous := performNotesCompatRequest(router, stdhttp.MethodPost, "/api/v1/notes/reactions/delete", `{"noteId":"101"}`, "")
	require.Equal(t, stdhttp.StatusUnauthorized, anonymous.Code)
}

func TestNoteActionCompatSupportsLikeFavoriteAndState(t *testing.T) {
	content := &notesCompatContentClient{topic: &contentpb.TopicInfo{Id: 103, Type: "tweet", Status: contentStatusPublished, AuthorId: 42, Body: "hello", CreatedAt: 1700000000000}}
	reaction := &compatReactionClient{favoriteResponse: &reactionpb.ReactResponse{Success: true, Changed: true}, listFavoritesResponse: &reactionpb.FavoriteListResponse{Items: []*reactionpb.FavoriteInfo{{Entity: &reactionpb.EntityRef{EntityType: "topic", EntityId: 103}}}, Total: 1}}
	h := NewHandler(&clients.Clients{Content: content, Reaction: reaction, User: &notesCompatUserClient{}}, "Authorization", "Bearer", testJWTSecret)
	router := newCompatRouter(h)
	token := signedAuthToken(t, mapClaimsForNotesTest())

	like := performNotesCompatRequest(router, stdhttp.MethodPost, "/api/v1/notes/like", `{"noteId":"103"}`, token)
	require.Equal(t, stdhttp.StatusNoContent, like.Code, like.Body.String())
	require.Equal(t, int64(103), reaction.likeRequest.GetEntity().GetEntityId())

	favorite := performNotesCompatRequest(router, stdhttp.MethodPost, "/notes/favorites/create", `{"noteId":"103"}`, token)
	require.Equal(t, stdhttp.StatusNoContent, favorite.Code, favorite.Body.String())
	require.Equal(t, int64(103), reaction.favoriteRequest.GetEntity().GetEntityId())

	state := performNotesCompatRequest(router, stdhttp.MethodPost, "/api/notes/state", `{"noteId":"103"}`, token)
	require.Equal(t, stdhttp.StatusOK, state.Code, state.Body.String())
	var statePayload map[string]any
	require.NoError(t, json.Unmarshal(state.Body.Bytes(), &statePayload))
	require.Equal(t, true, statePayload["isFavorited"])
	require.Equal(t, false, statePayload["isRenoted"])
}

func TestNoteReactionsCompatListsUsersAndFiltersType(t *testing.T) {
	content := &notesCompatContentClient{topic: &contentpb.TopicInfo{Id: 104, Type: "tweet", Status: contentStatusPublished, AuthorId: 42, Body: "hello", CreatedAt: 1700000000000}}
	reaction := &compatReactionClient{listResponse: &reactionpb.ReactionListResponse{Items: []*reactionpb.ReactionInfo{{Id: 501, Entity: &reactionpb.EntityRef{EntityType: "topic", EntityId: 104}, UserId: 43, Reaction: "🔥", CreatedAt: 1700000001000}}}}
	users := &compatReactionUserClient{users: map[int64]*userpb.UserInfo{43: {Id: 43, Username: "bob", Nickname: "Bob"}}}
	h := NewHandler(&clients.Clients{Content: content, Reaction: reaction, User: users}, "Authorization", "Bearer", testJWTSecret)
	router := newCompatRouter(h)
	recorder := performNotesCompatRequest(router, stdhttp.MethodPost, "/api/v1/notes/reactions", `{"noteId":"104","type":"🔥","limit":5,"untilId":"900"}`, "")
	require.Equal(t, stdhttp.StatusOK, recorder.Code, recorder.Body.String())
	var payload []map[string]any
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &payload))
	require.Len(t, payload, 1)
	require.Equal(t, "501", payload[0]["id"])
	require.Equal(t, "bob", payload[0]["user"].(map[string]any)["username"])
	require.Equal(t, "🔥", reaction.listRequest.GetReaction())
	require.EqualValues(t, 104, reaction.listRequest.GetEntityId())
}

func TestUsersReactionsCompatPacksNoteAndAppliesCursor(t *testing.T) {
	content := &notesCompatContentClient{topic: &contentpb.TopicInfo{Id: 102, Type: "tweet", Status: contentStatusPublished, AuthorId: 42, Body: "reacted", CreatedAt: 1700000000000}}
	reaction := &compatReactionClient{listResponse: &reactionpb.ReactionListResponse{Items: []*reactionpb.ReactionInfo{{Id: 500, Entity: &reactionpb.EntityRef{EntityType: "topic", EntityId: 102}, UserId: 42, Reaction: "🔥", CreatedAt: 1700000001000}}}}
	h := NewHandler(&clients.Clients{Content: content, Reaction: reaction, User: &notesCompatUserClient{}}, "Authorization", "Bearer", testJWTSecret)
	router := newCompatRouter(h)

	recorder := performNotesCompatRequest(router, stdhttp.MethodPost, "/api/v1/users/reactions", `{"userId":"42","sinceId":"400"}`, "")
	require.Equal(t, stdhttp.StatusOK, recorder.Code, recorder.Body.String())
	var payload []map[string]any
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &payload))
	require.Len(t, payload, 1)
	require.Equal(t, "500", payload[0]["id"])
	require.Equal(t, "🔥", payload[0]["type"])
	require.Equal(t, "102", payload[0]["note"].(map[string]any)["id"])
	require.EqualValues(t, 400, reaction.listRequest.GetSinceId())
}

func TestReportUserAbuseCompatValidatesTargetAndWritesReport(t *testing.T) {
	users := &compatReportUserClient{users: map[int64]*userpb.UserInfo{
		42: {Id: 42, Username: "alice"},
		43: {Id: 43, Username: "bob"},
		44: {Id: 44, Username: "admin"},
	}}
	reaction := &compatReactionClient{}
	h := NewHandler(&clients.Clients{User: users, Reaction: reaction}, "Authorization", "Bearer", testJWTSecret)
	router := newCompatRouter(h)
	token := signedAuthToken(t, mapClaimsForNotesTest())

	recorded := performNotesCompatRequest(router, stdhttp.MethodPost, "/api/v1/users/report-abuse", `{"userId":"43","comment":"spam"}`, token)
	require.Equal(t, stdhttp.StatusNoContent, recorded.Code, recorded.Body.String())
	require.Equal(t, "user", reaction.reportRequest.GetEntity().GetEntityType())
	require.EqualValues(t, 43, reaction.reportRequest.GetEntity().GetEntityId())
	require.Equal(t, "abuse", reaction.reportRequest.GetReason())

	self := performNotesCompatRequest(router, stdhttp.MethodPost, "/users/report-abuse", `{"userId":"42","comment":"self"}`, token)
	require.Equal(t, stdhttp.StatusBadRequest, self.Code)
	require.Contains(t, self.Body.String(), `"legacy_code":"CANNOT_REPORT_YOURSELF"`)

	admin := performNotesCompatRequest(router, stdhttp.MethodPost, "/users/report-abuse", `{"userId":"44","comment":"admin"}`, token)
	require.Equal(t, stdhttp.StatusBadRequest, admin.Code)
	require.Contains(t, admin.Body.String(), `"legacy_code":"CANNOT_REPORT_THE_ADMIN"`)
}

func newCompatRouter(h *Handler) *gin.Engine {
	router := gin.New()
	NewInitControllers(h)(router)
	return router
}

type compatReactionClient struct {
	reactionpb.ReactionServiceClient
	createRequest         *reactionpb.CreateReactionRequest
	deleteRequest         *reactionpb.DeleteReactionRequest
	listRequest           *reactionpb.ListReactionsRequest
	reportRequest         *reactionpb.SubmitReportRequest
	createErr             error
	deleteErr             error
	listResponse          *reactionpb.ReactionListResponse
	likeRequest           *reactionpb.ReactRequest
	favoriteRequest       *reactionpb.ReactRequest
	unfavoriteRequest     *reactionpb.ReactRequest
	favoriteResponse      *reactionpb.ReactResponse
	listFavoritesResponse *reactionpb.FavoriteListResponse
}

func (c *compatReactionClient) CreateReaction(_ context.Context, request *reactionpb.CreateReactionRequest, _ ...grpc.CallOption) (*reactionpb.ReactionResponse, error) {
	c.createRequest = request
	if c.createErr != nil {
		return nil, c.createErr
	}
	return &reactionpb.ReactionResponse{Success: true, Changed: true}, nil
}

func (c *compatReactionClient) DeleteReaction(_ context.Context, request *reactionpb.DeleteReactionRequest, _ ...grpc.CallOption) (*reactionpb.ReactionResponse, error) {
	c.deleteRequest = request
	if c.deleteErr != nil {
		return nil, c.deleteErr
	}
	return &reactionpb.ReactionResponse{Success: true, Changed: true}, nil
}

func (c *compatReactionClient) ListReactions(_ context.Context, request *reactionpb.ListReactionsRequest, _ ...grpc.CallOption) (*reactionpb.ReactionListResponse, error) {
	c.listRequest = request
	if c.listResponse == nil {
		return &reactionpb.ReactionListResponse{}, nil
	}
	return c.listResponse, nil
}

func (c *compatReactionClient) Like(_ context.Context, request *reactionpb.ReactRequest, _ ...grpc.CallOption) (*reactionpb.ReactResponse, error) {
	c.likeRequest = request
	return &reactionpb.ReactResponse{Success: true, Changed: true}, nil
}

func (c *compatReactionClient) Favorite(_ context.Context, request *reactionpb.ReactRequest, _ ...grpc.CallOption) (*reactionpb.ReactResponse, error) {
	c.favoriteRequest = request
	if c.favoriteResponse != nil {
		return c.favoriteResponse, nil
	}
	return &reactionpb.ReactResponse{Success: true, Changed: true}, nil
}

func (c *compatReactionClient) Unfavorite(_ context.Context, request *reactionpb.ReactRequest, _ ...grpc.CallOption) (*reactionpb.ReactResponse, error) {
	c.unfavoriteRequest = request
	return &reactionpb.ReactResponse{Success: true, Changed: true}, nil
}

func (c *compatReactionClient) ListFavorites(_ context.Context, _ *reactionpb.ListFavoritesRequest, _ ...grpc.CallOption) (*reactionpb.FavoriteListResponse, error) {
	if c.listFavoritesResponse != nil {
		return c.listFavoritesResponse, nil
	}
	return &reactionpb.FavoriteListResponse{}, nil
}

func (c *compatReactionClient) SubmitReport(_ context.Context, request *reactionpb.SubmitReportRequest, _ ...grpc.CallOption) (*reactionpb.ReportResponse, error) {
	c.reportRequest = request
	return &reactionpb.ReportResponse{Success: true, Created: true}, nil
}

type compatReportUserClient struct {
	userpb.UserServiceClient
	users map[int64]*userpb.UserInfo
}

type compatReactionUserClient struct {
	userpb.UserServiceClient
	users map[int64]*userpb.UserInfo
}

func (c *compatReactionUserClient) GetUser(_ context.Context, request *userpb.UserIDRequest, _ ...grpc.CallOption) (*userpb.UserResponse, error) {
	user := c.users[request.GetId()]
	if user == nil {
		return nil, status.Error(codes.NotFound, "user not found")
	}
	return &userpb.UserResponse{Success: true, User: user}, nil
}

func (c *compatReactionUserClient) ListUsers(_ context.Context, request *userpb.ListUsersRequest, _ ...grpc.CallOption) (*userpb.UserListResponse, error) {
	items := make([]*userpb.UserInfo, 0, len(request.GetIds()))
	for _, id := range request.GetIds() {
		if user := c.users[id]; user != nil {
			items = append(items, user)
		}
	}
	return &userpb.UserListResponse{Items: items, Total: int64(len(items))}, nil
}

func (c *compatReportUserClient) GetUser(_ context.Context, request *userpb.UserIDRequest, _ ...grpc.CallOption) (*userpb.UserResponse, error) {
	user := c.users[request.GetId()]
	if user == nil {
		return nil, status.Error(codes.NotFound, "user not found")
	}
	return &userpb.UserResponse{Success: true, User: user}, nil
}
