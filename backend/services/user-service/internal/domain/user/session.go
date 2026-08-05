package user

import (
	"context"
	"strings"
	"time"
)

const (
	MaxSessionListLimit = 100
	maxClientIPRunes    = 64
	maxUserAgentRunes   = 512
	maxLoginMethodRunes = 32
	maxFailureRunes     = 64
)

type SessionClientInfo struct {
	IPAddress string
	UserAgent string
}

type UserSession struct {
	SessionID   string
	UserID      int64
	IPAddress   string
	UserAgent   string
	LoginMethod string
	CreatedAt   time.Time
	ExpiresAt   time.Time
	RevokedAt   *time.Time
}

type LoginEvent struct {
	ID            int64
	UserID        int64
	SessionID     string
	IPAddress     string
	UserAgent     string
	Success       bool
	FailureReason string
	CreatedAt     time.Time
}

type SessionRepository interface {
	RecordSession(context.Context, UserSession, LoginEvent) error
	RecordLoginEvent(context.Context, LoginEvent) error
	ListSessions(context.Context, int64, int) ([]UserSession, error)
	GetSession(context.Context, int64, string) (UserSession, error)
	RevokeSession(context.Context, int64, string, time.Time) (UserSession, error)
	ListLoginEvents(context.Context, int64, int) ([]LoginEvent, error)
}

func NormalizeSessionClientInfo(info SessionClientInfo) SessionClientInfo {
	return SessionClientInfo{
		IPAddress: truncateRunes(strings.TrimSpace(info.IPAddress), maxClientIPRunes),
		UserAgent: truncateRunes(strings.TrimSpace(info.UserAgent), maxUserAgentRunes),
	}
}

func NormalizeLoginMethod(value string) string {
	return strings.ToLower(truncateRunes(strings.TrimSpace(value), maxLoginMethodRunes))
}

func NormalizeLoginFailureReason(value string) string {
	return strings.ToLower(truncateRunes(strings.TrimSpace(value), maxFailureRunes))
}

func ValidSessionID(value string) bool {
	value = strings.TrimSpace(value)
	if len(value) < 16 || len(value) > 128 {
		return false
	}
	for _, char := range value {
		if (char >= 'a' && char <= 'z') || (char >= 'A' && char <= 'Z') || (char >= '0' && char <= '9') || char == '-' || char == '_' {
			continue
		}
		return false
	}
	return true
}

func (session UserSession) Validate() error {
	if session.UserID <= 0 {
		return ErrInvalidID
	}
	if !ValidSessionID(session.SessionID) {
		return ErrSessionIDInvalid
	}
	if NormalizeLoginMethod(session.LoginMethod) == "" {
		return ErrLoginMethodInvalid
	}
	if session.CreatedAt.IsZero() || !session.ExpiresAt.After(session.CreatedAt) {
		return ErrSessionExpiryInvalid
	}
	return nil
}

func (event LoginEvent) Validate() error {
	if event.ID <= 0 || event.UserID <= 0 {
		return ErrInvalidID
	}
	if event.SessionID != "" && !ValidSessionID(event.SessionID) {
		return ErrSessionIDInvalid
	}
	if event.CreatedAt.IsZero() {
		return ErrLoginEventInvalid
	}
	return nil
}

func SessionListLimit(value int) int {
	if value <= 0 {
		return 20
	}
	if value > MaxSessionListLimit {
		return MaxSessionListLimit
	}
	return value
}

func truncateRunes(value string, limit int) string {
	if limit <= 0 {
		return ""
	}
	runes := []rune(value)
	if len(runes) <= limit {
		return value
	}
	return string(runes[:limit])
}
