package app

import (
	"strings"
	"testing"
)

func TestContentOutboxOwnerIsUniquePerProcess(t *testing.T) {
	first := contentOutboxOwner("content-service")
	second := contentOutboxOwner("content-service")
	if first == second {
		t.Fatal("outbox owners must not be shared by concurrent service processes")
	}
	if !strings.HasPrefix(first, "content-service:") {
		t.Fatalf("owner = %q, want content-service prefix", first)
	}
}
