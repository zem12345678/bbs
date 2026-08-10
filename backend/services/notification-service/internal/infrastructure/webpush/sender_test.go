package webpush

import (
	"encoding/json"
	"testing"

	domain "notification-service/internal/domain/notification"
)

func TestMarshalPayloadUsesStringIDAndMessagesURL(t *testing.T) {
	body, err := marshalPayload(domain.WebPushDelivery{Notification: domain.Notification{
		ID: 9_007_199_254_740_993, Type: "comment", Title: "new comment", Content: "body",
	}})
	if err != nil {
		t.Fatalf("marshal payload: %v", err)
	}
	var got map[string]any
	if err := json.Unmarshal(body, &got); err != nil {
		t.Fatalf("decode payload: %v", err)
	}
	if got["id"] != "9007199254740993" || got["url"] != "/dashboard/messages" || got["body"] != "body" {
		t.Fatalf("payload = %#v", got)
	}
}
