package http

import (
	"testing"
	"time"
)

func TestGitHubAccountMeetsMinAge(t *testing.T) {
	now := time.Date(2026, 7, 6, 12, 0, 0, 0, time.UTC)
	if !githubAccountMeetsMinAge(time.Date(2023, 7, 6, 11, 59, 0, 0, time.UTC), 3, now) {
		t.Fatalf("expected account older than three years to pass")
	}
	if githubAccountMeetsMinAge(time.Date(2023, 7, 7, 0, 0, 0, 0, time.UTC), 3, now) {
		t.Fatalf("expected account younger than three years to fail")
	}
	if githubAccountMeetsMinAge(time.Time{}, 3, now) {
		t.Fatalf("expected missing created_at to fail")
	}
}
