package http

import (
	"bytes"
	"context"
	"encoding/json"
	stdhttp "net/http"
	"net/http/httptest"
	"testing"

	"api-gateway/api/proto/contentpb"
	"api-gateway/api/proto/userpb"
	"api-gateway/internal/clients"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func TestNotesShowCompatMapsPublishedArticleAndTopicAliases(t *testing.T) {
	gin.SetMode(gin.TestMode)
	content := &notesCompatContentClient{
		article: &contentpb.ArticleInfo{Id: 900, Title: "Article title", Body: "Article body", Summary: "summary", AuthorId: 42, CreatedAt: 1700000000000, Status: contentStatusPublished, Tags: []string{"go"}},
		topic:   &contentpb.TopicInfo{Id: 901, Title: "Topic title", Body: "Topic body", AuthorId: 42, CreatedAt: 1700000001000, Status: contentStatusPublished, Tags: []string{"bbs"}},
	}
	h := NewHandler(&clients.Clients{Content: content, User: &notesCompatUserClient{}}, "Authorization", "Bearer", testJWTSecret)
	router := gin.New()
	NewInitControllers(h)(router)

	for _, path := range []string{"/notes/show", "/api/notes/show", "/api/v1/notes/show"} {
		t.Run(path, func(t *testing.T) {
			response := performNotesShowRequest(router, path, `{"noteId":"900"}`)
			require.Equal(t, stdhttp.StatusOK, response.Code, response.Body.String())
			var payload map[string]any
			require.NoError(t, json.Unmarshal(response.Body.Bytes(), &payload))
			require.Equal(t, "900", payload["id"])
			require.Equal(t, "Article body", payload["text"])
			require.Equal(t, "42", payload["userId"])
			require.NotContains(t, payload, "data")
		})
	}
	topicResponse := performNotesShowRequest(router, "/notes/show", `{"noteId":"901"}`)
	require.Equal(t, stdhttp.StatusOK, topicResponse.Code, topicResponse.Body.String())
	var topicPayload map[string]any
	require.NoError(t, json.Unmarshal(topicResponse.Body.Bytes(), &topicPayload))
	require.Equal(t, "901", topicPayload["id"])
	require.Equal(t, "Topic body", topicPayload["text"])
}

func TestNotesShowCompatMapsNoSuchAndUnknownRequest(t *testing.T) {
	content := &notesCompatContentClient{}
	h := NewHandler(&clients.Clients{Content: content, User: &notesCompatUserClient{}}, "Authorization", "Bearer", testJWTSecret)
	router := gin.New()
	NewInitControllers(h)(router)

	unknown := performNotesShowRequest(router, "/api/v1/notes/show", `{"noteId":"900","extra":true}`)
	require.Equal(t, stdhttp.StatusBadRequest, unknown.Code, unknown.Body.String())
	require.Contains(t, unknown.Body.String(), "INVALID_PARAM")
	missing := performNotesShowRequest(router, "/api/v1/notes/show", `{"noteId":"900"}`)
	require.Equal(t, stdhttp.StatusBadRequest, missing.Code, missing.Body.String())
	require.Contains(t, missing.Body.String(), "NO_SUCH_NOTE")
	require.Contains(t, missing.Body.String(), notesShowNoSuchNoteID)
}

func TestNotesCreateCompatPublishesTweetAndReturnsRawCreatedNote(t *testing.T) {
	content := &notesCompatContentClient{
		createdTopic: &contentpb.TopicInfo{Id: 902, Type: "tweet", Body: "hello #go", Tags: []string{"go"}, AuthorId: 42, CreatedAt: 1700000002000, Status: contentStatusPublished},
	}
	h := NewHandler(&clients.Clients{Content: content, User: &notesCompatUserClient{}}, "Authorization", "Bearer", testJWTSecret)
	router := gin.New()
	NewInitControllers(h)(router)

	recorder := performNotesCompatRequest(router, stdhttp.MethodPost, "/api/v1/notes/create", `{"text":"hello #go"}`, signedAuthToken(t, mapClaimsForNotesTest()))
	require.Equal(t, stdhttp.StatusOK, recorder.Code, recorder.Body.String())
	var payload map[string]any
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &payload))
	created, ok := payload["createdNote"].(map[string]any)
	require.True(t, ok)
	require.Equal(t, "902", created["id"])
	require.Equal(t, "hello #go", created["text"])
	require.Equal(t, []any{"go"}, created["tags"])
	require.Equal(t, "tweet", content.createRequest.GetType())
	require.Equal(t, int64(42), content.createRequest.GetAuthorId())
	require.NotEmpty(t, content.createRequest.GetSlug())
	require.Equal(t, int64(902), content.publishedID)
}

func TestNotesCreateCompatRejectsUnsupportedOptionsAndAnonymousRequests(t *testing.T) {
	h := NewHandler(&clients.Clients{Content: &notesCompatContentClient{}, User: &notesCompatUserClient{}}, "Authorization", "Bearer", testJWTSecret)
	router := gin.New()
	NewInitControllers(h)(router)

	unsupported := performNotesCompatRequest(router, stdhttp.MethodPost, "/notes/create", `{"text":"hello","visibility":"followers"}`, signedAuthToken(t, mapClaimsForNotesTest()))
	require.Equal(t, stdhttp.StatusBadRequest, unsupported.Code, unsupported.Body.String())
	require.Contains(t, unsupported.Body.String(), `"legacy_code":"INVALID_PARAM"`)

	anonymous := performNotesCompatRequest(router, stdhttp.MethodPost, "/notes/create", `{"text":"hello"}`, "")
	require.Equal(t, stdhttp.StatusUnauthorized, anonymous.Code, anonymous.Body.String())
}

func TestNotesDeleteCompatArchivesOwnedTweetAndReturnsNoContent(t *testing.T) {
	content := &notesCompatContentClient{
		topic: &contentpb.TopicInfo{Id: 903, Type: "tweet", Body: "remove me", AuthorId: 42, CreatedAt: 1700000003000, Status: contentStatusPublished},
	}
	h := NewHandler(&clients.Clients{Content: content}, "Authorization", "Bearer", testJWTSecret)
	router := gin.New()
	NewInitControllers(h)(router)

	recorder := performNotesCompatRequest(router, stdhttp.MethodPost, "/api/notes/delete", `{"noteId":"903"}`, signedAuthToken(t, mapClaimsForNotesTest()))
	require.Equal(t, stdhttp.StatusNoContent, recorder.Code, recorder.Body.String())
	require.Empty(t, recorder.Body.Bytes())
	require.Equal(t, int64(903), content.archivedTopicID)
}

func TestNotesDeleteCompatRejectsForeignAndMissingNotes(t *testing.T) {
	foreign := &notesCompatContentClient{
		topic: &contentpb.TopicInfo{Id: 904, Type: "tweet", Body: "not yours", AuthorId: 7, CreatedAt: 1700000004000, Status: contentStatusPublished},
	}
	h := NewHandler(&clients.Clients{Content: foreign}, "Authorization", "Bearer", testJWTSecret)
	router := gin.New()
	NewInitControllers(h)(router)

	denied := performNotesCompatRequest(router, stdhttp.MethodPost, "/api/v1/notes/delete", `{"noteId":"904"}`, signedAuthToken(t, mapClaimsForNotesTest()))
	require.Equal(t, stdhttp.StatusBadRequest, denied.Code, denied.Body.String())
	require.Contains(t, denied.Body.String(), `"legacy_code":"ACCESS_DENIED"`)

	missing := performNotesCompatRequest(router, stdhttp.MethodPost, "/api/v1/notes/delete", `{"noteId":"905"}`, signedAuthToken(t, mapClaimsForNotesTest()))
	require.Equal(t, stdhttp.StatusBadRequest, missing.Code, missing.Body.String())
	require.Contains(t, missing.Body.String(), `"legacy_code":"NO_SUCH_NOTE"`)
}

func TestNotesTimelineAndUserNotesCompatListPublishedTweets(t *testing.T) {
	content := &notesCompatContentClient{
		topics: []*contentpb.TopicInfo{
			{Id: 906, Type: "tweet", Body: "newer", AuthorId: 42, CreatedAt: 1700000006000, Status: contentStatusPublished},
			{Id: 907, Type: "tweet", Body: "older", AuthorId: 42, CreatedAt: 1700000005000, Status: contentStatusPublished},
			{Id: 908, Type: "topic", Body: "forum topic", AuthorId: 42, CreatedAt: 1700000007000, Status: contentStatusPublished},
		},
	}
	h := NewHandler(&clients.Clients{Content: content, User: &notesCompatUserClient{}}, "Authorization", "Bearer", testJWTSecret)
	router := gin.New()
	NewInitControllers(h)(router)

	timeline := performNotesCompatRequest(router, stdhttp.MethodPost, "/api/v1/notes/timeline", `{"limit":10,"sinceId":"905"}`, signedAuthToken(t, mapClaimsForNotesTest()))
	require.Equal(t, stdhttp.StatusOK, timeline.Code, timeline.Body.String())
	var timelineItems []map[string]any
	require.NoError(t, json.Unmarshal(timeline.Body.Bytes(), &timelineItems))
	require.Len(t, timelineItems, 2)
	require.Equal(t, "906", timelineItems[0]["id"])
	require.Equal(t, "907", timelineItems[1]["id"])
	require.Equal(t, "tweet", content.listTopicsRequest.GetType())

	userNotes := performNotesCompatRequest(router, stdhttp.MethodPost, "/users/notes", `{"userId":"42","untilDate":1700000006000}`, "")
	require.Equal(t, stdhttp.StatusOK, userNotes.Code, userNotes.Body.String())
	var userItems []map[string]any
	require.NoError(t, json.Unmarshal(userNotes.Body.Bytes(), &userItems))
	require.Len(t, userItems, 1)
	require.Equal(t, "907", userItems[0]["id"])
}

func TestUsersNotesCompatMapsMissingUserAndInvalidParameters(t *testing.T) {
	h := NewHandler(&clients.Clients{Content: &notesCompatContentClient{}, User: &notesCompatUserClient{}}, "Authorization", "Bearer", testJWTSecret)
	router := gin.New()
	NewInitControllers(h)(router)

	missing := performNotesCompatRequest(router, stdhttp.MethodPost, "/api/v1/users/notes", `{"userId":"99"}`, "")
	require.Equal(t, stdhttp.StatusBadRequest, missing.Code, missing.Body.String())
	require.Contains(t, missing.Body.String(), `"legacy_code":"NO_SUCH_USER"`)

	invalid := performNotesCompatRequest(router, stdhttp.MethodPost, "/api/v1/users/notes", `{"userId":"42","limit":101}`, "")
	require.Equal(t, stdhttp.StatusBadRequest, invalid.Code, invalid.Body.String())
	require.Contains(t, invalid.Body.String(), `"legacy_code":"INVALID_PARAM"`)
}

func performNotesCompatRequest(router stdhttp.Handler, method, path, body, token string) *httptest.ResponseRecorder {
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(method, path, bytes.NewBufferString(body))
	request.Header.Set("Content-Type", "application/json")
	if token != "" {
		request.Header.Set("Authorization", "Bearer "+token)
	}
	router.ServeHTTP(recorder, request)
	return recorder
}

func mapClaimsForNotesTest() jwt.MapClaims {
	return jwt.MapClaims{"sub": "42", "username": "alice"}
}

func performNotesShowRequest(router stdhttp.Handler, path, body string) *httptest.ResponseRecorder {
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(stdhttp.MethodPost, path, bytes.NewBufferString(body))
	request.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(recorder, request)
	return recorder
}

type notesCompatContentClient struct {
	contentpb.ContentServiceClient
	article           *contentpb.ArticleInfo
	topic             *contentpb.TopicInfo
	topics            []*contentpb.TopicInfo
	listTopicsRequest *contentpb.ListTopicsRequest
	createdTopic      *contentpb.TopicInfo
	createRequest     *contentpb.CreateTopicRequest
	publishedID       int64
	archivedTopicID   int64
}

func (client *notesCompatContentClient) CreateTopic(_ context.Context, request *contentpb.CreateTopicRequest, _ ...grpc.CallOption) (*contentpb.TopicResponse, error) {
	client.createRequest = request
	if client.createdTopic == nil {
		return nil, status.Error(codes.InvalidArgument, "create topic unavailable")
	}
	return &contentpb.TopicResponse{Success: true, Topic: client.createdTopic}, nil
}

func (client *notesCompatContentClient) PublishTopic(_ context.Context, request *contentpb.TopicIDRequest, _ ...grpc.CallOption) (*contentpb.TopicResponse, error) {
	client.publishedID = request.GetId()
	return &contentpb.TopicResponse{Success: true, Topic: client.createdTopic}, nil
}

func (client *notesCompatContentClient) ArchiveTopic(_ context.Context, request *contentpb.TopicIDRequest, _ ...grpc.CallOption) (*contentpb.TopicResponse, error) {
	client.archivedTopicID = request.GetId()
	return &contentpb.TopicResponse{Success: true}, nil
}

func (client *notesCompatContentClient) ListTopics(_ context.Context, request *contentpb.ListTopicsRequest, _ ...grpc.CallOption) (*contentpb.TopicListResponse, error) {
	client.listTopicsRequest = request
	if client.topics == nil {
		return &contentpb.TopicListResponse{}, nil
	}
	start := int(request.GetOffset())
	if start >= len(client.topics) {
		return &contentpb.TopicListResponse{Total: int64(len(client.topics))}, nil
	}
	end := start + int(request.GetLimit())
	if end <= start || end > len(client.topics) {
		end = len(client.topics)
	}
	return &contentpb.TopicListResponse{Items: client.topics[start:end], Total: int64(len(client.topics))}, nil
}

func (client *notesCompatContentClient) GetArticle(_ context.Context, request *contentpb.GetArticleRequest, _ ...grpc.CallOption) (*contentpb.ArticleResponse, error) {
	if client.article == nil || client.article.GetId() != request.GetId() {
		return nil, status.Error(codes.NotFound, "article not found")
	}
	return &contentpb.ArticleResponse{Success: true, Article: client.article}, nil
}

func (client *notesCompatContentClient) GetTopic(_ context.Context, request *contentpb.GetTopicRequest, _ ...grpc.CallOption) (*contentpb.TopicResponse, error) {
	if client.topic == nil || client.topic.GetId() != request.GetId() {
		return nil, status.Error(codes.NotFound, "topic not found")
	}
	return &contentpb.TopicResponse{Success: true, Topic: client.topic}, nil
}

type notesCompatUserClient struct {
	userpb.UserServiceClient
}

func (client *notesCompatUserClient) GetUser(_ context.Context, request *userpb.UserIDRequest, _ ...grpc.CallOption) (*userpb.UserResponse, error) {
	if request.GetId() != 42 {
		return nil, status.Error(codes.NotFound, "user not found")
	}
	return &userpb.UserResponse{Success: true, User: &userpb.UserInfo{Id: 42, Username: "alice", Nickname: "Alice"}}, nil
}

var _ clients.ContentClient = (*notesCompatContentClient)(nil)
var _ clients.UserClient = (*notesCompatUserClient)(nil)
