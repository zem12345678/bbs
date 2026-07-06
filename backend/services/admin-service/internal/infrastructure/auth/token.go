package auth

import (
	"strconv"
	"time"

	domain "admin/internal/domain/admin"

	"github.com/golang-jwt/jwt/v5"
)

type TokenManager struct {
	secret []byte
	ttl    time.Duration
}

type adminClaims struct {
	UserID   int64    `json:"user_id"`
	Username string   `json:"username"`
	Roles    []string `json:"roles,omitempty"`
	jwt.RegisteredClaims
}

func NewTokenManager(secret string, ttl time.Duration) *TokenManager {
	return &TokenManager{secret: []byte(secret), ttl: ttl}
}

func (m *TokenManager) Issue(user domain.AdminUser, roles []string) (domain.AdminToken, error) {
	now := time.Now()
	expiresAt := now.Add(m.ttl)
	claims := adminClaims{
		UserID:   user.ID,
		Username: user.Username,
		Roles:    roles,
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   strconv.FormatInt(user.ID, 10),
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(expiresAt),
		},
	}
	token, err := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString(m.secret)
	if err != nil {
		return domain.AdminToken{}, err
	}
	return domain.AdminToken{AccessToken: token, ExpiresAt: expiresAt.Unix()}, nil
}

func (m *TokenManager) Parse(accessToken string) (domain.TokenClaims, error) {
	if accessToken == "" {
		return domain.TokenClaims{}, domain.ErrInvalidToken
	}
	claims := &adminClaims{}
	token, err := jwt.ParseWithClaims(accessToken, claims, func(token *jwt.Token) (any, error) {
		if token.Method != jwt.SigningMethodHS256 {
			return nil, domain.ErrInvalidToken
		}
		return m.secret, nil
	})
	if err != nil || token == nil || !token.Valid {
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
	if userID <= 0 {
		return domain.TokenClaims{}, domain.ErrInvalidToken
	}
	return domain.TokenClaims{UserID: userID, Username: claims.Username}, nil
}
