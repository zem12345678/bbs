package http

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"api-gateway/api/proto/contentpb"
	"api-gateway/api/proto/userpb"
	"api-gateway/internal/clients"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
)

func TestChannelRoutesMapPublicFiltersAndAuthenticatedActor(t *testing.T) {
	content := &channelHTTPClient{}
	router := channelTestRouter(content)
	token := signedAuthToken(t, jwt.MapClaims{"sub": "42", "username": "owner"})

	request := httptest.NewRequest(http.MethodGet, "/api/v1/channels?q=engine&category_id=7&uncategorized=true&limit=8&offset=3", nil)
	request.Header.Set("Authorization", "Bearer "+token)
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, request)

	require.Equal(t, http.StatusOK, recorder.Code, recorder.Body.String())
	require.Len(t, content.listRequests, 1)
	require.Equal(t, "engine", content.listRequests[0].GetQuery())
	require.EqualValues(t, 7, content.listRequests[0].GetCategoryId())
	require.True(t, content.listRequests[0].GetUncategorized())
	require.EqualValues(t, 42, content.listRequests[0].GetViewerUserId())
	require.EqualValues(t, 8, content.listRequests[0].GetLimit())
	require.EqualValues(t, 3, content.listRequests[0].GetOffset())

	unauthorized := httptest.NewRecorder()
	router.ServeHTTP(unauthorized, httptest.NewRequest(http.MethodPost, "/api/v1/channels", bytes.NewBufferString(`{"owner_id":"999","name":"Engineering"}`)))
	require.Equal(t, http.StatusUnauthorized, unauthorized.Code, unauthorized.Body.String())
	require.Nil(t, content.createRequest)

	request = httptest.NewRequest(http.MethodPost, "/api/v1/channels", bytes.NewBufferString(`{"owner_id":"999","category_id":"7","name":"Engineering","description":"Build notes","color":"#123abc"}`))
	request.Header.Set("Authorization", "Bearer "+token)
	request.Header.Set("Content-Type", "application/json")
	recorder = httptest.NewRecorder()
	router.ServeHTTP(recorder, request)

	require.Equal(t, http.StatusOK, recorder.Code, recorder.Body.String())
	require.NotNil(t, content.createRequest)
	require.EqualValues(t, 42, content.createRequest.GetOwnerId())
	require.EqualValues(t, 7, content.createRequest.GetCategoryId())
}

func TestChannelRoutesMapOwnedFollowedFavoritesAndFeatured(t *testing.T) {
	content := &channelHTTPClient{}
	router := channelTestRouter(content)
	token := signedAuthToken(t, jwt.MapClaims{"sub": "42"})

	for _, path := range []string{"/api/v1/channels/owned", "/api/v1/channels/followed", "/api/v1/channels/favorites"} {
		request := httptest.NewRequest(http.MethodGet, path, nil)
		request.Header.Set("Authorization", "Bearer "+token)
		recorder := httptest.NewRecorder()
		router.ServeHTTP(recorder, request)
		require.Equal(t, http.StatusOK, recorder.Code, recorder.Body.String())
	}
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/api/v1/channels/featured", nil))
	require.Equal(t, http.StatusOK, recorder.Code, recorder.Body.String())

	require.Len(t, content.listRequests, 4)
	require.EqualValues(t, 42, content.listRequests[0].GetOwnerId())
	require.True(t, content.listRequests[0].GetIncludeArchived())
	require.EqualValues(t, 42, content.listRequests[1].GetFollowerUserId())
	require.EqualValues(t, 42, content.listRequests[2].GetFavoritedUserId())
	require.True(t, content.listRequests[3].GetFeatured())
}

func TestChannelRoutesMapOwnerMutationsAndChannelTimeline(t *testing.T) {
	content := &channelHTTPClient{}
	router := channelTestRouter(content)
	token := signedAuthToken(t, jwt.MapClaims{"sub": "42"})

	request := httptest.NewRequest(http.MethodPut, "/api/v1/channels/7001", bytes.NewBufferString(`{"actor_id":"999","name":"Updated","color":"#abcdef"}`))
	request.Header.Set("Authorization", "Bearer "+token)
	request.Header.Set("Content-Type", "application/json")
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, request)
	require.Equal(t, http.StatusOK, recorder.Code, recorder.Body.String())
	require.EqualValues(t, 42, content.updateRequest.GetActorId())

	for _, action := range []struct {
		method string
		path   string
		name   string
	}{
		{http.MethodPost, "/api/v1/channels/7001/follow", "follow"},
		{http.MethodDelete, "/api/v1/channels/7001/follow", "unfollow"},
		{http.MethodPost, "/api/v1/channels/7001/favorite", "favorite"},
		{http.MethodDelete, "/api/v1/channels/7001/favorite", "unfavorite"},
	} {
		request = httptest.NewRequest(action.method, action.path, nil)
		request.Header.Set("Authorization", "Bearer "+token)
		recorder = httptest.NewRecorder()
		router.ServeHTTP(recorder, request)
		require.Equal(t, http.StatusOK, recorder.Code, recorder.Body.String())
		require.Equal(t, action.name, content.action)
		require.EqualValues(t, 42, content.actionRequest.GetUserId())
	}

	request = httptest.NewRequest(http.MethodGet, "/api/v1/channels/7001", nil)
	request.Header.Set("Authorization", "Bearer "+token)
	recorder = httptest.NewRecorder()
	router.ServeHTTP(recorder, request)
	require.Equal(t, http.StatusOK, recorder.Code, recorder.Body.String())
	require.EqualValues(t, 42, content.getRequest.GetViewerUserId())
	require.True(t, content.getRequest.GetIncludeArchived())

	recorder = httptest.NewRecorder()
	router.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/api/v1/channels/categories", nil))
	require.Equal(t, http.StatusOK, recorder.Code, recorder.Body.String())
	require.NotNil(t, content.categoryRequest)

	recorder = httptest.NewRecorder()
	router.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/api/v1/channels/7001/topics?limit=6&offset=2", nil))
	require.Equal(t, http.StatusOK, recorder.Code, recorder.Body.String())
	require.NotNil(t, content.getRequest)
	require.True(t, content.getRequest.GetIncludeArchived())
	require.EqualValues(t, 7001, content.topicListRequest.GetChannelId())
	require.EqualValues(t, contentStatusPublished, content.topicListRequest.GetStatus())
	require.EqualValues(t, 6, content.topicListRequest.GetLimit())

	request = httptest.NewRequest(http.MethodDelete, "/api/v1/channels/7001", nil)
	request.Header.Set("Authorization", "Bearer "+token)
	recorder = httptest.NewRecorder()
	router.ServeHTTP(recorder, request)
	require.Equal(t, http.StatusOK, recorder.Code, recorder.Body.String())
	require.EqualValues(t, 42, content.archiveRequest.GetActorId())
}

func TestCreateTopicForwardsChannelID(t *testing.T) {
	content := &fakeTopicContentClient{}
	handler := NewHandler(&clients.Clients{
		Content: content,
		User:    &fakeUserClient{userResponse: &userpb.UserResponse{User: &userpb.UserInfo{Id: 42, Status: 1}}},
	}, "Authorization", "Bearer", testJWTSecret)
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Set("user_id", int64(42))
	c.Request = httptest.NewRequest(http.MethodPost, "/api/v1/topics", bytes.NewBufferString(`{"slug":"channel-topic","type":"topic","title":"Channel topic","body":"Body","channel_id":"7001","publish":false}`))
	c.Request.Header.Set("Content-Type", "application/json")

	handler.createTopic(c)

	require.Equal(t, http.StatusOK, recorder.Code, recorder.Body.String())
	require.EqualValues(t, 7001, content.createReq.GetChannelId())
}

func channelTestRouter(content contentpb.ContentServiceClient) *gin.Engine {
	gin.SetMode(gin.TestMode)
	handler := NewHandler(&clients.Clients{Content: content}, "Authorization", "Bearer", testJWTSecret)
	router := gin.New()
	NewInitControllers(handler)(router)
	return router
}

type channelHTTPClient struct {
	contentpb.ContentServiceClient
	createRequest    *contentpb.CreateChannelRequest
	updateRequest    *contentpb.UpdateChannelRequest
	archiveRequest   *contentpb.ArchiveChannelRequest
	getRequest       *contentpb.GetChannelRequest
	listRequests     []*contentpb.ListChannelsRequest
	topicListRequest *contentpb.ListTopicsRequest
	categoryRequest  *contentpb.ListChannelCategoriesRequest
	action           string
	actionRequest    *contentpb.ChannelUserRequest
}

func (c *channelHTTPClient) CreateChannel(_ context.Context, request *contentpb.CreateChannelRequest, _ ...grpc.CallOption) (*contentpb.ChannelResponse, error) {
	c.createRequest = request
	return &contentpb.ChannelResponse{Success: true, Channel: &contentpb.ChannelInfo{Id: 7001, OwnerId: request.GetOwnerId(), Name: request.GetName()}}, nil
}

func (c *channelHTTPClient) UpdateChannel(_ context.Context, request *contentpb.UpdateChannelRequest, _ ...grpc.CallOption) (*contentpb.ChannelResponse, error) {
	c.updateRequest = request
	return &contentpb.ChannelResponse{Success: true, Channel: &contentpb.ChannelInfo{Id: request.GetId(), OwnerId: request.GetActorId(), Name: request.GetName()}}, nil
}

func (c *channelHTTPClient) ArchiveChannel(_ context.Context, request *contentpb.ArchiveChannelRequest, _ ...grpc.CallOption) (*contentpb.ChannelResponse, error) {
	c.archiveRequest = request
	return &contentpb.ChannelResponse{Success: true, Channel: &contentpb.ChannelInfo{Id: request.GetId(), IsArchived: true}}, nil
}

func (c *channelHTTPClient) GetChannel(_ context.Context, request *contentpb.GetChannelRequest, _ ...grpc.CallOption) (*contentpb.ChannelResponse, error) {
	c.getRequest = request
	return &contentpb.ChannelResponse{Success: true, Channel: &contentpb.ChannelInfo{Id: request.GetId()}}, nil
}

func (c *channelHTTPClient) ListChannels(_ context.Context, request *contentpb.ListChannelsRequest, _ ...grpc.CallOption) (*contentpb.ChannelListResponse, error) {
	c.listRequests = append(c.listRequests, request)
	return &contentpb.ChannelListResponse{}, nil
}

func (c *channelHTTPClient) ListChannelCategories(_ context.Context, request *contentpb.ListChannelCategoriesRequest, _ ...grpc.CallOption) (*contentpb.ChannelCategoryListResponse, error) {
	c.categoryRequest = request
	return &contentpb.ChannelCategoryListResponse{}, nil
}

func (c *channelHTTPClient) ListTopics(_ context.Context, request *contentpb.ListTopicsRequest, _ ...grpc.CallOption) (*contentpb.TopicListResponse, error) {
	c.topicListRequest = request
	return &contentpb.TopicListResponse{}, nil
}

func (c *channelHTTPClient) FollowChannel(_ context.Context, request *contentpb.ChannelUserRequest, _ ...grpc.CallOption) (*contentpb.ChannelActionResponse, error) {
	c.action = "follow"
	c.actionRequest = request
	return &contentpb.ChannelActionResponse{Success: true}, nil
}

func (c *channelHTTPClient) UnfollowChannel(_ context.Context, request *contentpb.ChannelUserRequest, _ ...grpc.CallOption) (*contentpb.ChannelActionResponse, error) {
	c.action = "unfollow"
	c.actionRequest = request
	return &contentpb.ChannelActionResponse{Success: true}, nil
}

func (c *channelHTTPClient) FavoriteChannel(_ context.Context, request *contentpb.ChannelUserRequest, _ ...grpc.CallOption) (*contentpb.ChannelActionResponse, error) {
	c.action = "favorite"
	c.actionRequest = request
	return &contentpb.ChannelActionResponse{Success: true}, nil
}

func (c *channelHTTPClient) UnfavoriteChannel(_ context.Context, request *contentpb.ChannelUserRequest, _ ...grpc.CallOption) (*contentpb.ChannelActionResponse, error) {
	c.action = "unfavorite"
	c.actionRequest = request
	return &contentpb.ChannelActionResponse{Success: true}, nil
}
