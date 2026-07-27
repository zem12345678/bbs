package auth

import (
	"strconv"
	"strings"
	"time"

	domain "admin/internal/domain/admin"

	"github.com/golang-jwt/jwt/v5"
)

type TokenManager struct {
	secret     []byte
	accessTTL  time.Duration
	refreshTTL time.Duration
}

type adminClaims struct {
	UserID    int64    `json:"user_id"`
	Username  string   `json:"username"`
	Roles     []string `json:"roles,omitempty"`
	TokenType string   `json:"token_type"`
	SessionID string   `json:"session_id"`
	jwt.RegisteredClaims
}

const (
	tokenTypeAccess  = "access"
	tokenTypeRefresh = "refresh"
)

func NewTokenManager(secret string, accessTTL time.Duration, refreshTTL time.Duration) *TokenManager {
	return &TokenManager{secret: []byte(secret), accessTTL: accessTTL, refreshTTL: refreshTTL}
}

func (m *TokenManager) Issue(user domain.AdminUser, roles []string, sessionID string) (domain.AdminToken, error) {
	sessionID = strings.TrimSpace(sessionID)
	if user.ID <= 0 || sessionID == "" {
		return domain.AdminToken{}, domain.ErrInvalidToken
	}
	now := time.Now()
	accessToken, expiresAt, err := m.issue(user, roles, sessionID, tokenTypeAccess, m.accessTTL, now)
	if err != nil {
		return domain.AdminToken{}, err
	}
	refreshToken, refreshExpiresAt, err := m.issue(user, roles, sessionID, tokenTypeRefresh, m.refreshTTL, now)
	if err != nil {
		return domain.AdminToken{}, err
	}
	return domain.AdminToken{
		AccessToken:      accessToken,
		ExpiresAt:        expiresAt.Unix(),
		RefreshToken:     refreshToken,
		RefreshExpiresAt: refreshExpiresAt.Unix(),
	}, nil
}

func (m *TokenManager) issue(user domain.AdminUser, roles []string, sessionID string, tokenType string, ttl time.Duration, now time.Time) (string, time.Time, error) {
	expiresAt := now.Add(ttl)
	claims := adminClaims{
		UserID:    user.ID,
		Username:  user.Username,
		Roles:     roles,
		TokenType: tokenType,
		SessionID: sessionID,
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   strconv.FormatInt(user.ID, 10),
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(expiresAt),
		},
	}
	token, err := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString(m.secret)
	if err != nil {
		return "", time.Time{}, err
	}
	return token, expiresAt, nil
}

func (m *TokenManager) Parse(accessToken string) (domain.TokenClaims, error) {
	return m.parse(accessToken, tokenTypeAccess)
}

func (m *TokenManager) ParseRefresh(refreshToken string) (domain.TokenClaims, error) {
	return m.parse(refreshToken, tokenTypeRefresh)
}

func (m *TokenManager) parse(tokenText string, expectedTokenType string) (domain.TokenClaims, error) {
	if tokenText == "" {
		return domain.TokenClaims{}, domain.ErrInvalidToken
	}
	claims := &adminClaims{}
	token, err := jwt.ParseWithClaims(tokenText, claims, func(token *jwt.Token) (any, error) {
		if token.Method != jwt.SigningMethodHS256 {
			return nil, domain.ErrInvalidToken
		}
		return m.secret, nil
	})
	if err != nil || token == nil || !token.Valid {
		return domain.TokenClaims{}, domain.ErrInvalidToken
	}
	if claims.TokenType != expectedTokenType {
		return domain.TokenClaims{}, domain.ErrInvalidToken
	}
	userID := claims.UserID
	if userID <= 0 && claims.Subject != "" {
		parsed, err := strconv.ParseInt(claims.Subject, 10, 64)
		if err != nil {
			return domain.TokenClaims{}, domain.ErrInvalidToken
		}
		userID = parsed
	}
	if userID <= 0 || strings.TrimSpace(claims.SessionID) == "" {
		return domain.TokenClaims{}, domain.ErrInvalidToken
	}
	return domain.TokenClaims{UserID: userID, Username: claims.Username, SessionID: claims.SessionID}, nil
}
