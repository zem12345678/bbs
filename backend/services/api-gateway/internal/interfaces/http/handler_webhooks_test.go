package http

import (
	"context"
	"encoding/json"
	stdhttp "net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"api-gateway/api/proto/notificationpb"
	"api-gateway/internal/clients"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
)

const testWebhookID int64 = 9007199254740993

func TestCanonicalWebhookRoutesForwardAuthenticatedUserAndStringifyIDs(t *testing.T) {
	client := newWebhookHTTPClient()
	router := webhookTestRouter(client)
	token := signedAuthToken(t, jwt.MapClaims{"sub": "9223372036854770001"})

	list := performWebhookRequest(router, stdhttp.MethodGet, "/api/v1/users/me/webhooks", "", token)
	require.Equal(t, stdhttp.StatusOK, list.Code, list.Body.String())
	var listEnvelope struct {
		Data struct {
			Items []map[string]any `json:"items"`
			Total int              `json:"total"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(list.Body.Bytes(), &listEnvelope))
	require.Equal(t, 1, listEnvelope.Data.Total)
	require.Equal(t, "9007199254740993", listEnvelope.Data.Items[0]["id"])
	require.Equal(t, "9223372036854770001", listEnvelope.Data.Items[0]["userId"])
	require.Equal(t, int64(9223372036854770001), client.listRequest.GetUserId())
	require.NotContains(t, listEnvelope.Data.Items[0], "secret")

	create := performWebhookRequest(router, stdhttp.MethodPost, "/api/v1/users/me/webhooks", `{"name":"Deploy","url":"https://hooks.example/deploy","secret":"shared","on":["note","reply"]}`, token)
	require.Equal(t, stdhttp.StatusOK, create.Code, create.Body.String())
	require.Equal(t, int64(9223372036854770001), client.createRequest.GetUserId())
	require.Equal(t, "Deploy", client.createRequest.GetName())
	require.Equal(t, []string{"note", "reply"}, client.createRequest.GetOn())
	require.Contains(t, create.Body.String(), `"id":"9007199254740993"`)
	require.NotContains(t, create.Body.String(), `"secret"`)

	show := performWebhookRequest(router, stdhttp.MethodGet, "/api/v1/users/me/webhooks/9007199254740993", "", token)
	require.Equal(t, stdhttp.StatusOK, show.Code, show.Body.String())
	require.Equal(t, testWebhookID, client.showRequest.GetWebhookId())
	require.NotContains(t, show.Body.String(), `"secret"`)

	update := performWebhookRequest(router, stdhttp.MethodPut, "/api/v1/users/me/webhooks/9007199254740993", `{"name":"Renamed","active":false}`, token)
	require.Equal(t, stdhttp.StatusOK, update.Code, update.Body.String())
	require.Equal(t, testWebhookID, client.updateRequest.GetWebhookId())
	require.Equal(t, "Renamed", client.updateRequest.GetName())
	require.True(t, client.updateRequest.GetActiveSet())
	require.False(t, client.updateRequest.GetActive())
	require.False(t, client.updateRequest.GetSecretSet())
	require.NotContains(t, update.Body.String(), `"secret"`)

	remove := performWebhookRequest(router, stdhttp.MethodDelete, "/api/v1/users/me/webhooks/9007199254740993", "", token)
	require.Equal(t, stdhttp.StatusNoContent, remove.Code, remove.Body.String())
	require.Equal(t, testWebhookID, client.deleteRequest.GetWebhookId())

	testDelivery := performWebhookRequest(router, stdhttp.MethodPost, "/api/v1/users/me/webhooks/9007199254740993/test", `{"type":"reply","override":{"url":"https://test.example/hook","secret":"test-secret"}}`, token)
	require.Equal(t, stdhttp.StatusNoContent, testDelivery.Code, testDelivery.Body.String())
	require.Equal(t, testWebhookID, client.testRequest.GetWebhookId())
	require.Equal(t, "reply", client.testRequest.GetType())
	require.Equal(t, "https://test.example/hook", client.testRequest.GetOverrideUrl())
	require.Equal(t, "test-secret", client.testRequest.GetOverrideSecret())
	require.True(t, client.testRequest.GetOverrideSecretSet())
}

func TestMisskeyWebhookCompatibilityRoutesMatchDocumentedShapes(t *testing.T) {
	client := newWebhookHTTPClient()
	router := webhookTestRouter(client)
	token := signedAuthToken(t, jwt.MapClaims{"sub": "42"})

	list := performWebhookRequest(router, stdhttp.MethodPost, "/api/i/webhooks/list", "", token)
	require.Equal(t, stdhttp.StatusOK, list.Code, list.Body.String())
	var items []map[string]any
	require.NoError(t, json.Unmarshal(list.Body.Bytes(), &items))
	require.Len(t, items, 1)
	require.Equal(t, "9007199254740993", items[0]["id"])
	require.Equal(t, "9223372036854770001", items[0]["userId"])
	require.NotNil(t, items[0]["latestSentAt"])
	require.Equal(t, float64(204), items[0]["latestStatus"])
	require.Equal(t, "shared", items[0]["secret"])

	create := performWebhookRequest(router, stdhttp.MethodPost, "/api/i/webhooks/create", `{"name":"Deploy","url":"https://hooks.example/deploy","on":["note"]}`, token)
	require.Equal(t, stdhttp.StatusOK, create.Code, create.Body.String())
	var created map[string]any
	require.NoError(t, json.Unmarshal(create.Body.Bytes(), &created))
	require.Equal(t, "9007199254740993", created["id"])
	require.NotContains(t, created, "data")

	show := performWebhookRequest(router, stdhttp.MethodPost, "/i/webhooks/show", `{"webhookId":"9007199254740993"}`, token)
	require.Equal(t, stdhttp.StatusOK, show.Code, show.Body.String())
	require.Equal(t, testWebhookID, client.showRequest.GetWebhookId())

	update := performWebhookRequest(router, stdhttp.MethodPost, "/api/i/webhooks/update", `{"webhookId":"9007199254740993","secret":"","on":["reaction"],"active":true}`, token)
	require.Equal(t, stdhttp.StatusNoContent, update.Code, update.Body.String())
	require.Equal(t, []string{"reaction"}, client.updateRequest.GetOn())
	require.True(t, client.updateRequest.GetSecretSet())
	require.Empty(t, client.updateRequest.GetSecret())

	remove := performWebhookRequest(router, stdhttp.MethodPost, "/api/i/webhooks/delete", `{"webhookId":"9007199254740993"}`, token)
	require.Equal(t, stdhttp.StatusNoContent, remove.Code, remove.Body.String())

	testDelivery := performWebhookRequest(router, stdhttp.MethodPost, "/api/i/webhooks/test", `{"webhookId":"9007199254740993","type":"followed"}`, token)
	require.Equal(t, stdhttp.StatusNoContent, testDelivery.Code, testDelivery.Body.String())
	require.Equal(t, "followed", client.testRequest.GetType())
}

func TestWebhookRoutesEnforceAPITokenScopes(t *testing.T) {
	router := webhookTestRouter(newWebhookHTTPClient())
	readToken := webhookScopedToken(t, "read")
	writeToken := webhookScopedToken(t, "write")

	for _, test := range []struct {
		method string
		path   string
		body   string
		token  string
	}{
		{stdhttp.MethodPost, "/api/v1/users/me/webhooks", `{}`, readToken},
		{stdhttp.MethodPut, "/api/v1/users/me/webhooks/1", `{}`, readToken},
		{stdhttp.MethodDelete, "/api/v1/users/me/webhooks/1", "", readToken},
		{stdhttp.MethodPost, "/api/i/webhooks/create", `{}`, readToken},
		{stdhttp.MethodGet, "/api/v1/users/me/webhooks", "", writeToken},
		{stdhttp.MethodGet, "/api/v1/users/me/webhooks/1", "", writeToken},
		{stdhttp.MethodPost, "/api/v1/users/me/webhooks/1/test", `{}`, writeToken},
		{stdhttp.MethodPost, "/api/i/webhooks/test", `{}`, writeToken},
	} {
		response := performWebhookRequest(router, test.method, test.path, test.body, test.token)
		require.Equal(t, stdhttp.StatusForbidden, response.Code, test.path+": "+response.Body.String())
	}
}

func TestWebhookRoutesRejectInvalidIDsAndMissingRequiredFields(t *testing.T) {
	client := newWebhookHTTPClient()
	router := webhookTestRouter(client)
	token := signedAuthToken(t, jwt.MapClaims{"sub": "42"})

	invalidID := performWebhookRequest(router, stdhttp.MethodGet, "/api/v1/users/me/webhooks/not-a-number", "", token)
	require.Equal(t, stdhttp.StatusBadRequest, invalidID.Code, invalidID.Body.String())
	require.Nil(t, client.showRequest)

	missingCreate := performWebhookRequest(router, stdhttp.MethodPost, "/api/i/webhooks/create", `{"name":"Deploy","on":["note"]}`, token)
	require.Equal(t, stdhttp.StatusBadRequest, missingCreate.Code, missingCreate.Body.String())
	require.Nil(t, client.createRequest)

	missingType := performWebhookRequest(router, stdhttp.MethodPost, "/api/i/webhooks/test", `{"webhookId":"1"}`, token)
	require.Equal(t, stdhttp.StatusBadRequest, missingType.Code, missingType.Body.String())
	require.Nil(t, client.testRequest)
}

func webhookTestRouter(client notificationpb.NotificationServiceClient) *gin.Engine {
	gin.SetMode(gin.TestMode)
	h := NewHandler(&clients.Clients{Notification: client}, "Authorization", "Bearer", testJWTSecret)
	router := gin.New()
	NewInitControllers(h)(router)
	return router
}

func performWebhookRequest(router stdhttp.Handler, method, path, body, token string) *httptest.ResponseRecorder {
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(method, path, strings.NewReader(body))
	if body != "" {
		request.Header.Set("Content-Type", "application/json")
	}
	request.Header.Set("Authorization", "Bearer "+token)
	router.ServeHTTP(recorder, request)
	return recorder
}

func webhookScopedToken(t *testing.T, scope string) string {
	t.Helper()
	claims := jwt.MapClaims{
		"sub": "42", "jti": "webhook-token-" + scope, "exp": time.Now().Add(time.Hour).Unix(), credentialVersionClaim: "0",
		"token_type": apiTokenType, "scopes": []string{scope},
	}
	return signedAuthToken(t, claims)
}

type webhookHTTPClient struct {
	notificationpb.NotificationServiceClient
	item          *notificationpb.Webhook
	listRequest   *notificationpb.ListWebhooksRequest
	createRequest *notificationpb.CreateWebhookRequest
	showRequest   *notificationpb.ShowWebhookRequest
	updateRequest *notificationpb.UpdateWebhookRequest
	deleteRequest *notificationpb.DeleteWebhookRequest
	testRequest   *notificationpb.TestWebhookRequest
}

func newWebhookHTTPClient() *webhookHTTPClient {
	return &webhookHTTPClient{item: &notificationpb.Webhook{
		Id: testWebhookID, UserId: 9223372036854770001, Name: "Deploy", Url: "https://hooks.example/deploy", Secret: "shared",
		On: []string{"note", "reply"}, Active: true, LatestSentAt: 1710000000000, LatestStatus: 204,
	}}
}

func (c *webhookHTTPClient) ListWebhooks(_ context.Context, req *notificationpb.ListWebhooksRequest, _ ...grpc.CallOption) (*notificationpb.WebhookListResponse, error) {
	c.listRequest = req
	return &notificationpb.WebhookListResponse{Items: []*notificationpb.Webhook{c.item}}, nil
}

func (c *webhookHTTPClient) CreateWebhook(_ context.Context, req *notificationpb.CreateWebhookRequest, _ ...grpc.CallOption) (*notificationpb.WebhookResponse, error) {
	c.createRequest = req
	return &notificationpb.WebhookResponse{Webhook: c.item}, nil
}

func (c *webhookHTTPClient) ShowWebhook(_ context.Context, req *notificationpb.ShowWebhookRequest, _ ...grpc.CallOption) (*notificationpb.WebhookResponse, error) {
	c.showRequest = req
	return &notificationpb.WebhookResponse{Webhook: c.item}, nil
}

func (c *webhookHTTPClient) UpdateWebhook(_ context.Context, req *notificationpb.UpdateWebhookRequest, _ ...grpc.CallOption) (*notificationpb.WebhookResponse, error) {
	c.updateRequest = req
	return &notificationpb.WebhookResponse{Webhook: c.item}, nil
}

func (c *webhookHTTPClient) DeleteWebhook(_ context.Context, req *notificationpb.DeleteWebhookRequest, _ ...grpc.CallOption) (*notificationpb.MutationResponse, error) {
	c.deleteRequest = req
	return &notificationpb.MutationResponse{Success: true}, nil
}

func (c *webhookHTTPClient) TestWebhook(_ context.Context, req *notificationpb.TestWebhookRequest, _ ...grpc.CallOption) (*notificationpb.MutationResponse, error) {
	c.testRequest = req
	return &notificationpb.MutationResponse{Success: true}, nil
}
