package http

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	stdhttp "net/http"
	"net/http/httptest"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"testing"
	"time"

	"api-gateway/api/proto/contentpb"
	"api-gateway/api/proto/filepb"
	"api-gateway/api/proto/notificationpb"
	"api-gateway/api/proto/reactionpb"
	"api-gateway/api/proto/userpb"
	"api-gateway/internal/clients"
	"api-gateway/internal/storage"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func TestBuildClipExportPaginatesAndMapsArticleTopicAndAuthors(t *testing.T) {
	items := make([]*reactionpb.CollectionItemInfo, 0, 101)
	articles := make(map[int64]*contentpb.ArticleInfo, 101)
	for index := int64(100); index >= 0; index-- {
		entityID := 1000 + index
		items = append(items, &reactionpb.CollectionItemInfo{
			Id: 2000 + index, CollectionId: 9, CreatedAt: 1800000000000 + index,
			Entity: &reactionpb.EntityRef{EntityType: "article", EntityId: entityID},
		})
		articles[entityID] = &contentpb.ArticleInfo{Id: entityID, Body: "article-" + strconv.FormatInt(entityID, 10), AuthorId: 51, CreatedAt: 1700000000000 + index}
	}
	items[len(items)-1].Entity.EntityType = "topic"
	delete(articles, items[len(items)-1].GetEntity().GetEntityId())
	topicID := items[len(items)-1].GetEntity().GetEntityId()
	reaction := &clipExportReactionStub{
		collections: []*reactionpb.CollectionInfo{{Id: 9, UserId: 42, Name: "private", Description: "kept", IsPublic: false, LastClippedAt: 1800000001000}},
		items:       map[int64][]*reactionpb.CollectionItemInfo{9: items},
	}
	content := &clipExportContentStub{
		articles: articles,
		topics: map[int64]*contentpb.TopicInfo{topicID: {
			Id: topicID, Body: "topic-body", AuthorId: 52, CreatedAt: 1700000000100,
			Poll: &contentpb.TopicPollInfo{Multiple: true, ExpiresAt: 1900000000000, Choices: []*contentpb.TopicPollChoiceInfo{{Text: "A", Votes: 2}, {Text: "B", Votes: 3}}},
		}},
	}
	users := &clipExportUserStub{users: map[int64]*userpb.UserInfo{
		51: {Id: 51, Username: "alice", Nickname: "Alice"},
		52: {Id: 52, Username: "bob", Nickname: "Bob"},
	}}
	h := NewHandler(&clients.Clients{Reaction: reaction, Content: content, User: users}, "Authorization", "Bearer", testJWTSecret)

	payload, err := h.buildClipExport(context.Background(), 42)
	require.NoError(t, err)
	var exported []clipExportRecord
	require.NoError(t, json.Unmarshal(payload, &exported))
	require.Len(t, exported, 1)
	require.Equal(t, "private", exported[0].Name)
	require.Len(t, exported[0].ClipNotes, 101)
	require.Equal(t, "2000", exported[0].ClipNotes[0].ID)
	require.Equal(t, "2100", exported[0].ClipNotes[100].ID)
	require.Equal(t, "topic-body", exported[0].ClipNotes[0].Note.Text)
	require.Equal(t, []string{"A", "B"}, exported[0].ClipNotes[0].Note.Poll.Choices)
	require.Equal(t, []int64{2, 3}, exported[0].ClipNotes[0].Note.Poll.Votes)
	var raw []struct {
		ClipNotes []struct {
			Note map[string]any `json:"note"`
		} `json:"clipNotes"`
	}
	require.NoError(t, json.Unmarshal(payload, &raw))
	require.Contains(t, raw[0].ClipNotes[0].Note, "poll")
	require.NotContains(t, raw[0].ClipNotes[1].Note, "poll")
	require.Equal(t, []int64{0, 2099}, reaction.itemAfterIDs)
	require.Equal(t, 2, users.calls)
}

func TestBuildClipExportPaginatesCollectionsAndKeepsEmptyClip(t *testing.T) {
	collections := make([]*reactionpb.CollectionInfo, 0, 101)
	for id := int64(101); id >= 1; id-- {
		collections = append(collections, &reactionpb.CollectionInfo{Id: id, UserId: 42, Name: "clip-" + strconv.FormatInt(id, 10)})
	}
	reaction := &clipExportReactionStub{collections: collections, items: map[int64][]*reactionpb.CollectionItemInfo{}}
	h := NewHandler(&clients.Clients{Reaction: reaction, Content: &clipExportContentStub{}, User: &clipExportUserStub{}}, "Authorization", "Bearer", testJWTSecret)

	payload, err := h.buildClipExport(context.Background(), 42)

	require.NoError(t, err)
	var exported []clipExportRecord
	require.NoError(t, json.Unmarshal(payload, &exported))
	require.Len(t, exported, 101)
	require.Equal(t, "1", exported[0].ID)
	require.Equal(t, "101", exported[100].ID)
	require.Empty(t, exported[0].ClipNotes)
	require.Equal(t, []int64{0, 100}, reaction.collectionAfterIDs)

	empty := &clipExportReactionStub{items: map[int64][]*reactionpb.CollectionItemInfo{}}
	h = NewHandler(&clients.Clients{Reaction: empty, Content: &clipExportContentStub{}, User: &clipExportUserStub{}}, "Authorization", "Bearer", testJWTSecret)
	payload, err = h.buildClipExport(context.Background(), 42)
	require.NoError(t, err)
	require.JSONEq(t, `[]`, string(payload))
}

func TestExportClipsRegistersFileAndIgnoresNotificationFailure(t *testing.T) {
	gin.SetMode(gin.TestMode)
	reaction := &clipExportReactionStub{collections: []*reactionpb.CollectionInfo{
		{Id: 2, UserId: 42, Name: "private", IsPublic: false},
		{Id: 1, UserId: 42, Name: "public", IsPublic: true},
	}, items: map[int64][]*reactionpb.CollectionItemInfo{}}
	fileClient := &fakeUserFileClient{createResp: &filepb.FileResponse{File: &filepb.File{Id: 9001, OwnerId: 42}}}
	notifications := &clipExportNotificationStub{err: status.Error(codes.Unavailable, "notification down")}
	store := newFakeUserFileStore()
	permit := &clipExportPermitStub{}
	h := NewHandlerWithAttachmentStore(&clients.Clients{
		Reaction: reaction, Content: &clipExportContentStub{}, User: &clipExportUserStub{},
		File: fileClient, NotificationInternal: notifications,
	}, "Authorization", "Bearer", testJWTSecret, store)
	h.SetClipExportGate(&clipExportGateStub{permit: permit})

	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(stdhttp.MethodPost, "/i/export-clips", strings.NewReader(`{}`))
	c.Set("user_id", int64(42))
	h.exportClips(c)

	require.Equal(t, stdhttp.StatusNoContent, c.Writer.Status(), recorder.Body.String())
	require.True(t, permit.committed)
	require.False(t, permit.released)
	require.Equal(t, int64(42), fileClient.createReq.GetOwnerId())
	require.Equal(t, "exports", fileClient.createReq.GetBizType())
	require.Equal(t, "application/json", fileClient.createReq.GetContentType())
	require.Regexp(t, regexp.MustCompile(`^clips-\d{4}-\d{2}-\d{2}-\d{2}-\d{2}-\d{2}\.json$`), fileClient.createReq.GetOriginalName())
	require.Equal(t, int64(42), notifications.req.GetRecipientId())
	require.Equal(t, int64(9001), notifications.req.GetFileId())
	require.Equal(t, "clip", notifications.req.GetExportedEntity())
	require.Len(t, store.objects, 1)
	for _, object := range store.objects {
		var raw []map[string]any
		require.NoError(t, json.Unmarshal(object.data, &raw))
		require.Equal(t, []string{"public", "private"}, []string{raw[0]["name"].(string), raw[1]["name"].(string)})
		_, hasPublicFlag := raw[1]["isPublic"]
		require.False(t, hasPublicFlag)
	}
}

func TestExportClipsCleansObjectAndReleasesPermitWhenFileRegistrationFails(t *testing.T) {
	gin.SetMode(gin.TestMode)
	fileClient := &fakeUserFileClient{createErr: status.Error(codes.InvalidArgument, "invalid metadata")}
	store := newFakeUserFileStore()
	permit := &clipExportPermitStub{}
	h := NewHandlerWithAttachmentStore(&clients.Clients{
		Reaction: &clipExportReactionStub{items: map[int64][]*reactionpb.CollectionItemInfo{}}, Content: &clipExportContentStub{},
		User: &clipExportUserStub{}, File: fileClient,
	}, "Authorization", "Bearer", testJWTSecret, store)
	h.SetClipExportGate(&clipExportGateStub{permit: permit})

	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(stdhttp.MethodPost, "/i/export-clips", strings.NewReader(`{}`))
	c.Set("user_id", int64(42))
	h.exportClips(c)

	require.Equal(t, stdhttp.StatusBadRequest, recorder.Code)
	require.Empty(t, store.objects)
	require.Len(t, store.deletedKeys, 1)
	require.True(t, permit.released)
	require.False(t, permit.committed)
}

func TestExportClipsPreservesObjectWhenFileRegistrationOutcomeIsUnknown(t *testing.T) {
	gin.SetMode(gin.TestMode)
	for _, code := range []codes.Code{codes.Unavailable, codes.DeadlineExceeded} {
		t.Run(code.String(), func(t *testing.T) {
			fileClient := &fakeUserFileClient{createErr: status.Error(code, "file response unavailable")}
			store := newFakeUserFileStore()
			permit := &clipExportPermitStub{}
			h := NewHandlerWithAttachmentStore(&clients.Clients{
				Reaction: &clipExportReactionStub{items: map[int64][]*reactionpb.CollectionItemInfo{}}, Content: &clipExportContentStub{},
				User: &clipExportUserStub{}, File: fileClient,
			}, "Authorization", "Bearer", testJWTSecret, store)
			h.SetClipExportGate(&clipExportGateStub{permit: permit})

			recorder := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(recorder)
			c.Request = httptest.NewRequest(stdhttp.MethodPost, "/i/export-clips", strings.NewReader(`{}`))
			c.Set("user_id", int64(42))
			h.exportClips(c)

			require.NotEqual(t, stdhttp.StatusNoContent, recorder.Code)
			require.Len(t, store.objects, 1)
			require.Empty(t, store.deletedKeys)
			require.True(t, permit.released)
			require.False(t, permit.committed)
		})
	}
}

func TestExportClipsCleansObjectWhenFileRegistrationResponseHasNoFile(t *testing.T) {
	gin.SetMode(gin.TestMode)
	store := newFakeUserFileStore()
	permit := &clipExportPermitStub{}
	h := NewHandlerWithAttachmentStore(&clients.Clients{
		Reaction: &clipExportReactionStub{items: map[int64][]*reactionpb.CollectionItemInfo{}}, Content: &clipExportContentStub{},
		User: &clipExportUserStub{}, File: &fakeUserFileClient{createResp: &filepb.FileResponse{}},
	}, "Authorization", "Bearer", testJWTSecret, store)
	h.SetClipExportGate(&clipExportGateStub{permit: permit})

	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(stdhttp.MethodPost, "/i/export-clips", strings.NewReader(`{}`))
	c.Set("user_id", int64(42))
	h.exportClips(c)

	require.Equal(t, stdhttp.StatusBadGateway, recorder.Code)
	require.Empty(t, store.objects)
	require.Len(t, store.deletedKeys, 1)
	require.True(t, permit.released)
	require.False(t, permit.committed)
}

func TestExportClipsTreatsDeliveredFileAsSuccessWhenRateLimitCommitFails(t *testing.T) {
	gin.SetMode(gin.TestMode)
	requestCtx, cancelRequest := context.WithCancel(context.Background())
	defer cancelRequest()
	fileClient := &fakeUserFileClient{createResp: &filepb.FileResponse{File: &filepb.File{Id: 9001, OwnerId: 42}}, onCreate: cancelRequest}
	notifications := &clipExportNotificationStub{}
	store := newFakeUserFileStore()
	permit := &clipExportPermitStub{commitErr: errors.New("redis unavailable")}
	h := NewHandlerWithAttachmentStore(&clients.Clients{
		Reaction: &clipExportReactionStub{items: map[int64][]*reactionpb.CollectionItemInfo{}}, Content: &clipExportContentStub{},
		User: &clipExportUserStub{}, File: fileClient, NotificationInternal: notifications,
	}, "Authorization", "Bearer", testJWTSecret, store)
	h.SetClipExportGate(&clipExportGateStub{permit: permit})

	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(stdhttp.MethodPost, "/i/export-clips", strings.NewReader(`{}`)).WithContext(requestCtx)
	c.Set("user_id", int64(42))
	h.exportClips(c)

	require.Equal(t, stdhttp.StatusNoContent, c.Writer.Status(), recorder.Body.String())
	require.True(t, permit.committed)
	require.NoError(t, permit.commitContextError)
	require.False(t, permit.released)
	require.NotNil(t, notifications.req)
	require.NoError(t, notifications.contextError)
	require.Len(t, store.objects, 1)
}

func TestExportClipsMapsConcurrentAttemptToRateLimit(t *testing.T) {
	gin.SetMode(gin.TestMode)
	h := &Handler{clipExportGate: &clipExportGateStub{err: errClipExportInProgress}}
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(stdhttp.MethodPost, "/i/export-clips", strings.NewReader(`{}`))
	c.Set("user_id", int64(42))

	_, ok := h.beginClipExport(c, 42)

	require.False(t, ok)
	require.Equal(t, stdhttp.StatusTooManyRequests, recorder.Code)
}

func TestExportClipsReleasesPermitWhenStorageFails(t *testing.T) {
	gin.SetMode(gin.TestMode)
	store := &clipExportFailingStore{uploadErr: errors.New("storage down")}
	permit := &clipExportPermitStub{}
	fileClient := &fakeUserFileClient{}
	h := NewHandlerWithAttachmentStore(&clients.Clients{
		Reaction: &clipExportReactionStub{items: map[int64][]*reactionpb.CollectionItemInfo{}}, Content: &clipExportContentStub{},
		User: &clipExportUserStub{}, File: fileClient,
	}, "Authorization", "Bearer", testJWTSecret, store)
	h.SetClipExportGate(&clipExportGateStub{permit: permit})

	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(stdhttp.MethodPost, "/i/export-clips", strings.NewReader(`{}`))
	c.Set("user_id", int64(42))
	h.exportClips(c)

	require.Equal(t, stdhttp.StatusBadGateway, recorder.Code)
	require.Nil(t, fileClient.createReq)
	require.True(t, permit.released)
}

func TestClipExportRequiresInteractiveSession(t *testing.T) {
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(stdhttp.MethodPost, "/i/export-clips", nil)
	c.Set("auth_token_type", apiTokenType)

	(&Handler{}).requireInteractiveAuth()(c)

	require.True(t, c.IsAborted())
	require.Equal(t, stdhttp.StatusForbidden, recorder.Code)
}

func TestClipExportRoutesApplyInteractiveAuthenticationAndRateLimit(t *testing.T) {
	gin.SetMode(gin.TestMode)
	for _, path := range []string{"/i/export-clips", "/api/i/export-clips", "/api/v1/i/export-clips"} {
		t.Run(path, func(t *testing.T) {
			h := NewHandlerWithAttachmentStore(&clients.Clients{
				Reaction: &clipExportReactionStub{}, Content: &clipExportContentStub{}, User: &clipExportUserStub{}, File: &fakeUserFileClient{},
			}, "Authorization", "Bearer", testJWTSecret, newFakeUserFileStore())
			h.SetClipExportGate(&clipExportGateStub{err: errClipExportRateLimited})
			router := gin.New()
			NewInitControllers(h)(router)
			request := httptest.NewRequest(stdhttp.MethodPost, path, strings.NewReader(`{}`))
			request.Header.Set("Authorization", "Bearer "+signedAuthToken(t, jwt.MapClaims{"sub": "42"}))
			request.Header.Set("Content-Type", "application/json")
			recorder := httptest.NewRecorder()

			router.ServeHTTP(recorder, request)

			require.Equal(t, stdhttp.StatusTooManyRequests, recorder.Code, recorder.Body.String())
		})
	}

	h := NewHandlerWithAttachmentStore(&clients.Clients{
		Reaction: &clipExportReactionStub{}, Content: &clipExportContentStub{}, User: &clipExportUserStub{}, File: &fakeUserFileClient{},
	}, "Authorization", "Bearer", testJWTSecret, newFakeUserFileStore())
	h.SetClipExportGate(&clipExportGateStub{err: errClipExportRateLimited})
	router := gin.New()
	NewInitControllers(h)(router)
	request := httptest.NewRequest(stdhttp.MethodPost, "/api/v1/i/export-clips", strings.NewReader(`{}`))
	request.Header.Set("Authorization", "Bearer "+signedAuthToken(t, jwt.MapClaims{
		"sub": "42", "jti": "clip-export-api-token", "exp": time.Now().Add(time.Hour).Unix(),
		credentialVersionClaim: "0", "token_type": apiTokenType, "scopes": []string{"read"},
	}))
	request.Header.Set("Content-Type", "application/json")
	recorder := httptest.NewRecorder()

	router.ServeHTTP(recorder, request)

	require.Equal(t, stdhttp.StatusForbidden, recorder.Code, recorder.Body.String())
}

type clipExportReactionStub struct {
	reactionpb.ReactionServiceClient
	collections        []*reactionpb.CollectionInfo
	items              map[int64][]*reactionpb.CollectionItemInfo
	collectionOffsets  []int32
	itemOffsets        []int32
	collectionAfterIDs []int64
	itemAfterIDs       []int64
}

func (s *clipExportReactionStub) ListCollections(_ context.Context, req *reactionpb.ListCollectionsRequest, _ ...grpc.CallOption) (*reactionpb.ListCollectionsResponse, error) {
	if req.GetAscendingById() {
		s.collectionAfterIDs = append(s.collectionAfterIDs, req.GetAfterId())
		items := clipExportCollectionPageAfterID(s.collections, req.GetAfterId(), req.GetLimit())
		return &reactionpb.ListCollectionsResponse{Items: items, Total: int64(len(s.collections))}, nil
	}
	s.collectionOffsets = append(s.collectionOffsets, req.GetOffset())
	items := clipExportPage(s.collections, req.GetOffset(), req.GetLimit())
	return &reactionpb.ListCollectionsResponse{Items: items, Total: int64(len(s.collections))}, nil
}

func (s *clipExportReactionStub) ListCollectionItems(_ context.Context, req *reactionpb.ListCollectionItemsRequest, _ ...grpc.CallOption) (*reactionpb.CollectionItemsResponse, error) {
	if req.GetAscendingById() {
		s.itemAfterIDs = append(s.itemAfterIDs, req.GetAfterId())
		all := s.items[req.GetCollectionId()]
		items := clipExportItemPageAfterID(all, req.GetAfterId(), req.GetLimit())
		return &reactionpb.CollectionItemsResponse{Items: items, Total: int64(len(all))}, nil
	}
	s.itemOffsets = append(s.itemOffsets, req.GetOffset())
	all := s.items[req.GetCollectionId()]
	items := clipExportPage(all, req.GetOffset(), req.GetLimit())
	return &reactionpb.CollectionItemsResponse{Items: items, Total: int64(len(all))}, nil
}

func clipExportCollectionPageAfterID(items []*reactionpb.CollectionInfo, afterID int64, limit int32) []*reactionpb.CollectionInfo {
	filtered := make([]*reactionpb.CollectionInfo, 0, len(items))
	for _, item := range items {
		if item.GetId() > afterID {
			filtered = append(filtered, item)
		}
	}
	sort.Slice(filtered, func(i, j int) bool { return filtered[i].GetId() < filtered[j].GetId() })
	if len(filtered) > int(limit) {
		filtered = filtered[:limit]
	}
	return filtered
}

func clipExportItemPageAfterID(items []*reactionpb.CollectionItemInfo, afterID int64, limit int32) []*reactionpb.CollectionItemInfo {
	filtered := make([]*reactionpb.CollectionItemInfo, 0, len(items))
	for _, item := range items {
		if item.GetId() > afterID {
			filtered = append(filtered, item)
		}
	}
	sort.Slice(filtered, func(i, j int) bool { return filtered[i].GetId() < filtered[j].GetId() })
	if len(filtered) > int(limit) {
		filtered = filtered[:limit]
	}
	return filtered
}

func clipExportPage[T any](items []T, offset, limit int32) []T {
	start := int(offset)
	if start >= len(items) {
		return nil
	}
	end := min(len(items), start+int(limit))
	return items[start:end]
}

type clipExportContentStub struct {
	contentpb.ContentServiceClient
	articles map[int64]*contentpb.ArticleInfo
	topics   map[int64]*contentpb.TopicInfo
}

func (s *clipExportContentStub) GetArticle(_ context.Context, req *contentpb.GetArticleRequest, _ ...grpc.CallOption) (*contentpb.ArticleResponse, error) {
	return &contentpb.ArticleResponse{Article: s.articles[req.GetId()]}, nil
}

func (s *clipExportContentStub) GetTopic(_ context.Context, req *contentpb.GetTopicRequest, _ ...grpc.CallOption) (*contentpb.TopicResponse, error) {
	return &contentpb.TopicResponse{Topic: s.topics[req.GetId()]}, nil
}

type clipExportUserStub struct {
	clients.UserClient
	users map[int64]*userpb.UserInfo
	calls int
}

func (s *clipExportUserStub) GetUser(_ context.Context, req *userpb.UserIDRequest, _ ...grpc.CallOption) (*userpb.UserResponse, error) {
	s.calls++
	return &userpb.UserResponse{User: s.users[req.GetId()]}, nil
}

type clipExportNotificationStub struct {
	notificationpb.InternalNotificationServiceClient
	req          *notificationpb.CreateExportCompletedNotificationRequest
	err          error
	contextError error
}

func (s *clipExportNotificationStub) CreateExportCompletedNotification(ctx context.Context, req *notificationpb.CreateExportCompletedNotificationRequest, _ ...grpc.CallOption) (*notificationpb.MutationResponse, error) {
	s.req = req
	s.contextError = ctx.Err()
	if s.err != nil {
		return nil, s.err
	}
	return &notificationpb.MutationResponse{Success: true}, nil
}

type clipExportGateStub struct {
	permit *clipExportPermitStub
	err    error
}

func (s *clipExportGateStub) Begin(context.Context, int64) (ClipExportPermit, error) {
	if s.err != nil {
		return nil, s.err
	}
	return s.permit, nil
}

type clipExportPermitStub struct {
	commitErr          error
	commitContextError error
	committed          bool
	released           bool
}

func (s *clipExportPermitStub) Commit(ctx context.Context) error {
	s.committed = true
	s.commitContextError = ctx.Err()
	return s.commitErr
}

func (s *clipExportPermitStub) Release(context.Context) error {
	s.released = true
	return nil
}

type clipExportFailingStore struct {
	uploadErr error
}

func (s *clipExportFailingStore) Upload(context.Context, string, io.Reader, int64, string) error {
	return s.uploadErr
}

func (*clipExportFailingStore) Open(context.Context, string) (io.ReadCloser, storage.ObjectInfo, error) {
	return nil, storage.ObjectInfo{}, storage.ErrObjectNotFound
}

func (*clipExportFailingStore) Delete(context.Context, string) error { return nil }

var _ storage.ObjectStore = (*clipExportFailingStore)(nil)
