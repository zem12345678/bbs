package mall

import (
	"context"
	"errors"
	"testing"
	"time"

	domain "mall-service/internal/domain/mall"
)

func TestOutboxDispatcherReturnsFailedStateWriteError(t *testing.T) {
	markErr := errors.New("write failed state")
	repo := &outboxDispatcherRepositoryStub{
		events:        []domain.OutboxEvent{{EventID: "evt-failed", Attempt: 1}},
		markFailedErr: markErr,
	}
	dispatcher := NewOutboxDispatcher(repo, &outboxDispatcherPublisherStub{err: errors.New("kafka unavailable")}, OutboxDispatcherOptions{
		Owner:       "worker-a",
		MaxAttempts: 3,
		RetryDelay:  time.Second,
	})

	processed, err := dispatcher.DispatchOnce(context.Background())
	if !errors.Is(err, markErr) {
		t.Fatalf("DispatchOnce() error = %v, want state write error", err)
	}
	if processed != 0 {
		t.Fatalf("DispatchOnce() processed = %d, want 0", processed)
	}
	if repo.markFailedCalls != 1 {
		t.Fatalf("MarkOutboxEventFailed() calls = %d, want 1", repo.markFailedCalls)
	}
}

func TestOutboxDispatcherReturnsDeadLetterStateWriteError(t *testing.T) {
	markErr := errors.New("write dead letter state")
	repo := &outboxDispatcherRepositoryStub{
		events:            []domain.OutboxEvent{{EventID: "evt-dead", Attempt: 3}},
		markDeadLetterErr: markErr,
	}
	dispatcher := NewOutboxDispatcher(repo, &outboxDispatcherPublisherStub{err: errors.New("kafka unavailable")}, OutboxDispatcherOptions{
		Owner:       "worker-a",
		MaxAttempts: 3,
	})

	processed, err := dispatcher.DispatchOnce(context.Background())
	if !errors.Is(err, markErr) {
		t.Fatalf("DispatchOnce() error = %v, want state write error", err)
	}
	if processed != 0 {
		t.Fatalf("DispatchOnce() processed = %d, want 0", processed)
	}
	if repo.markDeadLetterCalls != 1 {
		t.Fatalf("MarkOutboxEventDeadLetter() calls = %d, want 1", repo.markDeadLetterCalls)
	}
}

func TestOutboxDispatcherRunLogsDispatchErrors(t *testing.T) {
	markErr := errors.New("write failed state")
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	repo := &outboxDispatcherRepositoryStub{
		events:          []domain.OutboxEvent{{EventID: "evt-log", Attempt: 1}},
		markFailedErr:   markErr,
		afterMarkFailed: cancel,
	}
	log := &expiredOrderTestLogger{}
	dispatcher := NewOutboxDispatcher(repo, &outboxDispatcherPublisherStub{err: errors.New("kafka unavailable")}, OutboxDispatcherOptions{
		Owner:       "worker-a",
		MaxAttempts: 3,
		Interval:    time.Hour,
		Log:         log,
	})

	dispatcher.run(ctx)

	if len(log.errors) != 1 {
		t.Fatalf("logged errors = %d, want 1", len(log.errors))
	}
	if log.errors[0] != "dispatch mall outbox events failed" {
		t.Fatalf("logged error message = %q", log.errors[0])
	}
	var loggedErr error
	for _, field := range log.errorFields[0] {
		if field.Key == "error" {
			loggedErr, _ = field.Value.(error)
			break
		}
	}
	if !errors.Is(loggedErr, markErr) {
		t.Fatalf("logged error = %v, want state write error", loggedErr)
	}
}

type outboxDispatcherRepositoryStub struct {
	domain.Repository

	events              []domain.OutboxEvent
	claimErr            error
	markFailedErr       error
	markDeadLetterErr   error
	markFailedCalls     int
	markDeadLetterCalls int
	afterMarkFailed     func()
}

func (r *outboxDispatcherRepositoryStub) ClaimPendingOutboxEvents(context.Context, string, int, time.Duration) ([]domain.OutboxEvent, error) {
	return r.events, r.claimErr
}

func (r *outboxDispatcherRepositoryStub) MarkOutboxEventFailed(context.Context, string, string, string, time.Time) error {
	r.markFailedCalls++
	if r.afterMarkFailed != nil {
		r.afterMarkFailed()
	}
	return r.markFailedErr
}

func (r *outboxDispatcherRepositoryStub) MarkOutboxEventDeadLetter(context.Context, string, string, string) error {
	r.markDeadLetterCalls++
	return r.markDeadLetterErr
}

type outboxDispatcherPublisherStub struct {
	err error
}

func (p *outboxDispatcherPublisherStub) PublishOutboxEvent(context.Context, domain.OutboxEvent) error {
	return p.err
}
