package storage

import (
	"context"
	"errors"
	"io"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/require"
)

func TestObjectCleanupQueueRecordsIntentBeforeDeleting(t *testing.T) {
	server := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: server.Addr()})
	t.Cleanup(func() { require.NoError(t, client.Close()) })
	store := &cleanupObjectStore{}
	store.delete = func(ctx context.Context, key string) error {
		score, err := client.ZScore(ctx, objectCleanupQueueKey, key).Result()
		require.NoError(t, err)
		require.NotZero(t, score)
		return nil
	}
	queue := NewObjectCleanupQueue(store, client, nil)

	require.NoError(t, queue.Delete(context.Background(), "files/7/test.bin"))
	require.Equal(t, int64(0), client.ZCard(context.Background(), objectCleanupQueueKey).Val())
}

func TestObjectCleanupQueueRetriesPersistedFailure(t *testing.T) {
	server := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: server.Addr()})
	t.Cleanup(func() { require.NoError(t, client.Close()) })
	now := time.Unix(1_700_000_000, 0)
	store := &cleanupObjectStore{err: errors.New("storage unavailable")}
	queue := NewObjectCleanupQueue(store, client, nil)
	queue.now = func() time.Time { return now }

	err := queue.Delete(context.Background(), "files/7/retry.bin")
	require.ErrorContains(t, err, "storage unavailable")
	require.Equal(t, objectCleanupAttempts, store.deleteCalls())
	require.Equal(t, float64(now.Add(objectCleanupRetryDelay).UnixMilli()), client.ZScore(context.Background(), objectCleanupQueueKey, "files/7/retry.bin").Val())

	store.setError(nil)
	now = now.Add(objectCleanupRetryDelay)
	require.NoError(t, queue.processDue(context.Background()))
	require.Equal(t, objectCleanupAttempts+1, store.deleteCalls())
	require.Equal(t, int64(0), client.ZCard(context.Background(), objectCleanupQueueKey).Val())
}

func TestObjectCleanupQueueTreatsMissingObjectAsDeleted(t *testing.T) {
	server := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: server.Addr()})
	t.Cleanup(func() { require.NoError(t, client.Close()) })
	store := &cleanupObjectStore{err: ErrObjectNotFound}
	queue := NewObjectCleanupQueue(store, client, nil)

	require.NoError(t, queue.Delete(context.Background(), "files/7/missing.bin"))
	require.Equal(t, 1, store.deleteCalls())
	require.Equal(t, int64(0), client.ZCard(context.Background(), objectCleanupQueueKey).Val())
}

func TestObjectCleanupQueueDoesNotDeleteBeforeIntentIsPersisted(t *testing.T) {
	server := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: server.Addr()})
	t.Cleanup(func() { require.NoError(t, client.Close()) })
	server.Close()
	store := &cleanupObjectStore{}
	queue := NewObjectCleanupQueue(store, client, nil)

	err := queue.Delete(context.Background(), "files/7/not-queued.bin")

	require.ErrorContains(t, err, "schedule object cleanup")
	require.Equal(t, 0, store.deleteCalls())
}

func TestObjectCleanupQueueStartRequiresRedis(t *testing.T) {
	server := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: server.Addr()})
	t.Cleanup(func() { require.NoError(t, client.Close()) })
	server.Close()
	queue := NewObjectCleanupQueue(&cleanupObjectStore{}, client, nil)

	require.ErrorContains(t, queue.Start(), "verify object cleanup queue")
}

type cleanupObjectStore struct {
	mu     sync.Mutex
	err    error
	calls  int
	delete func(context.Context, string) error
}

func (s *cleanupObjectStore) Upload(context.Context, string, io.Reader, int64, string) error {
	return errors.New("unexpected upload")
}

func (s *cleanupObjectStore) Open(context.Context, string) (io.ReadCloser, ObjectInfo, error) {
	return io.NopCloser(strings.NewReader("")), ObjectInfo{}, errors.New("unexpected open")
}

func (s *cleanupObjectStore) Delete(ctx context.Context, key string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.calls++
	if s.delete != nil {
		return s.delete(ctx, key)
	}
	return s.err
}

func (s *cleanupObjectStore) setError(err error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.err = err
}

func (s *cleanupObjectStore) deleteCalls() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.calls
}
