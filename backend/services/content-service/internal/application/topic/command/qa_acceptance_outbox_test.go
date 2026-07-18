package command

import (
	"context"
	"errors"
	"testing"
	"time"

	domain "content-service/internal/domain/topic"
)

func TestQAAcceptanceOutboxDispatcherRetriesFailedPublish(t *testing.T) {
	event := domain.QAAcceptanceOutboxEvent{EventID: "content.qa.accepted:101:9001", TopicID: 101, MessageKey: "101", Payload: []byte(`{"event_id":"content.qa.accepted:101:9001"}`), Attempt: 1}
	repository := &fakeQAAcceptanceOutboxRepository{events: []domain.QAAcceptanceOutboxEvent{event}}
	publisher := &fakeQAAcceptanceOutboxPublisher{errors: []error{errors.New("kafka unavailable")}}
	dispatcher := NewQAAcceptanceOutboxDispatcher(repository, publisher, QAAcceptanceOutboxDispatcherOptions{Owner: "test-owner", RetryDelay: time.Millisecond})

	processed, err := dispatcher.DispatchOnce(context.Background())
	if err != nil {
		t.Fatalf("first DispatchOnce() error = %v", err)
	}
	if processed != 1 || len(repository.failed) != 1 || len(repository.published) != 0 {
		t.Fatalf("first dispatch = processed:%d failed:%d published:%d", processed, len(repository.failed), len(repository.published))
	}

	repository.events = []domain.QAAcceptanceOutboxEvent{event}
	processed, err = dispatcher.DispatchOnce(context.Background())
	if err != nil {
		t.Fatalf("retry DispatchOnce() error = %v", err)
	}
	if processed != 1 || len(publisher.events) != 2 || len(repository.published) != 1 {
		t.Fatalf("retry dispatch = processed:%d published calls:%d published:%d", processed, len(publisher.events), len(repository.published))
	}
	if repository.published[0] != event.EventID {
		t.Fatalf("published event id = %q, want %q", repository.published[0], event.EventID)
	}
}

type fakeQAAcceptanceOutboxRepository struct {
	events    []domain.QAAcceptanceOutboxEvent
	failed    []string
	published []string
}

func (r *fakeQAAcceptanceOutboxRepository) AcceptTopicCommentWithOutbox(context.Context, int64, int64, int64, time.Time, domain.QAAcceptanceOutboxEvent) (*domain.Topic, bool, error) {
	return nil, false, nil
}

func (r *fakeQAAcceptanceOutboxRepository) EnsureQAAcceptanceOutboxEvent(context.Context, domain.QAAcceptanceOutboxEvent) error {
	return nil
}

func (r *fakeQAAcceptanceOutboxRepository) ClaimPendingQAAcceptanceOutboxEvents(context.Context, string, int, time.Duration) ([]domain.QAAcceptanceOutboxEvent, error) {
	events := r.events
	r.events = nil
	return events, nil
}

func (r *fakeQAAcceptanceOutboxRepository) MarkQAAcceptanceOutboxEventPublished(_ context.Context, eventID, _ string) error {
	r.published = append(r.published, eventID)
	return nil
}

func (r *fakeQAAcceptanceOutboxRepository) MarkQAAcceptanceOutboxEventFailed(_ context.Context, eventID, _ string, _ string, _ time.Time) error {
	r.failed = append(r.failed, eventID)
	return nil
}

type fakeQAAcceptanceOutboxPublisher struct {
	events []domain.QAAcceptanceOutboxEvent
	errors []error
}

func (p *fakeQAAcceptanceOutboxPublisher) PublishQAAcceptanceOutboxEvent(_ context.Context, event domain.QAAcceptanceOutboxEvent) error {
	p.events = append(p.events, event)
	if len(p.errors) == 0 {
		return nil
	}
	err := p.errors[0]
	p.errors = p.errors[1:]
	return err
}
