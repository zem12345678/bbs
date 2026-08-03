package user

import (
	"encoding/json"
	"testing"
	"time"
)

func TestUserEventCarriesSearchProjectionWithoutCredentials(t *testing.T) {
	createdAt := time.Date(2026, time.July, 29, 8, 0, 0, 0, time.UTC)
	updatedAt := createdAt.Add(time.Hour)
	event := NewUpdatedEvent(&User{
		ID:           42,
		Username:     "alice",
		Email:        "alice@example.com",
		PasswordHash: "secret-hash",
		Nickname:     "Alice",
		Status:       StatusActive,
		CreatedAt:    createdAt,
		UpdatedAt:    updatedAt,
	})

	payload, err := json.Marshal(event)
	if err != nil {
		t.Fatalf("marshal event: %v", err)
	}
	var fields map[string]any
	if err := json.Unmarshal(payload, &fields); err != nil {
		t.Fatalf("unmarshal event: %v", err)
	}
	for _, forbidden := range []string{"password_hash", "credential_version"} {
		if _, exists := fields[forbidden]; exists {
			t.Fatalf("event exposes %q: %s", forbidden, payload)
		}
	}
	if fields["user_id"] != float64(42) || fields["username"] != "alice" || fields["email"] != "alice@example.com" || fields["nickname"] != "Alice" {
		t.Fatalf("unexpected user event identity: %s", payload)
	}
	if fields["status"] != float64(StatusActive) || fields["created_at"] != float64(createdAt.UnixMilli()) || fields["updated_at"] != float64(updatedAt.UnixMilli()) {
		t.Fatalf("unexpected public search projection: %s", payload)
	}
}
