package chat

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"api-gateway/api/proto/chatpb"
	"api-gateway/pkg/ratelimit"

	"github.com/alicebob/miniredis/v2"
	"github.com/gorilla/websocket"
	"github.com/redis/go-redis/v9"
	"google.golang.org/grpc"
)

func TestWebSocketBindsCommandsToTicketUser(t *testing.T) {
	backend := newTicketBackend()
	client := &chatClientStub{}
	service := testRealtimeService(backend, client, []string{"http://allowed.test"})
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, request *http.Request) {
		if err := service.ServeWebSocket(w, request, request.URL.Query().Get("ticket")); err != nil {
			http.Error(w, err.Error(), http.StatusUnauthorized)
		}
	}))
	defer server.Close()

	token, _, err := service.IssueTicket(context.Background(), 42)
	if err != nil {
		t.Fatal(err)
	}
	header := http.Header{"Origin": []string{"http://allowed.test"}}
	connection, response, err := websocket.DefaultDialer.Dial(websocketURL(server.URL)+"?ticket="+token, header)
	if err != nil {
		if response != nil {
			t.Fatalf("dial websocket: %v (status %d)", err, response.StatusCode)
		}
		t.Fatal(err)
	}
	defer connection.Close()
	readEventType(t, connection, "session.ready")

	writeClientEvent(t, connection, map[string]any{
		"type": "room.subscribe", "request_id": "sub-1",
		"payload": map[string]any{"room_numbers": []string{"AB12CD3E"}, "user_id": "999"},
	})
	readEventType(t, connection, "room.subscribed")
	writeClientEvent(t, connection, map[string]any{
		"type": "message.send", "request_id": "msg-1",
		"payload": map[string]any{"room_no": "AB12CD3E", "client_message_id": "00000000-0000-4000-8000-000000000001", "body": "hello", "user_id": "999"},
	})
	readEventType(t, connection, "message.ack")
	writeClientEvent(t, connection, map[string]any{
		"type": "read.advance", "request_id": "read-1",
		"payload": map[string]any{"room_no": "AB12CD3E", "read_seq": "1", "user_id": "999"},
	})
	readEventType(t, connection, "read.advanced")

	client.mu.Lock()
	defer client.mu.Unlock()
	if client.validateUserID != 42 || client.sendUserID != 42 || client.readUserID != 42 {
		t.Fatalf("command user ids = %d/%d/%d, want 42", client.validateUserID, client.sendUserID, client.readUserID)
	}
}

func TestInvalidatedSessionClosesBeforeCommandRPC(t *testing.T) {
	backend := newTicketBackend()
	client := &chatClientStub{}
	service := testRealtimeService(backend, client, nil)
	validator := &sessionValidatorStub{err: ErrSessionInvalid}
	service.SetSessionValidator(validator)
	connection := newConnection(nil, service.hub, service, 42)
	connection.ticket = Ticket{UserID: 42, TokenFingerprint: strings.Repeat("a", 64)}
	if err := service.hub.Register(connection); err != nil {
		t.Fatal(err)
	}
	defer service.hub.Unregister(connection)

	service.HandleCommand(context.Background(), connection, ClientEnvelope{
		Type: "room.subscribe", RequestID: "sub-1",
		Payload: json.RawMessage(`{"room_numbers":["AB12CD3E"]}`),
	})

	select {
	case <-connection.done:
	case <-time.After(time.Second):
		t.Fatal("invalidated session did not close")
	}
	if validator.calls != 1 {
		t.Fatalf("session validation calls = %d, want 1", validator.calls)
	}
	client.mu.Lock()
	defer client.mu.Unlock()
	if client.validateCalls != 0 {
		t.Fatalf("subscription RPC calls = %d, want 0", client.validateCalls)
	}
}

func TestRoomSubscribedSerializesRoomIDAsExactString(t *testing.T) {
	const roomID int64 = 9223372036854770000

	encoded := encodeServerEvent("room.subscribed", "sub-1", map[string]any{
		"subscriptions": roomSubscriptionEvents([]RoomSubscription{{RoomID: roomID, RoomNo: "AB12CD3E"}}),
	})
	var event struct {
		Payload struct {
			Subscriptions []struct {
				RoomID json.RawMessage `json:"room_id"`
			} `json:"subscriptions"`
		} `json:"payload"`
	}
	if err := json.Unmarshal(encoded, &event); err != nil {
		t.Fatal(err)
	}
	if len(event.Payload.Subscriptions) != 1 {
		t.Fatalf("subscriptions = %d, want 1", len(event.Payload.Subscriptions))
	}
	want := `"9223372036854770000"`
	if got := string(event.Payload.Subscriptions[0].RoomID); got != want {
		t.Fatalf("room_id JSON token = %s, want %s", got, want)
	}
	if got := (RoomSubscription{RoomID: roomID}).RoomID; got != roomID {
		t.Fatalf("internal room ID = %d, want %d", got, roomID)
	}
}

func TestWebSocketCommandAcksIncludeRequestedRoomNo(t *testing.T) {
	service := testRealtimeService(newTicketBackend(), &chatClientStub{}, nil)
	connection := newConnection(nil, service.hub, service, 42)
	if err := service.hub.Register(connection); err != nil {
		t.Fatal(err)
	}
	defer service.hub.Unregister(connection)

	service.handleSubscribe(context.Background(), connection, ClientEnvelope{
		Type: "room.subscribe", RequestID: "sub-1",
		Payload: json.RawMessage(`{"room_numbers":["AB12CD3E"]}`),
	})
	assertOutboundEventType(t, connection.outbound, "room.subscribed")

	service.handleSend(context.Background(), connection, ClientEnvelope{
		Type: "message.send", RequestID: "msg-1",
		Payload: json.RawMessage(`{"room_no":"AB12CD3E","client_message_id":"00000000-0000-4000-8000-000000000001","body":"hello"}`),
	})
	var messageAck struct {
		Type    string `json:"type"`
		Payload struct {
			Message struct {
				RoomNo string `json:"room_no"`
			} `json:"message"`
		} `json:"payload"`
	}
	if err := json.Unmarshal(readOutboundEvent(t, connection.outbound), &messageAck); err != nil {
		t.Fatal(err)
	}
	if messageAck.Type != "message.ack" || messageAck.Payload.Message.RoomNo != "AB12CD3E" {
		t.Fatalf("message ack = %#v, want room_no AB12CD3E", messageAck)
	}

	service.handleRead(context.Background(), connection, ClientEnvelope{
		Type: "read.advance", RequestID: "read-1",
		Payload: json.RawMessage(`{"room_no":"AB12CD3E","read_seq":"1"}`),
	})
	var readAck struct {
		Type    string `json:"type"`
		Payload struct {
			RoomNo string `json:"room_no"`
		} `json:"payload"`
	}
	if err := json.Unmarshal(readOutboundEvent(t, connection.outbound), &readAck); err != nil {
		t.Fatal(err)
	}
	if readAck.Type != "read.advanced" || readAck.Payload.RoomNo != "AB12CD3E" {
		t.Fatalf("read ack = %#v, want room_no AB12CD3E", readAck)
	}
}

func TestRoomSubscribedWaitsForRedisReadinessWhileRouteIsAlreadyInstalled(t *testing.T) {
	backend := newTicketBackend()
	service := testRealtimeService(backend, &chatClientStub{}, nil)
	subscriber := &gatedSubscriber{
		waitStarted: make(chan string, 1),
		release:     make(chan struct{}),
	}
	service.hub.SetSubscriber(subscriber)
	connection := newConnection(nil, service.hub, service, 42)
	if err := service.hub.Register(connection); err != nil {
		t.Fatal(err)
	}
	defer service.hub.Unregister(connection)

	done := make(chan struct{})
	go func() {
		service.handleSubscribe(context.Background(), connection, ClientEnvelope{
			Type: "room.subscribe", RequestID: "sub-1",
			Payload: json.RawMessage(`{"room_numbers":["AB12CD3E"]}`),
		})
		close(done)
	}()
	select {
	case channel := <-subscriber.waitStarted:
		if channel != roomChannel(8) {
			t.Fatalf("waited for channel %q, want %q", channel, roomChannel(8))
		}
	case <-time.After(time.Second):
		t.Fatal("room subscription did not wait for redis readiness")
	}
	assertNoOutboundEvent(t, connection.outbound)

	// The Hub route is intentionally installed before Redis confirms the
	// subscription. A message immediately following the Redis acknowledgement
	// can therefore not fall into an acknowledgement-to-route gap.
	service.hub.Broadcast(roomChannel(8), []byte(`{"eventId":"message-1","eventType":"chat.message.created.v1","version":1,"payload":{"roomId":8,"seq":1}}`))
	assertOutboundEventType(t, connection.outbound, "message.created")

	close(subscriber.release)
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("room subscribe did not finish after redis readiness")
	}
	assertOutboundEventType(t, connection.outbound, "room.subscribed")
}

func TestRoomSubscribedCanReceiveAnImmediateRedisPublish(t *testing.T) {
	server := miniredis.RunT(t)
	redisClient := redis.NewClient(&redis.Options{Addr: server.Addr()})
	service := NewService(redisClient, &chatClientStub{}, Options{})
	if err := service.Start(); err != nil {
		t.Fatal(err)
	}
	connection := newConnection(nil, service.hub, service, 42)
	if err := service.hub.Register(connection); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		service.hub.Unregister(connection)
		_ = service.Stop()
		server.Close()
	})

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	service.handleSubscribe(ctx, connection, ClientEnvelope{
		Type: "room.subscribe", RequestID: "sub-1",
		Payload: json.RawMessage(`{"room_numbers":["AB12CD3E"]}`),
	})
	assertOutboundEventType(t, connection.outbound, "room.subscribed")

	payload := `{"eventId":"message-1","eventType":"chat.message.created.v1","version":1,"payload":{"roomId":8,"seq":1}}`
	if err := redisClient.Publish(context.Background(), roomChannel(8), payload).Err(); err != nil {
		t.Fatal(err)
	}
	assertOutboundEventType(t, connection.outbound, "message.created")
}

func TestWebSocketSendRateLimitReturnsErrorEventBeforeRPC(t *testing.T) {
	backend := newTicketBackend()
	client := &chatClientStub{}
	service := testRealtimeService(backend, client, []string{"http://allowed.test"})
	service.sendLimit = &wsRateLimiterStub{limited: true}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, request *http.Request) {
		if err := service.ServeWebSocket(w, request, request.URL.Query().Get("ticket")); err != nil {
			http.Error(w, err.Error(), http.StatusUnauthorized)
		}
	}))
	defer server.Close()

	token, _, err := service.IssueTicket(context.Background(), 42)
	if err != nil {
		t.Fatal(err)
	}
	connection, response, err := websocket.DefaultDialer.Dial(
		websocketURL(server.URL)+"?ticket="+token,
		http.Header{"Origin": []string{"http://allowed.test"}},
	)
	if err != nil {
		if response != nil {
			t.Fatalf("dial websocket: %v (status %d)", err, response.StatusCode)
		}
		t.Fatal(err)
	}
	defer connection.Close()
	readEventType(t, connection, "session.ready")

	writeClientEvent(t, connection, map[string]any{
		"type": "room.subscribe", "request_id": "sub-1",
		"payload": map[string]any{"room_numbers": []string{"AB12CD3E"}},
	})
	readEventType(t, connection, "room.subscribed")
	writeClientEvent(t, connection, map[string]any{
		"type": "message.send", "request_id": "msg-1",
		"payload": map[string]any{
			"room_no": "AB12CD3E", "client_message_id": "00000000-0000-4000-8000-000000000001", "body": "hello",
		},
	})
	code := readErrorCode(t, connection)
	if code != "rate_limited" {
		t.Fatalf("websocket error code = %q, want rate_limited", code)
	}
	client.mu.Lock()
	defer client.mu.Unlock()
	if client.sendCalls != 0 {
		t.Fatalf("send RPC calls = %d, want 0", client.sendCalls)
	}
}

func TestWebSocketSubscribeRateLimitReturnsErrorEventBeforeRPC(t *testing.T) {
	backend := newTicketBackend()
	client := &chatClientStub{}
	service := testRealtimeService(backend, client, nil)
	service.subscribeLimit = &wsRateLimiterStub{limited: true}
	connection := newConnection(nil, service.hub, service, 42)
	if err := service.hub.Register(connection); err != nil {
		t.Fatal(err)
	}
	defer service.hub.Unregister(connection)

	service.handleSubscribe(context.Background(), connection, ClientEnvelope{
		Type: "room.subscribe", RequestID: "sub-1",
		Payload: json.RawMessage(`{"room_numbers":["AB12CD3E"]}`),
	})
	if code := outboundErrorCode(t, connection.outbound); code != "rate_limited" {
		t.Fatalf("subscription rate-limit error = %q, want rate_limited", code)
	}
	client.mu.Lock()
	defer client.mu.Unlock()
	if client.validateCalls != 0 {
		t.Fatalf("subscription validation RPC calls = %d, want 0", client.validateCalls)
	}
}

func TestWebSocketReadRateLimitReturnsErrorEventBeforeRPC(t *testing.T) {
	backend := newTicketBackend()
	client := &chatClientStub{}
	service := testRealtimeService(backend, client, nil)
	service.readLimit = &wsRateLimiterStub{limited: true}
	connection := newConnection(nil, service.hub, service, 42)
	if err := service.hub.Register(connection); err != nil {
		t.Fatal(err)
	}
	defer service.hub.Unregister(connection)

	service.handleSubscribe(context.Background(), connection, ClientEnvelope{
		Type: "room.subscribe", RequestID: "sub-1",
		Payload: json.RawMessage(`{"room_numbers":["AB12CD3E"]}`),
	})
	assertOutboundEventType(t, connection.outbound, "room.subscribed")
	service.handleRead(context.Background(), connection, ClientEnvelope{
		Type: "read.advance", RequestID: "read-1",
		Payload: json.RawMessage(`{"room_no":"AB12CD3E","read_seq":"1"}`),
	})
	if code := outboundErrorCode(t, connection.outbound); code != "rate_limited" {
		t.Fatalf("read rate-limit error = %q, want rate_limited", code)
	}
	client.mu.Lock()
	defer client.mu.Unlock()
	if client.readCalls != 0 {
		t.Fatalf("read RPC calls = %d, want 0", client.readCalls)
	}
}

func TestWebSocketReadRateLimitIsSharedAcrossGatewayInstances(t *testing.T) {
	server := miniredis.RunT(t)
	redisA := redis.NewClient(&redis.Options{Addr: server.Addr()})
	redisB := redis.NewClient(&redis.Options{Addr: server.Addr()})
	client := &chatClientStub{}
	serviceA := NewService(redisA, client, Options{
		ReadLimiter: ratelimit.NewRedisSlidingWindowLimiter(redisA, time.Minute, 1),
	})
	serviceB := NewService(redisB, client, Options{
		ReadLimiter: ratelimit.NewRedisSlidingWindowLimiter(redisB, time.Minute, 1),
	})
	serviceA.hub.SetSubscriber(&recordingSubscriber{})
	serviceB.hub.SetSubscriber(&recordingSubscriber{})
	connectionA := newConnection(nil, serviceA.hub, serviceA, 42)
	connectionB := newConnection(nil, serviceB.hub, serviceB, 42)
	if err := serviceA.hub.Register(connectionA); err != nil {
		t.Fatal(err)
	}
	if err := serviceB.hub.Register(connectionB); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		serviceA.hub.Unregister(connectionA)
		serviceB.hub.Unregister(connectionB)
		_ = redisA.Close()
		_ = redisB.Close()
		server.Close()
	})

	for _, candidate := range []struct {
		service    *Service
		connection *Connection
		requestID  string
	}{
		{serviceA, connectionA, "sub-a"},
		{serviceB, connectionB, "sub-b"},
	} {
		candidate.service.handleSubscribe(context.Background(), candidate.connection, ClientEnvelope{
			Type: "room.subscribe", RequestID: candidate.requestID,
			Payload: json.RawMessage(`{"room_numbers":["AB12CD3E"]}`),
		})
		assertOutboundEventType(t, candidate.connection.outbound, "room.subscribed")
	}

	serviceA.handleRead(context.Background(), connectionA, ClientEnvelope{
		Type: "read.advance", RequestID: "read-a",
		Payload: json.RawMessage(`{"room_no":"AB12CD3E","read_seq":"1"}`),
	})
	assertOutboundEventType(t, connectionA.outbound, "read.advanced")
	serviceB.handleRead(context.Background(), connectionB, ClientEnvelope{
		Type: "read.advance", RequestID: "read-b",
		Payload: json.RawMessage(`{"room_no":"AB12CD3E","read_seq":"2"}`),
	})
	if code := outboundErrorCode(t, connectionB.outbound); code != "rate_limited" {
		t.Fatalf("cross-instance read rate-limit error = %q, want rate_limited", code)
	}
	client.mu.Lock()
	defer client.mu.Unlock()
	if client.readCalls != 1 {
		t.Fatalf("read RPC calls = %d, want 1", client.readCalls)
	}
}

func TestWebSocketSubscribeRateLimitIsSharedAcrossGatewayInstances(t *testing.T) {
	server := miniredis.RunT(t)
	redisA := redis.NewClient(&redis.Options{Addr: server.Addr()})
	redisB := redis.NewClient(&redis.Options{Addr: server.Addr()})
	client := &chatClientStub{}
	serviceA := NewService(redisA, client, Options{
		SubscribeLimiter: ratelimit.NewRedisSlidingWindowLimiter(redisA, time.Minute, 1),
	})
	serviceB := NewService(redisB, client, Options{
		SubscribeLimiter: ratelimit.NewRedisSlidingWindowLimiter(redisB, time.Minute, 1),
	})
	serviceA.hub.SetSubscriber(&recordingSubscriber{})
	serviceB.hub.SetSubscriber(&recordingSubscriber{})
	connectionA := newConnection(nil, serviceA.hub, serviceA, 42)
	connectionB := newConnection(nil, serviceB.hub, serviceB, 42)
	if err := serviceA.hub.Register(connectionA); err != nil {
		t.Fatal(err)
	}
	if err := serviceB.hub.Register(connectionB); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		serviceA.hub.Unregister(connectionA)
		serviceB.hub.Unregister(connectionB)
		_ = redisA.Close()
		_ = redisB.Close()
		server.Close()
	})

	serviceA.handleSubscribe(context.Background(), connectionA, ClientEnvelope{
		Type: "room.subscribe", RequestID: "sub-a",
		Payload: json.RawMessage(`{"room_numbers":["AB12CD3E"]}`),
	})
	assertOutboundEventType(t, connectionA.outbound, "room.subscribed")
	serviceB.handleSubscribe(context.Background(), connectionB, ClientEnvelope{
		Type: "room.subscribe", RequestID: "sub-b",
		Payload: json.RawMessage(`{"room_numbers":["AB12CD3E"]}`),
	})
	if code := outboundErrorCode(t, connectionB.outbound); code != "rate_limited" {
		t.Fatalf("cross-instance subscription rate-limit error = %q, want rate_limited", code)
	}
	client.mu.Lock()
	defer client.mu.Unlock()
	if client.validateCalls != 1 {
		t.Fatalf("subscription validation RPC calls = %d, want 1", client.validateCalls)
	}
}

func TestRejectedOriginDoesNotConsumeTicket(t *testing.T) {
	backend := newTicketBackend()
	service := testRealtimeService(backend, &chatClientStub{}, []string{"http://allowed.test"})
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, request *http.Request) {
		err := service.ServeWebSocket(w, request, request.URL.Query().Get("ticket"))
		if errors.Is(err, ErrOriginRejected) {
			http.Error(w, err.Error(), http.StatusForbidden)
		} else if err != nil {
			http.Error(w, err.Error(), http.StatusUnauthorized)
		}
	}))
	defer server.Close()

	token, _, err := service.IssueTicket(context.Background(), 7)
	if err != nil {
		t.Fatal(err)
	}
	_, response, err := websocket.DefaultDialer.Dial(
		websocketURL(server.URL)+"?ticket="+token,
		http.Header{"Origin": []string{"http://evil.test"}},
	)
	if err == nil || response == nil || response.StatusCode != http.StatusForbidden {
		t.Fatalf("rejected origin response = %#v, err = %v", response, err)
	}
	connection, _, err := websocket.DefaultDialer.Dial(
		websocketURL(server.URL)+"?ticket="+token,
		http.Header{"Origin": []string{"http://allowed.test"}},
	)
	if err != nil {
		t.Fatalf("ticket was consumed by rejected origin: %v", err)
	}
	_ = connection.Close()
}

func TestWebSocketConnectionLimitRejectsBeforeUpgrade(t *testing.T) {
	server := miniredis.RunT(t)
	redisClient := redis.NewClient(&redis.Options{Addr: server.Addr()})
	service := NewService(redisClient, &chatClientStub{}, Options{
		TicketTTL:             time.Minute,
		MaxConnectionsPerUser: 1,
	})
	existing := newConnection(nil, service.hub, service, 42)
	existing.clientIP = "198.51.100.20"
	if err := service.hub.Register(existing); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		service.hub.Unregister(existing)
		_ = redisClient.Close()
		server.Close()
	})

	token, _, err := service.IssueTicket(context.Background(), 42)
	if err != nil {
		t.Fatal(err)
	}
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/api/v1/chat/ws?ticket="+token, nil)
	err = service.ServeWebSocketWithClientIP(recorder, request, token, existing.clientIP)
	if !errors.Is(err, ErrUserConnectionLimit) {
		t.Fatalf("connection limit error = %v, want %v", err, ErrUserConnectionLimit)
	}
	if recorder.Body.Len() != 0 || recorder.Header().Get("Upgrade") != "" {
		t.Fatalf("capacity rejection unexpectedly upgraded response: %#v / %q", recorder.Header(), recorder.Body.String())
	}
}

func TestWebSocketConnectionLimitIsSharedAcrossGatewayInstances(t *testing.T) {
	server := miniredis.RunT(t)
	redisA := redis.NewClient(&redis.Options{Addr: server.Addr()})
	redisB := redis.NewClient(&redis.Options{Addr: server.Addr()})
	serviceA := NewService(redisA, nil, Options{TicketTTL: time.Minute, MaxConnectionsPerUser: 1})
	serviceB := NewService(redisB, nil, Options{TicketTTL: time.Minute, MaxConnectionsPerUser: 1})
	upgradeServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, request *http.Request) {
		if err := serviceA.ServeWebSocketWithClientIP(w, request, request.URL.Query().Get("ticket"), "198.51.100.20"); err != nil {
			http.Error(w, err.Error(), http.StatusServiceUnavailable)
		}
	}))
	var firstConnection *websocket.Conn
	t.Cleanup(func() {
		if firstConnection != nil {
			_ = firstConnection.Close()
		}
		upgradeServer.Close()
		_ = serviceA.Stop()
		_ = serviceB.Stop()
		server.Close()
	})

	firstTicket, _, err := serviceA.IssueTicket(context.Background(), 42)
	if err != nil {
		t.Fatal(err)
	}
	firstConnection, response, err := websocket.DefaultDialer.Dial(websocketURL(upgradeServer.URL)+"?ticket="+firstTicket, nil)
	if err != nil {
		if response != nil {
			t.Fatalf("first websocket dial: %v (status %d)", err, response.StatusCode)
		}
		t.Fatal(err)
	}
	readEventType(t, firstConnection, "session.ready")

	secondTicket, _, err := serviceB.IssueTicket(context.Background(), 42)
	if err != nil {
		t.Fatal(err)
	}
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/api/v1/chat/ws?ticket="+secondTicket, nil)
	err = serviceB.ServeWebSocketWithClientIP(recorder, request, secondTicket, "198.51.100.21")
	if !errors.Is(err, ErrUserConnectionLimit) {
		t.Fatalf("shared connection limit error = %v, want %v", err, ErrUserConnectionLimit)
	}
	if recorder.Body.Len() != 0 || recorder.Header().Get("Upgrade") != "" {
		t.Fatalf("shared capacity rejection unexpectedly upgraded response: %#v / %q", recorder.Header(), recorder.Body.String())
	}
}

func testRealtimeService(backend *ticketBackend, client ChatClient, origins []string) *Service {
	hub := NewHub()
	service := &Service{client: client, tickets: NewTicketStore(backend, time.Minute), hub: hub}
	service.upgrader = websocket.Upgrader{CheckOrigin: originChecker(origins)}
	return service
}

func websocketURL(httpURL string) string { return "ws" + strings.TrimPrefix(httpURL, "http") }

func writeClientEvent(t *testing.T, connection *websocket.Conn, event any) {
	t.Helper()
	if err := connection.WriteJSON(event); err != nil {
		t.Fatal(err)
	}
}

func readEventType(t *testing.T, connection *websocket.Conn, expected string) {
	t.Helper()
	_ = connection.SetReadDeadline(time.Now().Add(3 * time.Second))
	_, data, err := connection.ReadMessage()
	if err != nil {
		t.Fatal(err)
	}
	var event struct {
		Type string `json:"type"`
	}
	if err := json.Unmarshal(data, &event); err != nil {
		t.Fatal(err)
	}
	if event.Type != expected {
		t.Fatalf("event type = %q, want %q (%s)", event.Type, expected, data)
	}
}

func readErrorCode(t *testing.T, connection *websocket.Conn) string {
	t.Helper()
	_ = connection.SetReadDeadline(time.Now().Add(3 * time.Second))
	_, data, err := connection.ReadMessage()
	if err != nil {
		t.Fatal(err)
	}
	var event struct {
		Type    string `json:"type"`
		Payload struct {
			Code string `json:"code"`
		} `json:"payload"`
	}
	if err := json.Unmarshal(data, &event); err != nil {
		t.Fatal(err)
	}
	if event.Type != "error" {
		t.Fatalf("event type = %q, want error (%s)", event.Type, data)
	}
	return event.Payload.Code
}

func outboundErrorCode(t *testing.T, outbound <-chan []byte) string {
	t.Helper()
	select {
	case event := <-outbound:
		var envelope struct {
			Type    string `json:"type"`
			Payload struct {
				Code string `json:"code"`
			} `json:"payload"`
		}
		if err := json.Unmarshal(event, &envelope); err != nil {
			t.Fatal(err)
		}
		if envelope.Type != "error" {
			t.Fatalf("event type = %q, want error (%s)", envelope.Type, event)
		}
		return envelope.Payload.Code
	case <-time.After(time.Second):
		t.Fatal("did not receive error event")
		return ""
	}
}

func assertOutboundEventType(t *testing.T, outbound <-chan []byte, expected string) {
	t.Helper()
	event := readOutboundEvent(t, outbound)
	var envelope struct {
		Type string `json:"type"`
	}
	if err := json.Unmarshal(event, &envelope); err != nil {
		t.Fatal(err)
	}
	if envelope.Type != expected {
		t.Fatalf("event type = %q, want %q (%s)", envelope.Type, expected, event)
	}
}

func readOutboundEvent(t *testing.T, outbound <-chan []byte) []byte {
	t.Helper()
	select {
	case event := <-outbound:
		return event
	case <-time.After(time.Second):
		t.Fatal("did not receive outbound event")
		return nil
	}
}

type chatClientStub struct {
	mu             sync.Mutex
	validateUserID int64
	validateCalls  int
	sendUserID     int64
	readUserID     int64
	sendCalls      int
	readCalls      int
}

func (c *chatClientStub) ValidateRoomSubscriptions(_ context.Context, request *chatpb.ValidateRoomSubscriptionsRequest, _ ...grpc.CallOption) (*chatpb.ValidateRoomSubscriptionsResponse, error) {
	c.mu.Lock()
	c.validateCalls++
	c.validateUserID = request.GetUserId()
	c.mu.Unlock()
	return &chatpb.ValidateRoomSubscriptionsResponse{
		RoomNumbers:   []string{"AB12CD3E"},
		Subscriptions: []*chatpb.RoomSubscription{{RoomId: 8, RoomNo: "AB12CD3E"}},
	}, nil
}

func (c *chatClientStub) SendMessage(_ context.Context, request *chatpb.SendMessageRequest, _ ...grpc.CallOption) (*chatpb.SendMessageResponse, error) {
	c.mu.Lock()
	c.sendCalls++
	c.sendUserID = request.GetUserId()
	c.mu.Unlock()
	return &chatpb.SendMessageResponse{Message: &chatpb.ChatMessage{
		Id: 10, RoomId: 8, Seq: 1, SenderId: request.GetUserId(), ClientMessageId: request.GetClientMessageId(), Body: request.GetBody(), CreatedAt: 1,
	}, LatestSeq: 1}, nil
}

type wsRateLimiterStub struct {
	limited bool
}

type sessionValidatorStub struct {
	err   error
	calls int
}

func (s *sessionValidatorStub) ValidateChatSession(_ context.Context, _ Ticket) error {
	s.calls++
	return s.err
}

type gatedSubscriber struct {
	recordingSubscriber
	waitStarted chan string
	release     chan struct{}
}

func (s *gatedSubscriber) Wait(ctx context.Context, channel string) error {
	select {
	case s.waitStarted <- channel:
	case <-ctx.Done():
		return ctx.Err()
	}
	select {
	case <-s.release:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (l *wsRateLimiterStub) Limit(context.Context, string) (bool, error) {
	return l.limited, nil
}

func (c *chatClientStub) AdvanceRead(_ context.Context, request *chatpb.AdvanceReadRequest, _ ...grpc.CallOption) (*chatpb.AdvanceReadResponse, error) {
	c.mu.Lock()
	c.readCalls++
	c.readUserID = request.GetUserId()
	c.mu.Unlock()
	return &chatpb.AdvanceReadResponse{
		Membership: &chatpb.Membership{RoomId: 8, UserId: request.GetUserId(), LastReadSeq: request.GetReadSeq()},
		LatestSeq:  1,
	}, nil
}
