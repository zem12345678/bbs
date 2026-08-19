package http

import (
	"context"
	"encoding/json"
	stdhttp "net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"api-gateway/api/proto/contentpb"
	"api-gateway/api/proto/notificationpb"
	"api-gateway/api/proto/userpb"
	"api-gateway/internal/clients"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func TestMisskeyNotificationsMapFiltersAndReturnRawArray(t *testing.T) {
	gin.SetMode(gin.TestMode)
	const notificationID int64 = 9007199254740993
	const noteID int64 = 9007199254740994
	notifications := &misskeyNotificationsClient{response: &notificationpb.ListNotificationsResponse{Items: []*notificationpb.Notification{
		{Id: notificationID, UserId: 42, Type: "like", ActorId: 8, EntityType: "article", EntityId: noteID, CreatedAt: 1720000000123},
		{Id: notificationID + 1, UserId: 42, Type: "export_completed", EntityType: "file", EntityId: noteID + 1, Title: "Clip 导出完成", CreatedAt: 1720000001123},
	}}}
	users := &misskeyNotificationsUserClient{users: map[int64]*userpb.UserInfo{
		7: {Id: 7, Username: "author", Nickname: "Author", CreatedAt: 1720000000000},
		8: {Id: 8, Username: "reader", Nickname: "Reader", CreatedAt: 1720000000000},
	}}
	content := &misskeyNotificationsContentClient{articles: map[int64]*contentpb.ArticleInfo{
		noteID: {Id: noteID, AuthorId: 7, Title: "Article", Body: "hello", Status: contentStatusPublished, CreatedAt: 1720000000000},
	}}
	router := misskeyNotificationsRouter(notifications, users, content)

	recorder := performMisskeyNotificationsRequest(t, router, "/api/v1/i/notifications", `{"limit":2,"sinceId":"10","includeTypes":["reaction","exportCompleted"],"excludeTypes":["follow"],"markAsRead":false}`)

	require.Equal(t, stdhttp.StatusOK, recorder.Code, recorder.Body.String())
	require.NotNil(t, notifications.listRequest)
	require.Equal(t, int64(42), notifications.listRequest.GetUserId())
	require.Equal(t, int32(2), notifications.listRequest.GetLimit())
	require.Equal(t, int64(10), notifications.listRequest.GetSinceId())
	require.True(t, notifications.listRequest.GetIncludeTypesSet())
	require.False(t, notifications.listRequest.GetExcludeTypesSet())
	require.Equal(t, []string{"like", "favorite", "export_completed"}, notifications.listRequest.GetIncludeTypes())
	require.Nil(t, notifications.markAllRequest)
	require.NotContains(t, recorder.Body.String(), `"data":`)

	var items []map[string]any
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &items))
	require.Len(t, items, 2)
	require.Equal(t, "reaction", items[0]["type"])
	require.Equal(t, "9007199254740993", items[0]["id"])
	require.Equal(t, "8", items[0]["userId"])
	require.Equal(t, ":thumbsup:", items[0]["reaction"])
	note := items[0]["note"].(map[string]any)
	require.Equal(t, "9007199254740994", note["id"])
	require.Equal(t, "exportCompleted", items[1]["type"])
	require.Equal(t, "clip", items[1]["exportedEntity"])
	require.Equal(t, "9007199254740995", items[1]["fileId"])
}

func TestMisskeyGroupedNotificationsCombineReactionsAndConsecutiveFollows(t *testing.T) {
	gin.SetMode(gin.TestMode)
	notifications := &misskeyNotificationsClient{response: &notificationpb.ListNotificationsResponse{Items: []*notificationpb.Notification{
		{Id: 30, UserId: 42, Type: "like", ActorId: 8, EntityType: "article", EntityId: 5001, CreatedAt: 1720000003000},
		{Id: 20, UserId: 42, Type: "favorite", ActorId: 9, EntityType: "article", EntityId: 5001, CreatedAt: 1720000002000},
		{Id: 10, UserId: 42, Type: "follow", ActorId: 10, CreatedAt: 1720000001000},
		{Id: 9, UserId: 42, Type: "follow", ActorId: 11, CreatedAt: 1720000000000},
	}}}
	users := &misskeyNotificationsUserClient{users: map[int64]*userpb.UserInfo{
		7: {Id: 7, Username: "author", CreatedAt: 1720000000000}, 8: {Id: 8, Username: "like", CreatedAt: 1720000000000},
		9: {Id: 9, Username: "favorite", CreatedAt: 1720000000000}, 10: {Id: 10, Username: "follow-one", CreatedAt: 1720000000000},
		11: {Id: 11, Username: "follow-two", CreatedAt: 1720000000000},
	}}
	content := &misskeyNotificationsContentClient{articles: map[int64]*contentpb.ArticleInfo{
		5001: {Id: 5001, AuthorId: 7, Title: "Article", Status: contentStatusPublished, CreatedAt: 1720000000000},
	}}
	router := misskeyNotificationsRouter(notifications, users, content)

	recorder := performMisskeyNotificationsRequest(t, router, "/i/notifications-grouped", `{}`)

	require.Equal(t, stdhttp.StatusOK, recorder.Code, recorder.Body.String())
	require.NotNil(t, notifications.markAllRequest)
	require.Equal(t, int64(42), notifications.markAllRequest.GetUserId())
	var items []map[string]any
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &items))
	require.Len(t, items, 2)
	require.Equal(t, "reaction:grouped", items[0]["type"])
	require.Equal(t, "20", items[0]["id"])
	require.Len(t, items[0]["reactions"].([]any), 2)
	require.Equal(t, "follow:grouped", items[1]["type"])
	require.Equal(t, "9", items[1]["id"])
	require.Len(t, items[1]["users"].([]any), 2)
}

func TestMisskeyNotificationsEmptyIncludeSkipsLookupAndRoutesRequireAuthentication(t *testing.T) {
	gin.SetMode(gin.TestMode)
	notifications := &misskeyNotificationsClient{}
	router := misskeyNotificationsRouter(notifications, &misskeyNotificationsUserClient{}, &misskeyNotificationsContentClient{})

	empty := performMisskeyNotificationsRequest(t, router, "/api/i/notifications", `{"includeTypes":[]}`)
	require.Equal(t, stdhttp.StatusOK, empty.Code, empty.Body.String())
	require.JSONEq(t, `[]`, empty.Body.String())
	require.Nil(t, notifications.listRequest)
	require.Nil(t, notifications.markAllRequest)

	for _, path := range []string{"/i/notifications", "/api/i/notifications", "/api/v1/i/notifications-grouped"} {
		recorder := httptest.NewRecorder()
		request := httptest.NewRequest(stdhttp.MethodPost, path, strings.NewReader(`{}`))
		request.Header.Set("Content-Type", "application/json")
		router.ServeHTTP(recorder, request)
		require.Equal(t, stdhttp.StatusUnauthorized, recorder.Code, path+": "+recorder.Body.String())
	}
}

func TestMisskeyNotificationsUseSeparateUserRateLimitKeys(t *testing.T) {
	gin.SetMode(gin.TestMode)
	listLimit := &misskeyNotificationRateLimitStub{limited: true}
	groupedLimit := &misskeyNotificationRateLimitStub{limited: true}
	h := NewHandler(&clients.Clients{Notification: &misskeyNotificationsClient{}}, "Authorization", "Bearer", testJWTSecret)
	h.SetNotificationRateLimits(NotificationRateLimits{List: listLimit, Grouped: groupedLimit})
	router := gin.New()
	NewInitControllers(h)(router)

	list := performMisskeyNotificationsRequest(t, router, "/api/i/notifications", `{}`)
	require.Equal(t, stdhttp.StatusTooManyRequests, list.Code, list.Body.String())
	grouped := performMisskeyNotificationsRequest(t, router, "/api/v1/i/notifications-grouped", `{}`)
	require.Equal(t, stdhttp.StatusTooManyRequests, grouped.Code, grouped.Body.String())
	require.Equal(t, []string{"rate:notifications:list:user:42"}, listLimit.keys)
	require.Equal(t, []string{"rate:notifications:grouped:user:42"}, groupedLimit.keys)
}

func misskeyNotificationsRouter(notificationClient notificationpb.NotificationServiceClient, userClient clients.UserClient, contentClient contentpb.ContentServiceClient) *gin.Engine {
	h := NewHandler(&clients.Clients{Notification: notificationClient, User: userClient, Content: contentClient}, "Authorization", "Bearer", testJWTSecret)
	router := gin.New()
	NewInitControllers(h)(router)
	return router
}

func performMisskeyNotificationsRequest(t *testing.T, router stdhttp.Handler, path, body string) *httptest.ResponseRecorder {
	t.Helper()
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(stdhttp.MethodPost, path, strings.NewReader(body))
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Authorization", "Bearer "+signedAuthToken(t, jwt.MapClaims{"sub": "42"}))
	router.ServeHTTP(recorder, request)
	return recorder
}

type misskeyNotificationsClient struct {
	notificationpb.NotificationServiceClient
	response       *notificationpb.ListNotificationsResponse
	listRequest    *notificationpb.ListNotificationsCompatRequest
	markAllRequest *notificationpb.MarkAllReadRequest
}

type misskeyNotificationRateLimitStub struct {
	keys    []string
	limited bool
}

func (l *misskeyNotificationRateLimitStub) Limit(_ context.Context, key string) (bool, error) {
	l.keys = append(l.keys, key)
	return l.limited, nil
}

func (c *misskeyNotificationsClient) ListNotificationsCompat(_ context.Context, request *notificationpb.ListNotificationsCompatRequest, _ ...grpc.CallOption) (*notificationpb.ListNotificationsResponse, error) {
	c.listRequest = request
	if c.response == nil {
		return &notificationpb.ListNotificationsResponse{}, nil
	}
	return c.response, nil
}

func (c *misskeyNotificationsClient) MarkAllRead(_ context.Context, request *notificationpb.MarkAllReadRequest, _ ...grpc.CallOption) (*notificationpb.MutationResponse, error) {
	c.markAllRequest = request
	return &notificationpb.MutationResponse{Success: true}, nil
}

type misskeyNotificationsUserClient struct {
	userpb.UserServiceClient
	users map[int64]*userpb.UserInfo
}

func (c *misskeyNotificationsUserClient) GetUser(_ context.Context, request *userpb.UserIDRequest, _ ...grpc.CallOption) (*userpb.UserResponse, error) {
	if user := c.users[request.GetId()]; user != nil {
		return &userpb.UserResponse{User: user}, nil
	}
	return nil, status.Error(codes.NotFound, "user not found")
}

type misskeyNotificationsContentClient struct {
	contentpb.ContentServiceClient
	articles map[int64]*contentpb.ArticleInfo
}

func (c *misskeyNotificationsContentClient) GetArticle(_ context.Context, request *contentpb.GetArticleRequest, _ ...grpc.CallOption) (*contentpb.ArticleResponse, error) {
	if article := c.articles[request.GetId()]; article != nil {
		return &contentpb.ArticleResponse{Article: article}, nil
	}
	return nil, status.Error(codes.NotFound, "article not found")
}
