package messaging

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"time"

	domain "reaction-service/internal/domain/reaction"
	"reaction-service/pkg/logger"

	"github.com/google/uuid"
	"github.com/segmentio/kafka-go"
	"google.golang.org/grpc/metadata"
)

type EventPublisher interface {
	PublishDomainEvents(ctx context.Context, events []domain.DomainEvent) error
}

type KafkaEventPublisher struct {
	writer *kafka.Writer
	log    logger.Logger
}

func NewKafkaEventPublisher(writer *kafka.Writer, log logger.Logger) *KafkaEventPublisher {
	return &KafkaEventPublisher{writer: writer, log: log}
}

func (p *KafkaEventPublisher) PublishDomainEvents(ctx context.Context, events []domain.DomainEvent) error {
	if len(events) == 0 {
		return nil
	}
	messages := make([]kafka.Message, 0, len(events))
	for _, event := range events {
		payload, err := marshalEvent(ctx, event)
		if err != nil {
			if p.log != nil {
				p.log.Error("marshal reaction event failed", logger.String("event", event.EventName()), logger.Error(err))
			}
			continue
		}
		messages = append(messages, kafka.Message{
			Key:   []byte(strconv.FormatInt(event.AggregateID(), 10)),
			Value: payload,
			Headers: []kafka.Header{
				{Key: "event_type", Value: []byte(event.EventName())},
				{Key: "producer", Value: []byte("reaction-service")},
			},
		})
	}
	if len(messages) == 0 {
		return nil
	}
	if err := p.writer.WriteMessages(ctx, messages...); err != nil {
		return fmt.Errorf("publish reaction events to kafka: %w", err)
	}
	if p.log != nil {
		p.log.Info("published reaction events", logger.Int("count", len(messages)))
	}
	return nil
}

func (p *KafkaEventPublisher) Close() error {
	return p.writer.Close()
}

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

func marshalEvent(ctx context.Context, event domain.DomainEvent) ([]byte, error) {
	payload, err := json.Marshal(event)
	if err != nil {
		return nil, err
	}
	return json.Marshal(eventEnvelope{
		EventID:      uuid.NewString(),
		EventType:    event.EventName(),
		EventVersion: 1,
		OccurredAt:   event.OccurredAt(),
		Producer:     "reaction-service",
		TenantID:     metadataValue(ctx, "x-organization-id"),
		AggregateID:  strconv.FormatInt(event.AggregateID(), 10),
		RequestID:    metadataValue(ctx, "x-request-id"),
		TraceID:      metadataValue(ctx, "x-correlation-id"),
		Payload:      payload,
	})
}

func metadataValue(ctx context.Context, key string) string {
	if md, ok := metadata.FromIncomingContext(ctx); ok {
		values := md.Get(key)
		if len(values) > 0 {
			return values[0]
		}
	}
	return ""
}
