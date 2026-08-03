package deletion

import (
	"context"
	"errors"
	"reflect"
	"testing"
	"time"

	domain "user-service/internal/domain/user"
)

func TestWorkerCompletesEveryPendingStepAndFinalizesAccount(t *testing.T) {
	now := time.Date(2026, 8, 3, 12, 0, 0, 0, time.UTC)
	repo := &workerRepositoryStub{claim: &domain.AccountDeletionClaim{
		Job: domain.AccountDeletionJob{ID: 91, UserID: 42, PolicyVersion: 3},
		Steps: []domain.AccountDeletionStep{
			{Service: "content-service", Status: domain.AccountJobPending},
			{Service: "comment-service", Status: domain.AccountJobSucceeded},
			{Service: "feed-service", Status: domain.AccountJobRetryWait},
		},
		LeaseOwner: "worker-a",
	}}
	var erased []string
	erasers := map[string]AccountDataEraser{
		"content-service": eraserFunc(func(_ context.Context, userID, jobID int64, policyVersion int32) error {
			erased = append(erased, "content-service")
			assertErasureIdentity(t, userID, jobID, policyVersion)
			return nil
		}),
		"feed-service": eraserFunc(func(_ context.Context, userID, jobID int64, policyVersion int32) error {
			erased = append(erased, "feed-service")
			assertErasureIdentity(t, userID, jobID, policyVersion)
			return nil
		}),
	}
	publisher := &workerPublisherStub{}
	cache := &workerCredentialCacheStub{}
	worker, err := NewWorker(repo, erasers, publisher, cache, nil, Options{WorkerID: "worker-a", RetryBase: time.Second})
	if err != nil {
		t.Fatalf("new worker: %v", err)
	}
	worker.now = func() time.Time { return now }
	worker.credential = func() (string, error) { return "final-credential", nil }

	processed, err := worker.RunOnce(context.Background())
	if err != nil {
		t.Fatalf("run once: %v", err)
	}
	if !processed {
		t.Fatal("run once processed=false, want true")
	}
	if !reflect.DeepEqual(erased, []string{"content-service", "feed-service"}) {
		t.Fatalf("erased services=%v", erased)
	}
	if !reflect.DeepEqual(repo.begun, erased) || !reflect.DeepEqual(repo.completed, erased) {
		t.Fatalf("begun=%v completed=%v erased=%v", repo.begun, repo.completed, erased)
	}
	if repo.finalization.Username != "__erased_2a" || repo.finalization.Email != "erased+2a+5b@invalid.local" || repo.finalization.CredentialVersion != "final-credential" {
		t.Fatalf("finalization=%+v", repo.finalization)
	}
	if cache.userID != 42 || cache.version != "final-credential" {
		t.Fatalf("credential cache user=%d version=%q", cache.userID, cache.version)
	}
	if len(publisher.events) != 1 || publisher.events[0].EventName() != "user.deleted" || publisher.events[0].AggregateID() != 42 {
		t.Fatalf("published events=%+v", publisher.events)
	}
}

func TestWorkerPersistsRetryAndStopsAfterFirstFailedStep(t *testing.T) {
	now := time.Date(2026, 8, 3, 13, 0, 0, 0, time.UTC)
	repo := &workerRepositoryStub{claim: &domain.AccountDeletionClaim{
		Job: domain.AccountDeletionJob{ID: 92, UserID: 43, PolicyVersion: 1},
		Steps: []domain.AccountDeletionStep{
			{Service: "content-service", Status: domain.AccountJobPending},
			{Service: "feed-service", Status: domain.AccountJobPending},
		},
		LeaseOwner: "worker-b",
	}, beginAttempts: 3}
	wantErr := errors.New("content erasure unavailable")
	worker, err := NewWorker(repo, map[string]AccountDataEraser{
		"content-service": eraserFunc(func(context.Context, int64, int64, int32) error { return wantErr }),
		"feed-service":    eraserFunc(func(context.Context, int64, int64, int32) error { t.Fatal("later step must not run"); return nil }),
	}, nil, nil, nil, Options{WorkerID: "worker-b", RetryBase: 2 * time.Second, MaxAttempts: 6})
	if err != nil {
		t.Fatalf("new worker: %v", err)
	}
	worker.now = func() time.Time { return now }

	processed, err := worker.RunOnce(context.Background())
	if !processed || !errors.Is(err, wantErr) {
		t.Fatalf("processed=%v error=%v", processed, err)
	}
	if repo.retryService != "content-service" || repo.retryError != wantErr.Error() || repo.retryMaxAttempts != 6 {
		t.Fatalf("retry service=%q error=%q max=%d", repo.retryService, repo.retryError, repo.retryMaxAttempts)
	}
	if wantRetryAt := now.Add(8 * time.Second); !repo.retryAt.Equal(wantRetryAt) {
		t.Fatalf("retry at=%v, want %v", repo.retryAt, wantRetryAt)
	}
	if repo.finalized {
		t.Fatal("failed job was finalized")
	}
}

func TestWorkerTreatsMissingEraserAsRetryableFailure(t *testing.T) {
	now := time.Date(2026, 8, 3, 14, 0, 0, 0, time.UTC)
	repo := &workerRepositoryStub{claim: &domain.AccountDeletionClaim{
		Job:        domain.AccountDeletionJob{ID: 93, UserID: 44},
		Steps:      []domain.AccountDeletionStep{{Service: "search-service", Status: domain.AccountJobPending}},
		LeaseOwner: "worker-c",
	}}
	worker, err := NewWorker(repo, nil, nil, nil, nil, Options{WorkerID: "worker-c"})
	if err != nil {
		t.Fatalf("new worker: %v", err)
	}
	worker.now = func() time.Time { return now }

	processed, err := worker.RunOnce(context.Background())
	if !processed || err == nil {
		t.Fatalf("processed=%v error=%v", processed, err)
	}
	if repo.retryService != "search-service" || repo.retryError == "" {
		t.Fatalf("retry service=%q error=%q", repo.retryService, repo.retryError)
	}
}

func TestWorkerReturnsIdleWhenNoJobIsClaimable(t *testing.T) {
	repo := &workerRepositoryStub{}
	worker, err := NewWorker(repo, nil, nil, nil, nil, Options{WorkerID: "worker-idle"})
	if err != nil {
		t.Fatalf("new worker: %v", err)
	}
	processed, err := worker.RunOnce(context.Background())
	if err != nil || processed {
		t.Fatalf("processed=%v error=%v", processed, err)
	}
}

func assertErasureIdentity(t *testing.T, userID, jobID int64, policyVersion int32) {
	t.Helper()
	if userID != 42 || jobID != 91 || policyVersion != 3 {
		t.Fatalf("erase identity user=%d job=%d policy=%d", userID, jobID, policyVersion)
	}
}

type eraserFunc func(context.Context, int64, int64, int32) error

func (f eraserFunc) EraseUserData(ctx context.Context, userID, jobID int64, policyVersion int32) error {
	return f(ctx, userID, jobID, policyVersion)
}

type workerRepositoryStub struct {
	claim            *domain.AccountDeletionClaim
	beginAttempts    int16
	begun            []string
	completed        []string
	retryService     string
	retryError       string
	retryAt          time.Time
	retryMaxAttempts int16
	finalization     domain.AccountAnonymization
	finalized        bool
}

func (r *workerRepositoryStub) ClaimAccountDeletionJob(context.Context, string, time.Time, time.Time) (*domain.AccountDeletionClaim, error) {
	claim := r.claim
	r.claim = nil
	return claim, nil
}

func (r *workerRepositoryStub) BeginAccountDeletionStep(_ context.Context, _ int64, service, _ string, _, _ time.Time) (int16, error) {
	r.begun = append(r.begun, service)
	if r.beginAttempts > 0 {
		return r.beginAttempts, nil
	}
	return 1, nil
}

func (r *workerRepositoryStub) CompleteAccountDeletionStep(_ context.Context, _ int64, service, _ string, _ time.Time) error {
	r.completed = append(r.completed, service)
	return nil
}

func (r *workerRepositoryStub) RetryAccountDeletionStep(_ context.Context, _ int64, service, _, lastError string, _ time.Time, retryAt time.Time, maxAttempts int16) error {
	r.retryService = service
	r.retryError = lastError
	r.retryAt = retryAt
	r.retryMaxAttempts = maxAttempts
	return nil
}

func (r *workerRepositoryStub) FinalizeAccountDeletionJob(_ context.Context, _ int64, _ string, anonymization domain.AccountAnonymization) (*domain.User, error) {
	r.finalization = anonymization
	r.finalized = true
	return &domain.User{ID: 42, Username: anonymization.Username, Email: anonymization.Email, Nickname: "已注销用户", PasswordHash: anonymization.PasswordHash, CredentialVersion: anonymization.CredentialVersion, Status: domain.StatusActive, AccountState: domain.AccountStateAnonymized, AccountStateVersion: 3, ProfileTheme: domain.ProfileThemeDefault, CreatedAt: anonymization.CompletedAt, UpdatedAt: anonymization.CompletedAt, DeletedAt: &anonymization.CompletedAt}, nil
}

type workerPublisherStub struct {
	events []domain.DomainEvent
}

func (p *workerPublisherStub) PublishDomainEvents(_ context.Context, events []domain.DomainEvent) error {
	p.events = append(p.events, events...)
	return nil
}

type workerCredentialCacheStub struct {
	userID  int64
	version string
}

func (c *workerCredentialCacheStub) SetCurrent(_ context.Context, userID int64, version string) error {
	c.userID = userID
	c.version = version
	return nil
}
