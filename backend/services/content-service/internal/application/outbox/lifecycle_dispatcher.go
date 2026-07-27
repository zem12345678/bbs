package outbox

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	domain "content-service/internal/domain/outbox"
)

const maxRetryDelay = time.Minute

type LifecycleDispatcher struct {
	repository    domain.LifecycleRepository
	publisher     domain.LifecyclePublisher
	owner         string
	batchSize     int
	leaseDuration time.Duration
	retryDelay    time.Duration
}

type LifecycleDispatcherOptions struct {
	Owner         string
	BatchSize     int
	LeaseDuration time.Duration
	RetryDelay    time.Duration
}

func NewLifecycleDispatcher(repository domain.LifecycleRepository, publisher domain.LifecyclePublisher, options LifecycleDispatcherOptions) *LifecycleDispatcher {
	if strings.TrimSpace(options.Owner) == "" {
		options.Owner = "content-service"
	}
	if options.BatchSize <= 0 {
		options.BatchSize = 20
	}
	if options.LeaseDuration <= 0 {
		options.LeaseDuration = 30 * time.Second
	}
	if options.RetryDelay <= 0 {
		options.RetryDelay = 3 * time.Second
	}
	return &LifecycleDispatcher{
		repository:    repository,
		publisher:     publisher,
		owner:         strings.TrimSpace(options.Owner),
		batchSize:     options.BatchSize,
		leaseDuration: options.LeaseDuration,
		retryDelay:    options.RetryDelay,
	}
}

func (d *LifecycleDispatcher) DispatchOnce(ctx context.Context) (int, error) {
	if d.repository == nil || d.publisher == nil {
		return 0, errors.New("content lifecycle outbox is unavailable")
	}
	events, err := d.repository.ClaimPendingLifecycleEvents(ctx, d.owner, d.batchSize, d.leaseDuration)
	if err != nil {
		return 0, fmt.Errorf("claim content lifecycle outbox events: %w", err)
	}
	processed := 0
	for _, event := range events {
		if err := d.dispatch(ctx, event); err != nil {
			return processed, err
		}
		processed++
	}
	return processed, nil
}

// DispatchEvent lets a successful command deliver its own event immediately.
// If another worker already owns it, false is returned and the background loop
// remains responsible for completion.
func (d *LifecycleDispatcher) DispatchEvent(ctx context.Context, eventID string) (bool, error) {
	if d.repository == nil || d.publisher == nil {
		return false, errors.New("content lifecycle outbox is unavailable")
	}
	event, err := d.repository.ClaimLifecycleEvent(ctx, strings.TrimSpace(eventID), d.owner, d.leaseDuration)
	if err != nil {
		return false, fmt.Errorf("claim content lifecycle outbox event %q: %w", eventID, err)
	}
	if event == nil {
		return false, nil
	}
	if err := d.dispatch(ctx, *event); err != nil {
		return true, err
	}
	return true, nil
}

func (d *LifecycleDispatcher) dispatch(ctx context.Context, event domain.LifecycleEvent) error {
	if err := d.publisher.PublishLifecycleEvent(ctx, event); err != nil {
		retryAt := time.Now().UTC().Add(retryDelay(d.retryDelay, event.Attempt))
		if markErr := d.repository.MarkLifecycleEventFailed(ctx, event.EventID, d.owner, event.Attempt, err.Error(), retryAt); markErr != nil {
			return fmt.Errorf("mark content lifecycle outbox event %q failed: %w", event.EventID, markErr)
		}
		return nil
	}
	if err := d.repository.MarkLifecycleEventPublished(ctx, event.EventID, d.owner, event.Attempt); err != nil {
		return fmt.Errorf("mark content lifecycle outbox event %q published: %w", event.EventID, err)
	}
	return nil
}

func retryDelay(base time.Duration, attempt int) time.Duration {
	if base <= 0 {
		base = time.Second
	}
	if attempt < 1 {
		attempt = 1
	}
	shift := attempt - 1
	if shift > 6 {
		shift = 6
	}
	delay := base * time.Duration(1<<shift)
	if delay > maxRetryDelay {
		return maxRetryDelay
	}
	return delay
}
