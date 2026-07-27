package http

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"testing"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/require"
)

func TestRedisTokenRevocationStoreHashesTokenAndUsesRemainingLifetime(t *testing.T) {
	ctx := context.Background()
	now := time.Date(2026, time.July, 25, 12, 0, 0, 0, time.UTC)
	commands := &fakeTokenRevocationCommands{}
	store := NewRedisTokenRevocationStore(commands)
	store.now = func() time.Time { return now }
	accessToken := "sensitive-access-token"

	err := store.Revoke(ctx, accessToken, now.Add(15*time.Minute))

	require.NoError(t, err)
	sum := sha256.Sum256([]byte(accessToken))
	expectedKey := tokenRevocationKeyPrefix + hex.EncodeToString(sum[:])
	require.Equal(t, expectedKey, commands.setKey)
	require.NotContains(t, commands.setKey, accessToken)
	require.Equal(t, "1", commands.setValue)
	require.Equal(t, 15*time.Minute, commands.setTTL)

	commands.existsCount = 1
	revoked, err := store.IsRevoked(ctx, accessToken)
	require.NoError(t, err)
	require.True(t, revoked)
	require.Equal(t, expectedKey, commands.existsKey)

	revoked, err = store.IsRevokedFingerprint(ctx, tokenRevocationFingerprint(accessToken))
	require.NoError(t, err)
	require.True(t, revoked)
	require.Equal(t, expectedKey, commands.existsKey)
}

func TestRedisTokenRevocationStoreRejectsExpiredToken(t *testing.T) {
	now := time.Date(2026, time.July, 25, 12, 0, 0, 0, time.UTC)
	commands := &fakeTokenRevocationCommands{}
	store := NewRedisTokenRevocationStore(commands)
	store.now = func() time.Time { return now }

	err := store.Revoke(context.Background(), "access-token", now)

	require.ErrorContains(t, err, "expired")
	require.Empty(t, commands.setKey)
}

type fakeTokenRevocationCommands struct {
	setKey      string
	setValue    interface{}
	setTTL      time.Duration
	setErr      error
	existsKey   string
	existsCount int64
	existsErr   error
}

func (c *fakeTokenRevocationCommands) Set(ctx context.Context, key string, value interface{}, expiration time.Duration) *redis.StatusCmd {
	c.setKey = key
	c.setValue = value
	c.setTTL = expiration
	cmd := redis.NewStatusCmd(ctx, "set", key)
	if c.setErr != nil {
		cmd.SetErr(c.setErr)
	} else {
		cmd.SetVal("OK")
	}
	return cmd
}

func (c *fakeTokenRevocationCommands) Exists(ctx context.Context, keys ...string) *redis.IntCmd {
	if len(keys) > 0 {
		c.existsKey = keys[0]
	}
	cmd := redis.NewIntCmd(ctx, "exists", keys)
	if c.existsErr != nil {
		cmd.SetErr(c.existsErr)
	} else {
		cmd.SetVal(c.existsCount)
	}
	return cmd
}

func TestRedisTokenRevocationStoreReturnsRedisErrors(t *testing.T) {
	commands := &fakeTokenRevocationCommands{existsErr: errors.New("redis unavailable")}
	store := NewRedisTokenRevocationStore(commands)

	_, err := store.IsRevoked(context.Background(), "access-token")

	require.ErrorContains(t, err, "redis unavailable")
}
