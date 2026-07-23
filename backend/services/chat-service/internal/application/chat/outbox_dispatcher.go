package chat

import (
	"context"
	"fmt"
	"time"

	domain "chat-service/internal/domain/chat"

	"go.uber.org/zap"
)

const maxOutboxRetryDelay = time.Minute

type OutboxDispatcher struct {
	repository     domain.OutboxRepository
	publisher      domain.OutboxPublisher
	owner          string
	batchSize      int
	leaseDuration  time.Duration
	interval       time.Duration
	retryDelay     time.Duration
	publishTimeout time.Duration
	logger         *zap.Logger
}

type OutboxDispatcherOptions struct {
	Owner          string
	BatchSize      int
	LeaseDuration  time.Duration
	Interval       time.Duration
	RetryDelay     time.Duration
	PublishTimeout time.Duration
	Logger         *zap.Logger
}

func NewOutboxDispatcher(repository domain.OutboxRepository, publisher domain.OutboxPublisher, options OutboxDispatcherOptions) *OutboxDispatcher {
	if options.Owner == "" {
		options.Owner = "chat-service"
	}
	if options.BatchSize <= 0 {
		options.BatchSize = 20
	}
	if options.LeaseDuration <= 0 {
		options.LeaseDuration = 60 * time.Second
	}
	if options.Interval <= 0 {
		options.Interval = time.Second
	}
	if options.RetryDelay <= 0 {
		options.RetryDelay = 3 * time.Second
	}
	if options.PublishTimeout <= 0 {
		options.PublishTimeout = 2 * time.Second
	}
	if options.Logger == nil {
		options.Logger = zap.NewNop()
	}
	return &OutboxDispatcher{
		repository: repository, publisher: publisher, owner: options.Owner,
		batchSize: options.BatchSize, leaseDuration: options.LeaseDuration,
		interval: options.Interval, retryDelay: options.RetryDelay,
		publishTimeout: options.PublishTimeout, logger: options.Logger,
	}
}

func (d *OutboxDispatcher) DispatchOnce(ctx context.Context) (int, error) {
	events, err := d.repository.ClaimPendingOutboxEvents(ctx, d.owner, d.batchSize, d.leaseDuration)
	if err != nil {
		return 0, fmt.Errorf("claim chat outbox events: %w", err)
	}
	processed := 0
	for _, event := range events {
		publishCtx, cancel := context.WithTimeout(ctx, d.publishTimeout)
		publishErr := d.publisher.PublishOutboxEvent(publishCtx, event)
		cancel()
		if publishErr != nil {
			retryAt := time.Now().UTC().Add(outboxRetryDelay(d.retryDelay, event.Attempt))
			if err := d.repository.MarkOutboxEventFailed(ctx, event.EventID, d.owner, publishErr.Error(), retryAt); err != nil {
				return processed, fmt.Errorf("mark chat outbox event %q failed: %w", event.EventID, err)
			}
			processed++
			continue
		}
		if err := d.repository.MarkOutboxEventPublished(ctx, event.EventID, d.owner); err != nil {
			return processed, fmt.Errorf("mark chat outbox event %q published: %w", event.EventID, err)
		}
		processed++
	}
	return processed, nil
}

func (d *OutboxDispatcher) Run(ctx context.Context) {
	for ctx.Err() == nil {
		if _, err := d.DispatchOnce(ctx); err != nil && ctx.Err() == nil {
			d.logger.Warn("dispatch chat outbox events failed", zap.Error(err), zap.String("owner", d.owner))
		}
		timer := time.NewTimer(d.interval)
		select {
		case <-ctx.Done():
			timer.Stop()
			return
		case <-timer.C:
		}
	}
}

func outboxRetryDelay(base time.Duration, attempt int) time.Duration {
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
	if delay > maxOutboxRetryDelay {
		return maxOutboxRetryDelay
	}
	return delay
}
