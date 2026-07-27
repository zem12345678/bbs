package persistence

import (
	"errors"
	"strings"
	"testing"
	"time"

	domain "admin/internal/domain/admin"
)

func TestValidateAdminSession(t *testing.T) {
	now := time.Date(2026, time.July, 26, 12, 0, 0, 0, time.UTC)
	validSessionID := strings.Repeat("a", maxAdminSessionIDLength)

	tests := []struct {
		name      string
		userID    int64
		sessionID string
		expiresAt time.Time
		want      string
	}{
		{
			name:      "valid",
			userID:    1,
			sessionID: validSessionID,
			expiresAt: now.Add(time.Hour),
			want:      validSessionID,
		},
		{
			name:      "trims session id",
			userID:    1,
			sessionID: "  " + validSessionID + "  ",
			expiresAt: now.Add(time.Hour),
			want:      validSessionID,
		},
		{
			name:      "missing user id",
			userID:    0,
			sessionID: validSessionID,
			expiresAt: now.Add(time.Hour),
		},
		{
			name:      "missing session id",
			userID:    1,
			sessionID: " ",
			expiresAt: now.Add(time.Hour),
		},
		{
			name:      "too long session id",
			userID:    1,
			sessionID: strings.Repeat("a", maxAdminSessionIDLength+1),
			expiresAt: now.Add(time.Hour),
		},
		{
			name:      "expired session",
			userID:    1,
			sessionID: validSessionID,
			expiresAt: now,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := validateAdminSession(tt.userID, tt.sessionID, tt.expiresAt, now)
			if tt.want == "" {
				if !errors.Is(err, domain.ErrInvalidToken) {
					t.Fatalf("validateAdminSession() error = %v, want ErrInvalidToken", err)
				}
				return
			}
			if err != nil {
				t.Fatalf("validateAdminSession() error = %v", err)
			}
			if got != tt.want {
				t.Fatalf("validateAdminSession() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestValidateAdminSessionID(t *testing.T) {
	validSessionID := strings.Repeat("a", maxAdminSessionIDLength)
	got, err := validateAdminSessionID(1, " "+validSessionID+" ")
	if err != nil {
		t.Fatalf("validateAdminSessionID() error = %v", err)
	}
	if got != validSessionID {
		t.Fatalf("validateAdminSessionID() = %q, want %q", got, validSessionID)
	}
}
