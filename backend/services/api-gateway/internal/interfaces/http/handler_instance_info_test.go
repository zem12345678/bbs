package http

import (
	"context"
	"encoding/json"
	stdhttp "net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"api-gateway/api/proto/adminpb"
	"api-gateway/api/proto/commentpb"
	"api-gateway/api/proto/contentpb"
	"api-gateway/api/proto/userpb"
	"api-gateway/internal/clients"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
)

func TestInstanceMetaUsesPublicSettingsAndHonorsDetailFlag(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := instanceInfoTestRouter(&clients.Clients{Admin: instanceInfoAdminClient{items: []*adminpb.SettingInfo{
		{Key: "site_name", Value: "示例社区"},
		{Key: "site_description", Value: "面向创作者的社区"},
		{Key: "seo_keywords", Value: "bbs, 创作"},
		{Key: "site_logo_url", Value: "https://cdn.example.com/logo.png"},
		{Key: "site_navigation", Value: `[{"key":"home","label":"首页"}]`},
		{Key: "auth.github.client_secret", Value: "must-not-be-exposed"},
	}, activeAds: []*adminpb.ActiveAdInfo{{
		Id: 9007199254740993, Url: "https://example.com/ad", Place: "horizontal", Ratio: 2,
		ImageUrl: "https://cdn.example.com/ad.png", DayOfWeek: 127,
	}}}})

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(stdhttp.MethodPost, "/api/v1/meta", strings.NewReader(`{"detail":false}`))
	request.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(recorder, request)

	require.Equal(t, stdhttp.StatusOK, recorder.Code, recorder.Body.String())
	data := decodeInstanceInfoData(t, recorder)
	require.Equal(t, "示例社区", data["name"])
	require.Equal(t, "示例社区", data["shortName"])
	require.Equal(t, "https://bbs.example.com", data["uri"])
	require.Equal(t, "面向创作者的社区", data["description"])
	require.Equal(t, "https://cdn.example.com/logo.png", data["iconUrl"])
	ads := data["ads"].([]any)
	require.Len(t, ads, 1)
	ad := ads[0].(map[string]any)
	require.Equal(t, "9007199254740993", ad["id"])
	require.Equal(t, "https://example.com/ad", ad["url"])
	require.Equal(t, "horizontal", ad["place"])
	require.Equal(t, float64(2), ad["ratio"])
	require.Equal(t, "https://cdn.example.com/ad.png", ad["imageUrl"])
	require.Equal(t, float64(127), ad["dayOfWeek"])
	require.Equal(t, float64(0), data["notesPerOneAd"])
	require.NotContains(t, ad, "memo")
	require.NotContains(t, ad, "priority")
	require.NotContains(t, ad, "startsAt")
	require.NotContains(t, ad, "expiresAt")
	require.NotContains(t, data, "navigation")
	require.NotContains(t, recorder.Body.String(), "must-not-be-exposed")

	detailRecorder := httptest.NewRecorder()
	router.ServeHTTP(detailRecorder, httptest.NewRequest(stdhttp.MethodGet, "/api/v1/meta", nil))
	require.Equal(t, stdhttp.StatusOK, detailRecorder.Code, detailRecorder.Body.String())
	detailData := decodeInstanceInfoData(t, detailRecorder)
	require.Equal(t, "bbs, 创作", detailData["seoKeywords"])
	require.Len(t, detailData["navigation"], 1)
	require.Contains(t, detailData, "features")
}

func TestInstanceStatsUsesPublishedAndActivePublicTotals(t *testing.T) {
	gin.SetMode(gin.TestMode)
	userClient := &instanceInfoUserClient{total: 5}
	contentClient := &instanceInfoContentClient{articleTotal: 7, topicTotal: 11}
	commentClient := &instanceInfoCommentClient{total: 3}
	router := instanceInfoTestRouter(&clients.Clients{
		User:    userClient,
		Content: contentClient,
		Comment: commentClient,
	})

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, httptest.NewRequest(stdhttp.MethodGet, "/api/v1/stats", nil))

	require.Equal(t, stdhttp.StatusOK, recorder.Code, recorder.Body.String())
	require.Equal(t, userStatusActive, userClient.request.GetStatus())
	require.Equal(t, contentStatusPublished, contentClient.articleRequest.GetStatus())
	require.Equal(t, contentStatusPublished, contentClient.topicRequest.GetStatus())
	require.Equal(t, int32(1), commentClient.request.GetStatus())
	data := decodeInstanceInfoData(t, recorder)
	require.Equal(t, float64(18), data["notesCount"])
	require.Equal(t, float64(18), data["originalNotesCount"])
	require.Equal(t, float64(5), data["usersCount"])
	require.Equal(t, float64(5), data["originalUsersCount"])
	require.Equal(t, float64(1), data["instances"])
	require.Equal(t, float64(0), data["driveUsageLocal"])
	require.Equal(t, float64(0), data["driveUsageRemote"])
	require.Equal(t, float64(7), data["articlesCount"])
	require.Equal(t, float64(11), data["topicsCount"])
	require.Equal(t, float64(3), data["commentsCount"])
}

func TestInstancePingAndServerInfoRoutes(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := instanceInfoTestRouter(&clients.Clients{})

	pingRecorder := httptest.NewRecorder()
	router.ServeHTTP(pingRecorder, httptest.NewRequest(stdhttp.MethodPost, "/api/v1/ping", nil))
	require.Equal(t, stdhttp.StatusOK, pingRecorder.Code, pingRecorder.Body.String())
	pingData := decodeInstanceInfoData(t, pingRecorder)
	require.Greater(t, pingData["pong"].(float64), float64(0))

	infoRecorder := httptest.NewRecorder()
	router.ServeHTTP(infoRecorder, httptest.NewRequest(stdhttp.MethodGet, "/api/v1/server-info", nil))
	require.Equal(t, stdhttp.StatusOK, infoRecorder.Code, infoRecorder.Body.String())
	infoData := decodeInstanceInfoData(t, infoRecorder)
	require.Equal(t, "bbs-api-gateway", infoData["machine"])
	require.Greater(t, infoData["cpu"].(map[string]any)["cores"].(float64), float64(0))
	require.Contains(t, infoData, "mem")
	require.Contains(t, infoData, "fs")
}

func instanceInfoTestRouter(c *clients.Clients) *gin.Engine {
	h := NewHandler(c, "Authorization", "Bearer", testJWTSecret)
	h.SetPublicBaseURL("https://bbs.example.com")
	router := gin.New()
	NewInitControllers(h)(router)
	return router
}

func decodeInstanceInfoData(t *testing.T, recorder *httptest.ResponseRecorder) map[string]any {
	t.Helper()
	var envelope struct {
		Data map[string]any `json:"data"`
	}
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &envelope))
	return envelope.Data
}

type instanceInfoAdminClient struct {
	adminpb.AdminServiceClient
	items     []*adminpb.SettingInfo
	activeAds []*adminpb.ActiveAdInfo
}

func (f instanceInfoAdminClient) ListPublicSettings(context.Context, *adminpb.ListPublicSettingsRequest, ...grpc.CallOption) (*adminpb.SettingListResponse, error) {
	return &adminpb.SettingListResponse{Items: f.items, Total: int64(len(f.items))}, nil
}

func (f instanceInfoAdminClient) ListActiveAds(context.Context, *adminpb.ListActiveAdsRequest, ...grpc.CallOption) (*adminpb.ActiveAdListResponse, error) {
	return &adminpb.ActiveAdListResponse{Items: f.activeAds}, nil
}

type instanceInfoUserClient struct {
	userpb.UserServiceClient
	total   int64
	request *userpb.ListUsersRequest
}

func (f *instanceInfoUserClient) ListUsers(_ context.Context, req *userpb.ListUsersRequest, _ ...grpc.CallOption) (*userpb.UserListResponse, error) {
	f.request = req
	return &userpb.UserListResponse{Total: f.total}, nil
}

type instanceInfoContentClient struct {
	contentpb.ContentServiceClient
	articleTotal   int64
	topicTotal     int64
	articleRequest *contentpb.ListArticlesRequest
	topicRequest   *contentpb.ListTopicsRequest
}

func (f *instanceInfoContentClient) ListArticles(_ context.Context, req *contentpb.ListArticlesRequest, _ ...grpc.CallOption) (*contentpb.ArticleListResponse, error) {
	f.articleRequest = req
	return &contentpb.ArticleListResponse{Total: f.articleTotal}, nil
}

func (f *instanceInfoContentClient) ListTopics(_ context.Context, req *contentpb.ListTopicsRequest, _ ...grpc.CallOption) (*contentpb.TopicListResponse, error) {
	f.topicRequest = req
	return &contentpb.TopicListResponse{Total: f.topicTotal}, nil
}

type instanceInfoCommentClient struct {
	commentpb.CommentServiceClient
	total   int64
	request *commentpb.ListRecentCommentsRequest
}

func (f *instanceInfoCommentClient) ListRecentComments(_ context.Context, req *commentpb.ListRecentCommentsRequest, _ ...grpc.CallOption) (*commentpb.CommentListResponse, error) {
	f.request = req
	return &commentpb.CommentListResponse{Total: f.total}, nil
}
