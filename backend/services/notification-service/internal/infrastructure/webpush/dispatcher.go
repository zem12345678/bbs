package webpush

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	domain "notification-service/internal/domain/notification"
	"notification-service/pkg/logger"
)

const (
	defaultPollInterval       = time.Second
	defaultLockTimeout        = time.Minute
	defaultInitialBackoff     = 2 * time.Second
	defaultMaxBackoff         = 5 * time.Minute
	defaultCleanupInterval    = time.Hour
	defaultCompletedRetention = 7 * 24 * time.Hour
	defaultFinalizeTimeout    = 5 * time.Second
	defaultBatchSize          = 50
	defaultCleanupBatchSize   = 1000
	defaultMaxAttempts        = 5
	maxStoredErrorBytes       = 1000
)

type deliverySender interface {
	Send(context.Context, domain.WebPushDelivery) (int, error)
}

type Dispatcher struct {
	repo             domain.WebPushOutboxRepository
	sender           deliverySender
	log              logger.Logger
	pollInterval     time.Duration
	lockTimeout      time.Duration
	initialBackoff   time.Duration
	maxBackoff       time.Duration
	cleanupInterval  time.Duration
	retention        time.Duration
	batchSize        int
	cleanupBatchSize int
	maxAttempts      int32
	now              func() time.Time
	lastCleanup      time.Time

	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup
}

func NewDispatcher(repo domain.WebPushOutboxRepository, sender deliverySender, log logger.Logger) *Dispatcher {
	return &Dispatcher{
		repo:             repo,
		sender:           sender,
		log:              log,
		pollInterval:     defaultPollInterval,
		lockTimeout:      defaultLockTimeout,
		initialBackoff:   defaultInitialBackoff,
		maxBackoff:       defaultMaxBackoff,
		cleanupInterval:  defaultCleanupInterval,
		retention:        defaultCompletedRetention,
		batchSize:        defaultBatchSize,
		cleanupBatchSize: defaultCleanupBatchSize,
		maxAttempts:      defaultMaxAttempts,
		now:              time.Now,
	}
}

func (d *Dispatcher) Start() error {
	if d.repo == nil || d.sender == nil {
		return errors.New("web push dispatcher is not configured")
	}
	if d.cancel != nil {
		return nil
	}
	d.ctx, d.cancel = context.WithCancel(context.Background())
	d.wg.Add(1)
	go d.run()
	return nil
}

func (d *Dispatcher) Stop() error {
	if d.cancel == nil {
		return nil
	}
	d.cancel()
	d.wg.Wait()
	d.cancel = nil
	return nil
}

func (d *Dispatcher) run() {
	defer d.wg.Done()
	ticker := time.NewTicker(d.pollInterval)
	defer ticker.Stop()
	for {
		d.dispatchBatch(d.ctx)
		select {
		case <-d.ctx.Done():
			return
		case <-ticker.C:
		}
	}
}

func (d *Dispatcher) dispatchBatch(ctx context.Context) {
	now := d.now()
	d.cleanupCompleted(ctx, now)
	deliveries, err := d.repo.ClaimWebPushDeliveries(ctx, d.batchSize, now, d.lockTimeout)
	if err != nil {
		if ctx.Err() == nil {
			d.logError("claim web push deliveries", err)
		}
		return
	}
	for index, delivery := range deliveries {
		if ctx.Err() != nil {
			d.releaseDeliveries(deliveries[index:])
			return
		}
		d.dispatchOne(ctx, delivery)
	}
}

func (d *Dispatcher) dispatchOne(ctx context.Context, delivery domain.WebPushDelivery) {
	statusCode, sendErr := d.sender.Send(ctx, delivery)
	now := d.now()
	finalizeCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), defaultFinalizeTimeout)
	defer cancel()
	if sendErr == nil && statusCode >= 200 && statusCode < 300 {
		if err := d.repo.CompleteWebPushDelivery(finalizeCtx, delivery.ID, now); err != nil {
			d.logError("complete web push delivery", err)
		}
		return
	}
	if errors.Is(sendErr, ErrUnsafeEndpoint) || statusCode == 404 || statusCode == 410 {
		if err := d.repo.ExpireWebPushSubscription(finalizeCtx, delivery.ID, delivery.SubscriptionID, now); err != nil {
			d.logError("expire web push subscription", err)
		}
		return
	}

	attemptCount := delivery.AttemptCount + 1
	exhausted := attemptCount >= d.maxAttempts
	message := webPushFailureMessage(statusCode, sendErr)
	nextAttempt := now.Add(d.retryDelay(attemptCount))
	if err := d.repo.RetryWebPushDelivery(finalizeCtx, delivery.ID, attemptCount, nextAttempt, message, exhausted); err != nil {
		d.logError("retry web push delivery", err)
	}
}

func (d *Dispatcher) releaseDeliveries(deliveries []domain.WebPushDelivery) {
	ids := make([]int64, 0, len(deliveries))
	for _, delivery := range deliveries {
		ids = append(ids, delivery.ID)
	}
	ctx, cancel := context.WithTimeout(context.Background(), defaultFinalizeTimeout)
	defer cancel()
	if err := d.repo.ReleaseWebPushDeliveries(ctx, ids); err != nil {
		d.logError("release web push delivery", err)
	}
}

func (d *Dispatcher) cleanupCompleted(ctx context.Context, now time.Time) {
	if !d.lastCleanup.IsZero() && now.Sub(d.lastCleanup) < d.cleanupInterval {
		return
	}
	d.lastCleanup = now
	cleanupCtx, cancel := context.WithTimeout(ctx, defaultFinalizeTimeout)
	defer cancel()
	if _, err := d.repo.CleanupCompletedWebPushDeliveries(cleanupCtx, now.Add(-d.retention), d.cleanupBatchSize); err != nil && ctx.Err() == nil {
		d.logError("cleanup completed web push deliveries", err)
	}
}

func (d *Dispatcher) logError(message string, err error) {
	if d.log != nil {
		d.log.Error(message, logger.Error(err))
	}
}

func (d *Dispatcher) retryDelay(attemptCount int32) time.Duration {
	delay := d.initialBackoff
	for attempt := int32(1); attempt < attemptCount && delay < d.maxBackoff; attempt++ {
		delay *= 2
		if delay > d.maxBackoff {
			return d.maxBackoff
		}
	}
	return delay
}

func webPushFailureMessage(statusCode int, err error) string {
	message := fmt.Sprintf("web push returned HTTP %d", statusCode)
	if err != nil {
		message = err.Error()
	}
	message = strings.TrimSpace(message)
	if len(message) > maxStoredErrorBytes {
		message = message[:maxStoredErrorBytes]
	}
	return message
}
