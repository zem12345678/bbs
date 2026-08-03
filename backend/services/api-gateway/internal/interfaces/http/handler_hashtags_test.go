package http

import (
	"context"
	"encoding/json"
	stdhttp "net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"api-gateway/api/proto/contentpb"
	"api-gateway/api/proto/userpb"
	"api-gateway/internal/clients"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
)

func TestTagsRouteStillUsesContentTagsAfterHandlerSplit(t *testing.T) {
	gin.SetMode(gin.TestMode)
	contentClient := &hashtagContentClient{
		tags: []*contentpb.TagInfo{{Name: "golang", Count: 3}},
	}
	router := hashtagTestRouter(contentClient, nil)

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, httptest.NewRequest(stdhttp.MethodGet, "/api/v1/tags?q=go&limit=3", nil))

	require.Equal(t, stdhttp.StatusOK, recorder.Code, recorder.Body.String())
	require.NotNil(t, contentClient.listTagsReq)
	require.Equal(t, "go", contentClient.listTagsReq.GetQuery())
	require.Equal(t, int32(3), contentClient.listTagsReq.GetLimit())
}

func TestSearchHashtagsReturnsPublicHashtagFields(t *testing.T) {
	gin.SetMode(gin.TestMode)
	contentClient := &hashtagContentClient{
		tags: []*contentpb.TagInfo{
			{Name: "golang", Count: 5},
			{Name: "go", Count: 2},
		},
	}
	router := hashtagTestRouter(contentClient, nil)

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(stdhttp.MethodPost, "/api/v1/hashtags/search", strings.NewReader(`{"query":"go","limit":2}`))
	request.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(recorder, request)

	require.Equal(t, stdhttp.StatusOK, recorder.Code, recorder.Body.String())
	require.NotNil(t, contentClient.listTagsReq)
	require.Equal(t, "go", contentClient.listTagsReq.GetQuery())
	require.Equal(t, contentStatusPublished, contentClient.listTagsReq.GetStatus())
	require.Equal(t, int32(2), contentClient.listTagsReq.GetLimit())

	var envelope struct {
		Data publicHashtagListResponse `json:"data"`
	}
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &envelope))
	require.Equal(t, int64(2), envelope.Data.Total)
	require.Len(t, envelope.Data.Items, 2)
	require.Equal(t, "golang", envelope.Data.Items[0].Tag)
	require.Equal(t, int64(5), envelope.Data.Items[0].MentionedUsersCount)
	require.Equal(t, int64(5), envelope.Data.Items[0].MentionedLocalUsersCount)
	require.Equal(t, int64(0), envelope.Data.Items[0].AttachedUsersCount)
}

func TestShowHashtagRequiresExactTagMatch(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := hashtagTestRouter(&hashtagContentClient{
		tags: []*contentpb.TagInfo{{Name: "golang", Count: 5}},
	}, nil)

	found := httptest.NewRecorder()
	foundRequest := httptest.NewRequest(stdhttp.MethodPost, "/api/v1/hashtags/show", strings.NewReader(`{"tag":"#golang"}`))
	foundRequest.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(found, foundRequest)

	require.Equal(t, stdhttp.StatusOK, found.Code, found.Body.String())
	var envelope struct {
		Data publicHashtagShowResponse `json:"data"`
	}
	require.NoError(t, json.Unmarshal(found.Body.Bytes(), &envelope))
	require.Equal(t, "golang", envelope.Data.Hashtag.Tag)

	missing := httptest.NewRecorder()
	missingRequest := httptest.NewRequest(stdhttp.MethodPost, "/api/v1/hashtags/show", strings.NewReader(`{"tag":"go"}`))
	missingRequest.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(missing, missingRequest)
	require.Equal(t, stdhttp.StatusNotFound, missing.Code, missing.Body.String())
}

func TestListHashtagUsersUsesPublishedTaggedArticleAuthors(t *testing.T) {
	gin.SetMode(gin.TestMode)
	contentClient := &hashtagContentClient{
		articles: []*contentpb.ArticleInfo{
			{Id: 1, AuthorId: 42},
			{Id: 2, AuthorId: 43},
			{Id: 3, AuthorId: 42},
			{Id: 4, AuthorId: 0},
		},
	}
	userClient := &hashtagUserClient{users: []*userpb.UserInfo{
		{Id: 43, Username: "bob", Status: userStatusActive},
		{Id: 42, Username: "alice", Email: "alice@example.test", Status: userStatusActive},
		{Id: 44, Username: "muted", Status: userStatusMuted},
	}}
	router := hashtagTestRouter(contentClient, userClient)

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(stdhttp.MethodPost, "/api/v1/hashtags/users", strings.NewReader(`{"tag":"#go","limit":10,"sort":"-follower"}`))
	request.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(recorder, request)

	require.Equal(t, stdhttp.StatusOK, recorder.Code, recorder.Body.String())
	require.NotNil(t, contentClient.listArticlesReq)
	require.Equal(t, contentStatusPublished, contentClient.listArticlesReq.GetStatus())
	require.Equal(t, "go", contentClient.listArticlesReq.GetTag())
	require.Equal(t, int32(100), contentClient.listArticlesReq.GetLimit())
	require.NotNil(t, userClient.listUsersReq)
	require.Equal(t, []int64{42, 43}, userClient.listUsersReq.GetIds())
	require.Equal(t, userStatusActive, userClient.listUsersReq.GetStatus())

	var envelope struct {
		Data publicUserListResponse `json:"data"`
	}
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &envelope))
	require.Equal(t, int64(2), envelope.Data.Total)
	require.Len(t, envelope.Data.Items, 2)
	require.EqualValues(t, 42, envelope.Data.Items[0].ID)
	require.EqualValues(t, 43, envelope.Data.Items[1].ID)
	require.NotContains(t, recorder.Body.String(), "alice@example.test")
}

func hashtagTestRouter(contentClient contentpb.ContentServiceClient, userClient userpb.UserServiceClient) *gin.Engine {
	h := NewHandler(&clients.Clients{Content: contentClient, User: userClient}, "Authorization", "Bearer", testJWTSecret)
	router := gin.New()
	NewInitControllers(h)(router)
	return router
}

type hashtagContentClient struct {
	contentpb.ContentServiceClient
	tags            []*contentpb.TagInfo
	articles        []*contentpb.ArticleInfo
	listTagsReq     *contentpb.ListTagsRequest
	autocompleteReq *contentpb.AutocompleteTagsRequest
	listArticlesReq *contentpb.ListArticlesRequest
}

func (c *hashtagContentClient) ListTags(_ context.Context, req *contentpb.ListTagsRequest, _ ...grpc.CallOption) (*contentpb.TagListResponse, error) {
	c.listTagsReq = req
	return &contentpb.TagListResponse{Items: c.tags}, nil
}

func (c *hashtagContentClient) AutocompleteTags(_ context.Context, req *contentpb.AutocompleteTagsRequest, _ ...grpc.CallOption) (*contentpb.TagListResponse, error) {
	c.autocompleteReq = req
	return &contentpb.TagListResponse{Items: c.tags}, nil
}

func (c *hashtagContentClient) ListArticles(_ context.Context, req *contentpb.ListArticlesRequest, _ ...grpc.CallOption) (*contentpb.ArticleListResponse, error) {
	c.listArticlesReq = req
	return &contentpb.ArticleListResponse{Items: c.articles, Total: int64(len(c.articles))}, nil
}

type hashtagUserClient struct {
	userpb.UserServiceClient
	users        []*userpb.UserInfo
	listUsersReq *userpb.ListUsersRequest
}

func (c *hashtagUserClient) ListUsers(_ context.Context, req *userpb.ListUsersRequest, _ ...grpc.CallOption) (*userpb.UserListResponse, error) {
	c.listUsersReq = req
	return &userpb.UserListResponse{Items: c.users, Total: int64(len(c.users))}, nil
}
