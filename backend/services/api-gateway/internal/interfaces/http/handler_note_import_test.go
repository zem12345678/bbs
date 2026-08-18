package http

import (
	"archive/zip"
	"bytes"
	"context"
	stdhttp "net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"api-gateway/api/proto/contentpb"
	"api-gateway/api/proto/filepb"
	"api-gateway/api/proto/userpb"
	"api-gateway/internal/clients"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
)

func TestParseMisskeyNoteImportMapsPollAndSkipsRenotes(t *testing.T) {
	payload := []byte(`[
  {"id":"n1","text":"hello\nworld","visibility":"public","tags":["one","#One"],"poll":{"multiple":true,"choices":["A","B"],"expiresAt":"2099-01-01T00:00:00Z"}},
  {"id":"n2","text":"private","visibility":"followers"},
  {"id":"n3","text":"renote","renoteId":"original"},
  {"id":"n4","text":null}
]`)

	records, err := parseMisskeyNoteImport(payload)

	require.NoError(t, err)
	require.Len(t, records, 2)
	require.Equal(t, "n1", records[0].ID)
	require.Equal(t, "public", records[0].Visibility)
	require.Equal(t, []string{"one"}, records[0].Tags)
	require.NotNil(t, records[0].Poll)
	require.Equal(t, []string{"A", "B"}, records[0].Poll.Choices)
	require.Equal(t, "followers", records[1].Visibility)
}

func TestParseActivityPubNoteImportConvertsHTMLAndVisibility(t *testing.T) {
	payload := []byte(`{"orderedItems":[{"type":"Create","object":{"id":"ap1","type":"Note","content":"<p>Hello &amp; world</p><p>Next</p>","summary":"CW","sensitive":true,"to":["https://www.w3.org/ns/activitystreams#Public"],"tag":[{"type":"Hashtag","name":"#bbs"}]}}]}`)

	records, err := parseActivityPubNoteImport(payload)

	require.NoError(t, err)
	require.Len(t, records, 1)
	require.Equal(t, "Hello & world\nNext", records[0].Text)
	require.Equal(t, "CW", records[0].CW)
	require.Equal(t, "public", records[0].Visibility)
	require.Equal(t, []string{"bbs"}, records[0].Tags)
}

func TestParseTwitterNoteImportReadsNestedArchiveEntry(t *testing.T) {
	var payload bytes.Buffer
	archive := zip.NewWriter(&payload)
	entry, err := archive.Create("account/data/tweets.js")
	require.NoError(t, err)
	_, err = entry.Write([]byte(`window.YTD.tweets.part0 = [{"tweet":{"id_str":"7","full_text":"hello #bbs","entities":{"hashtags":[{"text":"bbs"}]}}}];`))
	require.NoError(t, err)
	require.NoError(t, archive.Close())

	records, err := parseNoteImportRecords(payload.Bytes(), "Twitter")

	require.NoError(t, err)
	require.Len(t, records, 1)
	require.Equal(t, "7", records[0].ID)
	require.Equal(t, "hello #bbs", records[0].Text)
	require.Equal(t, []string{"bbs"}, records[0].Tags)
}

func TestParseInstagramNoteImportUsesPostAndSingleMediaTitles(t *testing.T) {
	payload := []byte(`[
  {"title":"multi post","media":[{"title":"attachment one"},{"title":"attachment two"}]},
  {"title":"container title","media":[{"title":"single media"}]},
  {"title":"","media":[{"title":"metadata one"},{"title":"metadata two"}]}
]`)

	records, err := parseInstagramNoteImport(payload)

	require.NoError(t, err)
	require.Len(t, records, 2)
	require.Equal(t, "multi post", records[0].Text)
	require.Equal(t, "single media", records[1].Text)
}

func TestParseFacebookNoteImportOnlyUsesPostText(t *testing.T) {
	payload := []byte(`[
  {"data":[{"post":"hello \u00e4\u00bd\u00a0\u00e5\u00a5\u00bd"}],"attachments":[{"data":[{"media":{"description":"attachment metadata"}}]}]},
  {"data":[],"attachments":[{"data":[{"media":{"description":"not a post"}}]}]}
]`)

	records, err := parseFacebookNoteImport(payload)

	require.NoError(t, err)
	require.Len(t, records, 1)
	require.Equal(t, "hello \u4f60\u597d", records[0].Text)
}

func TestApplyNoteImportCreatesArticleAndTopicAndKeepsPrivateAsDraft(t *testing.T) {
	content := &noteImportContentStub{nextID: 100}
	h := NewHandler(&clients.Clients{Content: content}, "Authorization", "Bearer", testJWTSecret)

	result, err := h.applyNoteImport(context.Background(), 42, []noteImportRecord{
		{ID: "article", Text: "plain text", Visibility: "public"},
		{ID: "poll", Text: "poll text", Visibility: "followers", Poll: &noteImportPoll{Multiple: true, Choices: []string{"A", "B"}}},
	})

	require.NoError(t, err)
	require.Equal(t, noteImportResult{Imported: 2, Drafts: 1}, result)
	require.Len(t, content.articleRequests, 1)
	require.Equal(t, "plain text", content.articleRequests[0].GetBody())
	require.Len(t, content.topicRequests, 1)
	require.True(t, content.topicRequests[0].GetPoll().GetEnabled())
	require.Equal(t, []int64{101}, content.publishedArticles)
	require.Empty(t, content.publishedTopics)
}

func TestImportNoteRoutesRequireInteractiveAuthAndUseRateLimit(t *testing.T) {
	gin.SetMode(gin.TestMode)
	for _, path := range []string{"/i/import-notes", "/api/i/import-notes", "/api/v1/i/import-notes"} {
		t.Run(path, func(t *testing.T) {
			limiter := &noteImportLimiterStub{limited: true}
			h := newNoteImportTestHandler()
			h.SetNoteImportLimit(limiter)
			router := gin.New()
			NewInitControllers(h)(router)
			request := httptest.NewRequest(stdhttp.MethodPost, path, strings.NewReader(`{"fileId":"7","type":"Misskey"}`))
			request.Header.Set("Authorization", "Bearer "+signedAuthToken(t, jwt.MapClaims{"sub": "42"}))
			request.Header.Set("Content-Type", "application/json")
			recorder := httptest.NewRecorder()

			router.ServeHTTP(recorder, request)

			require.Equal(t, stdhttp.StatusTooManyRequests, recorder.Code, recorder.Body.String())
			require.Equal(t, []string{noteImportRateLimitKey(42)}, limiter.keys)
		})
	}

	limiter := &noteImportLimiterStub{limited: true}
	h := newNoteImportTestHandler()
	h.SetNoteImportLimit(limiter)
	router := gin.New()
	NewInitControllers(h)(router)
	request := httptest.NewRequest(stdhttp.MethodPost, "/api/v1/i/import-notes", strings.NewReader(`{"fileId":"7","type":"Misskey"}`))
	request.Header.Set("Authorization", "Bearer "+signedAuthToken(t, jwt.MapClaims{
		"sub": "42", "jti": "note-import-api-token", "exp": 4102444800,
		credentialVersionClaim: "0", "token_type": apiTokenType, "scopes": []string{"write"},
	}))
	request.Header.Set("Content-Type", "application/json")
	recorder := httptest.NewRecorder()

	router.ServeHTTP(recorder, request)

	require.Equal(t, stdhttp.StatusForbidden, recorder.Code, recorder.Body.String())
	require.Empty(t, limiter.keys)
}

func TestImportNotesReadsOwnedFileAndReturnsStats(t *testing.T) {
	payload := []byte(`[{"id":"n1","text":"hello","visibility":"public"}]`)
	store := newFakeUserFileStore()
	store.objects["imports/42/notes.json"] = fakeUserFileObject{data: payload, contentType: "application/json"}
	files := &fakeUserFileClient{getResp: &filepb.FileResponse{File: &filepb.File{Id: 7, OwnerId: 42, ObjectKey: "imports/42/notes.json", SizeBytes: int64(len(payload)), Status: "ACTIVE"}}}
	content := &noteImportContentStub{nextID: 10}
	users := &noteImportUserStub{user: &userpb.UserInfo{Id: 42, Status: userStatusActive, AccountState: "active", EmailVerified: true}}
	limiter := &noteImportLimiterStub{}
	h := NewHandlerWithAttachmentStore(&clients.Clients{File: files, Content: content, User: users}, "Authorization", "Bearer", testJWTSecret, store)
	h.SetNoteImportLimit(limiter)

	recorder := performNoteImport(h, `{"fileId":"7","type":"Misskey"}`, 42)

	require.Equal(t, stdhttp.StatusOK, recorder.Code, recorder.Body.String())
	require.Equal(t, int64(42), files.getReq.GetOwnerId())
	require.Equal(t, []string{"imports/42/notes.json"}, store.openKeys)
	require.Equal(t, []string{noteImportRateLimitKey(42)}, limiter.keys)
	require.Len(t, content.articleRequests, 1)
	require.Equal(t, []int64{11}, content.publishedArticles)
	require.Contains(t, recorder.Body.String(), `"imported":1`)
}

func newNoteImportTestHandler() *Handler {
	h := NewHandlerWithAttachmentStore(&clients.Clients{
		File: &fakeUserFileClient{}, Content: &noteImportContentStub{}, User: &noteImportUserStub{},
	}, "Authorization", "Bearer", testJWTSecret, newFakeUserFileStore())
	h.SetPublicBaseURL("https://bbs.example.com")
	return h
}

func performNoteImport(h *Handler, body string, ownerID int64) *httptest.ResponseRecorder {
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(stdhttp.MethodPost, "/i/import-notes", strings.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")
	c.Set("user_id", ownerID)
	h.importNotes(c)
	c.Writer.WriteHeaderNow()
	return recorder
}

type noteImportContentStub struct {
	contentpb.ContentServiceClient
	nextID            int64
	articleRequests   []*contentpb.CreateArticleRequest
	topicRequests     []*contentpb.CreateTopicRequest
	publishedArticles []int64
	publishedTopics   []int64
}

func (s *noteImportContentStub) CreateArticle(_ context.Context, request *contentpb.CreateArticleRequest, _ ...grpc.CallOption) (*contentpb.ArticleResponse, error) {
	s.articleRequests = append(s.articleRequests, request)
	s.nextID++
	return &contentpb.ArticleResponse{Article: &contentpb.ArticleInfo{Id: s.nextID, AuthorId: request.GetAuthorId()}}, nil
}

func (s *noteImportContentStub) PublishArticle(_ context.Context, request *contentpb.ArticleIDRequest, _ ...grpc.CallOption) (*contentpb.ArticleResponse, error) {
	s.publishedArticles = append(s.publishedArticles, request.GetId())
	return &contentpb.ArticleResponse{Article: &contentpb.ArticleInfo{Id: request.GetId()}}, nil
}

func (s *noteImportContentStub) CreateTopic(_ context.Context, request *contentpb.CreateTopicRequest, _ ...grpc.CallOption) (*contentpb.TopicResponse, error) {
	s.topicRequests = append(s.topicRequests, request)
	s.nextID++
	return &contentpb.TopicResponse{Topic: &contentpb.TopicInfo{Id: s.nextID, AuthorId: request.GetAuthorId()}}, nil
}

func (s *noteImportContentStub) PublishTopic(_ context.Context, request *contentpb.TopicIDRequest, _ ...grpc.CallOption) (*contentpb.TopicResponse, error) {
	s.publishedTopics = append(s.publishedTopics, request.GetId())
	return &contentpb.TopicResponse{Topic: &contentpb.TopicInfo{Id: request.GetId()}}, nil
}

type noteImportUserStub struct {
	clients.UserClient
	user *userpb.UserInfo
}

func (s *noteImportUserStub) GetUser(_ context.Context, _ *userpb.UserIDRequest, _ ...grpc.CallOption) (*userpb.UserResponse, error) {
	return &userpb.UserResponse{User: s.user}, nil
}

type noteImportLimiterStub struct {
	limited bool
	keys    []string
}

func (s *noteImportLimiterStub) Limit(_ context.Context, key string) (bool, error) {
	s.keys = append(s.keys, key)
	return s.limited, nil
}
