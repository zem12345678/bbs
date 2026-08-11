package messaging

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	app "notification-service/internal/application/notification"
	domain "notification-service/internal/domain/notification"
)

func TestProjectorHandlesUserFollowEvents(t *testing.T) {
	t.Parallel()

	occurredAt := time.Date(2026, time.August, 7, 12, 30, 0, 0, time.UTC)
	payload, err := json.Marshal(followPayload{FollowerID: 11, FolloweeID: 22})
	if err != nil {
		t.Fatalf("marshal follow payload: %v", err)
	}
	tests := []struct {
		eventType  string
		wantUserID int64
		wantType   string
		wantActor  int64
	}{
		{eventType: "user.followed", wantUserID: 22, wantType: domain.NotificationTypeFollow, wantActor: 11},
		{eventType: "user.follow_requested", wantUserID: 22, wantType: domain.NotificationTypeFollowRequestReceived, wantActor: 11},
		{eventType: "user.follow_request_accepted", wantUserID: 11, wantType: domain.NotificationTypeFollowRequestAccepted, wantActor: 22},
	}
	for _, tt := range tests {
		t.Run(tt.eventType, func(t *testing.T) {
			t.Parallel()
			repo := &mallProjectorRepo{}
			projector := NewProjector(app.NewService(repo))
			eventID := "evt-" + tt.eventType
			if err := projector.HandleUser(context.Background(), eventEnvelope{
				EventID:    eventID,
				EventType:  tt.eventType,
				OccurredAt: occurredAt,
				Payload:    payload,
			}); err != nil {
				t.Fatalf("handle user event: %v", err)
			}
			if len(repo.created) != 1 {
				t.Fatalf("created notifications = %d, want 1", len(repo.created))
			}
			item := repo.created[0]
			if item.UserID != tt.wantUserID || item.Type != tt.wantType || item.ActorID != tt.wantActor || item.EntityType != "user" || item.EntityID != tt.wantActor {
				t.Fatalf("notification = %+v", item)
			}
			if len(repo.sourceEventIDs) != 1 || repo.sourceEventIDs[0] != eventID {
				t.Fatalf("source event IDs = %+v", repo.sourceEventIDs)
			}
			if len(repo.createdAt) != 1 || !repo.createdAt[0].Equal(occurredAt) {
				t.Fatalf("createdAt = %+v, want %s", repo.createdAt, occurredAt)
			}
		})
	}
}

func TestProjectorUserEventValidation(t *testing.T) {
	t.Parallel()

	repo := &mallProjectorRepo{}
	projector := NewProjector(app.NewService(repo))
	if err := projector.HandleUser(context.Background(), eventEnvelope{EventType: "user.updated", Payload: json.RawMessage(`not-json`)}); err != nil {
		t.Fatalf("unknown user event: %v", err)
	}
	if len(repo.created) != 0 {
		t.Fatalf("unknown event created %d notifications", len(repo.created))
	}
	if err := projector.HandleUser(context.Background(), eventEnvelope{EventType: "user.follow_requested", Payload: json.RawMessage(`not-json`)}); err == nil {
		t.Fatal("malformed follow request payload returned nil error")
	}
}

func TestProjectorEnqueuesFollowAndUnfollowWebhooks(t *testing.T) {
	t.Parallel()
	payload, err := json.Marshal(followPayload{FollowerID: 11, FolloweeID: 22})
	if err != nil {
		t.Fatalf("marshal follow payload: %v", err)
	}
	for _, test := range []struct {
		eventType string
		want      []webhookEventRecord
	}{
		{eventType: "user.followed", want: []webhookEventRecord{{userID: 11, eventType: "follow"}, {userID: 22, eventType: "followed"}}},
		{eventType: "user.unfollowed", want: []webhookEventRecord{{userID: 11, eventType: "unfollow"}}},
	} {
		t.Run(test.eventType, func(t *testing.T) {
			t.Parallel()
			repo := &mallProjectorRepo{}
			projector := NewProjector(app.NewService(repo))
			if err := projector.HandleUser(context.Background(), eventEnvelope{EventID: "evt-user", EventType: test.eventType, OccurredAt: time.Now().UTC(), Payload: payload}); err != nil {
				t.Fatalf("handle user event: %v", err)
			}
			if len(repo.webhookEvents) != len(test.want) {
				t.Fatalf("webhook events = %+v, want %d", repo.webhookEvents, len(test.want))
			}
			for index, want := range test.want {
				got := repo.webhookEvents[index]
				if got.userID != want.userID || got.eventType != want.eventType || got.eventID != "evt-user" || len(got.payload) == 0 {
					t.Fatalf("webhook event[%d] = %+v, want user=%d type=%s", index, got, want.userID, want.eventType)
				}
			}
		})
	}
}
