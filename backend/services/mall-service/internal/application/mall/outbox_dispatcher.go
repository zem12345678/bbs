package mall

import (
	"context"
	"fmt"
	"time"

	domain "mall-service/internal/domain/mall"
	"mall-service/pkg/logger"
)

type OutboxDispatcher struct {
	repository    domain.Repository
	publisher     domain.OutboxPublisher
	owner         string
	batchSize     int
	maxAttempts   int
	leaseDuration time.Duration
	interval      time.Duration
	retryDelay    time.Duration
	log           logger.Logger
}

type OutboxDispatcherOptions struct {
	Owner         string
	BatchSize     int
	MaxAttempts   int
	LeaseDuration time.Duration
	Interval      time.Duration
	RetryDelay    time.Duration
	Log           logger.Logger
}

func NewOutboxDispatcher(repository domain.Repository, publisher domain.OutboxPublisher, options OutboxDispatcherOptions) *OutboxDispatcher {
	if options.Owner == "" {
		options.Owner = "mall-service"
	}
	if options.BatchSize <= 0 {
		options.BatchSize = 100
	}
	if options.MaxAttempts <= 0 {
		options.MaxAttempts = 10
	}
	if options.LeaseDuration <= 0 {
		options.LeaseDuration = 30 * time.Second
	}
	if options.Interval <= 0 {
		options.Interval = time.Second
	}
	if options.RetryDelay <= 0 {
		options.RetryDelay = 3 * time.Second
	}
	if options.Log == nil {
		options.Log = logger.NewNopLogger()
	}
	return &OutboxDispatcher{
		repository:    repository,
		publisher:     publisher,
		owner:         options.Owner,
		batchSize:     options.BatchSize,
		maxAttempts:   options.MaxAttempts,
		leaseDuration: options.LeaseDuration,
		interval:      options.Interval,
		retryDelay:    options.RetryDelay,
		log:           options.Log,
	}
}

func (d *OutboxDispatcher) Start(ctx context.Context) {
	go d.run(ctx)
}

func (d *OutboxDispatcher) DispatchOnce(ctx context.Context) (int, error) {
	events, err := d.repository.ClaimPendingOutboxEvents(ctx, d.owner, d.batchSize, d.leaseDuration)
	if err != nil {
		return 0, err
	}
	for _, event := range events {
		if err := d.publisher.PublishOutboxEvent(ctx, event); err != nil {
			if event.Attempt >= d.maxAttempts {
				if markErr := d.repository.MarkOutboxEventDeadLetter(ctx, event.EventID, d.owner, err.Error()); markErr != nil {
					return 0, fmt.Errorf("mark outbox event %q dead letter: %w", event.EventID, markErr)
				}
				continue
			}
			if markErr := d.repository.MarkOutboxEventFailed(ctx, event.EventID, d.owner, err.Error(), time.Now().UTC().Add(d.retryDelay)); markErr != nil {
				return 0, fmt.Errorf("mark outbox event %q failed: %w", event.EventID, markErr)
			}
			continue
		}
		if err := d.repository.MarkOutboxEventPublished(ctx, event.EventID, d.owner); err != nil {
			return 0, fmt.Errorf("mark outbox event %q published: %w", event.EventID, err)
		}
	}
	return len(events), nil
}

func (d *OutboxDispatcher) run(ctx context.Context) {
	ticker := time.NewTicker(d.interval)
	defer ticker.Stop()
	for ctx.Err() == nil {
		if _, err := d.DispatchOnce(ctx); err != nil {
			d.log.Error("dispatch mall outbox events failed",
				logger.Error(err),
				logger.String("owner", d.owner),
				logger.Int("batch_size", d.batchSize),
			)
		}
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}
