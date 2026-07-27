package credential

import (
	"context"
	"testing"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/require"
)

func TestStoreUsesSharedGatewayCredentialVersionKey(t *testing.T) {
	commands := &fakeRedisCommands{}
	store := NewStore(commands)

	require.NoError(t, store.SetCurrent(context.Background(), 42, "rotated-version"))

	require.Equal(t, "bbs:auth:credential-version:42", commands.setKey)
	require.Equal(t, "rotated-version", commands.setValue)
	require.Zero(t, commands.setTTL)
}

func TestStoreDeletesStaleCredentialVersion(t *testing.T) {
	commands := &fakeRedisCommands{}
	store := NewStore(commands)

	require.NoError(t, store.Delete(context.Background(), 42))

	require.Equal(t, "bbs:auth:credential-version:42", commands.deleteKey)
}

type fakeRedisCommands struct {
	setKey    string
	setValue  interface{}
	setTTL    time.Duration
	deleteKey string
}

func (f *fakeRedisCommands) Set(ctx context.Context, key string, value interface{}, ttl time.Duration) *redis.StatusCmd {
	f.setKey = key
	f.setValue = value
	f.setTTL = ttl
	command := redis.NewStatusCmd(ctx, "set", key, value, ttl)
	command.SetVal("OK")
	return command
}

func (f *fakeRedisCommands) Del(ctx context.Context, keys ...string) *redis.IntCmd {
	if len(keys) > 0 {
		f.deleteKey = keys[0]
	}
	command := redis.NewIntCmd(ctx, "del", keys)
	command.SetVal(1)
	return command
}
