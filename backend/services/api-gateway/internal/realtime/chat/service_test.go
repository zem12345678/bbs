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

	"github.com/gorilla/websocket"
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

type chatClientStub struct {
	mu             sync.Mutex
	validateUserID int64
	sendUserID     int64
	readUserID     int64
	sendCalls      int
}

func (c *chatClientStub) ValidateRoomSubscriptions(_ context.Context, request *chatpb.ValidateRoomSubscriptionsRequest, _ ...grpc.CallOption) (*chatpb.ValidateRoomSubscriptionsResponse, error) {
	c.mu.Lock()
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

func (l *wsRateLimiterStub) Limit(context.Context, string) (bool, error) {
	return l.limited, nil
}

func (c *chatClientStub) AdvanceRead(_ context.Context, request *chatpb.AdvanceReadRequest, _ ...grpc.CallOption) (*chatpb.AdvanceReadResponse, error) {
	c.mu.Lock()
	c.readUserID = request.GetUserId()
	c.mu.Unlock()
	return &chatpb.AdvanceReadResponse{
		Membership: &chatpb.Membership{RoomId: 8, UserId: request.GetUserId(), LastReadSeq: request.GetReadSeq()},
		LatestSeq:  1,
	}, nil
}
