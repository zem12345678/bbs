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
	clipGate := NewRedisClipExportGate(client, 24*time.Hour, 15*time.Minute)
	antennaGate := NewRedisAntennaExportGate(client, time.Hour, 15*time.Minute)

	clipPermit, err := clipGate.Begin(context.Background(), 42)
	require.NoError(t, err)
	require.NoError(t, clipPermit.Commit(context.Background()))

	antennaPermit, err := antennaGate.Begin(context.Background(), 42)
	require.NoError(t, err)
	require.NoError(t, antennaPermit.Commit(context.Background()))
	_, err = antennaGate.Begin(context.Background(), 42)
	require.ErrorIs(t, err, errExportRateLimited)
	_, err = clipGate.Begin(context.Background(), 42)
	require.ErrorIs(t, err, errExportRateLimited)

	server.FastForward(time.Hour + time.Millisecond)
	antennaPermit, err = antennaGate.Begin(context.Background(), 42)
	require.NoError(t, err)
	require.NoError(t, antennaPermit.Release(context.Background()))
	_, err = clipGate.Begin(context.Background(), 42)
	require.ErrorIs(t, err, errExportRateLimited)
}
