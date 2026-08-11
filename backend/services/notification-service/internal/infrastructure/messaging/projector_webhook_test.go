package messaging

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	app "notification-service/internal/application/notification"
)

func TestProjectorEnqueuesPublishedContentWebhook(t *testing.T) {
	t.Parallel()
	for _, test := range []struct {
		eventType string
		payload   any
	}{
		{eventType: "article.published.v1", payload: articlePublishedPayload{ArticleID: 101, AuthorID: 7, Title: "Article"}},
		{eventType: "topic.published.v1", payload: topicPublishedPayload{TopicID: 102, AuthorID: 7, Title: "Topic"}},
	} {
		t.Run(test.eventType, func(t *testing.T) {
			t.Parallel()
			encoded, err := json.Marshal(test.payload)
			if err != nil {
				t.Fatalf("marshal payload: %v", err)
			}
			repo := &mallProjectorRepo{}
			projector := NewProjector(app.NewService(repo))
			occurredAt := time.Now().UTC()
			if err := projector.HandleArticle(context.Background(), eventEnvelope{EventID: "evt-note", EventType: test.eventType, OccurredAt: occurredAt, Payload: encoded}); err != nil {
				t.Fatalf("handle article event: %v", err)
			}
			if len(repo.webhookEvents) != 1 {
				t.Fatalf("webhook events = %+v", repo.webhookEvents)
			}
			got := repo.webhookEvents[0]
			if got.userID != 7 || got.eventType != "note" || got.eventID != "evt-note" || !got.createdAt.Equal(occurredAt) {
				t.Fatalf("webhook event = %+v", got)
			}
		})
	}
}
