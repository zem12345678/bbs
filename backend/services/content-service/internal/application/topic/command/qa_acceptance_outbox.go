package command

import (
	"context"
	"errors"
	"strings"
	"time"

	domain "content-service/internal/domain/topic"
	"content-service/internal/infrastructure/messaging"
)

type QAAcceptanceOutboxDispatcher struct {
	repository    domain.QAAcceptanceOutboxRepository
	publisher     messaging.QAAcceptanceOutboxPublisher
	owner         string
	batchSize     int
	leaseDuration time.Duration
	retryDelay    time.Duration
}

type QAAcceptanceOutboxDispatcherOptions struct {
	Owner         string
	BatchSize     int
	LeaseDuration time.Duration
	RetryDelay    time.Duration
}

func NewQAAcceptanceOutboxDispatcher(repository domain.QAAcceptanceOutboxRepository, publisher messaging.QAAcceptanceOutboxPublisher, options QAAcceptanceOutboxDispatcherOptions) *QAAcceptanceOutboxDispatcher {
	if options.Owner == "" {
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
	return &QAAcceptanceOutboxDispatcher{
		repository:    repository,
		publisher:     publisher,
		owner:         strings.TrimSpace(options.Owner),
		batchSize:     options.BatchSize,
		leaseDuration: options.LeaseDuration,
		retryDelay:    options.RetryDelay,
	}
}

func (d *QAAcceptanceOutboxDispatcher) DispatchOnce(ctx context.Context) (int, error) {
	if d.repository == nil || d.publisher == nil {
		return 0, errors.New("QA acceptance outbox is unavailable")
	}
	events, err := d.repository.ClaimPendingQAAcceptanceOutboxEvents(ctx, d.owner, d.batchSize, d.leaseDuration)
	if err != nil {
		return 0, err
	}
	for _, event := range events {
		if err := d.publisher.PublishQAAcceptanceOutboxEvent(ctx, event); err != nil {
			if markErr := d.repository.MarkQAAcceptanceOutboxEventFailed(ctx, event.EventID, d.owner, err.Error(), time.Now().UTC().Add(d.retryDelay)); markErr != nil {
				return 0, markErr
			}
			continue
		}
		if err := d.repository.MarkQAAcceptanceOutboxEventPublished(ctx, event.EventID, d.owner); err != nil {
			return 0, err
		}
	}
	return len(events), nil
}
