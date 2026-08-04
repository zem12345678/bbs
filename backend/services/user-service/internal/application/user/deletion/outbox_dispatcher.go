package deletion

import (
	"context"
	"fmt"
	"strings"
	"time"

	domain "user-service/internal/domain/user"
	"user-service/pkg/logger"
)

const maxAccountDeletionOutboxRetryDelay = time.Minute

type AccountDeletionOutboxPublisher interface {
	PublishAccountDeletionOutboxEvent(ctx context.Context, event domain.AccountDeletionOutboxEvent) error
}

type AccountDeletionOutboxDispatcher struct {
	repository     domain.AccountDeletionOutboxRepository
	publisher      AccountDeletionOutboxPublisher
	owner          string
	batchSize      int
	leaseDuration  time.Duration
	interval       time.Duration
	retryDelay     time.Duration
	publishTimeout time.Duration
	log            logger.Logger
	now            func() time.Time
}

type AccountDeletionOutboxDispatcherOptions struct {
	Owner          string
	BatchSize      int
	LeaseDuration  time.Duration
	Interval       time.Duration
	RetryDelay     time.Duration
	PublishTimeout time.Duration
	Logger         logger.Logger
}

func NewAccountDeletionOutboxDispatcher(repository domain.AccountDeletionOutboxRepository, publisher AccountDeletionOutboxPublisher, options AccountDeletionOutboxDispatcherOptions) *AccountDeletionOutboxDispatcher {
	owner := strings.TrimSpace(options.Owner)
	if owner == "" {
		owner = "user-service-account-deletion-outbox"
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
		options.PublishTimeout = 5 * time.Second
	}
	return &AccountDeletionOutboxDispatcher{
		repository: repository, publisher: publisher, owner: owner,
		batchSize: options.BatchSize, leaseDuration: options.LeaseDuration,
		interval: options.Interval, retryDelay: options.RetryDelay,
		publishTimeout: options.PublishTimeout, log: options.Logger,
		now: func() time.Time { return time.Now().UTC() },
	}
}

func (d *AccountDeletionOutboxDispatcher) DispatchOnce(ctx context.Context) (int, error) {
	if d == nil || d.repository == nil || d.publisher == nil {
		return 0, fmt.Errorf("account deletion outbox is unavailable")
	}
	now := d.now()
	events, err := d.repository.ClaimAccountDeletionOutboxEvents(ctx, d.owner, d.batchSize, now, now.Add(d.leaseDuration))
	if err != nil {
		return 0, fmt.Errorf("claim account deletion outbox events: %w", err)
	}
	processed := 0
	for _, event := range events {
		publishCtx, cancel := context.WithTimeout(ctx, d.publishTimeout)
		publishErr := d.publisher.PublishAccountDeletionOutboxEvent(publishCtx, event)
		cancel()
		if publishErr != nil {
			failedAt := d.now()
			retryAt := failedAt.Add(accountDeletionOutboxRetryDelay(d.retryDelay, event.Attempt))
			if markErr := d.repository.MarkAccountDeletionOutboxFailed(ctx, event.EventID, d.owner, publishErr.Error(), failedAt, retryAt); markErr != nil {
				return processed, fmt.Errorf("mark account deletion outbox event %q failed: %w", event.EventID, markErr)
			}
			processed++
			continue
		}
		if err := d.repository.MarkAccountDeletionOutboxPublished(ctx, event.EventID, d.owner, d.now()); err != nil {
			return processed, fmt.Errorf("mark account deletion outbox event %q published: %w", event.EventID, err)
		}
		processed++
	}
	return processed, nil
}

func (d *AccountDeletionOutboxDispatcher) Run(ctx context.Context) {
	if d == nil {
		return
	}
	for ctx.Err() == nil {
		if _, err := d.DispatchOnce(ctx); err != nil && ctx.Err() == nil && d.log != nil {
			d.log.Warn("dispatch account deletion outbox events failed", logger.String("owner", d.owner), logger.Error(err))
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

func accountDeletionOutboxRetryDelay(base time.Duration, attempt int) time.Duration {
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
	if delay > maxAccountDeletionOutboxRetryDelay {
		return maxAccountDeletionOutboxRetryDelay
	}
	return delay
}
