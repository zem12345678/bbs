package outbox

import (
	"context"
	"errors"
	"time"
)

var ErrLeaseLost = errors.New("content lifecycle outbox lease lost")

// LifecycleEvent is a persisted content visibility change. Its payload is the
// original Kafka envelope so retries retain the same event identity.
type LifecycleEvent struct {
	EventID    string
	MessageKey string
	EventType  string
	Payload    []byte
	// Attempt is incremented for every claim and fences stale lease holders
	// from completing a newer claim.
	Attempt int
}

type LifecycleRepository interface {
	ClaimPendingLifecycleEvents(ctx context.Context, owner string, limit int, leaseDuration time.Duration) ([]LifecycleEvent, error)
	ClaimLifecycleEvent(ctx context.Context, eventID, owner string, leaseDuration time.Duration) (*LifecycleEvent, error)
	MarkLifecycleEventPublished(ctx context.Context, eventID, owner string, attempt int) error
	MarkLifecycleEventFailed(ctx context.Context, eventID, owner string, attempt int, message string, nextAttemptAt time.Time) error
}

type LifecyclePublisher interface {
	PublishLifecycleEvent(ctx context.Context, event LifecycleEvent) error
}
