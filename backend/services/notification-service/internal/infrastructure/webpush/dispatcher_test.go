package webpush

import (
	"context"
	"errors"
	"net"
	"reflect"
	"testing"
	"time"

	domain "notification-service/internal/domain/notification"
)

type fakeOutbox struct {
	delivery         domain.WebPushDelivery
	deliveries       []domain.WebPushDelivery
	claimed          bool
	complete         int
	expire           int
	retry            int
	lastExhaust      bool
	lastAttempt      int32
	released         []int64
	finalizeCanceled bool
	releaseCanceled  bool
	cleanupCount     int
	cleanupBefore    time.Time
	cleanupLimit     int
}

func (f *fakeOutbox) ClaimWebPushDeliveries(context.Context, int, time.Time, time.Duration) ([]domain.WebPushDelivery, error) {
	if len(f.deliveries) > 0 && !f.claimed {
		f.claimed = true
		return f.deliveries, nil
	}
	if f.delivery.ID == 0 {
		return nil, nil
	}
	delivery := f.delivery
	f.delivery.ID = 0
	return []domain.WebPushDelivery{delivery}, nil
}
func (f *fakeOutbox) ReleaseWebPushDeliveries(ctx context.Context, deliveryIDs []int64) error {
	f.releaseCanceled = ctx.Err() != nil
	f.released = append(f.released, deliveryIDs...)
	return nil
}
func (f *fakeOutbox) CompleteWebPushDelivery(ctx context.Context, _ int64, _ time.Time) error {
	f.finalizeCanceled = ctx.Err() != nil
	f.complete++
	return nil
}
func (f *fakeOutbox) ExpireWebPushSubscription(ctx context.Context, _ int64, _ int64, _ time.Time) error {
	f.finalizeCanceled = ctx.Err() != nil
	f.expire++
	return nil
}
func (f *fakeOutbox) RetryWebPushDelivery(ctx context.Context, _ int64, attempt int32, _ time.Time, _ string, exhausted bool) error {
	f.finalizeCanceled = ctx.Err() != nil
	f.retry++
	f.lastAttempt = attempt
	f.lastExhaust = exhausted
	return nil
}
func (f *fakeOutbox) CleanupCompletedWebPushDeliveries(_ context.Context, before time.Time, limit int) (int64, error) {
	f.cleanupCount++
	f.cleanupBefore = before
	f.cleanupLimit = limit
	return 0, nil
}

type cancelingSender struct {
	cancel context.CancelFunc
	status int
	err    error
}

func (s cancelingSender) Send(context.Context, domain.WebPushDelivery) (int, error) {
	s.cancel()
	return s.status, s.err
}

func TestDispatcherCancellationFinalizesCurrentAndReleasesRemaining(t *testing.T) {
	tests := []struct {
		name     string
		status   int
		err      error
		complete int
		retry    int
	}{
		{name: "complete", status: 201, complete: 1},
		{name: "retry", err: context.Canceled, retry: 1},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			outbox := &fakeOutbox{deliveries: []domain.WebPushDelivery{{ID: 7}, {ID: 8}, {ID: 9}}}
			ctx, cancel := context.WithCancel(context.Background())
			dispatcher := NewDispatcher(outbox, cancelingSender{cancel: cancel, status: tt.status, err: tt.err}, nil)

			dispatcher.dispatchBatch(ctx)

			if outbox.complete != tt.complete || outbox.retry != tt.retry || outbox.finalizeCanceled {
				t.Fatalf("current delivery complete=%d retry=%d contextCanceled=%v", outbox.complete, outbox.retry, outbox.finalizeCanceled)
			}
			if !reflect.DeepEqual(outbox.released, []int64{8, 9}) || outbox.releaseCanceled {
				t.Fatalf("released deliveries = %v, contextCanceled=%v", outbox.released, outbox.releaseCanceled)
			}
		})
	}
}

func TestDispatcherThrottlesCompletedDeliveryCleanup(t *testing.T) {
	now := time.Date(2026, 8, 10, 12, 0, 0, 0, time.UTC)
	outbox := &fakeOutbox{}
	dispatcher := NewDispatcher(outbox, fakeSender{}, nil)
	dispatcher.now = func() time.Time { return now }

	dispatcher.dispatchBatch(context.Background())
	dispatcher.dispatchBatch(context.Background())
	if outbox.cleanupCount != 1 || !outbox.cleanupBefore.Equal(now.Add(-defaultCompletedRetention)) || outbox.cleanupLimit != defaultCleanupBatchSize {
		t.Fatalf("cleanup count=%d before=%v limit=%d", outbox.cleanupCount, outbox.cleanupBefore, outbox.cleanupLimit)
	}

	now = now.Add(defaultCleanupInterval)
	dispatcher.dispatchBatch(context.Background())
	if outbox.cleanupCount != 2 {
		t.Fatalf("cleanup count after interval = %d", outbox.cleanupCount)
	}
}

type fakeSender struct {
	status int
	err    error
}

func (f fakeSender) Send(context.Context, domain.WebPushDelivery) (int, error) {
	return f.status, f.err
}

func TestDispatcherDeliveryOutcomes(t *testing.T) {
	tests := []struct {
		name      string
		status    int
		err       error
		attempt   int32
		complete  int
		expire    int
		retry     int
		exhausted bool
	}{
		{name: "success", status: 201, complete: 1},
		{name: "gone", status: 410, expire: 1},
		{name: "retry", status: 503, retry: 1},
		{name: "unsafe", err: ErrUnsafeEndpoint, expire: 1},
		{name: "exhausted", status: 503, attempt: 4, retry: 1, exhausted: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			outbox := &fakeOutbox{delivery: domain.WebPushDelivery{ID: 1, SubscriptionID: 2, AttemptCount: tt.attempt}}
			dispatcher := NewDispatcher(outbox, fakeSender{status: tt.status, err: tt.err}, nil)
			dispatcher.dispatchBatch(context.Background())
			if outbox.complete != tt.complete || outbox.expire != tt.expire || outbox.retry != tt.retry || outbox.lastExhaust != tt.exhausted {
				t.Fatalf("outcome complete=%d expire=%d retry=%d exhausted=%v", outbox.complete, outbox.expire, outbox.retry, outbox.lastExhaust)
			}
		})
	}
}

func TestValidateDispatchEndpointBlocksNonPublicIPs(t *testing.T) {
	for _, address := range []string{
		"127.0.0.1",
		"10.0.0.1",
		"100.64.0.1",
		"198.18.0.1",
		"192.0.2.1",
		"198.51.100.1",
		"203.0.113.1",
		"2001:db8::1",
		"3fff::1",
		"fc00::1",
		"fe80::1",
	} {
		resolver := fakeResolver{addresses: []string{address}}
		if err := validateDispatchEndpoint(context.Background(), "https://push.example/endpoint", resolver); !errors.Is(err, ErrUnsafeEndpoint) {
			t.Fatalf("non-public endpoint %s error = %v", address, err)
		}
	}
	resolver := fakeResolver{addresses: []string{"8.8.8.8", "2606:4700:4700::1111"}}
	if err := validateDispatchEndpoint(context.Background(), "https://push.example/endpoint", resolver); err != nil {
		t.Fatalf("public endpoint error = %v", err)
	}
}

type fakeResolver struct{ addresses []string }

func (r fakeResolver) LookupIPAddr(context.Context, string) ([]net.IPAddr, error) {
	addresses := make([]net.IPAddr, 0, len(r.addresses))
	for _, value := range r.addresses {
		addresses = append(addresses, net.IPAddr{IP: net.ParseIP(value)})
	}
	return addresses, nil
}
