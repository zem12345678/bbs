package http

import (
	"bytes"
	"context"
	"encoding/json"
	stdhttp "net/http"
	"net/http/httptest"
	"testing"

	"api-gateway/api/proto/contentpb"
	"api-gateway/api/proto/searchpb"
	"api-gateway/internal/clients"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func TestNotesSearchCompatMapsLocalArticleAndCursorFilters(t *testing.T) {
	gin.SetMode(gin.TestMode)
	search := &notesSearchCompatSearchClient{articleHits: []*searchpb.ArticleHit{{Article: &searchpb.ArticleDocument{Id: 900, AuthorId: 42, CreatedAt: 1700000000000}}}}
	content := &notesSearchCompatContentClient{article: &contentpb.ArticleInfo{Id: 900, Title: "Search result", Body: "body", AuthorId: 42, CreatedAt: 1700000000000, Status: contentStatusPublished, Tags: []string{"go"}}}
	h := NewHandler(&clients.Clients{Search: search, Content: content, User: &notesCompatUserClient{}}, "Authorization", "Bearer", testJWTSecret)
	router := gin.New()
	NewInitControllers(h)(router)

	response := performNotesSearchCompatRequest(router, "/api/v1/notes/search", `{"query":"body","sinceId":"10","untilId":"1000","limit":5,"offset":0,"host":"."}`)
	require.Equal(t, stdhttp.StatusOK, response.Code, response.Body.String())
	var payload []map[string]any
	require.NoError(t, json.Unmarshal(response.Body.Bytes(), &payload))
	require.Len(t, payload, 1)
	require.Equal(t, "900", payload[0]["id"])
	require.Equal(t, "body", payload[0]["text"])
}

func TestNotesSearchCompatRejectsUnsupportedRemoteAndFileFilters(t *testing.T) {
	search := &notesSearchCompatSearchClient{}
	h := NewHandler(&clients.Clients{Search: search}, "Authorization", "Bearer", testJWTSecret)
	router := gin.New()
	NewInitControllers(h)(router)
	for _, body := range []string{`{"query":"x","host":"example.com"}`, `{"query":"x","filetype":"image"}`, `{"query":"x","sinceId":"0"}`} {
		response := performNotesSearchCompatRequest(router, "/notes/search", body)
		require.Equal(t, stdhttp.StatusBadRequest, response.Code, response.Body.String())
		require.Contains(t, response.Body.String(), "INVALID_PARAM")
	}
}

func performNotesSearchCompatRequest(router stdhttp.Handler, path, body string) *httptest.ResponseRecorder {
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(stdhttp.MethodPost, path, bytes.NewBufferString(body))
	request.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(recorder, request)
	return recorder
}

type notesSearchCompatSearchClient struct {
	searchpb.SearchServiceClient
	articleHits []*searchpb.ArticleHit
	topicHits   []*searchpb.TopicHit
}

func (client *notesSearchCompatSearchClient) SearchArticles(_ context.Context, _ *searchpb.SearchArticlesRequest, _ ...grpc.CallOption) (*searchpb.SearchArticlesResponse, error) {
	return &searchpb.SearchArticlesResponse{Items: client.articleHits, Total: int64(len(client.articleHits))}, nil
}

func (client *notesSearchCompatSearchClient) SearchTopics(_ context.Context, _ *searchpb.SearchTopicsRequest, _ ...grpc.CallOption) (*searchpb.SearchTopicsResponse, error) {
	return &searchpb.SearchTopicsResponse{Items: client.topicHits, Total: int64(len(client.topicHits))}, nil
}

type notesSearchCompatContentClient struct {
	contentpb.ContentServiceClient
	article *contentpb.ArticleInfo
	topic   *contentpb.TopicInfo
}

func (client *notesSearchCompatContentClient) GetArticle(_ context.Context, request *contentpb.GetArticleRequest, _ ...grpc.CallOption) (*contentpb.ArticleResponse, error) {
	if client.article == nil || client.article.GetId() != request.GetId() {
		return nil, status.Error(codes.NotFound, "article not found")
	}
	return &contentpb.ArticleResponse{Article: client.article}, nil
}

func (client *notesSearchCompatContentClient) GetTopic(_ context.Context, request *contentpb.GetTopicRequest, _ ...grpc.CallOption) (*contentpb.TopicResponse, error) {
	if client.topic == nil || client.topic.GetId() != request.GetId() {
		return nil, status.Error(codes.NotFound, "topic not found")
	}
	return &contentpb.TopicResponse{Topic: client.topic}, nil
}

var _ clients.SearchClient = (*notesSearchCompatSearchClient)(nil)
var _ clients.ContentClient = (*notesSearchCompatContentClient)(nil)
var _ clients.UserClient = (*notesCompatUserClient)(nil)
