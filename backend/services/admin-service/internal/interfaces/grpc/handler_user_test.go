package grpc

import (
	"testing"

	domain "admin/internal/domain/admin"
)

func TestToPbUserMapsBackgroundURL(t *testing.T) {
	user := toPbUser(domain.User{
		ID:            42,
		Username:      "alice",
		BackgroundURL: "https://example.test/background.webp",
	})

	if user.GetId() != 42 {
		t.Fatalf("id = %d", user.GetId())
	}
	if user.GetBackgroundUrl() != "https://example.test/background.webp" {
		t.Fatalf("background url = %q", user.GetBackgroundUrl())
	}
}
