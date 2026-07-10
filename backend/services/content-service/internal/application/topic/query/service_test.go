package query

import (
	"context"
	"testing"
	"time"

	domain "content-service/internal/domain/topic"
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
	service := NewService(repo)

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
func (f *fakeTopicRepo) ListTopics(context.Context, domain.Status, domain.Type, string, int64, int64, int, int) ([]*domain.Topic, error) {
	return nil, nil
}
func (f *fakeTopicRepo) UpdateTopicStatus(context.Context, int64, domain.Status, *time.Time) error {
	return nil
}
func (f *fakeTopicRepo) IncrementTopicViewCount(_ context.Context, id int64) (int64, error) {
	f.incrementedID = id
	return f.nextViewCount, nil
}
