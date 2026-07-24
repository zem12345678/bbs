//go:build integration

package chat

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strconv"
	"testing"
	"time"

	"api-gateway/pkg/ratelimt"

	"github.com/gorilla/websocket"
	"github.com/redis/go-redis/v9"
)

func TestTwoRealtimeInstancesShareRedis(t *testing.T) {
	redisAddr := os.Getenv("BBS_GATEWAY_TEST_REDIS_ADDR")
	if redisAddr == "" {
		t.Skip("set BBS_GATEWAY_TEST_REDIS_ADDR to run the Redis cluster integration test")
	}
	ctx := context.Background()
	cleanupClient := redis.NewClient(&redis.Options{Addr: redisAddr})
	defer cleanupClient.Close()
	if err := cleanupClient.Ping(ctx).Err(); err != nil {
		t.Fatalf("ping Redis: %v", err)
	}

	userID := time.Now().UnixNano()%1_000_000_000 + 1_000_000
	rateKey := "rate:chat:send:" + strconv.FormatInt(userID, 10)
	t.Cleanup(func() { _ = cleanupClient.Del(context.Background(), rateKey).Err() })

	redisA := redis.NewClient(&redis.Options{Addr: redisAddr})
	redisB := redis.NewClient(&redis.Options{Addr: redisAddr})
	client := &chatClientStub{}
	serviceA := NewService(redisA, client, Options{
		TicketTTL: time.Minute, AllowedOrigins: []string{"http://allowed.test"},
		SendLimiter: ratelimit.NewRedisSlidingWindowLimiter(redisA, time.Minute, 1),
	})
	serviceB := NewService(redisB, client, Options{
		TicketTTL: time.Minute, AllowedOrigins: []string{"http://allowed.test"},
		SendLimiter: ratelimit.NewRedisSlidingWindowLimiter(redisB, time.Minute, 1),
	})
	if err := serviceA.Start(); err != nil {
		t.Fatal(err)
	}
	if err := serviceB.Start(); err != nil {
		t.Fatal(err)
	}
	defer serviceA.Stop()
	defer serviceB.Stop()

	serverA := realtimeTestServer(serviceA)
	serverB := realtimeTestServer(serviceB)
	defer serverA.Close()
	defer serverB.Close()

	ticketA, _, err := serviceA.IssueTicket(ctx, userID)
	if err != nil {
		t.Fatal(err)
	}
	connectionB := dialRealtime(t, serverB.URL, ticketA)
	defer connectionB.Close()
	readEventType(t, connectionB, "session.ready")

	_, replayResponse, replayErr := websocket.DefaultDialer.Dial(
		websocketURL(serverA.URL)+"?ticket="+ticketA,
		http.Header{"Origin": []string{"http://allowed.test"}},
	)
	if replayErr == nil || replayResponse == nil || replayResponse.StatusCode != http.StatusUnauthorized {
		t.Fatalf("cross-instance ticket replay status = %v, error = %v", replayResponse, replayErr)
	}

	ticketB, _, err := serviceB.IssueTicket(ctx, userID)
	if err != nil {
		t.Fatal(err)
	}
	connectionA := dialRealtime(t, serverA.URL, ticketB)
	defer connectionA.Close()
	readEventType(t, connectionA, "session.ready")

	for _, connection := range []*websocket.Conn{connectionA, connectionB} {
		writeClientEvent(t, connection, map[string]any{
			"type": "room.subscribe", "request_id": "sub",
			"payload": map[string]any{"room_numbers": []string{"AB12CD3E"}},
		})
		readEventType(t, connection, "room.subscribed")
	}
	waitForRedisSubscribers(t, cleanupClient, "chat:room:8", 2)

	publishDurableTestEvent(t, cleanupClient, "chat:room:8", "message-1", "chat.message.created.v1", userID)
	readEventType(t, connectionA, "message.created")
	readEventType(t, connectionB, "message.created")

	publishDurableTestEvent(t, cleanupClient, "chat:user:"+strconv.FormatInt(userID, 10), "membership-1", "chat.membership.joined.v1", userID)
	publishDurableTestEvent(t, cleanupClient, "chat:room:8", "membership-1", "chat.membership.joined.v1", userID)
	readEventType(t, connectionA, "room.member.joined")
	readEventType(t, connectionB, "room.member.joined")
	publishReadAdvancedTestEvent(t, cleanupClient, userID)
	readReadAdvancedEvent(t, connectionA, userID)
	readReadAdvancedEvent(t, connectionB, userID)
	publishDurableTestEvent(t, cleanupClient, "chat:room:8", "message-2", "chat.message.created.v1", userID)
	readEventType(t, connectionA, "message.created")
	readEventType(t, connectionB, "message.created")

	writeClientEvent(t, connectionA, map[string]any{
		"type": "message.send", "request_id": "send-a",
		"payload": map[string]any{"room_no": "AB12CD3E", "client_message_id": "00000000-0000-4000-8000-000000000001", "body": "first"},
	})
	readEventType(t, connectionA, "message.ack")
	writeClientEvent(t, connectionB, map[string]any{
		"type": "message.send", "request_id": "send-b",
		"payload": map[string]any{"room_no": "AB12CD3E", "client_message_id": "00000000-0000-4000-8000-000000000002", "body": "second"},
	})
	if code := readErrorCode(t, connectionB); code != "rate_limited" {
		t.Fatalf("shared limiter error code = %q", code)
	}
}

func realtimeTestServer(service *Service) *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, request *http.Request) {
		if err := service.ServeWebSocket(w, request, request.URL.Query().Get("ticket")); err != nil {
			http.Error(w, err.Error(), http.StatusUnauthorized)
		}
	}))
}

func dialRealtime(t *testing.T, serverURL, ticket string) *websocket.Conn {
	t.Helper()
	connection, response, err := websocket.DefaultDialer.Dial(
		websocketURL(serverURL)+"?ticket="+ticket,
		http.Header{"Origin": []string{"http://allowed.test"}},
	)
	if err != nil {
		if response != nil {
			t.Fatalf("dial websocket: %v (status %d)", err, response.StatusCode)
		}
		t.Fatal(err)
	}
	return connection
}

func waitForRedisSubscribers(t *testing.T, client *redis.Client, channel string, expected int64) {
	t.Helper()
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		counts, err := client.PubSubNumSub(context.Background(), channel).Result()
		if err == nil && counts[channel] == expected {
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatalf("Redis subscribers for %s did not reach %d", channel, expected)
}

func publishDurableTestEvent(t *testing.T, client *redis.Client, channel, eventID, eventType string, userID int64) {
	t.Helper()
	payload := fmt.Sprintf(`{"eventId":%q,"eventType":%q,"version":1,"payload":{"roomId":8,"userId":%d,"seq":1}}`, eventID, eventType, userID)
	if err := client.Publish(context.Background(), channel, payload).Err(); err != nil {
		t.Fatal(err)
	}
}

func publishReadAdvancedTestEvent(t *testing.T, client *redis.Client, userID int64) {
	t.Helper()
	payload := fmt.Sprintf(`{"eventId":"read-1","eventType":"chat.read.advanced.v1","version":1,"payload":{"roomId":8,"userId":%d,"lastReadSeq":1,"latestSeq":2}}`, userID)
	if err := client.Publish(context.Background(), "chat:user:"+strconv.FormatInt(userID, 10), payload).Err(); err != nil {
		t.Fatal(err)
	}
}

func readReadAdvancedEvent(t *testing.T, connection *websocket.Conn, userID int64) {
	t.Helper()
	_ = connection.SetReadDeadline(time.Now().Add(3 * time.Second))
	_, data, err := connection.ReadMessage()
	if err != nil {
		t.Fatal(err)
	}
	var event struct {
		Type    string `json:"type"`
		Payload struct {
			Payload struct {
				RoomID      string `json:"room_id"`
				UserID      string `json:"user_id"`
				LastReadSeq string `json:"last_read_seq"`
				LatestSeq   string `json:"latest_seq"`
			} `json:"payload"`
		} `json:"payload"`
	}
	if err := json.Unmarshal(data, &event); err != nil {
		t.Fatal(err)
	}
	if event.Type != "read.advanced" {
		t.Fatalf("event type = %q, want read.advanced (%s)", event.Type, data)
	}
	if event.Payload.Payload.RoomID != "8" || event.Payload.Payload.UserID != strconv.FormatInt(userID, 10) || event.Payload.Payload.LastReadSeq != "1" || event.Payload.Payload.LatestSeq != "2" {
		t.Fatalf("read payload = %#v, want room=8 user=%d read=1 latest=2", event.Payload.Payload, userID)
	}
}
