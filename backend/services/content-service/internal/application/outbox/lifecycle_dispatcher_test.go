package outbox

import (
	"context"
	"errors"
	"testing"
	"time"

	domain "content-service/internal/domain/outbox"
)

func TestLifecycleDispatcherRetriesFailedPublish(t *testing.T) {
	event := domain.LifecycleEvent{
		EventID:    "content.article.hidden:101:1",
		MessageKey: "101",
		EventType:  "article.hidden.v1",
		Payload:    []byte(`{"event_id":"content.article.hidden:101:1"}`),
		Attempt:    1,
	}
	repository := &lifecycleRepositoryStub{pending: []domain.LifecycleEvent{event}}
	publisher := &lifecyclePublisherStub{errors: []error{errors.New("kafka unavailable")}}
	dispatcher := NewLifecycleDispatcher(repository, publisher, LifecycleDispatcherOptions{Owner: "test-owner", RetryDelay: time.Millisecond})

	processed, err := dispatcher.DispatchOnce(context.Background())
	if err != nil {
		t.Fatalf("first DispatchOnce() error = %v", err)
	}
	if processed != 1 || len(repository.failed) != 1 || len(repository.published) != 0 {
		t.Fatalf("first dispatch = processed:%d failed:%d published:%d", processed, len(repository.failed), len(repository.published))
	}
	if len(repository.failedAttempts) != 1 || repository.failedAttempts[0] != event.Attempt {
		t.Fatalf("failed attempt = %v, want %d", repository.failedAttempts, event.Attempt)
	}

	repository.pending = []domain.LifecycleEvent{event}
	processed, err = dispatcher.DispatchOnce(context.Background())
	if err != nil {
		t.Fatalf("retry DispatchOnce() error = %v", err)
	}
	if processed != 1 || len(publisher.events) != 2 || len(repository.published) != 1 {
		t.Fatalf("retry dispatch = processed:%d publish calls:%d published:%d", processed, len(publisher.events), len(repository.published))
	}
	if repository.published[0] != event.EventID {
		t.Fatalf("published event id = %q, want %q", repository.published[0], event.EventID)
	}
	if len(repository.publishedAttempts) != 1 || repository.publishedAttempts[0] != event.Attempt {
		t.Fatalf("published attempt = %v, want %d", repository.publishedAttempts, event.Attempt)
	}
}

type lifecycleRepositoryStub struct {
	pending           []domain.LifecycleEvent
	failed            []string
	failedAttempts    []int
	published         []string
	publishedAttempts []int
}

func (r *lifecycleRepositoryStub) ClaimPendingLifecycleEvents(context.Context, string, int, time.Duration) ([]domain.LifecycleEvent, error) {
	events := r.pending
	r.pending = nil
	return events, nil
}

func (r *lifecycleRepositoryStub) ClaimLifecycleEvent(_ context.Context, eventID, _ string, _ time.Duration) (*domain.LifecycleEvent, error) {
	for index, event := range r.pending {
		if event.EventID != eventID {
			continue
		}
		r.pending = append(r.pending[:index], r.pending[index+1:]...)
		return &event, nil
	}
	return nil, nil
}

func (r *lifecycleRepositoryStub) MarkLifecycleEventPublished(_ context.Context, eventID, _ string, attempt int) error {
	r.published = append(r.published, eventID)
	r.publishedAttempts = append(r.publishedAttempts, attempt)
	return nil
}

func (r *lifecycleRepositoryStub) MarkLifecycleEventFailed(_ context.Context, eventID, _ string, attempt int, _ string, _ time.Time) error {
	r.failed = append(r.failed, eventID)
	r.failedAttempts = append(r.failedAttempts, attempt)
	return nil
}

type lifecyclePublisherStub struct {
	events []domain.LifecycleEvent
	errors []error
}

func (p *lifecyclePublisherStub) PublishLifecycleEvent(_ context.Context, event domain.LifecycleEvent) error {
	p.events = append(p.events, event)
	if len(p.errors) == 0 {
		return nil
	}
	err := p.errors[0]
	p.errors = p.errors[1:]
	return err
}
