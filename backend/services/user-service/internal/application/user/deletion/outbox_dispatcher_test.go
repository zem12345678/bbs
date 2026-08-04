package deletion

import (
	"context"
	"errors"
	"testing"
	"time"

	domain "user-service/internal/domain/user"
)

func TestAccountDeletionOutboxDispatcherPublishesAndMarksEvent(t *testing.T) {
	now := time.Date(2026, 8, 4, 1, 2, 3, 0, time.UTC)
	repo := &outboxRepositoryStub{events: []domain.AccountDeletionOutboxEvent{{
		EventID: "event-1", AggregateID: 42, EventType: "user.deleted", MessageKey: "42", Payload: []byte(`{"user_id":42}`), Attempt: 1, OccurredAt: now,
	}}}
	publisher := &outboxPublisherStub{}
	dispatcher := NewAccountDeletionOutboxDispatcher(repo, publisher, AccountDeletionOutboxDispatcherOptions{
		Owner: "worker-a", BatchSize: 4, LeaseDuration: time.Minute, RetryDelay: time.Second,
	})
	dispatcher.now = func() time.Time { return now }

	processed, err := dispatcher.DispatchOnce(context.Background())
	if err != nil || processed != 1 {
		t.Fatalf("processed=%d err=%v", processed, err)
	}
	if len(publisher.events) != 1 || publisher.events[0].EventID != "event-1" {
		t.Fatalf("published events=%+v", publisher.events)
	}
	if repo.publishedID != "event-1" || repo.publishedOwner != "worker-a" {
		t.Fatalf("published id=%q owner=%q", repo.publishedID, repo.publishedOwner)
	}
}

func TestAccountDeletionOutboxDispatcherMarksFailedWithBackoff(t *testing.T) {
	now := time.Date(2026, 8, 4, 1, 2, 3, 0, time.UTC)
	repo := &outboxRepositoryStub{events: []domain.AccountDeletionOutboxEvent{{
		EventID: "event-2", AggregateID: 42, EventType: "user.deleted", MessageKey: "42", Payload: []byte(`{"user_id":42}`), Attempt: 3, OccurredAt: now,
	}}}
	wantErr := errors.New("kafka unavailable")
	dispatcher := NewAccountDeletionOutboxDispatcher(repo, &outboxPublisherStub{err: wantErr}, AccountDeletionOutboxDispatcherOptions{
		Owner: "worker-a", RetryDelay: 2 * time.Second,
	})
	dispatcher.now = func() time.Time { return now }

	processed, err := dispatcher.DispatchOnce(context.Background())
	if err != nil || processed != 1 {
		t.Fatalf("processed=%d err=%v", processed, err)
	}
	if repo.failedID != "event-2" || repo.failedError != wantErr.Error() {
		t.Fatalf("failed id=%q error=%q", repo.failedID, repo.failedError)
	}
	if want := now.Add(8 * time.Second); !repo.retryAt.Equal(want) {
		t.Fatalf("retryAt=%v want=%v", repo.retryAt, want)
	}
}

type outboxRepositoryStub struct {
	events         []domain.AccountDeletionOutboxEvent
	failedID       string
	failedError    string
	retryAt        time.Time
	publishedID    string
	publishedOwner string
}

func (r *outboxRepositoryStub) ClaimAccountDeletionOutboxEvents(context.Context, string, int, time.Time, time.Time) ([]domain.AccountDeletionOutboxEvent, error) {
	events := r.events
	r.events = nil
	return events, nil
}

func (r *outboxRepositoryStub) MarkAccountDeletionOutboxFailed(_ context.Context, eventID, _ string, lastError string, _, retryAt time.Time) error {
	r.failedID, r.failedError, r.retryAt = eventID, lastError, retryAt
	return nil
}

func (r *outboxRepositoryStub) MarkAccountDeletionOutboxPublished(_ context.Context, eventID, owner string, _ time.Time) error {
	r.publishedID, r.publishedOwner = eventID, owner
	return nil
}

type outboxPublisherStub struct {
	events []domain.AccountDeletionOutboxEvent
	err    error
}

func (p *outboxPublisherStub) PublishAccountDeletionOutboxEvent(_ context.Context, event domain.AccountDeletionOutboxEvent) error {
	if p.err != nil {
		return p.err
	}
	p.events = append(p.events, event)
	return nil
}
