package messaging

import (
	"bytes"
	"encoding/json"
	"time"
)

type eventEnvelope struct {
	EventID      string          `json:"event_id"`
	EventType    string          `json:"event_type"`
	EventVersion int32           `json:"event_version"`
	OccurredAt   time.Time       `json:"occurred_at"`
	Producer     string          `json:"producer"`
	TenantID     string          `json:"tenant_id"`
	AggregateID  string          `json:"aggregate_id"`
	RequestID    string          `json:"request_id"`
	TraceID      string          `json:"trace_id"`
	Payload      json.RawMessage `json:"payload"`
}

func decodeEnvelope(value []byte, env *eventEnvelope) error {
	value = bytes.TrimPrefix(bytes.TrimSpace(value), []byte{0xEF, 0xBB, 0xBF})
	if err := json.Unmarshal(value, env); err != nil {
		return err
	}
	if len(env.Payload) == 0 {
		env.Payload = json.RawMessage(value)
	}
	return nil
}
