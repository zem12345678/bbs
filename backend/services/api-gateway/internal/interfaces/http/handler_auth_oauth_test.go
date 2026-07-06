package http

import (
	"testing"
	"time"

	"golang.org/x/crypto/bcrypt"
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

func TestWebmasterPasswordMatches(t *testing.T) {
	if !webmasterPasswordMatches("webmaster123", "webmaster123") {
		t.Fatalf("expected plaintext compatibility match")
	}
	if webmasterPasswordMatches("webmaster123", "wrong") {
		t.Fatalf("expected plaintext mismatch")
	}
	hash, err := bcrypt.GenerateFromPassword([]byte("webmaster123"), bcrypt.DefaultCost)
	if err != nil {
		t.Fatalf("hash password: %v", err)
	}
	if !webmasterPasswordMatches(string(hash), "webmaster123") {
		t.Fatalf("expected bcrypt match")
	}
	if webmasterPasswordMatches(string(hash), "wrong") {
		t.Fatalf("expected bcrypt mismatch")
	}
}
