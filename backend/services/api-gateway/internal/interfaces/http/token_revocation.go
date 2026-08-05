package http

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"strings"
	"time"

	"github.com/redis/go-redis/v9"
)

const (
	tokenRevocationKeyPrefix   = "bbs:auth:revoked-token:"
	sessionRevocationKeyPrefix = "bbs:auth:revoked-session:"
)

var errTokenRevocationUnavailable = errors.New("authorization token revocation is unavailable")

// TokenRevocationStore records access tokens that must no longer be accepted.
type TokenRevocationStore interface {
	Revoke(context.Context, string, time.Time) error
	IsRevoked(context.Context, string) (bool, error)
	IsRevokedFingerprint(context.Context, string) (bool, error)
	// RevokeSession invalidates every token carrying the session id so a
	// single device can be signed out without touching the other sessions.
	RevokeSession(context.Context, string, time.Time) error
	IsSessionRevoked(context.Context, string) (bool, error)
}

// TokenRevocationCommands is the Redis subset used by RedisTokenRevocationStore.
type TokenRevocationCommands interface {
	Set(context.Context, string, interface{}, time.Duration) *redis.StatusCmd
	Exists(context.Context, ...string) *redis.IntCmd
}

// RedisTokenRevocationStore stores a one-way hash of each revoked access token.
type RedisTokenRevocationStore struct {
	redis TokenRevocationCommands
	now   func() time.Time
}

func NewRedisTokenRevocationStore(client TokenRevocationCommands) *RedisTokenRevocationStore {
	return &RedisTokenRevocationStore{redis: client, now: time.Now}
}

func (s *RedisTokenRevocationStore) Revoke(ctx context.Context, token string, expiresAt time.Time) error {
	if s == nil || s.redis == nil {
		return errTokenRevocationUnavailable
	}
	ttl := expiresAt.Sub(s.now())
	if ttl <= 0 {
		return errors.New("authorization token has expired")
	}
	return s.redis.Set(ctx, tokenRevocationKey(token), "1", ttl).Err()
}

func (s *RedisTokenRevocationStore) IsRevoked(ctx context.Context, token string) (bool, error) {
	return s.IsRevokedFingerprint(ctx, tokenRevocationFingerprint(token))
}

func (s *RedisTokenRevocationStore) IsRevokedFingerprint(ctx context.Context, fingerprint string) (bool, error) {
	if s == nil || s.redis == nil {
		return false, errTokenRevocationUnavailable
	}
	if len(fingerprint) != sha256.Size*2 {
		return false, errors.New("invalid authorization token fingerprint")
	}
	count, err := s.redis.Exists(ctx, tokenRevocationKeyPrefix+fingerprint).Result()
	if err != nil {
		return false, err
	}
	return count > 0, nil
}

func (s *RedisTokenRevocationStore) RevokeSession(ctx context.Context, sessionID string, expiresAt time.Time) error {
	if s == nil || s.redis == nil {
		return errTokenRevocationUnavailable
	}
	sessionID = strings.TrimSpace(sessionID)
	if sessionID == "" {
		return errors.New("session id is required")
	}
	ttl := expiresAt.Sub(s.now())
	if ttl <= 0 {
		return errors.New("session has expired")
	}
	return s.redis.Set(ctx, sessionRevocationKeyPrefix+sessionID, "1", ttl).Err()
}

func (s *RedisTokenRevocationStore) IsSessionRevoked(ctx context.Context, sessionID string) (bool, error) {
	if s == nil || s.redis == nil {
		return false, errTokenRevocationUnavailable
	}
	sessionID = strings.TrimSpace(sessionID)
	if sessionID == "" {
		return false, nil
	}
	count, err := s.redis.Exists(ctx, sessionRevocationKeyPrefix+sessionID).Result()
	if err != nil {
		return false, err
	}
	return count > 0, nil
}

func tokenRevocationKey(token string) string {
	return tokenRevocationKeyPrefix + tokenRevocationFingerprint(token)
}

func tokenRevocationFingerprint(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}
