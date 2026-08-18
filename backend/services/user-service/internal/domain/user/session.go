package user

import (
	"context"
	"strings"
	"time"
)

const (
	MaxSessionListLimit  = 100
	MaxAPITokenNameRunes = 128
	APITokenScopeRead    = "read"
	APITokenScopeWrite   = "write"
	maxClientIPRunes     = 64
	maxUserAgentRunes    = 512
	maxLoginMethodRunes  = 32
	maxFailureRunes      = 64
)

type SessionClientInfo struct {
	IPAddress string
	UserAgent string
}

type UserSession struct {
	SessionID                 string
	UserID                    int64
	IPAddress                 string
	UserAgent                 string
	LoginMethod               string
	APITokenName              string
	APITokenScopes            []string
	APITokenCredentialVersion string
	APITokenCredentialValid   bool
	CreatedAt                 time.Time
	ExpiresAt                 time.Time
	RevokedAt                 *time.Time
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
	CreateAPIToken(context.Context, UserSession) error
	RecordLoginEvent(context.Context, LoginEvent) error
	ListSessions(context.Context, int64, int) ([]UserSession, error)
	ListAPITokens(context.Context, int64, int, int) ([]UserSession, int64, error)
	RevokeAPIToken(context.Context, int64, string, time.Time) (UserSession, error)
	GetSession(context.Context, int64, string) (UserSession, error)
	RevokeSession(context.Context, int64, string, time.Time) (UserSession, error)
	ListLoginEvents(context.Context, int64, int) ([]LoginEvent, error)
}

type LoginEventKeysetRepository interface {
	ListLoginEventsAfterID(context.Context, int64, int64, int) ([]LoginEvent, error)
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

func NormalizeAPITokenName(value string) (string, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return "", ErrAPITokenNameRequired
	}
	if len([]rune(value)) > MaxAPITokenNameRunes {
		return "", ErrAPITokenNameTooLong
	}
	return value, nil
}

func NormalizeAPITokenScopes(values []string) ([]string, error) {
	seen := make(map[string]bool, len(values))
	for _, value := range values {
		scope := strings.ToLower(strings.TrimSpace(value))
		if scope != APITokenScopeRead && scope != APITokenScopeWrite {
			return nil, ErrAPITokenScopeInvalid
		}
		seen[scope] = true
	}
	if len(seen) == 0 {
		return nil, ErrAPITokenScopeInvalid
	}
	scopes := make([]string, 0, len(seen))
	if seen[APITokenScopeRead] {
		scopes = append(scopes, APITokenScopeRead)
	}
	if seen[APITokenScopeWrite] {
		scopes = append(scopes, APITokenScopeWrite)
	}
	return scopes, nil
}

func APITokenScopesValue(scopes []string) (string, error) {
	normalized, err := NormalizeAPITokenScopes(scopes)
	if err != nil {
		return "", err
	}
	return strings.Join(normalized, ","), nil
}

func ParseAPITokenScopes(value string) []string {
	if strings.TrimSpace(value) == "" {
		return nil
	}
	scopes, err := NormalizeAPITokenScopes(strings.Split(value, ","))
	if err != nil {
		return nil
	}
	return scopes
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
	if NormalizeLoginMethod(session.LoginMethod) == "api_token" {
		if _, err := NormalizeAPITokenName(session.APITokenName); err != nil {
			return err
		}
		if _, err := NormalizeAPITokenScopes(session.APITokenScopes); err != nil {
			return err
		}
		if strings.TrimSpace(session.APITokenCredentialVersion) == "" {
			return ErrInvalidCredentialVersion
		}
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
