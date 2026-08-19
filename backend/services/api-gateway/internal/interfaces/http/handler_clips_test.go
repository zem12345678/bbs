package http

import (
	"context"
	"encoding/json"
	stdhttp "net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"api-gateway/api/proto/contentpb"
	"api-gateway/api/proto/reactionpb"
	"api-gateway/api/proto/userpb"
	"api-gateway/internal/clients"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
)

func TestClipCompatibilityPublicEndpointsRejectInvalidRequests(t *testing.T) {
	gin.SetMode(gin.TestMode)
	cases := []struct {
		name    string
		handler gin.HandlerFunc
	}{
		{name: "public list", handler: (&Handler{}).listPublicClips},
		{name: "note list", handler: (&Handler{}).listNoteClips},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			recorder := httptest.NewRecorder()
			ctx, _ := gin.CreateTestContext(recorder)
			ctx.Request = httptest.NewRequest(stdhttp.MethodPost, "/", strings.NewReader(`{}`))
			ctx.Request.Header.Set("Content-Type", "application/json")
			tc.handler(ctx)
			require.Equal(t, stdhttp.StatusBadRequest, recorder.Code)
			require.Contains(t, recorder.Body.String(), "invalid_argument")
		})
	}
}

func TestListPublicClipsMapsCursorAndLastClippedAt(t *testing.T) {
	const collectionID int64 = 9007199254740993
	reaction := &publicClipReactionClient{publicResp: &reactionpb.ListCollectionsResponse{Items: []*reactionpb.CollectionInfo{{
		Id: collectionID, UserId: 42, Name: "Reading", IsPublic: true, CreatedAt: 1800000000000, LastClippedAt: 1800000001000,
	}}}}
	h := NewHandler(&clients.Clients{Reaction: reaction, User: &publicClipUserClient{}}, "Authorization", "Bearer", testJWTSecret)
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(stdhttp.MethodPost, "/users/clips", strings.NewReader(`{"userId":"42","limit":7,"sinceId":"9","untilId":"3"}`))
	c.Request.Header.Set("Content-Type", "application/json")
	h.listPublicClips(c)
	require.Equal(t, stdhttp.StatusOK, recorder.Code, recorder.Body.String())
	var clips []misskeyClip
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &clips))
	require.Len(t, clips, 1)
	require.Equal(t, "9007199254740993", clips[0].ID)
	require.Equal(t, "2027-01-15T08:00:01Z", *clips[0].LastClippedAt)
	var rawClips []map[string]any
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &rawClips))
	_, hasNotesCount := rawClips[0]["notesCount"]
	require.False(t, hasNotesCount)
	require.EqualValues(t, 42, reaction.publicReq.GetUserId())
	require.EqualValues(t, 9, reaction.publicReq.GetSinceId())
	require.EqualValues(t, 3, reaction.publicReq.GetUntilId())
}

func TestListNoteClipsResolvesPublishedArticle(t *testing.T) {
	const noteID int64 = 9007199254740999
	reaction := &publicClipReactionClient{entityResp: &reactionpb.ListCollectionsResponse{Items: []*reactionpb.CollectionInfo{{Id: 7, UserId: 42, Name: "Reading", IsPublic: true}}}}
	content := &publicClipContentClient{article: &contentpb.ArticleInfo{Id: noteID, Status: contentStatusPublished, AuthorId: 42}}
	h := NewHandler(&clients.Clients{Reaction: reaction, Content: content, User: &publicClipUserClient{}}, "Authorization", "Bearer", testJWTSecret)
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(stdhttp.MethodPost, "/notes/clips", strings.NewReader(`{"noteId":"9007199254740999"}`))
	c.Request.Header.Set("Content-Type", "application/json")
	h.listNoteClips(c)
	require.Equal(t, stdhttp.StatusOK, recorder.Code, recorder.Body.String())
	require.Equal(t, "article", reaction.entityReq.GetEntity().GetEntityType())
	require.EqualValues(t, noteID, reaction.entityReq.GetEntity().GetEntityId())
}

func TestUpdateClipDistinguishesNullAndOmittedDescription(t *testing.T) {
	const clipID int64 = 9007199254740993
	for _, tc := range []struct {
		name     string
		body     string
		wantDesc string
	}{
		{name: "explicit null clears", body: `{"clipId":"9007199254740993","description":null}`, wantDesc: ""},
		{name: "omitted preserves", body: `{"clipId":"9007199254740993"}`, wantDesc: "existing"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			reaction := &publicClipReactionClient{collectionResp: &reactionpb.CollectionResponse{
				Collection: &reactionpb.CollectionInfo{Id: clipID, UserId: 42, Name: "Reading", Description: "existing"},
			}}
			h := NewHandler(&clients.Clients{Reaction: reaction, User: &publicClipUserClient{}}, "Authorization", "Bearer", testJWTSecret)
			recorder := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(recorder)
			c.Set("user_id", int64(42))
			c.Request = httptest.NewRequest(stdhttp.MethodPost, "/clips/update", strings.NewReader(tc.body))
			c.Request.Header.Set("Content-Type", "application/json")

			h.updateClip(c)
			require.Equal(t, stdhttp.StatusOK, recorder.Code, recorder.Body.String())
			require.NotNil(t, reaction.updateReq)
			require.Equal(t, tc.wantDesc, reaction.updateReq.GetDescription())
		})
	}
}

func TestListClipNotesForwardsBothCursors(t *testing.T) {
	const noteID int64 = 9007199254740999
	reaction := &publicClipReactionClient{itemsResp: &reactionpb.CollectionItemsResponse{Items: []*reactionpb.CollectionItemInfo{{
		Id: 1, CollectionId: 7, Entity: &reactionpb.EntityRef{EntityType: "article", EntityId: noteID},
	}}}}
	content := &publicClipContentClient{article: &contentpb.ArticleInfo{Id: noteID, Status: contentStatusPublished, AuthorId: 42}}
	h := NewHandler(&clients.Clients{Reaction: reaction, Content: content, User: &publicClipUserClient{}}, "Authorization", "Bearer", testJWTSecret)
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Set("user_id", int64(42))
	c.Request = httptest.NewRequest(stdhttp.MethodPost, "/clips/notes", strings.NewReader(`{"clipId":"7","limit":1,"sinceId":"9007199254740990","untilId":"9007199254741000"}`))
	c.Request.Header.Set("Content-Type", "application/json")

	h.listClipNotes(c)
	require.Equal(t, stdhttp.StatusOK, recorder.Code, recorder.Body.String())
	require.NotNil(t, reaction.itemsReq)
	require.EqualValues(t, 9007199254740990, reaction.itemsReq.GetSinceId())
	require.EqualValues(t, 9007199254741000, reaction.itemsReq.GetUntilId())
}

type publicClipReactionClient struct {
	reactionpb.ReactionServiceClient
	publicResp     *reactionpb.ListCollectionsResponse
	entityResp     *reactionpb.ListCollectionsResponse
	collectionResp *reactionpb.CollectionResponse
	updateReq      *reactionpb.UpdateCollectionRequest
	itemsResp      *reactionpb.CollectionItemsResponse
	itemsReq       *reactionpb.ListPublicCollectionItemsRequest
	publicReq      *reactionpb.ListPublicCollectionsRequest
	entityReq      *reactionpb.ListPublicCollectionsForEntityRequest
}

func (f *publicClipReactionClient) GetCollection(_ context.Context, _ *reactionpb.GetCollectionRequest, _ ...grpc.CallOption) (*reactionpb.CollectionResponse, error) {
	return f.collectionResp, nil
}

func (f *publicClipReactionClient) ListFavorites(context.Context, *reactionpb.ListFavoritesRequest, ...grpc.CallOption) (*reactionpb.FavoriteListResponse, error) {
	return &reactionpb.FavoriteListResponse{}, nil
}

func (f *publicClipReactionClient) UpdateCollection(_ context.Context, req *reactionpb.UpdateCollectionRequest, _ ...grpc.CallOption) (*reactionpb.CollectionResponse, error) {
	f.updateReq = req
	return &reactionpb.CollectionResponse{Success: true, Collection: &reactionpb.CollectionInfo{Id: req.GetId(), UserId: req.GetUserId(), Name: req.GetName(), Description: req.GetDescription()}}, nil
}

func (f *publicClipReactionClient) ListPublicCollectionItems(_ context.Context, req *reactionpb.ListPublicCollectionItemsRequest, _ ...grpc.CallOption) (*reactionpb.CollectionItemsResponse, error) {
	f.itemsReq = req
	return f.itemsResp, nil
}

func (f *publicClipReactionClient) ListPublicCollections(_ context.Context, req *reactionpb.ListPublicCollectionsRequest, _ ...grpc.CallOption) (*reactionpb.ListCollectionsResponse, error) {
	f.publicReq = req
	return f.publicResp, nil
}

func (f *publicClipReactionClient) ListPublicCollectionsForEntity(_ context.Context, req *reactionpb.ListPublicCollectionsForEntityRequest, _ ...grpc.CallOption) (*reactionpb.ListCollectionsResponse, error) {
	f.entityReq = req
	return f.entityResp, nil
}

func (f *publicClipReactionClient) GetCounts(context.Context, *reactionpb.EntityRequest, ...grpc.CallOption) (*reactionpb.CountsResponse, error) {
	return &reactionpb.CountsResponse{}, nil
}

type publicClipUserClient struct {
	clients.UserClient
}

func (f *publicClipUserClient) GetUser(_ context.Context, req *userpb.UserIDRequest, _ ...grpc.CallOption) (*userpb.UserResponse, error) {
	return &userpb.UserResponse{User: &userpb.UserInfo{Id: req.GetId(), Username: "reader", Nickname: "Reader"}}, nil
}

type publicClipContentClient struct {
	contentpb.ContentServiceClient
	article *contentpb.ArticleInfo
}

func (f *publicClipContentClient) GetArticle(_ context.Context, _ *contentpb.GetArticleRequest, _ ...grpc.CallOption) (*contentpb.ArticleResponse, error) {
	return &contentpb.ArticleResponse{Article: f.article}, nil
}

func (f *publicClipContentClient) GetTopic(context.Context, *contentpb.GetTopicRequest, ...grpc.CallOption) (*contentpb.TopicResponse, error) {
	return &contentpb.TopicResponse{}, nil
}
