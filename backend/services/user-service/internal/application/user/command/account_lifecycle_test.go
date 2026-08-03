package command

import (
	"context"
	"errors"
	"reflect"
	"testing"
	"time"

	domain "user-service/internal/domain/user"
	infraMFA "user-service/internal/infrastructure/mfa"
)

func TestRequestAccountDeletionRejectsWrongPasswordWithoutCreatingJob(t *testing.T) {
	ctx := context.Background()
	repo := newAccountLifecycleMemoryRepo()
	svc := NewService(repo, &fakeIDGen{next: 30_000}, nil, nil, "test-secret", time.Hour, 8, nil, nil, nil)
	u := registerAccountLifecycleUser(t, ctx, svc, "deletion_wrong_password")

	if _, err := svc.RequestAccountDeletion(ctx, u.ID, "wrong-password", ""); !errors.Is(err, domain.ErrInvalidPassword) {
		t.Fatalf("request deletion with wrong password error = %v, want ErrInvalidPassword", err)
	}
	if repo.requestCalls != 0 {
		t.Fatalf("deletion request calls = %d, want 0", repo.requestCalls)
	}
}

func TestRequestAccountDeletionRequiresValidMFAWhenEnabled(t *testing.T) {
	ctx := context.Background()
	repo := newAccountLifecycleMemoryRepo()
	manager, err := infraMFA.New("account-lifecycle-test-encryption-key", "Test Community")
	if err != nil {
		t.Fatalf("new MFA manager: %v", err)
	}
	svc := NewService(repo, &fakeIDGen{next: 31_000}, nil, nil, "test-secret", time.Hour, 8, nil, nil, nil, manager)
	u := registerAccountLifecycleUser(t, ctx, svc, "deletion_mfa")
	secretCiphertext, err := manager.EncryptSecret("JBSWY3DPEHPK3PXP")
	if err != nil {
		t.Fatalf("encrypt MFA secret: %v", err)
	}
	enabledAt := time.Now()
	repo.states[u.ID] = domain.MFAState{
		UserID:           u.ID,
		SecretCiphertext: secretCiphertext,
		EnabledAt:        &enabledAt,
		LastTOTPStep:     -1,
	}

	for _, code := range []string{"", "not-a-valid-code"} {
		if _, err := svc.RequestAccountDeletion(ctx, u.ID, "password-123", code); !errors.Is(err, domain.ErrMFACodeInvalid) {
			t.Fatalf("request deletion with MFA code %q error = %v, want ErrMFACodeInvalid", code, err)
		}
	}
	if repo.requestCalls != 0 {
		t.Fatalf("deletion request calls = %d, want 0", repo.requestCalls)
	}
}

func TestRequestAccountDeletionRotatesCredentialsAndCreatesExpectedJob(t *testing.T) {
	ctx := context.Background()
	repo := newAccountLifecycleMemoryRepo()
	cache := newCredentialVersionCacheStub()
	idgen := &fakeIDGen{next: 32_000}
	svc := NewService(repo, idgen, nil, nil, "test-secret", time.Hour, 8, nil, nil, cache)
	u := registerAccountLifecycleUser(t, ctx, svc, "deletion_success")
	oldCredentialVersion := u.CredentialVersion

	result, err := svc.RequestAccountDeletion(ctx, u.ID, "password-123", "")
	if err != nil {
		t.Fatalf("request account deletion: %v", err)
	}
	if repo.requestCalls != 1 {
		t.Fatalf("deletion request calls = %d, want 1", repo.requestCalls)
	}
	if repo.lastRequest.UserID != u.ID || repo.lastRequest.ActorUserID != u.ID {
		t.Fatalf("deletion request identity = user:%d actor:%d, want %d", repo.lastRequest.UserID, repo.lastRequest.ActorUserID, u.ID)
	}
	if repo.lastRequest.ExpectedCredentialVersion != oldCredentialVersion {
		t.Fatalf("expected credential version = %q, want %q", repo.lastRequest.ExpectedCredentialVersion, oldCredentialVersion)
	}
	if repo.lastRequest.CredentialVersion == "" || repo.lastRequest.CredentialVersion == oldCredentialVersion {
		t.Fatalf("rotated credential version = %q, old = %q", repo.lastRequest.CredentialVersion, oldCredentialVersion)
	}
	wantSteps := domain.AccountDeletionSteps()
	if !reflect.DeepEqual(repo.lastRequest.Steps, wantSteps) {
		t.Fatalf("deletion steps = %v, want %v", repo.lastRequest.Steps, wantSteps)
	}
	if result.State != domain.AccountStateDeletionPending || result.CredentialVersion != repo.lastRequest.CredentialVersion {
		t.Fatalf("account lifecycle result = %+v", result)
	}
	if result.ActiveDeletionJob == nil || result.ActiveDeletionJob.ID != repo.lastRequest.JobID || result.ActiveDeletionJob.TotalSteps != int32(len(wantSteps)) {
		t.Fatalf("active deletion job = %+v, request job ID = %d", result.ActiveDeletionJob, repo.lastRequest.JobID)
	}
	if cached := cache.versions[u.ID]; cached != result.CredentialVersion {
		t.Fatalf("cached credential version = %q, want %q", cached, result.CredentialVersion)
	}
}

func TestRequestAccountDeletionRejectsProtectedAccount(t *testing.T) {
	ctx := context.Background()
	repo := newAccountLifecycleMemoryRepo()
	svc := NewService(repo, &fakeIDGen{next: 33_000}, nil, nil, "test-secret", time.Hour, 8, nil, nil, nil)
	u := registerAccountLifecycleUser(t, ctx, svc, "protected_account")
	repo.users[u.ID].ProtectedAccount = true

	if _, err := svc.RequestAccountDeletion(ctx, u.ID, "password-123", ""); !errors.Is(err, domain.ErrAccountProtected) {
		t.Fatalf("request protected account deletion error = %v, want ErrAccountProtected", err)
	}
	if repo.requestCalls != 0 {
		t.Fatalf("deletion request calls = %d, want 0", repo.requestCalls)
	}
}

func TestLoginVerifiesPasswordBeforeRevealingAccountLifecycleState(t *testing.T) {
	ctx := context.Background()
	repo := newAccountLifecycleMemoryRepo()
	svc := NewService(repo, &fakeIDGen{next: 34_000}, nil, nil, "test-secret", time.Hour, 8, nil, nil, nil)
	u := registerAccountLifecycleUser(t, ctx, svc, "pending_login")
	repo.users[u.ID].AccountState = domain.AccountStateDeletionPending

	if _, _, err := svc.Login(ctx, u.Username, "wrong-password"); !errors.Is(err, domain.ErrInvalidPassword) {
		t.Fatalf("login with wrong password error = %v, want ErrInvalidPassword", err)
	}
	if _, _, err := svc.Login(ctx, u.Username, "password-123"); !errors.Is(err, domain.ErrAccountDeletionPending) {
		t.Fatalf("login with correct password error = %v, want ErrAccountDeletionPending", err)
	}
}

type accountLifecycleMemoryRepo struct {
	*mfaMemoryRepo
	requestCalls int
	lastRequest  domain.AccountDeletionRequest
}

func newAccountLifecycleMemoryRepo() *accountLifecycleMemoryRepo {
	return &accountLifecycleMemoryRepo{mfaMemoryRepo: newMFAMemoryRepo()}
}

func (r *accountLifecycleMemoryRepo) GetAccountLifecycle(ctx context.Context, userID int64) (domain.AccountLifecycle, error) {
	u, err := r.FindByID(ctx, userID)
	if err != nil {
		return domain.AccountLifecycle{}, err
	}
	return domain.AccountLifecycle{
		UserID:              u.ID,
		State:               domain.NormalizeAccountState(u.AccountState),
		StateVersion:        u.AccountStateVersion,
		CredentialVersion:   domain.NormalizeCredentialVersion(u.CredentialVersion),
		Protected:           u.ProtectedAccount,
		DeletionRequestedAt: u.DeletionRequestedAt,
		DeletedAt:           u.DeletedAt,
	}, nil
}

func (r *accountLifecycleMemoryRepo) RequestAccountDeletion(_ context.Context, request domain.AccountDeletionRequest) (domain.AccountLifecycle, error) {
	r.requestCalls++
	r.lastRequest = request
	r.lastRequest.Steps = append([]string(nil), request.Steps...)
	u, ok := r.users[request.UserID]
	if !ok {
		return domain.AccountLifecycle{}, domain.ErrNotFound
	}
	u = cloneUser(u)
	u.AccountState = domain.AccountStateDeletionPending
	u.AccountStateVersion++
	u.CredentialVersion = request.CredentialVersion
	u.DeletionRequestedAt = &request.RequestedAt
	u.UpdatedAt = request.RequestedAt
	r.users[u.ID] = u
	job := &domain.AccountDeletionJob{
		ID:            request.JobID,
		UserID:        request.UserID,
		Status:        domain.AccountJobPending,
		PolicyVersion: request.PolicyVersion,
		TotalSteps:    int32(len(request.Steps)),
		CreatedAt:     request.RequestedAt,
		UpdatedAt:     request.RequestedAt,
	}
	return domain.AccountLifecycle{
		UserID:              u.ID,
		State:               u.AccountState,
		StateVersion:        u.AccountStateVersion,
		CredentialVersion:   u.CredentialVersion,
		DeletionRequestedAt: u.DeletionRequestedAt,
		ActiveDeletionJob:   job,
	}, nil
}

func registerAccountLifecycleUser(t *testing.T, ctx context.Context, svc *Service, username string) *domain.User {
	t.Helper()
	u, _, err := svc.Register(ctx, domain.RegisterCmd{
		Username: username,
		Email:    username + "@example.com",
		Password: "password-123",
		Nickname: username,
	})
	if err != nil {
		t.Fatalf("register account lifecycle user: %v", err)
	}
	return u
}
