package messaging

import (
	"context"
	"encoding/json"
	"testing"

	domain "feed-service/internal/domain/feed"
)

func TestArticleConsumerHandlesViewedEvents(t *testing.T) {
	tests := []struct {
		name      string
		eventType string
		payload   any
		wantID    int64
		wantCount int64
	}{
		{
			name:      "article viewed",
			eventType: "article.viewed.v1",
			payload:   articleViewedPayload{ArticleID: 12, ViewCount: 34},
			wantID:    12,
			wantCount: 34,
		},
		{
			name:      "topic viewed",
			eventType: "topic.viewed.v1",
			payload:   topicViewedPayload{TopicID: 56, ViewCount: 78},
			wantID:    56,
			wantCount: 78,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			raw, err := json.Marshal(tt.payload)
			if err != nil {
				t.Fatalf("marshal payload: %v", err)
			}
			projector := &fakeFeedProjector{}
			consumer := &ArticleConsumer{projector: projector}

			if err := consumer.handle(context.Background(), eventEnvelope{EventType: tt.eventType, Payload: raw}); err != nil {
				t.Fatalf("handle returned error: %v", err)
			}
			if projector.viewID != tt.wantID || projector.viewCount != tt.wantCount {
				t.Fatalf("view count update id=%d count=%d", projector.viewID, projector.viewCount)
			}
		})
	}
}

func TestArticleConsumerProjectsTopicCategory(t *testing.T) {
	raw, err := json.Marshal(topicPublishedPayload{TopicID: 91, Title: "topic", CategoryID: 7})
	if err != nil {
		t.Fatalf("marshal payload: %v", err)
	}
	projector := &fakeFeedProjector{}
	consumer := &ArticleConsumer{projector: projector}

	if err := consumer.handle(context.Background(), eventEnvelope{EventType: "topic.published.v1", Payload: raw}); err != nil {
		t.Fatalf("handle returned error: %v", err)
	}
	if projector.topic.CategoryID != 7 {
		t.Fatalf("expected category 7, got %d", projector.topic.CategoryID)
	}
}

func TestArticleConsumerRemovesArchivingTopic(t *testing.T) {
	projector := &fakeFeedProjector{}
	consumer := &ArticleConsumer{projector: projector}

	if err := consumer.handle(context.Background(), eventEnvelope{EventType: "topic.archiving.v1", AggregateID: "91"}); err != nil {
		t.Fatalf("handle returned error: %v", err)
	}
	if projector.removedTopicID != 91 {
		t.Fatalf("removed topic id = %d, want 91", projector.removedTopicID)
	}
}

type fakeFeedProjector struct {
	viewID         int64
	viewCount      int64
	topic          domain.Item
	removedTopicID int64
}

func (f *fakeFeedProjector) UpsertArticle(context.Context, domain.Item) error { return nil }
func (f *fakeFeedProjector) UpsertTopic(_ context.Context, item domain.Item) error {
	f.topic = item
	return nil
}
func (f *fakeFeedProjector) RemoveArticle(context.Context, int64) error { return nil }
func (f *fakeFeedProjector) RemoveTopic(_ context.Context, id int64) error {
	f.removedTopicID = id
	return nil
}
func (f *fakeFeedProjector) SetViewCount(_ context.Context, id int64, count int64) error {
	f.viewID = id
	f.viewCount = count
	return nil
}
