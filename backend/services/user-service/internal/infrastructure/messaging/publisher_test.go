package messaging

import (
	"encoding/json"
	"testing"
	"time"

	domain "user-service/internal/domain/user"
)

func TestMarshalAccountDeletionOutboxEventPreservesStableIdentity(t *testing.T) {
	event := domain.AccountDeletionOutboxEvent{
		EventID: "event-42", AggregateID: 42, EventType: "user.deleted", MessageKey: "42",
		Payload: []byte(`{"user_id":42,"account_state":"anonymized"}`), OccurredAt: time.Date(2026, 8, 4, 1, 2, 3, 0, time.UTC),
	}
	payload, err := marshalAccountDeletionOutboxEvent(event)
	if err != nil {
		t.Fatalf("marshal event: %v", err)
	}
	var envelope eventEnvelope
	if err := json.Unmarshal(payload, &envelope); err != nil {
		t.Fatalf("decode envelope: %v", err)
	}
	if envelope.EventID != event.EventID || envelope.EventType != event.EventType || envelope.AggregateID != "42" || !envelope.OccurredAt.Equal(event.OccurredAt) {
		t.Fatalf("envelope=%+v", envelope)
	}
	if string(envelope.Payload) != string(event.Payload) {
		t.Fatalf("payload=%s want=%s", envelope.Payload, event.Payload)
	}
}
