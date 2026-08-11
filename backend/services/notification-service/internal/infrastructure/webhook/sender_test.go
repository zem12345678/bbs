package webhook

import (
	"context"
	"encoding/json"
	"net"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	domain "notification-service/internal/domain/notification"
)

type staticResolver struct {
	addresses []net.IPAddr
}

func (r staticResolver) LookupIPAddr(context.Context, string) ([]net.IPAddr, error) {
	return r.addresses, nil
}

func TestSenderPostsMisskeyCompatibleEnvelope(t *testing.T) {
	t.Parallel()
	wantCreatedAt := time.Date(2026, time.August, 11, 12, 30, 0, 0, time.UTC)
	var gotHeaders http.Header
	var gotBody map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotHeaders = r.Header.Clone()
		if err := json.NewDecoder(r.Body).Decode(&gotBody); err != nil {
			t.Fatalf("decode webhook body: %v", err)
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()

	resolver := staticResolver{addresses: []net.IPAddr{{IP: net.ParseIP("127.0.0.1")}}}
	sender := NewSender(domain.WebhookConfig{ServerURL: "https://bbs.example.test", AllowPrivateEndpoints: true})
	sender.resolver = resolver
	sender.client = newSafeHTTPClient(resolver, true)
	status, err := sender.Send(context.Background(), domain.WebhookDelivery{
		WebhookID: 9007199254740993,
		UserID:    42,
		EventID:   "evt-1",
		EventType: domain.WebhookEventNote,
		URL:       server.URL,
		Secret:    "shared-secret",
		Payload:   []byte(`{"note":{"id":"note-1"}}`),
		CreatedAt: wantCreatedAt,
	})
	if err != nil {
		t.Fatalf("send webhook: %v", err)
	}
	if status != http.StatusNoContent {
		t.Fatalf("status = %d, want %d", status, http.StatusNoContent)
	}
	if gotHeaders.Get("X-Misskey-Hook-Id") != "9007199254740993" || gotHeaders.Get("X-Misskey-Hook-Secret") != "shared-secret" {
		t.Fatalf("webhook headers = %+v", gotHeaders)
	}
	if gotHeaders.Get("X-Misskey-Host") != "bbs.example.test" {
		t.Fatalf("X-Misskey-Host = %q", gotHeaders.Get("X-Misskey-Host"))
	}
	if gotBody["server"] != "https://bbs.example.test" || gotBody["hookId"] != "9007199254740993" || gotBody["userId"] != "42" || gotBody["eventId"] != "evt-1" || gotBody["type"] != "note" {
		t.Fatalf("envelope = %+v", gotBody)
	}
	if gotBody["createdAt"] != float64(wantCreatedAt.UnixMilli()) {
		t.Fatalf("createdAt = %v", gotBody["createdAt"])
	}
}

func TestValidateEndpointRejectsPrivateAddressesUnlessExplicitlyAllowed(t *testing.T) {
	t.Parallel()
	resolver := staticResolver{addresses: []net.IPAddr{{IP: net.ParseIP("127.0.0.1")}}}
	if err := validateEndpoint(context.Background(), "https://hooks.example.test/callback", resolver, false); err != ErrUnsafeEndpoint {
		t.Fatalf("private endpoint error = %v, want %v", err, ErrUnsafeEndpoint)
	}
	if err := validateEndpoint(context.Background(), "http://hooks.example.test/callback", resolver, false); err != ErrUnsafeEndpoint {
		t.Fatalf("http endpoint error = %v, want %v", err, ErrUnsafeEndpoint)
	}
	if err := validateEndpoint(context.Background(), "http://hooks.example.test/callback", resolver, true); err != nil {
		t.Fatalf("allowed local endpoint error = %v", err)
	}
}

func TestMarshalWebhookPayloadRejectsInvalidJSON(t *testing.T) {
	t.Parallel()
	_, err := marshalWebhookPayload("https://bbs.example.test", domain.WebhookDelivery{WebhookID: 1, UserID: 1, EventID: "evt", EventType: "note", CreatedAt: time.Unix(0, 0), Payload: []byte("not-json")})
	if err == nil {
		t.Fatal("invalid payload returned nil error")
	}
}
