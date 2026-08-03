package persistence

import (
	"context"
	"errors"
	"os"
	"sync"
	"testing"
	"time"

	domain "user-service/internal/domain/user"
	infraMFA "user-service/internal/infrastructure/mfa"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

func TestMFARepoPostgresIntegration(t *testing.T) {
	if os.Getenv("BBS_PG_SMOKE") != "1" {
		t.Skip("set BBS_PG_SMOKE=1 to run PostgreSQL MFA transaction test")
	}
	dsn := os.Getenv("BBS_USER_PG_DSN")
	if dsn == "" {
		dsn = "postgres://bbs_user_app:local_user_pass@127.0.0.1:5432/bbs?sslmode=disable&search_path=bbs_user"
	}
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("open postgres: %v", err)
	}
	repo := NewRepo(db)
	ctx := context.Background()
	now := time.Now().UTC().Truncate(time.Millisecond)
	userID := now.UnixNano()
	testUser := integrationUserListUser(userID, now)
	if err := db.Create(&testUser).Error; err != nil {
		t.Fatalf("create test user: %v", err)
	}
	t.Cleanup(func() {
		_ = db.Exec("DELETE FROM users WHERE id = ?", userID).Error
	})

	manager, err := infraMFA.New("postgres-integration-mfa-encryption-key", "Integration Community")
	if err != nil {
		t.Fatalf("new MFA manager: %v", err)
	}
	enrollment, err := manager.NewTOTP("integration-user")
	if err != nil {
		t.Fatalf("new TOTP enrollment: %v", err)
	}
	ciphertext, err := manager.EncryptSecret(enrollment.Secret)
	if err != nil {
		t.Fatalf("encrypt TOTP secret: %v", err)
	}
	if err := repo.SavePendingTOTP(ctx, userID, ciphertext, now); err != nil {
		t.Fatalf("save pending TOTP: %v", err)
	}
	var stored struct {
		SecretCiphertext        string
		PendingSecretCiphertext string
	}
	if err := db.Raw("SELECT secret_ciphertext, pending_secret_ciphertext FROM user_mfa_totp WHERE user_id = ?", userID).Scan(&stored).Error; err != nil {
		t.Fatalf("read stored TOTP: %v", err)
	}
	if stored.SecretCiphertext != "" || stored.PendingSecretCiphertext != ciphertext || stored.PendingSecretCiphertext == enrollment.Secret {
		t.Fatalf("stored TOTP secret is not pending ciphertext: %+v", stored)
	}

	codes, hashes, err := manager.NewRecoveryCodes(4)
	if err != nil {
		t.Fatalf("new recovery codes: %v", err)
	}
	if err := repo.EnableTOTP(ctx, userID, ciphertext, hashes, now.Add(time.Second)); err != nil {
		t.Fatalf("enable TOTP: %v", err)
	}
	state, err := repo.GetMFAState(ctx, userID)
	if err != nil {
		t.Fatalf("get enabled MFA state: %v", err)
	}
	if !state.Enabled() || state.SecretCiphertext != ciphertext || state.PendingSecretCiphertext != "" || state.RecoveryCodesRemaining != 4 {
		t.Fatalf("enabled MFA state = %+v", state)
	}

	assertOneMFAOperationWon(t, runConcurrentMFAOperations(
		func() error { return repo.UseTOTPStep(ctx, userID, 60_000_000, now.Add(2*time.Second)) },
		func() error { return repo.UseTOTPStep(ctx, userID, 60_000_000, now.Add(2*time.Second)) },
	), domain.ErrMFACodeReplayed)

	assertOneMFAOperationWon(t, runConcurrentMFAOperations(
		func() error {
			return repo.UseMFARecoveryCode(ctx, userID, manager.HashRecoveryCode(codes[0]), now.Add(3*time.Second))
		},
		func() error {
			return repo.UseMFARecoveryCode(ctx, userID, manager.HashRecoveryCode(codes[0]), now.Add(3*time.Second))
		},
	), domain.ErrMFACodeInvalid)
	state, err = repo.GetMFAState(ctx, userID)
	if err != nil || state.RecoveryCodesRemaining != 3 {
		t.Fatalf("remaining recovery codes = %d, error = %v", state.RecoveryCodesRemaining, err)
	}

	exhaustedToken := manager.HashChallenge("exhausted-login-challenge")
	if err := repo.CreateMFALoginChallenge(ctx, domain.MFALoginChallenge{
		TokenHash: exhaustedToken,
		UserID:    userID,
		ExpiresAt: now.Add(10 * time.Minute),
		CreatedAt: now,
	}); err != nil {
		t.Fatalf("create challenge for attempt limit: %v", err)
	}
	for attempt := 0; attempt < domain.MFAMaxLoginAttempts; attempt++ {
		if err := repo.RecordMFALoginFailure(ctx, exhaustedToken, now.Add(time.Duration(attempt+1)*time.Second)); err != nil {
			t.Fatalf("record failed attempt %d: %v", attempt+1, err)
		}
	}
	if _, err := repo.GetMFALoginChallenge(ctx, exhaustedToken, now.Add(6*time.Second)); !errors.Is(err, domain.ErrMFAChallengeAttemptsExceeded) {
		t.Fatalf("exhausted challenge error = %v", err)
	}

	totpToken := manager.HashChallenge("concurrent-totp-login-challenge")
	if err := repo.CreateMFALoginChallenge(ctx, domain.MFALoginChallenge{
		TokenHash: totpToken,
		UserID:    userID,
		ExpiresAt: now.Add(10 * time.Minute),
		CreatedAt: now,
	}); err != nil {
		t.Fatalf("create TOTP challenge: %v", err)
	}
	assertOneMFAOperationWon(t, runConcurrentMFAOperations(
		func() error {
			return repo.CompleteMFALoginWithTOTP(ctx, totpToken, userID, 60_000_001, now.Add(7*time.Second))
		},
		func() error {
			return repo.CompleteMFALoginWithTOTP(ctx, totpToken, userID, 60_000_001, now.Add(7*time.Second))
		},
	), domain.ErrMFAChallengeInvalid)

	recoveryToken := manager.HashChallenge("concurrent-recovery-login-challenge")
	if err := repo.CreateMFALoginChallenge(ctx, domain.MFALoginChallenge{
		TokenHash: recoveryToken,
		UserID:    userID,
		ExpiresAt: now.Add(10 * time.Minute),
		CreatedAt: now,
	}); err != nil {
		t.Fatalf("create recovery challenge: %v", err)
	}
	assertOneMFAOperationWon(t, runConcurrentMFAOperations(
		func() error {
			return repo.CompleteMFALoginWithRecoveryCode(ctx, recoveryToken, userID, manager.HashRecoveryCode(codes[1]), now.Add(8*time.Second))
		},
		func() error {
			return repo.CompleteMFALoginWithRecoveryCode(ctx, recoveryToken, userID, manager.HashRecoveryCode(codes[1]), now.Add(8*time.Second))
		},
	), domain.ErrMFAChallengeInvalid)

	expiredToken := manager.HashChallenge("expired-login-challenge")
	if err := repo.CreateMFALoginChallenge(ctx, domain.MFALoginChallenge{
		TokenHash: expiredToken,
		UserID:    userID,
		ExpiresAt: now.Add(time.Second),
		CreatedAt: now,
	}); err != nil {
		t.Fatalf("create expired challenge: %v", err)
	}
	if _, err := repo.GetMFALoginChallenge(ctx, expiredToken, now.Add(2*time.Second)); !errors.Is(err, domain.ErrMFAChallengeExpired) {
		t.Fatalf("expired challenge error = %v", err)
	}

	if err := repo.DisableTOTP(ctx, userID); err != nil {
		t.Fatalf("disable TOTP: %v", err)
	}
	if _, err := repo.GetMFAState(ctx, userID); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("disabled MFA state error = %v", err)
	}
}

func runConcurrentMFAOperations(operations ...func() error) <-chan error {
	errorsCh := make(chan error, len(operations))
	start := make(chan struct{})
	var wg sync.WaitGroup
	for _, operation := range operations {
		operation := operation
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			errorsCh <- operation()
		}()
	}
	close(start)
	wg.Wait()
	close(errorsCh)
	return errorsCh
}

func assertOneMFAOperationWon(t *testing.T, results <-chan error, allowedLoserErrors ...error) {
	t.Helper()
	successes := 0
	failures := 0
	for err := range results {
		if err == nil {
			successes++
			continue
		}
		allowed := false
		for _, expected := range allowedLoserErrors {
			if errors.Is(err, expected) {
				allowed = true
				break
			}
		}
		if !allowed {
			t.Fatalf("unexpected concurrent operation error: %v", err)
		}
		failures++
	}
	if successes != 1 || failures != 1 {
		t.Fatalf("concurrent operation results: successes=%d failures=%d", successes, failures)
	}
}
