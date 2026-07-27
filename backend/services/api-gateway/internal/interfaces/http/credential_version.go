package http

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"api-gateway/api/proto/userpb"

	"github.com/redis/go-redis/v9"
	"google.golang.org/grpc"
)

const (
	// credentialVersionClaim and credentialVersionKeyPrefix are shared with
	// user-service. User-service writes one opaque current version per user when
	// a password is changed or reset; tokens carry that version in the cv claim.
	credentialVersionClaim     = "cv"
	credentialVersionInitial   = "0"
	credentialVersionKeyPrefix = "bbs:auth:credential-version:"
)

var errCredentialVersionUnavailable = errors.New("credential version validation is unavailable")

// CredentialVersionStore returns the user's current password credential version.
type CredentialVersionStore interface {
	Current(context.Context, int64) (string, error)
}

// CredentialVersionAuthority is the durable source used when Redis no longer
// contains a user's credential version (for example after an eviction or flush).
type CredentialVersionAuthority interface {
	Current(context.Context, int64) (string, error)
}

type UserCredentialVersionClient interface {
	GetCredentialVersion(context.Context, *userpb.UserIDRequest, ...grpc.CallOption) (*userpb.CredentialVersionResponse, error)
}

// CredentialVersionCommands is the Redis subset used by RedisCredentialVersionStore.
type CredentialVersionCommands interface {
	Get(context.Context, string) *redis.StringCmd
	Set(context.Context, string, interface{}, time.Duration) *redis.StatusCmd
}

// RedisCredentialVersionStore mirrors the PostgreSQL-authoritative version
// from user-service. Redis is deliberately not trusted on its own: a cache
// value can be stale after a credential rotation, so each authentication check
// validates the durable value and repairs the mirror when necessary.
type RedisCredentialVersionStore struct {
	redis     CredentialVersionCommands
	authority CredentialVersionAuthority
}

func NewRedisCredentialVersionStore(client CredentialVersionCommands, authority CredentialVersionAuthority) *RedisCredentialVersionStore {
	return &RedisCredentialVersionStore{redis: client, authority: authority}
}

func (s *RedisCredentialVersionStore) Current(ctx context.Context, userID int64) (string, error) {
	if s == nil || s.redis == nil {
		return "", errCredentialVersionUnavailable
	}
	if userID <= 0 {
		return "", fmt.Errorf("invalid credential version user id: %d", userID)
	}
	cached, err := s.redis.Get(ctx, credentialVersionKey(userID)).Result()
	cacheMiss := errors.Is(err, redis.Nil)
	if err != nil && !cacheMiss {
		return "", err
	}
	if !cacheMiss {
		cached, err = validatedCredentialVersion(cached)
		if err != nil {
			return "", err
		}
	}

	version, err := s.currentFromAuthority(ctx, userID)
	if err != nil {
		return "", err
	}
	if cacheMiss || cached != version {
		if err := s.redis.Set(ctx, credentialVersionKey(userID), version, 0).Err(); err != nil {
			return "", err
		}
	}
	return version, nil
}

func (s *RedisCredentialVersionStore) currentFromAuthority(ctx context.Context, userID int64) (string, error) {
	if s.authority == nil {
		return "", errCredentialVersionUnavailable
	}
	version, err := s.authority.Current(ctx, userID)
	if err != nil {
		return "", err
	}
	version, err = validatedCredentialVersion(version)
	if err != nil {
		return "", err
	}
	return version, nil
}

func validatedCredentialVersion(version string) (string, error) {
	version = strings.TrimSpace(version)
	if version == "" {
		return "", errors.New("credential version is empty")
	}
	return version, nil
}

type userCredentialVersionAuthority struct {
	user UserCredentialVersionClient
}

// NewUserCredentialVersionAuthority adapts the internal user-service RPC to
// the durable lookup used by the gateway's Redis cache.
func NewUserCredentialVersionAuthority(user UserCredentialVersionClient) CredentialVersionAuthority {
	return &userCredentialVersionAuthority{user: user}
}

func (a *userCredentialVersionAuthority) Current(ctx context.Context, userID int64) (string, error) {
	if a == nil || a.user == nil {
		return "", errCredentialVersionUnavailable
	}
	if userID <= 0 {
		return "", fmt.Errorf("invalid credential version user id: %d", userID)
	}
	resp, err := a.user.GetCredentialVersion(ctx, &userpb.UserIDRequest{Id: userID})
	if err != nil {
		return "", err
	}
	if resp == nil || resp.GetUserId() != userID {
		return "", errors.New("invalid credential version response")
	}
	return validatedCredentialVersion(resp.GetCredentialVersion())
}

func credentialVersionKey(userID int64) string {
	return credentialVersionKeyPrefix + strconv.FormatInt(userID, 10)
}
