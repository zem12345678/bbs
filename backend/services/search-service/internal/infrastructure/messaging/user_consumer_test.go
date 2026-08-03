package messaging

import (
	"context"
	"encoding/json"
	"testing"

	domain "search-service/internal/domain/search"
)

func TestUserConsumerIndexesOnlySearchProjection(t *testing.T) {
	raw, err := json.Marshal(map[string]any{
		"user_id":    42,
		"username":   "alice",
		"nickname":   "Alice",
		"status":     userStatusActive,
		"created_at": int64(1000),
		"updated_at": int64(2000),
		"email":      "alice@example.com",
	})
	if err != nil {
		t.Fatalf("marshal payload: %v", err)
	}
	indexer := &fakeUserIndexer{}
	consumer := &UserConsumer{indexer: indexer}

	if err := consumer.handle(context.Background(), eventEnvelope{EventType: "user.updated", AggregateID: "42", Payload: raw}); err != nil {
		t.Fatalf("handle returned error: %v", err)
	}
	if indexer.doc.ID != 42 || indexer.doc.Username != "alice" || indexer.doc.Nickname != "Alice" {
		t.Fatalf("indexed identity = %#v", indexer.doc)
	}
	if indexer.doc.Status != userStatusActive || indexer.doc.CreatedAt != 1000 || indexer.doc.UpdatedAt != 2000 {
		t.Fatalf("indexed user document = %#v", indexer.doc)
	}
	if indexer.deletedID != 0 {
		t.Fatalf("deleted user id = %d, want none", indexer.deletedID)
	}
}

func TestUserConsumerRemovesNonPublicUser(t *testing.T) {
	raw, err := json.Marshal(userPayload{UserID: 42, Username: "alice", Status: 2})
	if err != nil {
		t.Fatalf("marshal payload: %v", err)
	}
	indexer := &fakeUserIndexer{}
	consumer := &UserConsumer{indexer: indexer}

	if err := consumer.handle(context.Background(), eventEnvelope{EventType: "user.updated", AggregateID: "42", Payload: raw}); err != nil {
		t.Fatalf("handle returned error: %v", err)
	}
	if indexer.deletedID != 42 {
		t.Fatalf("deleted user id = %d, want 42", indexer.deletedID)
	}
	if indexer.doc.ID != 0 {
		t.Fatalf("indexed user = %#v, want none", indexer.doc)
	}
}

func TestUserConsumerDeletesLegacyEventWithoutStatusSnapshot(t *testing.T) {
	raw, err := json.Marshal(userPayload{Username: "alice", Nickname: "Alice"})
	if err != nil {
		t.Fatalf("marshal payload: %v", err)
	}
	indexer := &fakeUserIndexer{}
	consumer := &UserConsumer{indexer: indexer}

	if err := consumer.handle(context.Background(), eventEnvelope{EventType: "user.created", AggregateID: "42", Payload: raw}); err != nil {
		t.Fatalf("handle returned error: %v", err)
	}
	if indexer.deletedID != 42 || indexer.doc.ID != 0 {
		t.Fatalf("legacy event result = %#v, deleted=%d", indexer.doc, indexer.deletedID)
	}
}

type fakeUserIndexer struct {
	doc       domain.UserDocument
	deletedID int64
}

func (f *fakeUserIndexer) EnsureUserIndex(context.Context) error { return nil }
func (f *fakeUserIndexer) IndexUser(_ context.Context, doc domain.UserDocument) error {
	f.doc = doc
	return nil
}
func (f *fakeUserIndexer) DeleteUser(_ context.Context, id int64) error {
	f.deletedID = id
	return nil
}
