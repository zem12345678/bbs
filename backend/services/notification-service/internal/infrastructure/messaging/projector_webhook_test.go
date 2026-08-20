package messaging

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"google.golang.org/grpc"
	"notification-service/api/proto/userpb"
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

func TestProjectorCreatesPublishedNoteNotificationsForSubscribers(t *testing.T) {
	repo := &mallProjectorRepo{}
	subscribers := &publishedSubscriberClient{items: []*userpb.NoteNotificationSubscriber{
		{EdgeId: 1001, UserId: 11},
		{EdgeId: 1002, UserId: 12},
	}}
	projector := NewProjector(app.NewService(repo), subscribers)
	payload, err := json.Marshal(articlePublishedPayload{ArticleID: 901, AuthorID: 7, Title: "New article"})
	if err != nil {
		t.Fatalf("marshal payload: %v", err)
	}
	occurredAt := time.Now().UTC()
	if err := projector.HandleArticle(context.Background(), eventEnvelope{
		EventID:    "evt-published",
		EventType:  "article.published.v1",
		OccurredAt: occurredAt,
		Payload:    payload,
	}); err != nil {
		t.Fatalf("handle article event: %v", err)
	}
	if len(repo.created) != 2 {
		t.Fatalf("created notifications = %+v", repo.created)
	}
	for index, item := range repo.created {
		wantUserID := int64(11 + index)
		if item.UserID != wantUserID || item.Type != "note" || item.ActorID != 7 || item.EntityType != "article" || item.EntityID != 901 || item.SourceID != 901 {
			t.Fatalf("notification[%d] = %+v", index, item)
		}
	}
	if len(repo.sourceEventIDs) != 2 || repo.sourceEventIDs[0] != "evt-published" || repo.sourceEventIDs[1] != "evt-published" {
		t.Fatalf("source event IDs = %+v", repo.sourceEventIDs)
	}
}

type publishedSubscriberClient struct {
	userpb.UserServiceClient
	items []*userpb.NoteNotificationSubscriber
}

func (c *publishedSubscriberClient) ListNoteNotificationSubscribers(_ context.Context, _ *userpb.ListNoteNotificationSubscribersRequest, _ ...grpc.CallOption) (*userpb.NoteNotificationSubscribersResponse, error) {
	return &userpb.NoteNotificationSubscribersResponse{Items: c.items}, nil
}
