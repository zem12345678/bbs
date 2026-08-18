package http

import (
	"context"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/require"
)

func TestRedisClipExportGateLocksExecutionAndCountsOnlyCommittedExports(t *testing.T) {
	server := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: server.Addr()})
	t.Cleanup(func() { _ = client.Close() })
	gate := NewRedisClipExportGate(client, 24*time.Hour, 15*time.Minute)

	permit, err := gate.Begin(context.Background(), 42)
	require.NoError(t, err)
	require.NotNil(t, permit)
	_, err = gate.Begin(context.Background(), 42)
	require.ErrorIs(t, err, errClipExportInProgress)

	require.NoError(t, permit.Release(context.Background()))
	permit, err = gate.Begin(context.Background(), 42)
	require.NoError(t, err)
	require.NoError(t, permit.Commit(context.Background()))
	require.NoError(t, permit.Commit(context.Background()))
	_, err = gate.Begin(context.Background(), 42)
	require.ErrorIs(t, err, errClipExportRateLimited)

	server.FastForward(24*time.Hour + time.Millisecond)
	permit, err = gate.Begin(context.Background(), 42)
	require.NoError(t, err)
	require.NoError(t, permit.Release(context.Background()))
}

func TestRedisClipExportGateRejectsInvalidConfiguration(t *testing.T) {
	gate := NewRedisClipExportGate(nil, time.Hour, time.Minute)
	_, err := gate.Begin(context.Background(), 42)
	require.EqualError(t, err, "export gate unavailable")
}

func TestRedisExportGatesUseIndependentEntityKeys(t *testing.T) {
	server := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: server.Addr()})
	t.Cleanup(func() { _ = client.Close() })
	gates := []ExportGate{
		NewRedisAccountDataExportGate(client, 72*time.Hour, 15*time.Minute),
		NewRedisAntennaExportGate(client, time.Hour, 15*time.Minute),
		NewRedisBlockingExportGate(client, time.Hour, 15*time.Minute),
		NewRedisClipExportGate(client, 24*time.Hour, 15*time.Minute),
		NewRedisFavoriteExportGate(client, 24*time.Hour, 15*time.Minute),
		NewRedisFollowingExportGate(client, time.Hour, 15*time.Minute),
		NewRedisMuteExportGate(client, time.Hour, 15*time.Minute),
		NewRedisNoteExportGate(client, 24*time.Hour, 15*time.Minute),
		NewRedisUserListExportGate(client, time.Minute, 15*time.Minute),
	}

	for _, gate := range gates {
		permit, err := gate.Begin(context.Background(), 42)
		require.NoError(t, err)
		require.NoError(t, permit.Commit(context.Background()))
	}
	for _, gate := range gates {
		_, err := gate.Begin(context.Background(), 42)
		require.ErrorIs(t, err, errExportRateLimited)
	}

	server.FastForward(time.Hour + time.Millisecond)
	for _, gate := range []ExportGate{gates[1], gates[2], gates[5], gates[6], gates[8]} {
		permit, err := gate.Begin(context.Background(), 42)
		require.NoError(t, err)
		require.NoError(t, permit.Release(context.Background()))
	}
	_, err := gates[0].Begin(context.Background(), 42)
	require.ErrorIs(t, err, errExportRateLimited)
	_, err = gates[3].Begin(context.Background(), 42)
	require.ErrorIs(t, err, errExportRateLimited)
	_, err = gates[4].Begin(context.Background(), 42)
	require.ErrorIs(t, err, errExportRateLimited)
	_, err = gates[7].Begin(context.Background(), 42)
	require.ErrorIs(t, err, errExportRateLimited)
}
