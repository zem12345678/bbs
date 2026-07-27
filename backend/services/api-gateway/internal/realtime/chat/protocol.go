package chat

import (
	"bytes"
	"encoding/json"
	"errors"
	"strconv"
	"strings"
)

const (
	maxClientMessageBytes = 64 << 10
	maxRequestIDLength    = 128
	maxRoomSubscriptions  = 100
)

type ClientEnvelope struct {
	Type      string          `json:"type"`
	RequestID string          `json:"request_id,omitempty"`
	Payload   json.RawMessage `json:"payload"`
}

func decodeClientEnvelope(data []byte) (ClientEnvelope, error) {
	if len(data) > maxClientMessageBytes {
		return ClientEnvelope{}, errors.New("websocket message is too large")
	}
	var envelope ClientEnvelope
	if err := json.Unmarshal(data, &envelope); err != nil {
		return ClientEnvelope{}, errors.New("invalid websocket message")
	}
	envelope.Type = strings.TrimSpace(envelope.Type)
	if envelope.Type == "" {
		return ClientEnvelope{}, errors.New("websocket message type is required")
	}
	if len([]rune(envelope.RequestID)) > maxRequestIDLength {
		return ClientEnvelope{}, errors.New("websocket request id is too long")
	}
	if len(envelope.Payload) == 0 || string(envelope.Payload) == "null" {
		envelope.Payload = json.RawMessage(`{}`)
	}
	return envelope, nil
}

func encodeServerEvent(eventType, requestID string, payload any) []byte {
	encoded, err := json.Marshal(struct {
		Type      string `json:"type"`
		RequestID string `json:"request_id,omitempty"`
		Payload   any    `json:"payload,omitempty"`
	}{Type: eventType, RequestID: requestID, Payload: payload})
	if err != nil {
		return []byte(`{"type":"error","payload":{"code":"internal_error"}}`)
	}
	return encoded
}

type subscribePayload struct {
	RoomNumbers []string `json:"room_numbers"`
}

func parseSubscribePayload(raw json.RawMessage) ([]string, error) {
	var payload subscribePayload
	if err := json.Unmarshal(raw, &payload); err != nil {
		return nil, errors.New("invalid room subscription payload")
	}
	if len(payload.RoomNumbers) > maxRoomSubscriptions {
		return nil, errors.New("at most 100 room subscriptions are allowed")
	}
	seen := make(map[string]struct{}, len(payload.RoomNumbers))
	rooms := make([]string, 0, len(payload.RoomNumbers))
	for _, room := range payload.RoomNumbers {
		room = strings.ToUpper(strings.TrimSpace(room))
		if room == "" {
			continue
		}
		if _, ok := seen[room]; ok {
			continue
		}
		seen[room] = struct{}{}
		rooms = append(rooms, room)
	}
	return rooms, nil
}

type sendPayload struct {
	RoomNo          string `json:"room_no"`
	ClientMessageID string `json:"client_message_id"`
	Body            string `json:"body"`
}

type readPayload struct {
	RoomNo  string      `json:"room_no"`
	ReadSeq flexibleInt `json:"read_seq"`
}

type flexibleInt int64

func (value *flexibleInt) UnmarshalJSON(data []byte) error {
	text := strings.TrimSpace(string(data))
	if strings.HasPrefix(text, `"`) {
		var raw string
		if err := json.Unmarshal(data, &raw); err != nil {
			return err
		}
		text = strings.TrimSpace(raw)
	}
	parsed, err := strconv.ParseInt(text, 10, 64)
	if err != nil {
		return errors.New("invalid integer")
	}
	*value = flexibleInt(parsed)
	return nil
}

func decodePayload(raw json.RawMessage, target any, message string) error {
	if err := json.Unmarshal(raw, target); err != nil {
		return errors.New(message)
	}
	return nil
}

func validateMessagePayload(payload sendPayload) error {
	if strings.TrimSpace(payload.RoomNo) == "" || strings.TrimSpace(payload.ClientMessageID) == "" {
		return errors.New("room_no and client_message_id are required")
	}
	if strings.TrimSpace(payload.Body) == "" {
		return errors.New("message body is required")
	}
	if len([]rune(payload.Body)) > 4000 {
		return errors.New("message body is too long")
	}
	return nil
}

func normalizeDurableEvent(raw []byte) (eventID, eventType string, payload any, ok bool) {
	var envelope struct {
		EventID   string          `json:"eventId"`
		EventType string          `json:"eventType"`
		Version   int             `json:"version"`
		Payload   json.RawMessage `json:"payload"`
	}
	if json.Unmarshal(raw, &envelope) != nil || envelope.EventID == "" || envelope.Version != 1 {
		return "", "", nil, false
	}
	switch envelope.EventType {
	case "chat.message.created.v1":
		eventType = "message.created"
	case "chat.message.deleted.v1":
		eventType = "message.deleted"
	case "chat.membership.joined.v1":
		eventType = "room.member.joined"
	case "chat.read.advanced.v1":
		eventType = "read.advanced"
	case "chat.announcement.updated.v1":
		eventType = "announcement.updated"
	default:
		return "", "", nil, false
	}
	var rawPayload map[string]any
	decoder := json.NewDecoder(bytes.NewReader(envelope.Payload))
	decoder.UseNumber()
	if decoder.Decode(&rawPayload) != nil {
		return "", "", nil, false
	}
	normalized := map[string]any{
		"event_id":   envelope.EventID,
		"event_type": envelope.EventType,
		"payload":    normalizeDurablePayload(rawPayload),
	}
	return envelope.EventID, eventType, normalized, true
}

func normalizeDurablePayload(input map[string]any) map[string]any {
	output := make(map[string]any, len(input))
	for key, value := range input {
		name := durableFieldName(key)
		switch nested := value.(type) {
		case map[string]any:
			output[name] = normalizeDurablePayload(nested)
		case json.Number:
			if isDurableIntegerField(key) {
				output[name] = nested.String()
			} else {
				output[name] = nested
			}
		default:
			output[name] = value
		}
	}
	return output
}

func durableFieldName(key string) string {
	known := map[string]string{
		"roomId": "room_id", "roomNo": "room_no", "userId": "user_id", "senderId": "sender_id",
		"messageId": "message_id", "clientMessageId": "client_message_id", "lastReadSeq": "last_read_seq",
		"latestSeq": "latest_seq", "announcementVersion": "announcement_version", "createdAt": "created_at",
		"updatedAt": "updated_at", "deletedAt": "deleted_at",
	}
	if value, ok := known[key]; ok {
		return value
	}
	return key
}

func isDurableIntegerField(key string) bool {
	return strings.HasSuffix(key, "Id") || strings.HasSuffix(key, "Seq") || strings.HasSuffix(key, "Version") || key == "seq" || key == "createdAt" || key == "updatedAt" || key == "deletedAt"
}

func int64String(value int64) string { return strconv.FormatInt(value, 10) }
