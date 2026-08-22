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

func performNotesShowRequest(router stdhttp.Handler, path, body string) *httptest.ResponseRecorder {
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(stdhttp.MethodPost, path, bytes.NewBufferString(body))
	request.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(recorder, request)
	return recorder
}

type notesCompatContentClient struct {
	contentpb.ContentServiceClient
	article *contentpb.ArticleInfo
	topic   *contentpb.TopicInfo
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
