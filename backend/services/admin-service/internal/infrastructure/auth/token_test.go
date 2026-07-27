package auth

import (
	"errors"
	"testing"
	"time"

	domain "admin/internal/domain/admin"
)

func TestTokenManagerIssuesSessionBoundAccessAndRefreshTokens(t *testing.T) {
	manager := NewTokenManager("test-secret", time.Hour, 24*time.Hour)
	token, err := manager.Issue(domain.AdminUser{ID: 7, Username: "admin"}, []string{"admin"}, "0123456789abcdef0123456789abcdef")
	if err != nil {
		t.Fatalf("Issue() error = %v", err)
	}

	accessClaims, err := manager.Parse(token.AccessToken)
	if err != nil {
		t.Fatalf("Parse(access) error = %v", err)
	}
	if accessClaims.UserID != 7 || accessClaims.Username != "admin" || accessClaims.SessionID != "0123456789abcdef0123456789abcdef" {
		t.Fatalf("Parse(access) claims = %#v", accessClaims)
	}
	refreshClaims, err := manager.ParseRefresh(token.RefreshToken)
	if err != nil {
		t.Fatalf("ParseRefresh(refresh) error = %v", err)
	}
	if refreshClaims.SessionID != accessClaims.SessionID {
		t.Fatalf("refresh session id = %q, want %q", refreshClaims.SessionID, accessClaims.SessionID)
	}
	if _, err := manager.Parse(token.RefreshToken); !errors.Is(err, domain.ErrInvalidToken) {
		t.Fatalf("Parse(refresh) error = %v, want ErrInvalidToken", err)
	}
	if _, err := manager.ParseRefresh(token.AccessToken); !errors.Is(err, domain.ErrInvalidToken) {
		t.Fatalf("ParseRefresh(access) error = %v, want ErrInvalidToken", err)
	}
}

func TestTokenManagerRejectsMissingSessionID(t *testing.T) {
	manager := NewTokenManager("test-secret", time.Hour, 24*time.Hour)
	_, err := manager.Issue(domain.AdminUser{ID: 7, Username: "admin"}, nil, " ")
	if !errors.Is(err, domain.ErrInvalidToken) {
		t.Fatalf("Issue() error = %v, want ErrInvalidToken", err)
	}
}
