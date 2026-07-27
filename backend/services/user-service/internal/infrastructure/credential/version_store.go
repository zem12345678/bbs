// Package credential mirrors durable credential versions for fast invalidation.
package credential

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/redis/go-redis/v9"
)

const keyPrefix = "bbs:auth:credential-version:"

var errUnavailable = errors.New("credential version cache unavailable")

type RedisCommands interface {
	Set(context.Context, string, interface{}, time.Duration) *redis.StatusCmd
	Del(context.Context, ...string) *redis.IntCmd
}

type Store struct {
	redis RedisCommands
}

func NewStore(client RedisCommands) *Store {
	return &Store{redis: client}
}

func (s *Store) SetCurrent(ctx context.Context, userID int64, version string) error {
	if s == nil || s.redis == nil {
		return errUnavailable
	}
	if userID <= 0 {
		return fmt.Errorf("invalid credential version user id: %d", userID)
	}
	if version = strings.TrimSpace(version); version == "" {
		return errors.New("credential version is empty")
	}
	return s.redis.Set(ctx, key(userID), version, 0).Err()
}

func (s *Store) Delete(ctx context.Context, userID int64) error {
	if s == nil || s.redis == nil {
		return errUnavailable
	}
	if userID <= 0 {
		return fmt.Errorf("invalid credential version user id: %d", userID)
	}
	return s.redis.Del(ctx, key(userID)).Err()
}

func key(userID int64) string {
	return keyPrefix + strconv.FormatInt(userID, 10)
}
