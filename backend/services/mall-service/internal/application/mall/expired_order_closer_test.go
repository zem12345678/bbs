package mall

import (
	"context"
	"errors"
	"testing"
	"time"

	domain "mall-service/internal/domain/mall"
	"mall-service/pkg/logger"

	"go.uber.org/zap"
)

func TestExpiredOrderCloserCloseOnceUsesConfiguredExpirationAndLimit(t *testing.T) {
	now := time.Date(2026, 7, 12, 8, 0, 0, 0, time.UTC)
	repo := &expiredOrderRepoStub{
		orders: []domain.Order{
			{ID: 1001, Status: domain.OrderStatusClosed},
			{ID: 1002, Status: domain.OrderStatusClosed},
		},
	}
	svc := NewService(repo, nil, 30*time.Minute)
	svc.SetClockForTest(func() time.Time { return now })
	closer := NewExpiredOrderCloser(svc, ExpiredOrderCloserOptions{
		ExpireAfter: 10 * time.Minute,
		Interval:    time.Hour,
		Limit:       2,
	})

	closed, err := closer.CloseOnce(context.Background())
	if err != nil {
		t.Fatalf("CloseOnce() error = %v", err)
	}
	if closed != 2 {
		t.Fatalf("CloseOnce() closed = %d, want 2", closed)
	}
	if repo.calls != 1 {
		t.Fatalf("CloseExpiredOrders() calls = %d, want 1", repo.calls)
	}
	if !repo.expireBefore.Equal(now.Add(-10 * time.Minute)) {
		t.Fatalf("expireBefore = %s, want %s", repo.expireBefore, now.Add(-10*time.Minute))
	}
	if repo.limit != 2 {
		t.Fatalf("limit = %d, want 2", repo.limit)
	}
	if !repo.closedAt.Equal(now) {
		t.Fatalf("closedAt = %s, want %s", repo.closedAt, now)
	}
}

func TestExpiredOrderCloserRunLogsCloseErrors(t *testing.T) {
	closeErr := errors.New("database unavailable")
	repo := &expiredOrderRepoStub{err: closeErr}
	svc := NewService(repo, nil, time.Minute)
	log := &expiredOrderTestLogger{}
	ctx, cancel := context.WithCancel(context.Background())
	repo.afterCall = cancel
	closer := NewExpiredOrderCloser(svc, ExpiredOrderCloserOptions{
		ExpireAfter: time.Minute,
		Interval:    time.Hour,
		Limit:       1,
		Log:         log,
	})

	closer.run(ctx)

	if repo.calls != 1 {
		t.Fatalf("CloseExpiredOrders() calls = %d, want 1", repo.calls)
	}
	if len(log.errors) != 1 {
		t.Fatalf("logged errors = %d, want 1", len(log.errors))
	}
	if log.errors[0] != "close expired mall orders failed" {
		t.Fatalf("logged error message = %q", log.errors[0])
	}
	if !fieldValueEqual(log.errorFields[0], "error", closeErr) {
		t.Fatalf("logged fields = %+v, want error field", log.errorFields[0])
	}
}

func TestExpiredOrderCloserRecoverPayingOnceUsesConfiguredThresholdAndLimit(t *testing.T) {
	now := time.Date(2026, 7, 12, 8, 0, 0, 0, time.UTC)
	repo := &expiredOrderRepoStub{}
	svc := NewService(repo, nil, 30*time.Minute)
	svc.SetClockForTest(func() time.Time { return now })
	closer := NewExpiredOrderCloser(svc, ExpiredOrderCloserOptions{
		ExpireAfter:        30 * time.Minute,
		RecoverPayingAfter: 5 * time.Minute,
		Interval:           time.Hour,
		Limit:              3,
	})

	result, err := closer.RecoverPayingOnce(context.Background())
	if err != nil {
		t.Fatalf("RecoverPayingOnce() error = %v", err)
	}
	if result.Recovered != 0 || result.Failed != 0 {
		t.Fatalf("RecoverPayingOnce() result = %+v, want zero", result)
	}
	if repo.recoverCalls != 1 {
		t.Fatalf("ListStalePayingOrders() calls = %d, want 1", repo.recoverCalls)
	}
	if !repo.startedBefore.Equal(now.Add(-5 * time.Minute)) {
		t.Fatalf("startedBefore = %s, want %s", repo.startedBefore, now.Add(-5*time.Minute))
	}
	if repo.recoverLimit != 3 {
		t.Fatalf("recover limit = %d, want 3", repo.recoverLimit)
	}
}

type expiredOrderRepoStub struct {
	domain.Repository

	orders        []domain.Order
	err           error
	calls         int
	expireBefore  time.Time
	limit         int
	closedAt      time.Time
	afterCall     func()
	recoverCalls  int
	startedBefore time.Time
	recoverLimit  int
}

func (r *expiredOrderRepoStub) CloseExpiredOrders(_ context.Context, expireBefore time.Time, limit int, closedAt time.Time) ([]domain.Order, error) {
	r.calls++
	r.expireBefore = expireBefore
	r.limit = limit
	r.closedAt = closedAt
	if r.afterCall != nil {
		r.afterCall()
	}
	return r.orders, r.err
}

func (r *expiredOrderRepoStub) ListStalePayingOrders(_ context.Context, startedBefore time.Time, limit int) ([]domain.PayingOrderPayment, error) {
	r.recoverCalls++
	r.startedBefore = startedBefore
	r.recoverLimit = limit
	return nil, nil
}

type expiredOrderTestLogger struct {
	errors      []string
	errorFields [][]logger.Field
}

func (l *expiredOrderTestLogger) Debug(string, ...logger.Field) {}

func (l *expiredOrderTestLogger) Info(string, ...logger.Field) {}

func (l *expiredOrderTestLogger) Warn(string, ...logger.Field) {}

func (l *expiredOrderTestLogger) Error(msg string, fields ...logger.Field) {
	l.errors = append(l.errors, msg)
	l.errorFields = append(l.errorFields, fields)
}

func (l *expiredOrderTestLogger) With(...logger.Field) logger.Logger {
	return l
}

func (l *expiredOrderTestLogger) GetZapLogger() *zap.Logger {
	return zap.NewNop()
}

func fieldValueEqual(fields []logger.Field, key string, value any) bool {
	for _, field := range fields {
		if field.Key == key && field.Value == value {
			return true
		}
	}
	return false
}
