package messaging

import (
	"context"
	"encoding/json"
	"testing"

	domain "search-service/internal/domain/search"
)

func TestArticleConsumerHandlesViewedEvents(t *testing.T) {
	tests := []struct {
		name       string
		eventType  string
		payload    any
		wantEntity string
		wantID     int64
		wantCount  int64
	}{
		{
			name:       "article viewed",
			eventType:  "article.viewed.v1",
			payload:    articleViewedPayload{ArticleID: 12, ViewCount: 34},
			wantEntity: "article",
			wantID:     12,
			wantCount:  34,
		},
		{
			name:       "topic viewed",
			eventType:  "topic.viewed.v1",
			payload:    topicViewedPayload{TopicID: 56, ViewCount: 78},
			wantEntity: "topic",
			wantID:     56,
			wantCount:  78,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			raw, err := json.Marshal(tt.payload)
			if err != nil {
				t.Fatalf("marshal payload: %v", err)
			}
			indexer := &fakeArticleIndexer{}
			consumer := &ArticleConsumer{indexer: indexer}

			if err := consumer.handle(context.Background(), eventEnvelope{EventType: tt.eventType, Payload: raw}); err != nil {
				t.Fatalf("handle returned error: %v", err)
			}
			if indexer.viewEntity != tt.wantEntity || indexer.viewID != tt.wantID || indexer.viewCount != tt.wantCount {
				t.Fatalf("view count update entity=%q id=%d count=%d", indexer.viewEntity, indexer.viewID, indexer.viewCount)
			}
		})
	}
}

func TestArticleConsumerProjectsTopicCategory(t *testing.T) {
	raw, err := json.Marshal(topicPublishedPayload{TopicID: 91, Title: "topic", CategoryID: 7})
	if err != nil {
		t.Fatalf("marshal payload: %v", err)
	}
	indexer := &fakeArticleIndexer{}
	consumer := &ArticleConsumer{indexer: indexer}

	if err := consumer.handle(context.Background(), eventEnvelope{EventType: "topic.published.v1", Payload: raw}); err != nil {
		t.Fatalf("handle returned error: %v", err)
	}
	if indexer.topic.CategoryID != 7 {
		t.Fatalf("expected category 7, got %d", indexer.topic.CategoryID)
	}
}

type fakeArticleIndexer struct {
	viewEntity string
	viewID     int64
	viewCount  int64
	topic      domain.TopicDocument
}

func (f *fakeArticleIndexer) EnsureArticleIndex(context.Context) error { return nil }
func (f *fakeArticleIndexer) EnsureTopicIndex(context.Context) error   { return nil }
func (f *fakeArticleIndexer) IndexArticle(context.Context, domain.ArticleDocument) error {
	return nil
}
func (f *fakeArticleIndexer) IndexTopic(_ context.Context, doc domain.TopicDocument) error {
	f.topic = doc
	return nil
}
func (f *fakeArticleIndexer) DeleteArticle(context.Context, int64) error { return nil }
func (f *fakeArticleIndexer) DeleteTopic(context.Context, int64) error   { return nil }
func (f *fakeArticleIndexer) SetArticleViewCount(_ context.Context, id int64, count int64) error {
	f.viewEntity = "article"
	f.viewID = id
	f.viewCount = count
	return nil
}
func (f *fakeArticleIndexer) SetTopicViewCount(_ context.Context, id int64, count int64) error {
	f.viewEntity = "topic"
	f.viewID = id
	f.viewCount = count
	return nil
}
