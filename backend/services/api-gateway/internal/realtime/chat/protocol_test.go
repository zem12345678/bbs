package chat

import (
	"encoding/json"
	"reflect"
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

func TestNormalizeDurableEventMapsDeletedMessages(t *testing.T) {
	raw, _ := json.Marshal(map[string]any{
		"eventId": "e2", "eventType": "chat.message.deleted.v1", "version": 1,
		"payload": map[string]any{"messageId": int64(9), "roomId": int64(42), "deletedAt": int64(123)},
	})
	eventID, eventType, payload, ok := normalizeDurableEvent(raw)
	if !ok || eventID != "e2" || eventType != "message.deleted" {
		t.Fatalf("normalized event = %q %q %#v %v", eventID, eventType, payload, ok)
	}
	normalized := payload.(map[string]any)
	durable := normalized["payload"].(map[string]any)
	if durable["message_id"] != "9" || durable["room_id"] != "42" || durable["deleted_at"] != "123" {
		t.Fatalf("normalized durable payload = %#v", durable)
	}
	encoded, _ := json.Marshal(payload)
	if string(encoded) == "" || string(encoded) == "null" {
		t.Fatal("normalized payload is empty")
	}
}

func TestNormalizeDurableEventMapsMembershipLeave(t *testing.T) {
	raw, _ := json.Marshal(map[string]any{
		"eventId": "e-left", "eventType": "chat.membership.left.v1", "version": 1,
		"payload": map[string]any{"roomId": int64(42), "userId": int64(7)},
	})
	eventID, eventType, payload, ok := normalizeDurableEvent(raw)
	if !ok || eventID != "e-left" || eventType != "room.member.left" {
		t.Fatalf("normalized event = %q %q %#v %v", eventID, eventType, payload, ok)
	}
	durable := payload.(map[string]any)["payload"].(map[string]any)
	if durable["room_id"] != "42" || durable["user_id"] != "7" {
		t.Fatalf("normalized durable payload = %#v", durable)
	}
}

func TestNormalizeDurableEventMapsMemberGovernance(t *testing.T) {
	tests := []struct {
		name      string
		eventType string
		wantType  string
		payload   map[string]any
		want      map[string]any
	}{
		{
			name: "role", eventType: "chat.membership.role_updated.v1", wantType: "room.member.role_updated",
			payload: map[string]any{"roomId": int64(42), "userId": int64(7), "role": 3},
			want:    map[string]any{"room_id": "42", "user_id": "7", "role": json.Number("3")},
		},
		{
			name: "muted", eventType: "chat.membership.muted.v1", wantType: "room.member.muted",
			payload: map[string]any{"roomId": int64(42), "userId": int64(7), "mutedUntil": int64(253402300799000)},
			want:    map[string]any{"room_id": "42", "user_id": "7", "muted_until": "253402300799000"},
		},
		{
			name: "unmuted", eventType: "chat.membership.unmuted.v1", wantType: "room.member.unmuted",
			payload: map[string]any{"roomId": int64(42), "userId": int64(7)},
			want:    map[string]any{"room_id": "42", "user_id": "7"},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			raw, _ := json.Marshal(map[string]any{
				"eventId": "e-" + test.name, "eventType": test.eventType, "version": 1, "payload": test.payload,
			})
			_, eventType, payload, ok := normalizeDurableEvent(raw)
			if !ok || eventType != test.wantType {
				t.Fatalf("normalized event type = %q, want %q; ok=%v", eventType, test.wantType, ok)
			}
			durable := payload.(map[string]any)["payload"].(map[string]any)
			if !reflect.DeepEqual(durable, test.want) {
				t.Fatalf("normalized durable payload = %#v, want %#v", durable, test.want)
			}
		})
	}
}
