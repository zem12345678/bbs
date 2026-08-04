package storage

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
)

const (
	objectCleanupQueueKey     = "bbs:file-object-cleanup:v1"
	objectCleanupBatchSize    = int64(100)
	objectCleanupAttempts     = 3
	objectCleanupPollInterval = 10 * time.Second
	objectCleanupRetryDelay   = 30 * time.Second
	objectCleanupStartTimeout = 3 * time.Second
)

// ObjectCleanupQueue durably records deterministic upload compensation before
// deleting the object. Multiple gateway instances may process the same member;
// object deletion and queue removal are both idempotent.
type ObjectCleanupQueue struct {
	store ObjectStore
	redis redis.Cmdable
	log   *zap.Logger

	now          func() time.Time
	pollInterval time.Duration
	retryDelay   time.Duration

	mu     sync.Mutex
	cancel context.CancelFunc
	done   chan struct{}
}

func NewObjectCleanupQueue(store ObjectStore, redisClient redis.Cmdable, logger *zap.Logger) *ObjectCleanupQueue {
	if logger == nil {
		logger = zap.NewNop()
	}
	return &ObjectCleanupQueue{
		store:        store,
		redis:        redisClient,
		log:          logger,
		now:          time.Now,
		pollInterval: objectCleanupPollInterval,
		retryDelay:   objectCleanupRetryDelay,
	}
}

// Delete records the cleanup intent before touching object storage, then makes
// a few immediate attempts. A failed deletion remains queued for reconciliation.
func (q *ObjectCleanupQueue) Delete(ctx context.Context, key string) error {
	if err := q.validate(key); err != nil {
		return err
	}
	if err := q.schedule(ctx, key, q.now()); err != nil {
		q.log.Error("persist uploaded object cleanup intent", zap.String("object_key", key), zap.Error(err))
		return err
	}
	deleteErr := q.deleteObject(ctx, key)
	if deleteErr == nil {
		if err := q.redis.ZRem(ctx, objectCleanupQueueKey, key).Err(); err != nil {
			q.log.Warn("remove completed object cleanup task", zap.String("object_key", key), zap.Error(err))
		}
		return nil
	}
	rescheduleErr := q.schedule(ctx, key, q.now().Add(q.retryDelay))
	q.log.Warn("defer uploaded object cleanup", zap.String("object_key", key), zap.Error(errors.Join(deleteErr, rescheduleErr)))
	return errors.Join(deleteErr, rescheduleErr)
}

func (q *ObjectCleanupQueue) Start() error {
	if err := q.validate("startup-probe"); err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(context.Background(), objectCleanupStartTimeout)
	defer cancel()
	if err := q.redis.Ping(ctx).Err(); err != nil {
		return fmt.Errorf("verify object cleanup queue: %w", err)
	}
	q.mu.Lock()
	defer q.mu.Unlock()
	if q.cancel != nil {
		return nil
	}
	runCtx, runCancel := context.WithCancel(context.Background())
	q.cancel = runCancel
	q.done = make(chan struct{})
	go q.run(runCtx, q.done)
	return nil
}

func (q *ObjectCleanupQueue) Stop() error {
	q.mu.Lock()
	cancel := q.cancel
	done := q.done
	q.cancel = nil
	q.done = nil
	q.mu.Unlock()
	if cancel == nil {
		return nil
	}
	cancel()
	<-done
	return nil
}

func (q *ObjectCleanupQueue) run(ctx context.Context, done chan<- struct{}) {
	defer close(done)
	q.reconcile(ctx)
	ticker := time.NewTicker(q.pollInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			q.reconcile(ctx)
		}
	}
}

func (q *ObjectCleanupQueue) reconcile(ctx context.Context) {
	if err := q.processDue(ctx); err != nil && !errors.Is(err, context.Canceled) {
		q.log.Warn("reconcile uploaded object cleanup", zap.Error(err))
	}
}

func (q *ObjectCleanupQueue) processDue(ctx context.Context) error {
	if err := q.validate("reconcile-probe"); err != nil {
		return err
	}
	keys, err := q.redis.ZRangeByScore(ctx, objectCleanupQueueKey, &redis.ZRangeBy{
		Min:   "-inf",
		Max:   strconv.FormatInt(q.now().UnixMilli(), 10),
		Count: objectCleanupBatchSize,
	}).Result()
	if err != nil {
		return fmt.Errorf("list object cleanup tasks: %w", err)
	}
	var reconcileErr error
	for _, key := range keys {
		if err := q.deleteObject(ctx, key); err != nil {
			reconcileErr = errors.Join(reconcileErr, err, q.schedule(ctx, key, q.now().Add(q.retryDelay)))
			continue
		}
		if err := q.redis.ZRem(ctx, objectCleanupQueueKey, key).Err(); err != nil {
			reconcileErr = errors.Join(reconcileErr, fmt.Errorf("remove object cleanup task %q: %w", key, err))
		}
	}
	return reconcileErr
}

func (q *ObjectCleanupQueue) schedule(ctx context.Context, key string, when time.Time) error {
	if err := q.redis.ZAdd(ctx, objectCleanupQueueKey, redis.Z{
		Score:  float64(when.UnixMilli()),
		Member: key,
	}).Err(); err != nil {
		return fmt.Errorf("schedule object cleanup %q: %w", key, err)
	}
	return nil
}

func (q *ObjectCleanupQueue) deleteObject(ctx context.Context, key string) error {
	var lastErr error
	for attempt := 0; attempt < objectCleanupAttempts; attempt++ {
		lastErr = q.store.Delete(ctx, key)
		if lastErr == nil || errors.Is(lastErr, ErrObjectNotFound) {
			return nil
		}
		if ctx.Err() != nil {
			break
		}
	}
	return fmt.Errorf("delete uploaded object %q: %w", key, lastErr)
}

func (q *ObjectCleanupQueue) validate(key string) error {
	if q == nil || q.store == nil || q.redis == nil {
		return errors.New("object cleanup dependencies are required")
	}
	if strings.TrimSpace(key) == "" {
		return errors.New("object cleanup key is required")
	}
	return nil
}
