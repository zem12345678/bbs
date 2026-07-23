package messaging

import (
	"context"
	"testing"

	domain "chat-service/internal/domain/chat"

	"github.com/segmentio/kafka-go"
)

func TestKafkaOutboxPublisherPreservesKeyPayloadAndHeaders(t *testing.T) {
	writer := &writerStub{}
	publisher := NewKafkaOutboxPublisher(writer)
	event := domain.OutboxEvent{
		EventID: "event-1", EventType: "chat.message.created.v1", EventVersion: 1,
		PartitionKey: "42", Payload: []byte(`{"eventId":"event-1"}`),
	}
	if err := publisher.PublishOutboxEvent(context.Background(), event); err != nil {
		t.Fatal(err)
	}
	if len(writer.messages) != 1 {
		t.Fatalf("messages = %d, want 1", len(writer.messages))
	}
	message := writer.messages[0]
	if string(message.Key) != "42" || string(message.Value) != string(event.Payload) {
		t.Fatalf("message key/value = %q/%q", message.Key, message.Value)
	}
	headers := make(map[string]string, len(message.Headers))
	for _, header := range message.Headers {
		headers[header.Key] = string(header.Value)
	}
	if headers["event_id"] != event.EventID || headers["event_type"] != event.EventType || headers["producer"] != "chat-service" {
		t.Fatalf("headers = %#v", headers)
	}
}

type writerStub struct {
	messages []kafka.Message
}

func (w *writerStub) WriteMessages(_ context.Context, messages ...kafka.Message) error {
	w.messages = append(w.messages, messages...)
	return nil
}

func (w *writerStub) Close() error { return nil }
