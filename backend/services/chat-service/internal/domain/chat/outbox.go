package chat

import (
	"context"
	"errors"
	"time"
)

var ErrOutboxLeaseLost = errors.New("chat outbox lease lost")

type OutboxEvent struct {
	DispatchID    int64
	EventID       string
	AggregateType string
	AggregateID   string
	EventType     string
	EventVersion  int
	PartitionKey  string
	Payload       []byte
	Attempt       int
	CreatedAt     time.Time
}

type OutboxRepository interface {
	ClaimPendingOutboxEvents(context.Context, string, int, time.Duration) ([]OutboxEvent, error)
	MarkOutboxEventPublished(context.Context, string, string) error
	MarkOutboxEventFailed(context.Context, string, string, string, time.Time) error
}

type OutboxPublisher interface {
	PublishOutboxEvent(context.Context, OutboxEvent) error
}
