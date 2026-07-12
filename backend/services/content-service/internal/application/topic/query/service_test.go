package query

import (
	"context"
	"testing"
	"time"

	domain "content-service/internal/domain/topic"
	"content-service/internal/infrastructure/messaging"
)

func TestGetByIDIncrementsTopicViewCount(t *testing.T) {
	repo := &fakeTopicRepo{
		topic: &domain.Topic{
			ID:        7,
			Slug:      "first-topic",
			Type:      domain.TypeTopic,
			Title:     "First topic",
			Body:      "body",
			AuthorID:  10,
			Status:    domain.StatusPublished,
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
			ViewCount: 8,
		},
		nextViewCount: 9,
	}
	publisher := &fakeTopicPublisher{}
	service := NewService(repo, publisher, nil)

	view, err := service.GetByID(context.Background(), 7)
	if err != nil {
		t.Fatalf("GetByID returned error: %v", err)
	}
	if repo.incrementedID != 7 {
		t.Fatalf("expected increment for topic 7, got %d", repo.incrementedID)
	}
	if got := view.Topic.ViewCount; got != 9 {
		t.Fatalf("expected returned view count 9, got %d", got)
	}
	if len(publisher.events) != 1 {
		t.Fatalf("expected 1 published event, got %d", len(publisher.events))
	}
	event, ok := publisher.events[0].(domain.TopicViewedEvent)
	if !ok {
		t.Fatalf("expected TopicViewedEvent, got %T", publisher.events[0])
	}
	if event.TopicID != 7 || event.ViewCount != 9 {
		t.Fatalf("unexpected viewed event payload: %#v", event)
	}
}

func TestGetByIDDoesNotIncrementViewCountForHiddenTopic(t *testing.T) {
	repo := &fakeTopicRepo{
		topic: &domain.Topic{
			ID:        8,
			Slug:      "hidden-topic",
			Type:      domain.TypeTopic,
			Title:     "Hidden topic",
			Body:      "body",
			AuthorID:  10,
			Status:    domain.StatusHidden,
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
			ViewCount: 8,
		},
		nextViewCount: 9,
	}
	publisher := &fakeTopicPublisher{}
	service := NewService(repo, publisher, nil)

	view, err := service.GetByID(context.Background(), 8)
	if err != nil {
		t.Fatalf("GetByID returned error: %v", err)
	}
	if repo.incrementedID != 0 {
		t.Fatalf("hidden topic should not be incremented, got %d", repo.incrementedID)
	}
	if got := view.Topic.ViewCount; got != 8 {
		t.Fatalf("expected returned view count 8, got %d", got)
	}
	if len(publisher.events) != 0 {
		t.Fatalf("expected no published events, got %d", len(publisher.events))
	}
}

type fakeTopicRepo struct {
	topic         *domain.Topic
	nextViewCount int64
	incrementedID int64
}

func (f *fakeTopicRepo) CreateTopic(context.Context, *domain.Topic) error { return nil }
func (f *fakeTopicRepo) UpdateTopic(context.Context, *domain.Topic) error { return nil }
func (f *fakeTopicRepo) FindTopicBySlug(context.Context, string) (*domain.Topic, error) {
	return f.topic, nil
}
func (f *fakeTopicRepo) FindTopicByID(context.Context, int64) (*domain.Topic, error) {
	return f.topic, nil
}
func (f *fakeTopicRepo) ListTopics(context.Context, domain.Status, domain.Type, string, int64, int64, string, int, int) ([]*domain.Topic, error) {
	return nil, nil
}
func (f *fakeTopicRepo) UpdateTopicStatus(context.Context, int64, domain.Status, *time.Time) error {
	return nil
}
func (f *fakeTopicRepo) AcceptTopicComment(context.Context, int64, int64, int64, time.Time) (*domain.Topic, bool, error) {
	return f.topic, false, nil
}
func (f *fakeTopicRepo) IncrementTopicViewCount(_ context.Context, id int64) (int64, error) {
	f.incrementedID = id
	return f.nextViewCount, nil
}

type fakeTopicPublisher struct {
	events []messaging.DomainEvent
}

func (f *fakeTopicPublisher) PublishDomainEvents(_ context.Context, events []messaging.DomainEvent) error {
	f.events = append(f.events, events...)
	return nil
}
