package messaging

import (
	"context"
	"strconv"

	domain "chat-service/internal/domain/chat"

	"github.com/segmentio/kafka-go"
)

type KafkaOutboxPublisher struct {
	writer kafkaWriter
}

type kafkaWriter interface {
	WriteMessages(context.Context, ...kafka.Message) error
	Close() error
}

func NewKafkaOutboxPublisher(writer kafkaWriter) *KafkaOutboxPublisher {
	return &KafkaOutboxPublisher{writer: writer}
}

func (p *KafkaOutboxPublisher) PublishOutboxEvent(ctx context.Context, event domain.OutboxEvent) error {
	return p.writer.WriteMessages(ctx, kafka.Message{
		Key:   []byte(event.PartitionKey),
		Value: event.Payload,
		Headers: []kafka.Header{
			{Key: "event_id", Value: []byte(event.EventID)},
			{Key: "event_type", Value: []byte(event.EventType)},
			{Key: "event_version", Value: []byte(strconv.Itoa(event.EventVersion))},
			{Key: "producer", Value: []byte("chat-service")},
		},
	})
}

func (p *KafkaOutboxPublisher) Close() error {
	return p.writer.Close()
}
