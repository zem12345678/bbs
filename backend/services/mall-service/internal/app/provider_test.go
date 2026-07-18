package app

import (
	"testing"

	mallapp "mall-service/internal/application/mall"
)

func TestMallOutboxTopicsRouteAllMallEvents(t *testing.T) {
	const topic = "mall.events"
	topics := mallOutboxTopics(topic)

	for _, eventType := range []string{
		mallapp.OrderPaidEventType,
		mallapp.OrderShippedEventType,
		mallapp.OrderCompletedEventType,
		mallapp.RefundApprovedEventType,
		mallapp.RefundRejectedEventType,
		mallapp.ReviewPublishedEventType,
		mallapp.ReviewHiddenEventType,
		mallapp.EntitlementRevokedEventType,
	} {
		if got := topics[eventType]; got != topic {
			t.Fatalf("topic for %q = %q, want %q", eventType, got, topic)
		}
	}
	if len(topics) != 8 {
		t.Fatalf("mapped mall event topics = %d, want 8", len(topics))
	}
}
