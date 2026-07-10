package upstream

import (
	"testing"

	"admin/api/proto/userpb"
)

func TestToDomainUserMapsBackgroundURL(t *testing.T) {
	user := toDomainUser(&userpb.UserInfo{
		Id:            42,
		Username:      "alice",
		BackgroundUrl: "https://example.test/background.webp",
	})

	if user.ID != 42 {
		t.Fatalf("id = %d", user.ID)
	}
	if user.BackgroundURL != "https://example.test/background.webp" {
		t.Fatalf("background url = %q", user.BackgroundURL)
	}
}
