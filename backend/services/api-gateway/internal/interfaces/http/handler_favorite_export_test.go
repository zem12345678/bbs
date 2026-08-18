package http

import (
	"context"
	"encoding/json"
	stdhttp "net/http"
	"net/http/httptest"
	"sort"
	"strconv"
	"strings"
	"testing"
	"time"

	"api-gateway/api/proto/contentpb"
	"api-gateway/api/proto/filepb"
	"api-gateway/api/proto/reactionpb"
	"api-gateway/api/proto/userpb"
	"api-gateway/internal/clients"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
)

func TestBuildFavoriteExportMergesArticleAndTopicKeysets(t *testing.T) {
	favorites := make([]*reactionpb.FavoriteInfo, 0, 101)
	articles := make(map[int64]*contentpb.ArticleInfo)
	topics := make(map[int64]*contentpb.TopicInfo)
	for i := int64(1); i <= 101; i++ {
		entityID := 1000 + i
		favorite := &reactionpb.FavoriteInfo{
			Id: 3000 + i, UserId: 42, CreatedAt: 1800000000000 + i,
			Entity: &reactionpb.EntityRef{EntityType: "article", EntityId: entityID},
		}
		if i == 101 {
			favorite.Entity.EntityType = "topic"
			topics[entityID] = &contentpb.TopicInfo{Id: entityID, Body: "topic body", AuthorId: 52, CreatedAt: 1700000000123,
				Poll: &contentpb.TopicPollInfo{Choices: []*contentpb.TopicPollChoiceInfo{{Text: "A", Votes: 2}}}}
		} else {
			articles[entityID] = &contentpb.ArticleInfo{Id: entityID, Body: "article-" + strconv.FormatInt(entityID, 10), AuthorId: 51, CreatedAt: 1700000000000 + i}
		}
		favorites = append(favorites, favorite)
	}
	reaction := &favoriteExportReactionStub{favorites: map[string][]*reactionpb.FavoriteInfo{"article": favorites[:100], "topic": favorites[100:]}}
	h := NewHandler(&clients.Clients{
		Reaction: reaction, Content: &clipExportContentStub{articles: articles, topics: topics},
		User: &clipExportUserStub{users: map[int64]*userpb.UserInfo{51: {Id: 51, Username: "alice", Nickname: "Alice"}, 52: {Id: 52, Username: "bob", Nickname: "Bob"}}},
	}, "Authorization", "Bearer", testJWTSecret)

	payload, err := h.buildFavoriteExport(context.Background(), 42)
	require.NoError(t, err)
	var exported []favoriteExportRecord
	require.NoError(t, json.Unmarshal(payload, &exported))
	require.Len(t, exported, 101)
	require.Equal(t, "3001", exported[0].ID)
	require.Equal(t, "3101", exported[100].ID)
	require.Equal(t, "2027-01-15T08:00:00.001Z", exported[0].CreatedAt)
	require.Equal(t, "2023-11-14T22:13:20.001Z", exported[0].Note.CreatedAt)
	require.Equal(t, "topic body", exported[100].Note.Text)
	require.Equal(t, []string{"A"}, exported[100].Note.Poll.Choices)
	var raw []map[string]any
	require.NoError(t, json.Unmarshal(payload, &raw))
	note := raw[0]["note"].(map[string]any)
	_, hasPoll := note["poll"]
	require.True(t, hasPoll)
	require.Nil(t, note["poll"])
	require.Equal(t, []int64{0, 3100}, reaction.afterIDs["article"])
	require.Equal(t, []int64{0}, reaction.afterIDs["topic"])
}

func TestBuildFavoriteExportReturnsEmptyArray(t *testing.T) {
	h := NewHandler(&clients.Clients{
		Reaction: &favoriteExportReactionStub{}, Content: &clipExportContentStub{}, User: &clipExportUserStub{},
	}, "Authorization", "Bearer", testJWTSecret)
	payload, err := h.buildFavoriteExport(context.Background(), 42)
	require.NoError(t, err)
	require.JSONEq(t, `[]`, string(payload))
}

func TestExportFavoritesRegistersJSONAndCompletionNotification(t *testing.T) {
	gin.SetMode(gin.TestMode)
	fileClient := &fakeUserFileClient{createResp: &filepb.FileResponse{File: &filepb.File{Id: 9010, OwnerId: 42}}}
	notifications := &clipExportNotificationStub{}
	store := newFakeUserFileStore()
	h := NewHandlerWithAttachmentStore(&clients.Clients{
		Reaction: &favoriteExportReactionStub{}, Content: &clipExportContentStub{}, User: &clipExportUserStub{},
		File: fileClient, NotificationInternal: notifications,
	}, "Authorization", "Bearer", testJWTSecret, store)
	h.SetFavoriteExportGate(&clipExportGateStub{permit: &clipExportPermitStub{}})
	h.SetPublicBaseURL("https://bbs.example.com")
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(stdhttp.MethodPost, "/i/export-favorites", strings.NewReader(`{}`))
	c.Set("user_id", int64(42))
	h.exportFavorites(c)

	require.Equal(t, stdhttp.StatusNoContent, c.Writer.Status(), recorder.Body.String())
	require.Equal(t, "application/json", fileClient.createReq.GetContentType())
	require.Equal(t, "favorite", notifications.req.GetExportedEntity())
	require.Len(t, store.objects, 1)
}

func TestFavoriteExportRoutesRequireInteractiveAuthAndApplyRateLimit(t *testing.T) {
	gin.SetMode(gin.TestMode)
	newHandler := func() *Handler {
		h := NewHandlerWithAttachmentStore(&clients.Clients{
			Reaction: &favoriteExportReactionStub{}, Content: &clipExportContentStub{}, User: &clipExportUserStub{}, File: &fakeUserFileClient{},
		}, "Authorization", "Bearer", testJWTSecret, newFakeUserFileStore())
		h.SetFavoriteExportGate(&clipExportGateStub{err: errExportRateLimited})
		return h
	}
	for _, path := range []string{"/i/export-favorites", "/api/i/export-favorites", "/api/v1/i/export-favorites"} {
		t.Run(path, func(t *testing.T) {
			router := gin.New()
			NewInitControllers(newHandler())(router)
			request := httptest.NewRequest(stdhttp.MethodPost, path, strings.NewReader(`{}`))
			request.Header.Set("Authorization", "Bearer "+signedAuthToken(t, jwt.MapClaims{"sub": "42"}))
			request.Header.Set("Content-Type", "application/json")
			recorder := httptest.NewRecorder()
			router.ServeHTTP(recorder, request)
			require.Equal(t, stdhttp.StatusTooManyRequests, recorder.Code, recorder.Body.String())
		})
	}

	router := gin.New()
	NewInitControllers(newHandler())(router)
	request := httptest.NewRequest(stdhttp.MethodPost, "/api/v1/i/export-favorites", strings.NewReader(`{}`))
	request.Header.Set("Authorization", "Bearer "+signedAuthToken(t, jwt.MapClaims{
		"sub": "42", "jti": "favorite-export-api-token", "exp": time.Now().Add(time.Hour).Unix(),
		credentialVersionClaim: "0", "token_type": apiTokenType, "scopes": []string{"read"},
	}))
	request.Header.Set("Content-Type", "application/json")
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, request)
	require.Equal(t, stdhttp.StatusForbidden, recorder.Code, recorder.Body.String())
}

type favoriteExportReactionStub struct {
	reactionpb.ReactionServiceClient
	favorites map[string][]*reactionpb.FavoriteInfo
	afterIDs  map[string][]int64
}

func (s *favoriteExportReactionStub) ListFavorites(_ context.Context, req *reactionpb.ListFavoritesRequest, _ ...grpc.CallOption) (*reactionpb.FavoriteListResponse, error) {
	if s.afterIDs == nil {
		s.afterIDs = map[string][]int64{}
	}
	s.afterIDs[req.GetEntityType()] = append(s.afterIDs[req.GetEntityType()], req.GetAfterId())
	items := make([]*reactionpb.FavoriteInfo, 0)
	for _, item := range s.favorites[req.GetEntityType()] {
		if item.GetId() > req.GetAfterId() {
			items = append(items, item)
		}
	}
	sort.Slice(items, func(i, j int) bool { return items[i].GetId() < items[j].GetId() })
	if len(items) > int(req.GetLimit()) {
		items = items[:req.GetLimit()]
	}
	return &reactionpb.FavoriteListResponse{Items: items, Total: int64(len(s.favorites[req.GetEntityType()]))}, nil
}
