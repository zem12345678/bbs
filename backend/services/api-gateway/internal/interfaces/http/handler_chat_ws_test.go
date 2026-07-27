package http

import (
	"context"
	"encoding/json"
	stdhttp "net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	realtimechat "api-gateway/internal/realtime/chat"

	"github.com/alicebob/miniredis/v2"
	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"github.com/gorilla/websocket"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/require"
)

func TestCreateChatWebSocketTicketUsesIPAndUserRateLimitKeys(t *testing.T) {
	gin.SetMode(gin.TestMode)
	server := miniredis.RunT(t)
	redisClient := redis.NewClient(&redis.Options{Addr: server.Addr()})
	t.Cleanup(func() {
		_ = redisClient.Close()
		server.Close()
	})

	limiter := &chatRateLimiterStub{}
	h := NewHandlerWithRealtime(nil, "Authorization", "Bearer", testJWTSecret, nil,
		realtimechat.NewService(redisClient, nil, realtimechat.Options{TicketTTL: time.Minute}),
	)
	h.SetChatTicketLimit(limiter)
	router := gin.New()
	NewInitControllers(h)(router)

	recorder := httptest.NewRecorder()
	req := httptest.NewRequest(stdhttp.MethodPost, "/api/v1/chat/ws-tickets", nil)
	req.RemoteAddr = "198.51.100.20:40321"
	accessToken := signedAuthToken(t, jwt.MapClaims{
		"sub": "42", "exp": time.Now().Add(time.Hour).Unix(), credentialVersionClaim: "credential-v2",
	})
	req.Header.Set("Authorization", "Bearer "+accessToken)
	router.ServeHTTP(recorder, req)

	require.Equal(t, stdhttp.StatusOK, recorder.Code, recorder.Body.String())
	require.Equal(t, chatTicketRateLimitKeys("198.51.100.20", 42), limiter.keys)
	require.Contains(t, recorder.Body.String(), `"ticket"`)
	var response struct {
		Data struct {
			Ticket string `json:"ticket"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &response))
	storedTicket, err := realtimechat.NewTicketStore(redisClient, time.Minute).Consume(context.Background(), response.Data.Ticket)
	require.NoError(t, err)
	require.Equal(t, tokenRevocationFingerprint(accessToken), storedTicket.TokenFingerprint)
	require.Equal(t, "credential-v2", storedTicket.CredentialVersion)
	require.True(t, storedTicket.CredentialVersionClaim)
	require.NotNil(t, storedTicket.TokenExpiresAt)
}

func TestCreateChatWebSocketTicketReturnsRateLimitedBeforeIssuingTicket(t *testing.T) {
	gin.SetMode(gin.TestMode)
	limiter := &chatRateLimiterStub{limited: true}
	h := NewHandlerWithRealtime(nil, "Authorization", "Bearer", testJWTSecret, nil,
		realtimechat.NewService(nil, nil, realtimechat.Options{}),
	)
	h.SetChatTicketLimit(limiter)
	h.SetChatTicketRetryAfter(time.Minute)
	router := gin.New()
	NewInitControllers(h)(router)

	recorder := httptest.NewRecorder()
	req := httptest.NewRequest(stdhttp.MethodPost, "/api/v1/chat/ws-tickets", nil)
	req.RemoteAddr = "198.51.100.20:40321"
	req.Header.Set("Authorization", "Bearer "+signedAuthToken(t, jwt.MapClaims{"sub": "42"}))
	router.ServeHTTP(recorder, req)

	require.Equal(t, stdhttp.StatusTooManyRequests, recorder.Code, recorder.Body.String())
	require.Contains(t, recorder.Body.String(), `"legacy_code":"rate_limited"`)
	require.Equal(t, "60", recorder.Header().Get("Retry-After"))
	require.Len(t, limiter.keys, 1)
	require.Equal(t, chatTicketRateLimitKeys("198.51.100.20", 42)[0], limiter.keys[0])
}

func TestServeChatWebSocketReturnsRateLimitedBeforeUpgradeWhenIPCapacityReached(t *testing.T) {
	gin.SetMode(gin.TestMode)
	redisServer := miniredis.RunT(t)
	redisClient := redis.NewClient(&redis.Options{Addr: redisServer.Addr()})
	service := realtimechat.NewService(redisClient, nil, realtimechat.Options{
		TicketTTL:           time.Minute,
		MaxConnectionsPerIP: 1,
	})
	upgradeServer := httptest.NewServer(stdhttp.HandlerFunc(func(w stdhttp.ResponseWriter, request *stdhttp.Request) {
		if err := service.ServeWebSocketWithClientIP(w, request, request.URL.Query().Get("ticket"), "198.51.100.20"); err != nil {
			stdhttp.Error(w, err.Error(), stdhttp.StatusServiceUnavailable)
		}
	}))
	var firstConnection *websocket.Conn
	t.Cleanup(func() {
		if firstConnection != nil {
			_ = firstConnection.Close()
		}
		upgradeServer.Close()
		_ = service.Stop()
		redisServer.Close()
	})

	firstTicket, _, err := service.IssueTicket(context.Background(), 42)
	require.NoError(t, err)
	firstConnection, response, err := websocket.DefaultDialer.Dial(
		"ws"+strings.TrimPrefix(upgradeServer.URL, "http")+"?ticket="+firstTicket,
		nil,
	)
	if err != nil {
		if response != nil {
			t.Fatalf("first websocket dial: %v (status %d)", err, response.StatusCode)
		}
		t.Fatal(err)
	}
	_ = firstConnection.SetReadDeadline(time.Now().Add(time.Second))
	_, _, err = firstConnection.ReadMessage()
	require.NoError(t, err)

	h := NewHandlerWithRealtime(nil, "Authorization", "Bearer", testJWTSecret, nil, service)
	router := gin.New()
	NewInitControllers(h)(router)
	ticketRecorder := httptest.NewRecorder()
	ticketRequest := httptest.NewRequest(stdhttp.MethodPost, "/api/v1/chat/ws-tickets", nil)
	ticketRequest.RemoteAddr = "198.51.100.20:40321"
	ticketRequest.Header.Set("Authorization", "Bearer "+signedAuthToken(t, jwt.MapClaims{"sub": "43"}))
	router.ServeHTTP(ticketRecorder, ticketRequest)
	require.Equal(t, stdhttp.StatusTooManyRequests, ticketRecorder.Code, ticketRecorder.Body.String())
	require.Equal(t, "30", ticketRecorder.Header().Get("Retry-After"))
	require.Contains(t, ticketRecorder.Body.String(), `"legacy_code":"rate_limited"`)

	secondTicket, _, err := service.IssueTicket(context.Background(), 43)
	require.NoError(t, err)
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(stdhttp.MethodGet, "/api/v1/chat/ws?ticket="+secondTicket, nil)
	request.RemoteAddr = "198.51.100.20:40321"
	router.ServeHTTP(recorder, request)

	require.Equal(t, stdhttp.StatusTooManyRequests, recorder.Code, recorder.Body.String())
	require.Contains(t, recorder.Body.String(), `"legacy_code":"rate_limited"`)
}
