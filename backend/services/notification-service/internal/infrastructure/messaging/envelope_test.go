package messaging

import (
	"encoding/json"
	"testing"

	"github.com/segmentio/kafka-go"
)

func TestDecodeEnvelopeFlatPayload(t *testing.T) {
	raw := []byte(`{"event_id":"evt-1","event_type":"mall.refund.approved.v1","occurred_at_unix_ms":1783450000000,"refund_id":4,"order_id":12,"user_id":332983416859402240}`)
	var env eventEnvelope

	if err := decodeEnvelope(raw, &env); err != nil {
		t.Fatalf("decodeEnvelope() error = %v", err)
	}
	if env.EventID != "evt-1" {
		t.Fatalf("EventID = %q, want evt-1", env.EventID)
	}
	if env.EventType != "mall.refund.approved.v1" {
		t.Fatalf("EventType = %q, want mall.refund.approved.v1", env.EventType)
	}
	if len(env.Payload) == 0 {
		t.Fatal("Payload is empty")
	}
	var payload mallRefundReviewedPayload
	if err := json.Unmarshal(env.Payload, &payload); err != nil {
		t.Fatalf("unmarshal payload error = %v", err)
	}
	if payload.RefundID != 4 || payload.UserID != 332983416859402240 {
		t.Fatalf("payload = %+v, want refund_id=4 user_id=332983416859402240", payload)
	}
}

func TestDecodeKafkaEnvelopeUsesHeadersAsFallback(t *testing.T) {
	msg := kafka.Message{
		Value: []byte(`{"refund_id":5,"order_id":13,"user_id":332983416859402240}`),
		Headers: []kafka.Header{
			{Key: "event_type", Value: []byte("mall.refund.rejected.v1")},
			{Key: "producer", Value: []byte("mall-service")},
		},
	}
	var env eventEnvelope

	if err := decodeKafkaEnvelope(msg, &env); err != nil {
		t.Fatalf("decodeKafkaEnvelope() error = %v", err)
	}
	if env.EventType != "mall.refund.rejected.v1" {
		t.Fatalf("EventType = %q, want mall.refund.rejected.v1", env.EventType)
	}
	if env.Producer != "mall-service" {
		t.Fatalf("Producer = %q, want mall-service", env.Producer)
	}
	if len(env.Payload) == 0 {
		t.Fatal("Payload is empty")
	}
}
