package http

import (
	"context"
	"encoding/json"
	stdhttp "net/http"
	"net/http/httptest"
	"testing"

	"api-gateway/api/proto/contentpb"
	"api-gateway/api/proto/searchpb"
	"api-gateway/api/proto/userpb"
	"api-gateway/internal/clients"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func TestSearchUsersUsesPublicActiveUserQuery(t *testing.T) {
	gin.SetMode(gin.TestMode)
	userClient := &fakeUserClient{
		users: []*userpb.UserInfo{
			{Id: 101, Username: "alice", Nickname: "Alice"},
		},
	}
	h := NewHandler(&clients.Clients{User: userClient}, "Authorization", "Bearer", testJWTSecret)
	router := gin.New()
	NewInitControllers(h)(router)

	recorder := httptest.NewRecorder()
	req := httptest.NewRequest(stdhttp.MethodGet, "/api/v1/search/users?q=ali&page=2&page_size=7", nil)
	router.ServeHTTP(recorder, req)

	require.Equal(t, stdhttp.StatusOK, recorder.Code, recorder.Body.String())
	require.Equal(t, 1, userClient.listUsersCalls)
	require.Equal(t, "ali", userClient.listUsersReq.GetQuery())
	require.Equal(t, userStatusActive, userClient.listUsersReq.GetStatus())
	require.Equal(t, int32(2), userClient.listUsersReq.GetPage())
	require.Equal(t, int32(7), userClient.listUsersReq.GetPageSize())

	var envelope struct {
		Data userpb.UserListResponse `json:"data"`
	}
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &envelope))
	require.Len(t, envelope.Data.Items, 1)
	require.Equal(t, "alice", envelope.Data.Items[0].GetUsername())
}

func TestSearchUsersRequiresKeyword(t *testing.T) {
	gin.SetMode(gin.TestMode)
	userClient := &fakeUserClient{}
	h := NewHandler(&clients.Clients{User: userClient}, "Authorization", "Bearer", testJWTSecret)
	router := gin.New()
	NewInitControllers(h)(router)

	recorder := httptest.NewRecorder()
	req := httptest.NewRequest(stdhttp.MethodGet, "/api/v1/search/users?q=+%20", nil)
	router.ServeHTTP(recorder, req)

	require.Equal(t, stdhttp.StatusBadRequest, recorder.Code, recorder.Body.String())
	require.Zero(t, userClient.listUsersCalls)
}

func TestSearchArticlesFiltersStaleNonPublicDocuments(t *testing.T) {
	gin.SetMode(gin.TestMode)
	searchClient := &fakeSearchVisibilityClient{articleResponse: &searchpb.SearchArticlesResponse{
		Items: []*searchpb.ArticleHit{
			{Article: &searchpb.ArticleDocument{Id: 101, Title: "published"}},
			{Article: &searchpb.ArticleDocument{Id: 102, Title: "hidden"}},
			{Article: &searchpb.ArticleDocument{Id: 103, Title: "archived"}},
			{Article: &searchpb.ArticleDocument{Id: 104, Title: "deleted"}},
		},
		Total: 4,
	}}
	contentClient := &fakeSearchVisibilityContentClient{articles: map[int64]*contentpb.ArticleInfo{
		101: {Id: 101, Status: contentStatusPublished},
		102: {Id: 102, Status: 3},
		103: {Id: 103, Status: 4},
	}}
	h := NewHandler(&clients.Clients{Content: contentClient, Search: searchClient}, "Authorization", "Bearer", testJWTSecret)
	router := gin.New()
	NewInitControllers(h)(router)

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, httptest.NewRequest(stdhttp.MethodGet, "/api/v1/search/articles?q=content", nil))

	require.Equal(t, stdhttp.StatusOK, recorder.Code, recorder.Body.String())
	var envelope struct {
		Data searchpb.SearchArticlesResponse `json:"data"`
	}
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &envelope))
	require.Len(t, envelope.Data.Items, 1)
	require.EqualValues(t, 101, envelope.Data.Items[0].GetArticle().GetId())
	require.EqualValues(t, 1, envelope.Data.Total)
	require.Len(t, contentClient.articleRequests, 4)
	for _, request := range contentClient.articleRequests {
		require.False(t, request.GetTrackView())
	}
}

func TestSearchTopicsFiltersStaleNonPublicDocuments(t *testing.T) {
	gin.SetMode(gin.TestMode)
	searchClient := &fakeSearchVisibilityClient{topicResponse: &searchpb.SearchTopicsResponse{
		Items: []*searchpb.TopicHit{
			{Topic: &searchpb.TopicDocument{Id: 201, Title: "published"}},
			{Topic: &searchpb.TopicDocument{Id: 202, Title: "hidden"}},
			{Topic: &searchpb.TopicDocument{Id: 203, Title: "archived"}},
		},
		Total: 3,
	}}
	contentClient := &fakeSearchVisibilityContentClient{topics: map[int64]*contentpb.TopicInfo{
		201: {Id: 201, Status: contentStatusPublished},
		202: {Id: 202, Status: 3},
		203: {Id: 203, Status: 4},
	}}
	h := NewHandler(&clients.Clients{Content: contentClient, Search: searchClient}, "Authorization", "Bearer", testJWTSecret)
	router := gin.New()
	NewInitControllers(h)(router)

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, httptest.NewRequest(stdhttp.MethodGet, "/api/v1/search/topics?q=content", nil))

	require.Equal(t, stdhttp.StatusOK, recorder.Code, recorder.Body.String())
	var envelope struct {
		Data searchpb.SearchTopicsResponse `json:"data"`
	}
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &envelope))
	require.Len(t, envelope.Data.Items, 1)
	require.EqualValues(t, 201, envelope.Data.Items[0].GetTopic().GetId())
	require.EqualValues(t, 1, envelope.Data.Total)
	require.Len(t, contentClient.topicRequests, 3)
	for _, request := range contentClient.topicRequests {
		require.False(t, request.GetTrackView())
	}
}

func TestSearchArticlesFailsClosedWhenContentStatusCannotBeVerified(t *testing.T) {
	gin.SetMode(gin.TestMode)
	searchClient := &fakeSearchVisibilityClient{articleResponse: &searchpb.SearchArticlesResponse{
		Items: []*searchpb.ArticleHit{{Article: &searchpb.ArticleDocument{Id: 101, Title: "private title"}}},
		Total: 1,
	}}
	contentClient := &fakeSearchVisibilityContentClient{articleErr: status.Error(codes.Unavailable, "content service unavailable")}
	h := NewHandler(&clients.Clients{Content: contentClient, Search: searchClient}, "Authorization", "Bearer", testJWTSecret)
	router := gin.New()
	NewInitControllers(h)(router)

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, httptest.NewRequest(stdhttp.MethodGet, "/api/v1/search/articles?q=content", nil))

	require.Equal(t, stdhttp.StatusServiceUnavailable, recorder.Code, recorder.Body.String())
	require.NotContains(t, recorder.Body.String(), "private title")
}

type fakeSearchVisibilityClient struct {
	searchpb.SearchServiceClient
	articleResponse *searchpb.SearchArticlesResponse
	topicResponse   *searchpb.SearchTopicsResponse
	articleErr      error
	topicErr        error
}

func (f *fakeSearchVisibilityClient) SearchArticles(_ context.Context, _ *searchpb.SearchArticlesRequest, _ ...grpc.CallOption) (*searchpb.SearchArticlesResponse, error) {
	if f.articleErr != nil {
		return nil, f.articleErr
	}
	return f.articleResponse, nil
}

func (f *fakeSearchVisibilityClient) SearchTopics(_ context.Context, _ *searchpb.SearchTopicsRequest, _ ...grpc.CallOption) (*searchpb.SearchTopicsResponse, error) {
	if f.topicErr != nil {
		return nil, f.topicErr
	}
	return f.topicResponse, nil
}

type fakeSearchVisibilityContentClient struct {
	contentpb.ContentServiceClient
	articles        map[int64]*contentpb.ArticleInfo
	topics          map[int64]*contentpb.TopicInfo
	articleErr      error
	topicErr        error
	articleRequests []*contentpb.GetArticleRequest
	topicRequests   []*contentpb.GetTopicRequest
}

func (f *fakeSearchVisibilityContentClient) GetArticle(_ context.Context, request *contentpb.GetArticleRequest, _ ...grpc.CallOption) (*contentpb.ArticleResponse, error) {
	f.articleRequests = append(f.articleRequests, request)
	if f.articleErr != nil {
		return nil, f.articleErr
	}
	article, ok := f.articles[request.GetId()]
	if !ok {
		return nil, status.Error(codes.NotFound, "article not found")
	}
	return &contentpb.ArticleResponse{Success: true, Article: article}, nil
}

func (f *fakeSearchVisibilityContentClient) GetTopic(_ context.Context, request *contentpb.GetTopicRequest, _ ...grpc.CallOption) (*contentpb.TopicResponse, error) {
	f.topicRequests = append(f.topicRequests, request)
	if f.topicErr != nil {
		return nil, f.topicErr
	}
	topic, ok := f.topics[request.GetId()]
	if !ok {
		return nil, status.Error(codes.NotFound, "topic not found")
	}
	return &contentpb.TopicResponse{Success: true, Topic: topic}, nil
}
