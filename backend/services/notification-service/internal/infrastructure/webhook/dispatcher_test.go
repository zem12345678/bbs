package webhook

import (
	"context"
	"testing"
	"time"

	domain "notification-service/internal/domain/notification"
)

type dispatcherRepo struct {
	completed bool
	revoked   bool
	retried   struct {
		attempt   int32
		exhausted bool
		status    int32
	}
}

func (*dispatcherRepo) ClaimWebhookDeliveries(context.Context, int, time.Time, time.Duration) ([]domain.WebhookDelivery, error) {
	return nil, nil
}
func (*dispatcherRepo) ReleaseWebhookDeliveries(context.Context, []domain.WebhookDelivery) error {
	return nil
}
func (r *dispatcherRepo) IsWebhookDeliveryActive(context.Context, domain.WebhookDelivery) (bool, error) {
	return !r.revoked, nil
}
func (r *dispatcherRepo) CompleteWebhookDelivery(context.Context, domain.WebhookDelivery, int32, time.Time) error {
	r.completed = true
	return nil
}
func (r *dispatcherRepo) RetryWebhookDelivery(_ context.Context, _ domain.WebhookDelivery, status int32, attempt int32, _ time.Time, _ string, exhausted bool, _ time.Time) error {
	r.retried.status = status
	r.retried.attempt = attempt
	r.retried.exhausted = exhausted
	return nil
}
func (*dispatcherRepo) CleanupCompletedWebhookDeliveries(context.Context, time.Time, int) (int64, error) {
	return 0, nil
}

type dispatcherSender struct {
	status int
	err    error
	calls  *int
}

func (s dispatcherSender) Send(context.Context, domain.WebhookDelivery) (int, error) {
	if s.calls != nil {
		*s.calls++
	}
	return s.status, s.err
}

func TestDispatcherFinalizesSuccessAndRetriesFailures(t *testing.T) {
	t.Parallel()
	delivery := domain.WebhookDelivery{ID: 1, WebhookID: 2, AttemptCount: 1}
	successRepo := &dispatcherRepo{}
	success := NewDispatcher(successRepo, dispatcherSender{status: 204}, nil)
	success.dispatchOne(context.Background(), delivery)
	if !successRepo.completed || successRepo.retried.attempt != 0 {
		t.Fatalf("success finalization = %+v", successRepo)
	}

	retryRepo := &dispatcherRepo{}
	retry := NewDispatcher(retryRepo, dispatcherSender{status: 503}, nil)
	retry.dispatchOne(context.Background(), delivery)
	if retryRepo.retried.status != 503 || retryRepo.retried.attempt != 2 || retryRepo.retried.exhausted {
		t.Fatalf("retry finalization = %+v", retryRepo.retried)
	}

	failedRepo := &dispatcherRepo{}
	failed := NewDispatcher(failedRepo, dispatcherSender{status: 400}, nil)
	failed.dispatchOne(context.Background(), delivery)
	if !failedRepo.retried.exhausted || failedRepo.retried.status != 400 {
		t.Fatalf("non-retryable finalization = %+v", failedRepo.retried)
	}
}

func TestDispatcherTreatsUnsafeEndpointAsTerminal(t *testing.T) {
	t.Parallel()
	repo := &dispatcherRepo{}
	d := NewDispatcher(repo, dispatcherSender{err: ErrUnsafeEndpoint}, nil)
	d.dispatchOne(context.Background(), domain.WebhookDelivery{ID: 1, WebhookID: 2})
	if !repo.retried.exhausted {
		t.Fatal("unsafe endpoint was scheduled for retry")
	}
}

func TestDispatcherSkipsRevokedDelivery(t *testing.T) {
	t.Parallel()
	repo := &dispatcherRepo{revoked: true}
	calls := 0
	d := NewDispatcher(repo, dispatcherSender{status: 204, calls: &calls}, nil)
	d.dispatchOne(context.Background(), domain.WebhookDelivery{ID: 1, WebhookID: 2})
	if calls != 0 || repo.completed || repo.retried.attempt != 0 {
		t.Fatalf("revoked delivery was sent or finalized: calls=%d repo=%+v", calls, repo)
	}
}
