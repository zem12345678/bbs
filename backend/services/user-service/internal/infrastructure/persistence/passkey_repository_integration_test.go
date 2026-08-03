package persistence

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"os"
	"testing"
	"time"

	domain "user-service/internal/domain/user"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

func TestPasskeyRepoPostgresIntegration(t *testing.T) {
	if os.Getenv("BBS_PG_SMOKE") != "1" {
		t.Skip("set BBS_PG_SMOKE=1 to run PostgreSQL passkey transaction test")
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
	anonymousChallengeHashes := make([]string, 0, 5)
	t.Cleanup(func() {
		if len(anonymousChallengeHashes) > 0 {
			_ = db.Where("token_hash IN ?", anonymousChallengeHashes).Delete(&passkeyChallengePO{}).Error
		}
		_ = db.Exec("DELETE FROM users WHERE id = ?", userID).Error
	})
	enabledAt := now
	if err := db.Create(&mfaTOTPPO{UserID: userID, SecretCiphertext: "encrypted-totp-secret", EnabledAt: &enabledAt, LastTOTPStep: -1, UpdatedAt: now}).Error; err != nil {
		t.Fatalf("enable MFA prerequisite: %v", err)
	}
	usedAt := now.Add(-time.Minute)
	staleChallenges := []passkeyChallengePO{
		{TokenHash: integrationHash(1), Ceremony: domain.PasskeyCeremonyPasswordless, SessionCiphertext: "expired", ExpiresAt: now.Add(-time.Minute), CreatedAt: now.Add(-2 * time.Minute)},
		{TokenHash: integrationHash(2), Ceremony: domain.PasskeyCeremonyPasswordless, SessionCiphertext: "used", ExpiresAt: now.Add(-time.Second), UsedAt: &usedAt, CreatedAt: now.Add(-2 * time.Minute)},
		{TokenHash: integrationHash(3), Ceremony: domain.PasskeyCeremonyPasswordless, SessionCiphertext: "exhausted", ExpiresAt: now.Add(-time.Second), Attempts: domain.PasskeyMaxChallengeAttempts, CreatedAt: now.Add(-2 * time.Minute)},
	}
	for _, challenge := range staleChallenges {
		anonymousChallengeHashes = append(anonymousChallengeHashes, challenge.TokenHash)
	}
	if err := db.Create(&staleChallenges).Error; err != nil {
		t.Fatalf("create stale passkey challenges: %v", err)
	}
	cleanupTrigger := integrationHash(4)
	anonymousChallengeHashes = append(anonymousChallengeHashes, cleanupTrigger)
	if err := repo.CreatePasskeyChallenge(ctx, domain.PasskeyChallenge{TokenHash: cleanupTrigger, Ceremony: domain.PasskeyCeremonyPasswordless, SessionCiphertext: "active", ExpiresAt: now.Add(time.Hour), CreatedAt: now}); err != nil {
		t.Fatalf("create cleanup trigger challenge: %v", err)
	}
	var staleCount int64
	if err := db.Model(&passkeyChallengePO{}).Where("token_hash IN ?", []string{integrationHash(1), integrationHash(2), integrationHash(3)}).Count(&staleCount).Error; err != nil || staleCount != 0 {
		t.Fatalf("stale passkey challenges after cleanup = %d, error = %v", staleCount, err)
	}

	credentialIDs := make([]string, 0, domain.PasskeyMaxCredentials)
	for index := 0; index < domain.PasskeyMaxCredentials-1; index++ {
		credentialID := fmt.Sprintf("credential-%02d", index)
		credentialIDs = append(credentialIDs, credentialID)
		createPasskeyForIntegration(t, repo, ctx, userID, credentialID, integrationHash(100+index), now.Add(time.Duration(index)*time.Millisecond))
	}
	concurrentIDs := []string{"credential-concurrent-a", "credential-concurrent-b"}
	for index, credentialID := range concurrentIDs {
		createPasskeyChallengeForIntegration(t, repo, ctx, userID, integrationHash(200+index), credentialID, now)
	}
	results := runConcurrentMFAOperations(
		func() error {
			return repo.CreatePasskeyFromChallenge(ctx, integrationHash(200), userID, integrationCredential(userID, concurrentIDs[0], 0), now.Add(time.Second))
		},
		func() error {
			return repo.CreatePasskeyFromChallenge(ctx, integrationHash(201), userID, integrationCredential(userID, concurrentIDs[1], 0), now.Add(time.Second))
		},
	)
	assertOneMFAOperationWon(t, results, domain.ErrPasskeyLimitReached)
	state, err := repo.GetPasskeyState(ctx, userID)
	if err != nil || len(state.Credentials) != domain.PasskeyMaxCredentials {
		t.Fatalf("exact passkey limit state = %+v, error = %v", state, err)
	}
	for _, candidate := range concurrentIDs {
		for _, credential := range state.Credentials {
			if credential.CredentialID == candidate {
				credentialIDs = append(credentialIDs, candidate)
			}
		}
	}
	if len(credentialIDs) != domain.PasskeyMaxCredentials {
		t.Fatalf("stored credential IDs = %d", len(credentialIDs))
	}

	if err := repo.SetPasskeyPasswordless(ctx, userID, true, now.Add(2*time.Second)); err != nil {
		t.Fatalf("enable passwordless: %v", err)
	}
	state, err = repo.GetPasskeyState(ctx, userID)
	if err != nil || !state.PasswordlessEnabled {
		t.Fatalf("passwordless state = %+v, error = %v", state, err)
	}

	targetID := credentialIDs[0]
	mfaTokenHash := integrationHash(300)
	if err := repo.CreateMFALoginChallenge(ctx, domain.MFALoginChallenge{TokenHash: mfaTokenHash, UserID: userID, ExpiresAt: now.Add(time.Hour), CreatedAt: now}); err != nil {
		t.Fatalf("create MFA login challenge: %v", err)
	}
	passkeyMFATokenHash := integrationHash(301)
	if err := repo.CreatePasskeyChallenge(ctx, domain.PasskeyChallenge{
		TokenHash: passkeyMFATokenHash, Ceremony: domain.PasskeyCeremonyMFA, UserID: userID, MFATokenHash: mfaTokenHash,
		SessionCiphertext: "encrypted-mfa-session", ExpiresAt: now.Add(time.Hour), CreatedAt: now,
	}); err != nil {
		t.Fatalf("create passkey MFA challenge: %v", err)
	}
	updated := integrationCredential(userID, targetID, 1)
	updated.CredentialCiphertext = "updated-after-mfa"
	assertOneMFAOperationWon(t, runConcurrentMFAOperations(
		func() error {
			return repo.CompletePasskeyMFALogin(ctx, passkeyMFATokenHash, mfaTokenHash, userID, updated, 1, now.Add(3*time.Second))
		},
		func() error {
			return repo.CompletePasskeyMFALogin(ctx, passkeyMFATokenHash, mfaTokenHash, userID, updated, 1, now.Add(3*time.Second))
		},
	), domain.ErrPasskeyChallengeInvalid)
	stored, err := repo.GetPasskeyByCredentialID(ctx, targetID)
	if err != nil || stored.Version != 2 || stored.CredentialCiphertext != "updated-after-mfa" || stored.LastUsedAt == nil {
		t.Fatalf("credential after MFA = %+v, error = %v", stored, err)
	}

	failureMFAToken := integrationHash(310)
	if err := repo.CreateMFALoginChallenge(ctx, domain.MFALoginChallenge{TokenHash: failureMFAToken, UserID: userID, ExpiresAt: now.Add(time.Hour), CreatedAt: now}); err != nil {
		t.Fatalf("create failed-attempt MFA challenge: %v", err)
	}
	failurePasskeyToken := integrationHash(311)
	if err := repo.CreatePasskeyChallenge(ctx, domain.PasskeyChallenge{
		TokenHash: failurePasskeyToken, Ceremony: domain.PasskeyCeremonyMFA, UserID: userID, MFATokenHash: failureMFAToken,
		SessionCiphertext: "encrypted-failure-session", ExpiresAt: now.Add(time.Hour), CreatedAt: now,
	}); err != nil {
		t.Fatalf("create failed-attempt passkey challenge: %v", err)
	}
	for attempt := 0; attempt < domain.PasskeyMaxChallengeAttempts; attempt++ {
		if err := repo.RecordPasskeyChallengeFailure(ctx, failurePasskeyToken, now.Add(time.Duration(4+attempt)*time.Second)); err != nil {
			t.Fatalf("record passkey failure %d: %v", attempt+1, err)
		}
	}
	if _, err := repo.GetPasskeyChallenge(ctx, failurePasskeyToken, now.Add(10*time.Second)); !errors.Is(err, domain.ErrPasskeyChallengeAttemptsExceeded) {
		t.Fatalf("exhausted passkey challenge error = %v", err)
	}
	if _, err := repo.GetMFALoginChallenge(ctx, failureMFAToken, now.Add(10*time.Second)); !errors.Is(err, domain.ErrMFAChallengeAttemptsExceeded) {
		t.Fatalf("linked MFA attempt limit error = %v", err)
	}

	passwordlessToken := integrationHash(320)
	anonymousChallengeHashes = append(anonymousChallengeHashes, passwordlessToken)
	if err := repo.CreatePasskeyChallenge(ctx, domain.PasskeyChallenge{TokenHash: passwordlessToken, Ceremony: domain.PasskeyCeremonyPasswordless, SessionCiphertext: "encrypted-passwordless-session", ExpiresAt: now.Add(time.Hour), CreatedAt: now}); err != nil {
		t.Fatalf("create passwordless challenge: %v", err)
	}
	updated = integrationCredential(userID, targetID, 2)
	updated.CredentialCiphertext = "updated-after-passwordless"
	assertOneMFAOperationWon(t, runConcurrentMFAOperations(
		func() error {
			return repo.CompletePasskeyPasswordlessLogin(ctx, passwordlessToken, userID, updated, 2, now.Add(11*time.Second))
		},
		func() error {
			return repo.CompletePasskeyPasswordlessLogin(ctx, passwordlessToken, userID, updated, 2, now.Add(11*time.Second))
		},
	), domain.ErrPasskeyChallengeInvalid)
	stored, err = repo.GetPasskeyByCredentialID(ctx, targetID)
	if err != nil || stored.Version != 3 || stored.CredentialCiphertext != "updated-after-passwordless" {
		t.Fatalf("credential after passwordless = %+v, error = %v", stored, err)
	}

	for _, credentialID := range credentialIDs[1:] {
		if err := repo.DeletePasskey(ctx, userID, credentialID, now.Add(12*time.Second)); err != nil {
			t.Fatalf("delete passkey %q: %v", credentialID, err)
		}
	}
	if err := repo.DeletePasskey(ctx, userID, targetID, now.Add(13*time.Second)); err != nil {
		t.Fatalf("delete last passkey: %v", err)
	}
	state, err = repo.GetPasskeyState(ctx, userID)
	if err != nil || len(state.Credentials) != 0 || state.PasswordlessEnabled {
		t.Fatalf("state after deleting last passkey = %+v, error = %v", state, err)
	}

	createPasskeyForIntegration(t, repo, ctx, userID, "cascade-credential", integrationHash(400), now.Add(14*time.Second))
	if err := repo.SetPasskeyPasswordless(ctx, userID, true, now.Add(15*time.Second)); err != nil {
		t.Fatalf("re-enable passwordless: %v", err)
	}
	if err := repo.DisableTOTP(ctx, userID); err != nil {
		t.Fatalf("disable TOTP: %v", err)
	}
	if _, err := repo.GetPasskeyState(ctx, userID); !errors.Is(err, domain.ErrMFANotEnabled) {
		t.Fatalf("passkey state after TOTP disable error = %v", err)
	}
	var remaining int64
	if err := db.Model(&passkeyCredentialPO{}).Where("user_id = ?", userID).Count(&remaining).Error; err != nil || remaining != 0 {
		t.Fatalf("passkeys after TOTP cascade = %d, error = %v", remaining, err)
	}
}

func createPasskeyForIntegration(t *testing.T, repo *Repo, ctx context.Context, userID int64, credentialID string, tokenHash string, now time.Time) {
	t.Helper()
	createPasskeyChallengeForIntegration(t, repo, ctx, userID, tokenHash, credentialID, now)
	if err := repo.CreatePasskeyFromChallenge(ctx, tokenHash, userID, integrationCredential(userID, credentialID, 0), now.Add(time.Millisecond)); err != nil {
		t.Fatalf("create passkey %q: %v", credentialID, err)
	}
}

func createPasskeyChallengeForIntegration(t *testing.T, repo *Repo, ctx context.Context, userID int64, tokenHash string, credentialID string, now time.Time) {
	t.Helper()
	if err := repo.CreatePasskeyChallenge(ctx, domain.PasskeyChallenge{
		TokenHash: tokenHash, Ceremony: domain.PasskeyCeremonyRegistration, UserID: userID, PasskeyName: credentialID,
		SessionCiphertext: "encrypted-registration-session", ExpiresAt: now.Add(time.Hour), CreatedAt: now,
	}); err != nil {
		t.Fatalf("create registration challenge %q: %v", credentialID, err)
	}
}

func integrationCredential(userID int64, credentialID string, version int64) domain.PasskeyCredential {
	return domain.PasskeyCredential{CredentialID: credentialID, UserID: userID, CredentialCiphertext: "encrypted-" + credentialID, Version: version, BackupEligible: true}
}

var integrationPasskeyRunID = time.Now().UnixNano()

func integrationHash(value int) string {
	return fmt.Sprintf("%x", sha256.Sum256([]byte(fmt.Sprintf("%d:%d", integrationPasskeyRunID, value))))
}
