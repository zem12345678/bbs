package http

import (
	"context"
	stdhttp "net/http"
	"net/http/httptest"
	"testing"

	"api-gateway/api/proto/contentpb"
	"api-gateway/internal/clients"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
)

func TestSitemapIndexUsesCanonicalURLAndPublishedContentTotals(t *testing.T) {
	gin.SetMode(gin.TestMode)
	content := &fakeSitemapContentClient{
		topics:   &contentpb.TopicListResponse{Total: 101},
		articles: &contentpb.ArticleListResponse{Total: 100},
	}
	router := sitemapTestRouter(content)

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, httptest.NewRequest(stdhttp.MethodGet, "/sitemap.xml", nil))

	require.Equal(t, stdhttp.StatusOK, recorder.Code, recorder.Body.String())
	require.Equal(t, "application/xml; charset=utf-8", recorder.Header().Get("Content-Type"))
	require.Equal(t, "public, max-age=300", recorder.Header().Get("Cache-Control"))
	require.Len(t, content.topicRequests, 1)
	require.Equal(t, contentStatusPublished, content.topicRequests[0].GetStatus())
	require.Equal(t, int32(1), content.topicRequests[0].GetLimit())
	require.Equal(t, "updated", content.topicRequests[0].GetSort())
	require.Len(t, content.articleRequests, 1)
	require.Equal(t, contentStatusPublished, content.articleRequests[0].GetStatus())
	require.Equal(t, int32(1), content.articleRequests[0].GetLimit())

	body := recorder.Body.String()
	require.Contains(t, body, `<?xml version="1.0" encoding="UTF-8"?>`)
	require.Contains(t, body, `<loc>https://bbs.example.com/sitemaps/static.xml</loc>`)
	require.Contains(t, body, `<loc>https://bbs.example.com/sitemaps/topics-1.xml</loc>`)
	require.Contains(t, body, `<loc>https://bbs.example.com/sitemaps/topics-2.xml</loc>`)
	require.Contains(t, body, `<loc>https://bbs.example.com/sitemaps/articles-1.xml</loc>`)
}

func TestTopicSitemapIncludesOnlyPublishedPublicURLs(t *testing.T) {
	gin.SetMode(gin.TestMode)
	content := &fakeSitemapContentClient{topics: &contentpb.TopicListResponse{
		Total: 1,
		Items: []*contentpb.TopicInfo{
			{Id: 101, Status: contentStatusPublished, UpdatedAt: 1_735_689_600_000},
			{Id: 102, Status: 1, UpdatedAt: 1_735_689_600_000},
		},
	}}
	router := sitemapTestRouter(content)

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, httptest.NewRequest(stdhttp.MethodGet, "/sitemaps/topics-1.xml", nil))

	require.Equal(t, stdhttp.StatusOK, recorder.Code, recorder.Body.String())
	require.Len(t, content.topicRequests, 1)
	require.Equal(t, contentStatusPublished, content.topicRequests[0].GetStatus())
	require.Equal(t, sitemapContentPageSize, content.topicRequests[0].GetLimit())
	require.Zero(t, content.topicRequests[0].GetOffset())
	require.Contains(t, recorder.Body.String(), `<loc>https://bbs.example.com/topic/101</loc>`)
	require.Contains(t, recorder.Body.String(), `<lastmod>2025-01-01</lastmod>`)
	require.NotContains(t, recorder.Body.String(), "/topic/102")
}

func TestArticleSitemapUsesRequestedPageOffset(t *testing.T) {
	gin.SetMode(gin.TestMode)
	content := &fakeSitemapContentClient{articles: &contentpb.ArticleListResponse{
		Total: 101,
		Items: []*contentpb.ArticleInfo{
			{Id: 201, Status: contentStatusPublished, PublishedAt: 1_735_689_600_000},
		},
	}}
	router := sitemapTestRouter(content)

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, httptest.NewRequest(stdhttp.MethodGet, "/sitemaps/articles-2.xml", nil))

	require.Equal(t, stdhttp.StatusOK, recorder.Code, recorder.Body.String())
	require.Len(t, content.articleRequests, 1)
	require.Equal(t, contentStatusPublished, content.articleRequests[0].GetStatus())
	require.Equal(t, sitemapContentPageSize, content.articleRequests[0].GetLimit())
	require.Equal(t, sitemapContentPageSize, content.articleRequests[0].GetOffset())
	require.Contains(t, recorder.Body.String(), `<loc>https://bbs.example.com/article/201</loc>`)
	require.Contains(t, recorder.Body.String(), `<lastmod>2025-01-01</lastmod>`)
}

func TestSitemapStaticAndRobotsDoNotRequireContentService(t *testing.T) {
	gin.SetMode(gin.TestMode)
	h := NewHandler(&clients.Clients{}, "Authorization", "Bearer", testJWTSecret)
	h.SetPublicBaseURL("https://bbs.example.com")
	router := gin.New()
	NewInitControllers(h)(router)

	staticRecorder := httptest.NewRecorder()
	router.ServeHTTP(staticRecorder, httptest.NewRequest(stdhttp.MethodGet, "/sitemaps/static.xml", nil))
	require.Equal(t, stdhttp.StatusOK, staticRecorder.Code, staticRecorder.Body.String())
	require.Contains(t, staticRecorder.Body.String(), `<loc>https://bbs.example.com/plaza</loc>`)
	require.NotContains(t, staticRecorder.Body.String(), "/chat")

	robotsRecorder := httptest.NewRecorder()
	router.ServeHTTP(robotsRecorder, httptest.NewRequest(stdhttp.MethodGet, "/robots.txt", nil))
	require.Equal(t, stdhttp.StatusOK, robotsRecorder.Code, robotsRecorder.Body.String())
	require.Equal(t, "User-agent: *\nAllow: /\nDisallow: /api/\nDisallow: /chat\nDisallow: /room/\nDisallow: /dashboard/\nSitemap: https://bbs.example.com/sitemap.xml\n", robotsRecorder.Body.String())
}

func TestSitemapContentPageRejectsInvalidNamesBeforeRPC(t *testing.T) {
	gin.SetMode(gin.TestMode)
	content := &fakeSitemapContentClient{}
	router := sitemapTestRouter(content)

	for _, path := range []string{"/sitemaps/private.xml", "/sitemaps/topics-0.xml", "/sitemaps/topics-999999999999999999.xml"} {
		recorder := httptest.NewRecorder()
		router.ServeHTTP(recorder, httptest.NewRequest(stdhttp.MethodGet, path, nil))
		require.Equal(t, stdhttp.StatusNotFound, recorder.Code, path+": "+recorder.Body.String())
	}
	require.Empty(t, content.topicRequests)
	require.Empty(t, content.articleRequests)
}

func sitemapTestRouter(content *fakeSitemapContentClient) *gin.Engine {
	h := NewHandler(&clients.Clients{Content: content}, "Authorization", "Bearer", testJWTSecret)
	h.SetPublicBaseURL("https://bbs.example.com")
	router := gin.New()
	NewInitControllers(h)(router)
	return router
}

type fakeSitemapContentClient struct {
	contentpb.ContentServiceClient
	topics          *contentpb.TopicListResponse
	articles        *contentpb.ArticleListResponse
	topicRequests   []*contentpb.ListTopicsRequest
	articleRequests []*contentpb.ListArticlesRequest
}

func (f *fakeSitemapContentClient) ListTopics(_ context.Context, req *contentpb.ListTopicsRequest, _ ...grpc.CallOption) (*contentpb.TopicListResponse, error) {
	f.topicRequests = append(f.topicRequests, req)
	if f.topics == nil {
		return &contentpb.TopicListResponse{}, nil
	}
	return f.topics, nil
}

func (f *fakeSitemapContentClient) ListArticles(_ context.Context, req *contentpb.ListArticlesRequest, _ ...grpc.CallOption) (*contentpb.ArticleListResponse, error) {
	f.articleRequests = append(f.articleRequests, req)
	if f.articles == nil {
		return &contentpb.ArticleListResponse{}, nil
	}
	return f.articles, nil
}
