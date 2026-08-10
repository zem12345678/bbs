package http

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"api-gateway/api/proto/notificationpb"
	"api-gateway/internal/clients"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
)

func TestWebPushRoutesUseAuthenticatedUser(t *testing.T) {
	gin.SetMode(gin.TestMode)
	client := &webPushHTTPClient{
		configResponse: &notificationpb.WebPushConfigResponse{Enabled: true, PublicKey: "vapid-public"},
		registerResponse: &notificationpb.WebPushSubscriptionResponse{
			Registered: true, State: "active", UserId: 9223372036854770001, Endpoint: "https://push.example/subscription", SendReadMessage: true,
		},
		showResponse: &notificationpb.WebPushSubscriptionResponse{
			Registered: true, State: "already-subscribed", UserId: 9223372036854770001, Endpoint: "https://push.example/subscription", SendReadMessage: true,
		},
		unregisterResponse: &notificationpb.MutationResponse{Success: true, Message: "ok"},
	}
	router := webPushTestRouter(client)
	token := signedAuthToken(t, jwt.MapClaims{"sub": "9223372036854770001"})

	config := performWebPushRequest(router, http.MethodGet, "/api/v1/sw/config", "", "")
	require.Equal(t, http.StatusOK, config.Code, config.Body.String())
	require.True(t, client.configCalled)

	register := performWebPushRequest(router, http.MethodPost, "/api/v1/sw/register", `{
		"endpoint":" https://push.example/subscription ",
		"auth":" auth-key ",
		"publickey":" p256dh-key ",
		"sendReadMessage":true
	}`, token)
	require.Equal(t, http.StatusOK, register.Code, register.Body.String())
	var registerPayload map[string]any
	require.NoError(t, json.Unmarshal(register.Body.Bytes(), &registerPayload))
	require.Equal(t, map[string]any{
		"state": "subscribed", "key": nil, "userId": "9223372036854770001",
		"endpoint": "https://push.example/subscription", "sendReadMessage": true,
	}, registerPayload)
	require.Equal(t, int64(9223372036854770001), client.registerRequest.GetUserId())
	require.Equal(t, "https://push.example/subscription", client.registerRequest.GetEndpoint())
	require.Equal(t, "auth-key", client.registerRequest.GetAuth())
	require.Equal(t, "p256dh-key", client.registerRequest.GetPublicKey())
	require.True(t, client.registerRequest.GetSendReadMessage())

	show := performWebPushRequest(router, http.MethodPost, "/api/v1/sw/show-registration", `{"endpoint":"https://push.example/subscription"}`, token)
	require.Equal(t, http.StatusOK, show.Code, show.Body.String())
	var showPayload map[string]any
	require.NoError(t, json.Unmarshal(show.Body.Bytes(), &showPayload))
	require.Equal(t, map[string]any{
		"userId": "9223372036854770001", "endpoint": "https://push.example/subscription", "sendReadMessage": true,
	}, showPayload)
	require.Equal(t, int64(9223372036854770001), client.showRequest.GetUserId())

	unregister := performWebPushRequest(router, http.MethodPost, "/api/v1/sw/unregister", `{"endpoint":"https://push.example/subscription"}`, token)
	require.Equal(t, http.StatusNoContent, unregister.Code, unregister.Body.String())
	require.Empty(t, unregister.Body.String())
	require.Equal(t, int64(9223372036854770001), client.unregisterRequest.GetUserId())
}

func TestWebPushShowMissingRegistrationReturnsNoContent(t *testing.T) {
	client := &webPushHTTPClient{showResponse: &notificationpb.WebPushSubscriptionResponse{Registered: false}}
	router := webPushTestRouter(client)
	token := signedAuthToken(t, jwt.MapClaims{"sub": "42"})

	response := performWebPushRequest(router, http.MethodPost, "/api/v1/sw/show-registration", `{"endpoint":"https://push.example/subscription"}`, token)

	require.Equal(t, http.StatusNoContent, response.Code, response.Body.String())
	require.Empty(t, response.Body.String())
}

func TestWebPushMutationRoutesRequireAuthentication(t *testing.T) {
	router := webPushTestRouter(&webPushHTTPClient{})
	for _, path := range []string{"/api/v1/sw/register", "/api/v1/sw/show-registration", "/api/v1/sw/unregister"} {
		recorder := performWebPushRequest(router, http.MethodPost, path, `{"endpoint":"https://push.example/subscription","auth":"a","publickey":"p"}`, "")
		require.Equal(t, http.StatusUnauthorized, recorder.Code, path+": "+recorder.Body.String())
	}
}

func TestWebPushRegisterRejectsInvalidBodies(t *testing.T) {
	client := &webPushHTTPClient{}
	router := webPushTestRouter(client)
	token := signedAuthToken(t, jwt.MapClaims{"sub": "42"})
	cases := []string{
		`{"endpoint":"http://push.example/subscription","auth":"a","publickey":"p"}`,
		`{"endpoint":"https://push.example/subscription","auth":"","publickey":"p"}`,
		`{"endpoint":"https://push.example/subscription","auth":"a","publickey":""}`,
		`{"endpoint":"https://push.example/subscription","auth":"a","publickey":"p","user_id":7}`,
		`{"endpoint":"https://push.example/subscription","auth":"a","publickey":"p"}{}`,
	}
	for _, body := range cases {
		recorder := performWebPushRequest(router, http.MethodPost, "/api/v1/sw/register", body, token)
		require.Equal(t, http.StatusBadRequest, recorder.Code, recorder.Body.String())
	}
	require.Nil(t, client.registerRequest)
}

func TestValidWebPushEndpointAllowsSecureAndLocalDevelopmentURLs(t *testing.T) {
	gin.SetMode(gin.TestMode)
	for _, endpoint := range []string{
		"https://push.example/subscription",
		"http://localhost:8080/push",
		"http://127.0.0.1:8080/push",
		"http://[::1]:8080/push",
	} {
		recorder := httptest.NewRecorder()
		ctx, _ := gin.CreateTestContext(recorder)
		actual, ok := validWebPushEndpoint(ctx, endpoint)
		require.True(t, ok, endpoint)
		require.Equal(t, endpoint, actual)
	}
}

func webPushTestRouter(client notificationpb.NotificationServiceClient) *gin.Engine {
	h := NewHandler(&clients.Clients{Notification: client}, "Authorization", "Bearer", testJWTSecret)
	router := gin.New()
	NewInitControllers(h)(router)
	return router
}

func performWebPushRequest(router http.Handler, method, path, body, token string) *httptest.ResponseRecorder {
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(method, path, strings.NewReader(body))
	if body != "" {
		request.Header.Set("Content-Type", "application/json")
	}
	if token != "" {
		request.Header.Set("Authorization", "Bearer "+token)
	}
	router.ServeHTTP(recorder, request)
	return recorder
}

type webPushHTTPClient struct {
	notificationpb.NotificationServiceClient
	configCalled       bool
	registerRequest    *notificationpb.RegisterWebPushSubscriptionRequest
	showRequest        *notificationpb.GetWebPushSubscriptionRequest
	unregisterRequest  *notificationpb.UnregisterWebPushSubscriptionRequest
	configResponse     *notificationpb.WebPushConfigResponse
	registerResponse   *notificationpb.WebPushSubscriptionResponse
	showResponse       *notificationpb.WebPushSubscriptionResponse
	unregisterResponse *notificationpb.MutationResponse
}

func (c *webPushHTTPClient) GetWebPushConfig(context.Context, *notificationpb.GetWebPushConfigRequest, ...grpc.CallOption) (*notificationpb.WebPushConfigResponse, error) {
	c.configCalled = true
	return c.configResponse, nil
}

func (c *webPushHTTPClient) RegisterWebPushSubscription(_ context.Context, req *notificationpb.RegisterWebPushSubscriptionRequest, _ ...grpc.CallOption) (*notificationpb.WebPushSubscriptionResponse, error) {
	c.registerRequest = req
	return c.registerResponse, nil
}

func (c *webPushHTTPClient) GetWebPushSubscription(_ context.Context, req *notificationpb.GetWebPushSubscriptionRequest, _ ...grpc.CallOption) (*notificationpb.WebPushSubscriptionResponse, error) {
	c.showRequest = req
	return c.showResponse, nil
}

func (c *webPushHTTPClient) UnregisterWebPushSubscription(_ context.Context, req *notificationpb.UnregisterWebPushSubscriptionRequest, _ ...grpc.CallOption) (*notificationpb.MutationResponse, error) {
	c.unregisterRequest = req
	return c.unregisterResponse, nil
}
