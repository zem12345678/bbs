package http

import (
	"context"
	"encoding/json"
	"errors"
	stdhttp "net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"api-gateway/api/proto/contentpb"
	"api-gateway/api/proto/reactionpb"
	"api-gateway/internal/clients"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func TestPinNoteAcceptsPublicArticle(t *testing.T) {
	gin.SetMode(gin.TestMode)
	const noteID int64 = 9007199254740993
	reaction := &pinHTTPReactionClient{}
	content := &pinHTTPContentClient{articles: map[int64]*contentpb.ArticleInfo{
		noteID: {Id: noteID, AuthorId: 7, Status: contentStatusPublished},
	}}
	h := NewHandler(&clients.Clients{Reaction: reaction, Content: content}, "Authorization", "Bearer", testJWTSecret)
	recorder, c := pinContext(stdhttp.MethodPost, "/api/v1/i/pin", `{"noteId":"9007199254740993"}`, 42)

	h.pinNote(c)

	require.Equal(t, stdhttp.StatusOK, recorder.Code, recorder.Body.String())
	require.Equal(t, int64(42), reaction.pinReq.GetUserId())
	require.Equal(t, "article", reaction.pinReq.GetEntity().GetEntityType())
	require.Equal(t, noteID, reaction.pinReq.GetEntity().GetEntityId())
}

func TestUnpinNoteAllowsOwnDraftAndIsIdempotent(t *testing.T) {
	gin.SetMode(gin.TestMode)
	reaction := &pinHTTPReactionClient{}
	content := &pinHTTPContentClient{topics: map[int64]*contentpb.TopicInfo{
		9: {Id: 9, AuthorId: 42, Status: 1},
	}}
	h := NewHandler(&clients.Clients{Reaction: reaction, Content: content}, "Authorization", "Bearer", testJWTSecret)
	recorder, c := pinContext(stdhttp.MethodPost, "/i/unpin", `{"noteId":9}`, 42)

	h.unpinNote(c)

	require.Equal(t, stdhttp.StatusOK, recorder.Code, recorder.Body.String())
	require.NotNil(t, reaction.unpinReq)
	require.Equal(t, "topic", reaction.unpinReq.GetEntity().GetEntityType())
	require.Equal(t, int64(9), reaction.unpinReq.GetEntity().GetEntityId())
}

func TestUnpinNoteReleasesStalePinnedContentWithoutContentLookup(t *testing.T) {
	gin.SetMode(gin.TestMode)
	reaction := &pinHTTPReactionClient{listResp: &reactionpb.PinListResponse{Items: []*reactionpb.PinInfo{{
		Entity: &reactionpb.EntityRef{EntityType: "article", EntityId: 9},
	}}}}
	h := NewHandler(&clients.Clients{Reaction: reaction}, "Authorization", "Bearer", testJWTSecret)
	recorder, c := pinContext(stdhttp.MethodPost, "/i/unpin", `{"noteId":9}`, 42)

	h.unpinNote(c)

	require.Equal(t, stdhttp.StatusOK, recorder.Code, recorder.Body.String())
	require.NotNil(t, reaction.unpinReq)
	require.Equal(t, "article", reaction.unpinReq.GetEntity().GetEntityType())
	require.Equal(t, int64(9), reaction.unpinReq.GetEntity().GetEntityId())
}

func TestPinNoteRejectsOtherUsersDraftAndMapsPinLimit(t *testing.T) {
	gin.SetMode(gin.TestMode)
	content := &pinHTTPContentClient{articles: map[int64]*contentpb.ArticleInfo{
		8: {Id: 8, AuthorId: 7, Status: 1},
		9: {Id: 9, AuthorId: 7, Status: contentStatusPublished},
	}}
	reaction := &pinHTTPReactionClient{pinErr: status.Error(codes.ResourceExhausted, "REACTION_PIN_LIMIT_EXCEEDED")}
	h := NewHandler(&clients.Clients{Reaction: reaction, Content: content}, "Authorization", "Bearer", testJWTSecret)

	privateRecorder, privateContext := pinContext(stdhttp.MethodPost, "/i/pin", `{"noteId":8}`, 42)
	h.pinNote(privateContext)
	require.Equal(t, stdhttp.StatusNotFound, privateRecorder.Code, privateRecorder.Body.String())
	require.Nil(t, reaction.pinReq)

	limitRecorder, limitContext := pinContext(stdhttp.MethodPost, "/i/pin", `{"noteId":9}`, 42)
	h.pinNote(limitContext)
	require.Equal(t, stdhttp.StatusTooManyRequests, limitRecorder.Code, limitRecorder.Body.String())
}

func TestPinActionsUseUserScopedRateLimit(t *testing.T) {
	gin.SetMode(gin.TestMode)
	content := &pinHTTPContentClient{articles: map[int64]*contentpb.ArticleInfo{
		9: {Id: 9, AuthorId: 7, Status: contentStatusPublished},
	}}
	for _, tc := range []struct {
		name       string
		limiter    *pinRateLimitStub
		wantStatus int
		wantMsg    string
	}{
		{name: "limited", limiter: &pinRateLimitStub{limited: true}, wantStatus: stdhttp.StatusTooManyRequests, wantMsg: "pin rate limit exceeded"},
		{name: "unavailable", limiter: &pinRateLimitStub{err: errors.New("redis unavailable")}, wantStatus: stdhttp.StatusServiceUnavailable, wantMsg: "pin rate limiter unavailable"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			reaction := &pinHTTPReactionClient{}
			h := NewHandler(&clients.Clients{Reaction: reaction, Content: content}, "Authorization", "Bearer", testJWTSecret)
			h.SetPinActionLimit(tc.limiter)
			recorder, c := pinContext(stdhttp.MethodPost, "/api/v1/i/pin", `{"noteId":9}`, 42)

			h.pinNote(c)

			require.Equal(t, tc.wantStatus, recorder.Code, recorder.Body.String())
			require.Equal(t, []string{"rate:pins:user:42"}, tc.limiter.keys)
			require.Nil(t, reaction.pinReq)
			require.Contains(t, recorder.Body.String(), tc.wantMsg)
		})
	}
}

func TestListPinnedContentFiltersByViewerVisibility(t *testing.T) {
	gin.SetMode(gin.TestMode)
	reaction := &pinHTTPReactionClient{listResp: &reactionpb.PinListResponse{Items: []*reactionpb.PinInfo{
		{Id: 1, UserId: 42, Entity: &reactionpb.EntityRef{EntityType: "article", EntityId: 11}, CreatedAt: 100},
		{Id: 2, UserId: 42, Entity: &reactionpb.EntityRef{EntityType: "article", EntityId: 12}, CreatedAt: 101},
		{Id: 3, UserId: 42, Entity: &reactionpb.EntityRef{EntityType: "topic", EntityId: 13}, CreatedAt: 102},
	}}}
	content := &pinHTTPContentClient{
		articles: map[int64]*contentpb.ArticleInfo{
			11: {Id: 11, AuthorId: 42, Status: contentStatusPublished, Title: "public"},
			12: {Id: 12, AuthorId: 42, Status: 1, Title: "draft"},
		},
		topics: map[int64]*contentpb.TopicInfo{
			13: {Id: 13, AuthorId: 42, Status: contentStatusArchived, Title: "archived"},
		},
	}
	h := NewHandler(&clients.Clients{Reaction: reaction, Content: content}, "Authorization", "Bearer", testJWTSecret)

	publicRecorder, publicContext := pinContext(stdhttp.MethodGet, "/api/v1/users/42/pinned", "", 7)
	h.listPinnedForUser(publicContext, 42, 7)
	require.Equal(t, stdhttp.StatusOK, publicRecorder.Code, publicRecorder.Body.String())
	var publicEnvelope struct {
		Data struct {
			Items []json.RawMessage `json:"items"`
			Total int64             `json:"total"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(publicRecorder.Body.Bytes(), &publicEnvelope))
	require.Equal(t, int64(1), publicEnvelope.Data.Total)
	var publicItem struct {
		ID string `json:"id"`
	}
	require.NoError(t, json.Unmarshal(publicEnvelope.Data.Items[0], &publicItem))
	require.Equal(t, "11", publicItem.ID)

	ownerRecorder, ownerContext := pinContext(stdhttp.MethodGet, "/api/v1/users/me/pinned", "", 42)
	h.listPinnedForUser(ownerContext, 42, 42)
	var ownerEnvelope struct {
		Data struct {
			Items []json.RawMessage `json:"items"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(ownerRecorder.Body.Bytes(), &ownerEnvelope))
	require.Len(t, ownerEnvelope.Data.Items, 2)
	var ownerItem struct {
		ID string `json:"id"`
	}
	require.NoError(t, json.Unmarshal(ownerEnvelope.Data.Items[1], &ownerItem))
	require.Equal(t, "12", ownerItem.ID)
}

func TestPinnedContentListKeepsNestedIDsJSONSafe(t *testing.T) {
	gin.SetMode(gin.TestMode)
	const contentID int64 = 9007199254740993
	reaction := &pinHTTPReactionClient{listResp: &reactionpb.PinListResponse{Items: []*reactionpb.PinInfo{{
		Id:        1,
		UserId:    42,
		Entity:    &reactionpb.EntityRef{EntityType: "article", EntityId: contentID},
		CreatedAt: 100,
	}}}}
	content := &pinHTTPContentClient{articles: map[int64]*contentpb.ArticleInfo{
		contentID: {Id: contentID, AuthorId: contentID, Status: contentStatusPublished},
	}}
	h := NewHandler(&clients.Clients{Reaction: reaction, Content: content}, "Authorization", "Bearer", testJWTSecret)
	recorder, c := pinContext(stdhttp.MethodGet, "/api/v1/users/42/pinned", "", 7)

	h.listPinnedForUser(c, 42, 7)

	require.Equal(t, stdhttp.StatusOK, recorder.Code, recorder.Body.String())
	require.Contains(t, recorder.Body.String(), `"id":"9007199254740993"`)
	require.Contains(t, recorder.Body.String(), `"author_id":"9007199254740993"`)
	require.NotContains(t, recorder.Body.String(), `"id":9007199254740993`)
	require.NotContains(t, recorder.Body.String(), `"author_id":9007199254740993`)
}

func TestPinCompatibilityRoutesRequireInteractiveAuthentication(t *testing.T) {
	gin.SetMode(gin.TestMode)
	h := NewHandler(&clients.Clients{}, "Authorization", "Bearer", testJWTSecret)
	router := gin.New()
	NewInitControllers(h)(router)
	for _, path := range []string{"/i/pin", "/api/i/unpin", "/api/v1/i/pin"} {
		recorder := httptest.NewRecorder()
		request := httptest.NewRequest(stdhttp.MethodPost, path, strings.NewReader(`{"noteId":9}`))
		request.Header.Set("Content-Type", "application/json")
		router.ServeHTTP(recorder, request)
		require.Equal(t, stdhttp.StatusUnauthorized, recorder.Code, path+": "+recorder.Body.String())
	}
}

func pinContext(method, path, body string, userID int64) (*httptest.ResponseRecorder, *gin.Context) {
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(method, path, strings.NewReader(body))
	if body != "" {
		c.Request.Header.Set("Content-Type", "application/json")
	}
	c.Set("user_id", userID)
	return recorder, c
}

type pinHTTPReactionClient struct {
	reactionpb.ReactionServiceClient
	pinReq   *reactionpb.ReactRequest
	unpinReq *reactionpb.ReactRequest
	listReq  *reactionpb.ListPinsRequest
	pinErr   error
	unpinErr error
	listErr  error
	listResp *reactionpb.PinListResponse
}

type pinRateLimitStub struct {
	limited bool
	err     error
	keys    []string
}

func (l *pinRateLimitStub) Limit(_ context.Context, key string) (bool, error) {
	l.keys = append(l.keys, key)
	return l.limited, l.err
}

func (f *pinHTTPReactionClient) Pin(_ context.Context, req *reactionpb.ReactRequest, _ ...grpc.CallOption) (*reactionpb.ReactResponse, error) {
	f.pinReq = req
	if f.pinErr != nil {
		return nil, f.pinErr
	}
	return &reactionpb.ReactResponse{Success: true, Changed: true, Count: 1}, nil
}

func (f *pinHTTPReactionClient) Unpin(_ context.Context, req *reactionpb.ReactRequest, _ ...grpc.CallOption) (*reactionpb.ReactResponse, error) {
	f.unpinReq = req
	if f.unpinErr != nil {
		return nil, f.unpinErr
	}
	return &reactionpb.ReactResponse{Success: true}, nil
}

func (f *pinHTTPReactionClient) ListPins(_ context.Context, req *reactionpb.ListPinsRequest, _ ...grpc.CallOption) (*reactionpb.PinListResponse, error) {
	f.listReq = req
	if f.listErr != nil {
		return nil, f.listErr
	}
	if f.listResp != nil {
		return f.listResp, nil
	}
	return &reactionpb.PinListResponse{}, nil
}

type pinHTTPContentClient struct {
	contentpb.ContentServiceClient
	articles map[int64]*contentpb.ArticleInfo
	topics   map[int64]*contentpb.TopicInfo
}

func (f *pinHTTPContentClient) GetArticle(_ context.Context, req *contentpb.GetArticleRequest, _ ...grpc.CallOption) (*contentpb.ArticleResponse, error) {
	if article := f.articles[req.GetId()]; article != nil {
		return &contentpb.ArticleResponse{Article: article}, nil
	}
	return nil, status.Error(codes.NotFound, "article not found")
}

func (f *pinHTTPContentClient) GetTopic(_ context.Context, req *contentpb.GetTopicRequest, _ ...grpc.CallOption) (*contentpb.TopicResponse, error) {
	if topic := f.topics[req.GetId()]; topic != nil {
		return &contentpb.TopicResponse{Topic: topic}, nil
	}
	return nil, status.Error(codes.NotFound, "topic not found")
}
