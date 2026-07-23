package chat

import (
	"encoding/json"
	"testing"
)

func TestDecodeClientEnvelopeAndSubscriptions(t *testing.T) {
	envelope, err := decodeClientEnvelope([]byte(`{"type":"room.subscribe","request_id":"r1","payload":{"room_numbers":["ab12cd3e","AB12CD3E"]}}`))
	if err != nil {
		t.Fatal(err)
	}
	rooms, err := parseSubscribePayload(envelope.Payload)
	if err != nil {
		t.Fatal(err)
	}
	if len(rooms) != 1 || rooms[0] != "AB12CD3E" {
		t.Fatalf("rooms = %#v", rooms)
	}
}

func TestDecodeClientEnvelopeRejectsOversizedInput(t *testing.T) {
	if _, err := decodeClientEnvelope(make([]byte, maxClientMessageBytes+1)); err == nil {
		t.Fatal("oversized websocket message unexpectedly accepted")
	}
}

func TestNormalizeDurableEventConvertsIDsToStrings(t *testing.T) {
	type event struct {
		EventID   string         `json:"eventId"`
		EventType string         `json:"eventType"`
		Version   int            `json:"version"`
		Payload   map[string]any `json:"payload"`
	}
	raw, _ := json.Marshal(event{EventID: "e1", EventType: "chat.message.created.v1", Version: 1, Payload: map[string]any{"roomId": int64(42), "seq": int64(9)}})
	eventID, eventType, payload, ok := normalizeDurableEvent(raw)
	if !ok || eventID != "e1" || eventType != "message.created" {
		t.Fatalf("normalized event = %q %q %#v %v", eventID, eventType, payload, ok)
	}
	encoded, _ := json.Marshal(payload)
	if string(encoded) == "" || string(encoded) == "null" {
		t.Fatal("normalized payload is empty")
	}
}
