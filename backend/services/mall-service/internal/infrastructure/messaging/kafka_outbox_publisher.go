package messaging

import (
	"context"

	domain "mall-service/internal/domain/mall"

	"github.com/segmentio/kafka-go"
)

type KafkaOutboxPublisher struct {
	writer        *kafka.Writer
	topicsByEvent map[string]string
}

func NewKafkaOutboxPublisher(writer *kafka.Writer, topicsByEvent map[string]string) *KafkaOutboxPublisher {
	return &KafkaOutboxPublisher{writer: writer, topicsByEvent: topicsByEvent}
}

func (p *KafkaOutboxPublisher) PublishOutboxEvent(ctx context.Context, event domain.OutboxEvent) error {
	topic := p.topicsByEvent[event.EventType]
	if topic == "" {
		topic = event.EventType
	}
	message := kafka.Message{
		Key:   []byte(event.MessageKey),
		Value: []byte(event.PayloadJSON),
		Headers: []kafka.Header{
			{Key: "event_type", Value: []byte(event.EventType)},
			{Key: "producer", Value: []byte("mall-service")},
		},
	}
	if p.writer.Topic == "" {
		message.Topic = topic
	}
	return p.writer.WriteMessages(ctx, message)
}

func (p *KafkaOutboxPublisher) Close() error {
	return p.writer.Close()
}
