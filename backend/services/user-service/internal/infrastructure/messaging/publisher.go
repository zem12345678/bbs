package messaging

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"

	domain "user-service/internal/domain/user"
	"user-service/pkg/logger"

	"github.com/google/uuid"
	"github.com/segmentio/kafka-go"
	"google.golang.org/grpc/metadata"
)

type EventPublisher interface {
	PublishDomainEvents(ctx context.Context, events []domain.DomainEvent) error
}

// AccountDeletionOutboxPublisher publishes the durable user.deleted event
// created by the account deletion transaction. The stored event ID is reused
// so retries remain idempotent for downstream consumers.
type AccountDeletionOutboxPublisher interface {
	PublishAccountDeletionOutboxEvent(ctx context.Context, event domain.AccountDeletionOutboxEvent) error
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
				p.log.Error("marshal user event failed", logger.String("event", event.EventName()), logger.Error(err))
			}
			continue
		}
		messages = append(messages, kafka.Message{
			Key:   []byte(strconv.FormatInt(event.AggregateID(), 10)),
			Value: payload,
			Headers: []kafka.Header{
				{Key: "event_type", Value: []byte(event.EventName())},
				{Key: "producer", Value: []byte("user-service")},
			},
		})
	}
	if len(messages) == 0 {
		return nil
	}
	if err := p.writer.WriteMessages(ctx, messages...); err != nil {
		return fmt.Errorf("publish user events to kafka: %w", err)
	}
	return nil
}

func (p *KafkaEventPublisher) PublishAccountDeletionOutboxEvent(ctx context.Context, event domain.AccountDeletionOutboxEvent) error {
	if p == nil || p.writer == nil {
		return fmt.Errorf("user event publisher is unavailable")
	}
	payload, err := marshalAccountDeletionOutboxEvent(event)
	if err != nil {
		return fmt.Errorf("marshal account deletion outbox event: %w", err)
	}
	if err := p.writer.WriteMessages(ctx, kafka.Message{
		Key:   []byte(event.MessageKey),
		Value: payload,
		Headers: []kafka.Header{
			{Key: "event_id", Value: []byte(event.EventID)},
			{Key: "event_type", Value: []byte(event.EventType)},
			{Key: "producer", Value: []byte("user-service")},
		},
	}); err != nil {
		return fmt.Errorf("publish account deletion event to kafka: %w", err)
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
		Producer:     "user-service",
		TenantID:     metadataValue(ctx, "x-organization-id"),
		AggregateID:  strconv.FormatInt(event.AggregateID(), 10),
		RequestID:    metadataValue(ctx, "x-request-id"),
		TraceID:      metadataValue(ctx, "x-correlation-id"),
		Payload:      payload,
	})
}

func marshalAccountDeletionOutboxEvent(event domain.AccountDeletionOutboxEvent) ([]byte, error) {
	if strings.TrimSpace(event.EventID) == "" || strings.TrimSpace(event.EventType) == "" || strings.TrimSpace(event.MessageKey) == "" {
		return nil, fmt.Errorf("account deletion outbox event identity is incomplete")
	}
	if len(event.Payload) == 0 {
		return nil, fmt.Errorf("account deletion outbox event payload is empty")
	}
	if !json.Valid(event.Payload) {
		return nil, fmt.Errorf("account deletion outbox event payload is invalid")
	}
	return json.Marshal(eventEnvelope{
		EventID:      event.EventID,
		EventType:    event.EventType,
		EventVersion: 1,
		OccurredAt:   event.OccurredAt,
		Producer:     "user-service",
		AggregateID:  strconv.FormatInt(event.AggregateID, 10),
		Payload:      json.RawMessage(event.Payload),
	})
}

func metadataValue(ctx context.Context, key string) string {
	if md, ok := metadata.FromIncomingContext(ctx); ok {
		values := md.Get(key)
		if len(values) > 0 {
			return values[0]
		}
	}
	if md, ok := metadata.FromOutgoingContext(ctx); ok {
		values := md.Get(key)
		if len(values) > 0 {
			return values[0]
		}
	}
	return ""
}
