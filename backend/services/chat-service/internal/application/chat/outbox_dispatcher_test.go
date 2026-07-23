package chat

import (
	"context"
	"errors"
	"testing"
	"time"

	domain "chat-service/internal/domain/chat"
)

func TestOutboxDispatcherPublishesAndMarksEvents(t *testing.T) {
	repository := &outboxRepositoryStub{events: []domain.OutboxEvent{{EventID: "e1", Attempt: 1}}}
	publisher := &outboxPublisherStub{}
	dispatcher := NewOutboxDispatcher(repository, publisher, OutboxDispatcherOptions{Owner: "worker-a"})
	processed, err := dispatcher.DispatchOnce(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if processed != 1 || repository.published != "e1" || publisher.calls != 1 {
		t.Fatalf("dispatch result = %d, published=%q, calls=%d", processed, repository.published, publisher.calls)
	}
}

func TestOutboxDispatcherMarksPublishFailureWithBackoff(t *testing.T) {
	repository := &outboxRepositoryStub{events: []domain.OutboxEvent{{EventID: "e1", Attempt: 3}}}
	publisher := &outboxPublisherStub{err: errors.New("kafka unavailable")}
	dispatcher := NewOutboxDispatcher(repository, publisher, OutboxDispatcherOptions{
		Owner: "worker-a", RetryDelay: 10 * time.Millisecond,
	})
	before := time.Now()
	processed, err := dispatcher.DispatchOnce(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if processed != 1 || repository.failed != "e1" {
		t.Fatalf("dispatch result = %d, failed=%q", processed, repository.failed)
	}
	if repository.retryAt.Before(before.Add(35 * time.Millisecond)) {
		t.Fatalf("retry time = %s, want exponential backoff", repository.retryAt)
	}
}

func TestOutboxDispatcherReturnsLeaseWriteError(t *testing.T) {
	markErr := errors.New("lease lost")
	repository := &outboxRepositoryStub{
		events: []domain.OutboxEvent{{EventID: "e1", Attempt: 1}}, markPublishedErr: markErr,
	}
	dispatcher := NewOutboxDispatcher(repository, &outboxPublisherStub{}, OutboxDispatcherOptions{Owner: "worker-a"})
	if _, err := dispatcher.DispatchOnce(context.Background()); !errors.Is(err, markErr) {
		t.Fatalf("DispatchOnce error = %v, want %v", err, markErr)
	}
}

type outboxRepositoryStub struct {
	events           []domain.OutboxEvent
	published        string
	failed           string
	retryAt          time.Time
	markPublishedErr error
}

func (r *outboxRepositoryStub) ClaimPendingOutboxEvents(context.Context, string, int, time.Duration) ([]domain.OutboxEvent, error) {
	return r.events, nil
}

func (r *outboxRepositoryStub) MarkOutboxEventPublished(_ context.Context, eventID, _ string) error {
	r.published = eventID
	return r.markPublishedErr
}

func (r *outboxRepositoryStub) MarkOutboxEventFailed(_ context.Context, eventID, _, _ string, retryAt time.Time) error {
	r.failed = eventID
	r.retryAt = retryAt
	return nil
}

type outboxPublisherStub struct {
	calls int
	err   error
}

func (p *outboxPublisherStub) PublishOutboxEvent(context.Context, domain.OutboxEvent) error {
	p.calls++
	return p.err
}
