package messaging

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"

	topicDomain "content-service/internal/domain/topic"
	"content-service/pkg/logger"

	"github.com/google/uuid"
	"github.com/segmentio/kafka-go"
	"google.golang.org/grpc/metadata"
)

type DomainEvent interface {
	EventName() string
	OccurredAt() time.Time
	AggregateID() int64
}

type identifiedEvent interface {
	EventID() string
}

const eventPublishTimeout = 2 * time.Second

type EventPublisher interface {
	PublishDomainEvents(ctx context.Context, events []DomainEvent) error
}

type QAAcceptanceOutboxPublisher interface {
	PublishQAAcceptanceOutboxEvent(ctx context.Context, event topicDomain.QAAcceptanceOutboxEvent) error
}

type KafkaEventPublisher struct {
	writer *kafka.Writer
	log    logger.Logger
}

func NewKafkaEventPublisher(writer *kafka.Writer, log logger.Logger) *KafkaEventPublisher {
	return &KafkaEventPublisher{writer: writer, log: log}
}

func (p *KafkaEventPublisher) PublishDomainEvents(ctx context.Context, events []DomainEvent) error {
	if len(events) == 0 {
		return nil
	}
	messages := make([]kafka.Message, 0, len(events))
	for _, event := range events {
		payload, err := marshalEvent(ctx, event)
		if err != nil {
			if p.log != nil {
				p.log.Error("marshal article event failed", logger.String("event", event.EventName()), logger.Error(err))
			}
			continue
		}
		messages = append(messages, kafka.Message{
			Key:   []byte(strconv.FormatInt(event.AggregateID(), 10)),
			Value: payload,
			Headers: []kafka.Header{
				{Key: "event_type", Value: []byte(event.EventName())},
				{Key: "producer", Value: []byte("content-service")},
			},
		})
	}
	if len(messages) == 0 {
		return nil
	}
	publishCtx, cancel := context.WithTimeout(ctx, eventPublishTimeout)
	defer cancel()
	if err := p.writer.WriteMessages(publishCtx, messages...); err != nil {
		return fmt.Errorf("publish content events to kafka: %w", err)
	}
	if p.log != nil {
		p.log.Info("published content events", logger.Int("count", len(messages)))
	}
	return nil
}

func (p *KafkaEventPublisher) Close() error {
	return p.writer.Close()
}

func (p *KafkaEventPublisher) PublishQAAcceptanceOutboxEvent(ctx context.Context, event topicDomain.QAAcceptanceOutboxEvent) error {
	message := kafka.Message{
		Key:   []byte(event.MessageKey),
		Value: event.Payload,
		Headers: []kafka.Header{
			{Key: "event_type", Value: []byte("content.qa.accepted.v1")},
			{Key: "producer", Value: []byte("content-service")},
		},
	}
	if err := p.writer.WriteMessages(ctx, message); err != nil {
		return fmt.Errorf("publish QA acceptance outbox event to kafka: %w", err)
	}
	return nil
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

func marshalEvent(ctx context.Context, event DomainEvent) ([]byte, error) {
	payload, err := json.Marshal(event)
	if err != nil {
		return nil, err
	}
	eventID := uuid.NewString()
	if identified, ok := event.(identifiedEvent); ok {
		if value := strings.TrimSpace(identified.EventID()); value != "" {
			eventID = value
		}
	}
	return json.Marshal(eventEnvelope{
		EventID:      eventID,
		EventType:    event.EventName(),
		EventVersion: 1,
		OccurredAt:   event.OccurredAt(),
		Producer:     "content-service",
		TenantID:     metadataValue(ctx, "x-organization-id"),
		AggregateID:  strconv.FormatInt(event.AggregateID(), 10),
		RequestID:    metadataValue(ctx, "x-request-id"),
		TraceID:      metadataValue(ctx, "x-correlation-id"),
		Payload:      payload,
	})
}

func EncodeDomainEvent(ctx context.Context, event DomainEvent) ([]byte, error) {
	return marshalEvent(ctx, event)
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
