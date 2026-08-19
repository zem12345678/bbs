package command

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	domain "user-service/internal/domain/user"
	infraMFA "user-service/internal/infrastructure/mfa"

	"github.com/pquerna/otp/totp"
)

func TestMFAEnrollmentLoginRecoveryAndDisableFlow(t *testing.T) {
	ctx := context.Background()
	repo := newMFAMemoryRepo()
	manager, err := infraMFA.New("command-test-mfa-encryption-key-value", "Test Community")
	if err != nil {
		t.Fatalf("new MFA manager: %v", err)
	}
	svc := NewService(repo, &fakeIDGen{next: 20_001}, nil, nil, "test-secret", time.Hour, 8, nil, nil, nil, manager)
	u, _, err := svc.Register(ctx, domain.RegisterCmd{
		Username: "mfa_user",
		Email:    "mfa-user@example.com",
		Password: "password-123",
		Nickname: "MFA User",
	})
	if err != nil {
		t.Fatalf("register user: %v", err)
	}

	if _, err := svc.BeginTOTPEnrollment(ctx, u.ID, "wrong-password", ""); !errors.Is(err, domain.ErrInvalidPassword) {
		t.Fatalf("begin enrollment with wrong password error = %v", err)
	}
	enrollment, err := svc.BeginTOTPEnrollment(ctx, u.ID, "password-123", "")
	if err != nil {
		t.Fatalf("begin enrollment: %v", err)
	}
	if enrollment.Secret == "" || enrollment.QRDataURL == "" {
		t.Fatalf("incomplete enrollment: %+v", enrollment)
	}
	if _, err := svc.ConfirmTOTPEnrollment(ctx, u.ID, "invalid"); !errors.Is(err, domain.ErrMFACodeInvalid) {
		t.Fatalf("confirm invalid TOTP error = %v", err)
	}
	confirmationCode, err := totp.GenerateCode(enrollment.Secret, time.Now())
	if err != nil {
		t.Fatalf("generate confirmation code: %v", err)
	}
	recoveryCodes, err := svc.ConfirmTOTPEnrollment(ctx, u.ID, confirmationCode)
	if err != nil {
		t.Fatalf("confirm enrollment: %v", err)
	}
	if len(recoveryCodes) != mfaRecoveryCodeCount {
		t.Fatalf("recovery code count = %d", len(recoveryCodes))
	}
	status, err := svc.MFAStatus(ctx, u.ID)
	if err != nil {
		t.Fatalf("get MFA status: %v", err)
	}
	if !status.Enabled || status.RecoveryCodesRemaining != mfaRecoveryCodeCount || status.EnabledAt.IsZero() {
		t.Fatalf("enabled MFA status = %+v", status)
	}

	loggedInUser, challenge, err := svc.Login(ctx, "mfa_user", "password-123")
	if err != nil {
		t.Fatalf("start MFA login: %v", err)
	}
	if loggedInUser == nil || !challenge.MFARequired || challenge.MFAChallenge == "" || challenge.Value != "" || challenge.MFAChallengeExpiry.IsZero() {
		t.Fatalf("MFA login challenge = user:%+v token:%+v", loggedInUser, challenge)
	}
	if loggedInUser.LastLoginAt != nil {
		t.Fatalf("last login changed before MFA completion: %v", loggedInUser.LastLoginAt)
	}

	completedUser, token, err := svc.CompleteMFALogin(ctx, challenge.MFAChallenge, recoveryCodes[0])
	if err != nil {
		t.Fatalf("complete MFA login with recovery code: %v", err)
	}
	if completedUser == nil || completedUser.LastLoginAt == nil || token.Value == "" || token.MFARequired {
		t.Fatalf("completed MFA login = user:%+v token:%+v", completedUser, token)
	}
	if _, _, err := svc.CompleteMFALogin(ctx, challenge.MFAChallenge, recoveryCodes[0]); !errors.Is(err, domain.ErrMFAChallengeInvalid) {
		t.Fatalf("reused login challenge error = %v", err)
	}
	status, err = svc.MFAStatus(ctx, u.ID)
	if err != nil || status.RecoveryCodesRemaining != mfaRecoveryCodeCount-1 {
		t.Fatalf("status after recovery login = %+v, error = %v", status, err)
	}

	replacementCodes, err := svc.RegenerateMFARecoveryCodes(ctx, u.ID, "password-123", recoveryCodes[1])
	if err != nil {
		t.Fatalf("regenerate recovery codes: %v", err)
	}
	if len(replacementCodes) != mfaRecoveryCodeCount {
		t.Fatalf("replacement recovery code count = %d", len(replacementCodes))
	}
	if _, _, err := svc.Login(ctx, "mfa_user", "password-123"); err != nil {
		t.Fatalf("start second MFA login: %v", err)
	}
	if err := svc.DisableTOTP(ctx, u.ID, "password-123", replacementCodes[0]); err != nil {
		t.Fatalf("disable TOTP: %v", err)
	}
	status, err = svc.MFAStatus(ctx, u.ID)
	if err != nil {
		t.Fatalf("get disabled MFA status: %v", err)
	}
	if status.Enabled || status.RecoveryCodesRemaining != 0 || !status.EnabledAt.IsZero() {
		t.Fatalf("disabled MFA status = %+v", status)
	}
	_, directToken, err := svc.Login(ctx, "mfa_user", "password-123")
	if err != nil {
		t.Fatalf("login after disabling MFA: %v", err)
	}
	if directToken.Value == "" || directToken.MFARequired {
		t.Fatalf("direct login token after disable = %+v", directToken)
	}
}

func TestChangePasswordRequiresValidMFAWhenEnabled(t *testing.T) {
	ctx := context.Background()
	repo := newMFAMemoryRepo()
	manager, err := infraMFA.New("command-test-mfa-encryption-key-value", "Test Community")
	if err != nil {
		t.Fatalf("new MFA manager: %v", err)
	}
	svc := NewService(repo, &fakeIDGen{next: 20_051}, nil, nil, "test-secret", time.Hour, 8, nil, nil, nil, manager)
	u, _, err := svc.Register(ctx, domain.RegisterCmd{
		Username: "mfa_password_user",
		Email:    "mfa-password-user@example.com",
		Password: "password-123",
		Nickname: "MFA Password User",
	})
	if err != nil {
		t.Fatalf("register user: %v", err)
	}
	enrollment, err := svc.BeginTOTPEnrollment(ctx, u.ID, "password-123", "")
	if err != nil {
		t.Fatalf("begin enrollment: %v", err)
	}
	confirmationCode, err := totp.GenerateCode(enrollment.Secret, time.Now())
	if err != nil {
		t.Fatalf("generate confirmation code: %v", err)
	}
	recoveryCodes, err := svc.ConfirmTOTPEnrollment(ctx, u.ID, confirmationCode)
	if err != nil {
		t.Fatalf("confirm enrollment: %v", err)
	}

	if err := svc.ChangePassword(ctx, u.ID, "password-123", "changed-password-123", "invalid-code"); !errors.Is(err, domain.ErrMFACodeInvalid) {
		t.Fatalf("change password with invalid MFA code error = %v", err)
	}
	if err := svc.ChangePassword(ctx, u.ID, "password-123", "changed-password-123", recoveryCodes[0]); err != nil {
		t.Fatalf("change password with recovery code: %v", err)
	}
	if _, _, err := svc.Login(ctx, u.Username, "password-123"); !errors.Is(err, domain.ErrInvalidPassword) {
		t.Fatalf("login with old password error = %v", err)
	}
	if _, challenge, err := svc.Login(ctx, u.Username, "changed-password-123"); err != nil || !challenge.MFARequired {
		t.Fatalf("login with changed password challenge = %+v, error = %v", challenge, err)
	}
}

func TestMFALoginChallengeAttemptLimit(t *testing.T) {
	ctx := context.Background()
	repo := newMFAMemoryRepo()
	manager, err := infraMFA.New("command-test-mfa-encryption-key-value", "Test Community")
	if err != nil {
		t.Fatalf("new MFA manager: %v", err)
	}
	svc := NewService(repo, &fakeIDGen{next: 20_101}, nil, nil, "test-secret", time.Hour, 8, nil, nil, nil, manager)
	u, _, err := svc.Register(ctx, domain.RegisterCmd{
		Username: "mfa_limit_user",
		Email:    "mfa-limit-user@example.com",
		Password: "password-123",
		Nickname: "MFA Limit User",
	})
	if err != nil {
		t.Fatalf("register user: %v", err)
	}
	enrollment, err := svc.BeginTOTPEnrollment(ctx, u.ID, "password-123", "")
	if err != nil {
		t.Fatalf("begin enrollment: %v", err)
	}
	code, err := totp.GenerateCode(enrollment.Secret, time.Now())
	if err != nil {
		t.Fatalf("generate confirmation code: %v", err)
	}
	if _, err := svc.ConfirmTOTPEnrollment(ctx, u.ID, code); err != nil {
		t.Fatalf("confirm enrollment: %v", err)
	}
	_, challenge, err := svc.Login(ctx, "mfa_limit_user", "password-123")
	if err != nil {
		t.Fatalf("start MFA login: %v", err)
	}
	for attempt := 0; attempt < domain.MFAMaxLoginAttempts; attempt++ {
		if _, _, err := svc.CompleteMFALogin(ctx, challenge.MFAChallenge, "not-a-valid-code"); !errors.Is(err, domain.ErrMFACodeInvalid) {
			t.Fatalf("invalid MFA attempt %d error = %v", attempt+1, err)
		}
	}
	if _, _, err := svc.CompleteMFALogin(ctx, challenge.MFAChallenge, code); !errors.Is(err, domain.ErrMFAChallengeAttemptsExceeded) {
		t.Fatalf("MFA attempt after limit error = %v", err)
	}
}

func TestOAuthLoginRequiresEnabledMFAWithoutTouchingCommunityLogin(t *testing.T) {
	ctx := context.Background()
	repo := newMFAMemoryRepo()
	manager, err := infraMFA.New("command-test-mfa-encryption-key-value", "Test Community")
	if err != nil {
		t.Fatalf("new MFA manager: %v", err)
	}
	svc := NewService(repo, &fakeIDGen{next: 20_201}, nil, nil, "test-secret", time.Hour, 8, nil, nil, nil, manager)
	u, _, err := svc.Register(ctx, domain.RegisterCmd{
		Username: "oauth_mfa_user",
		Email:    "oauth-mfa-user@example.com",
		Password: "password-123",
		Nickname: "OAuth MFA User",
	})
	if err != nil {
		t.Fatalf("register user: %v", err)
	}
	oauthKey := [2]string{"github", "oauth-mfa-provider-user"}
	repo.oauthByKey[oauthKey] = u.ID
	repo.oauthAccount[oauthKey] = domain.OAuthAccount{
		Provider:       oauthKey[0],
		ProviderUserID: oauthKey[1],
		UserID:         u.ID,
	}

	enrollment, err := svc.BeginTOTPEnrollment(ctx, u.ID, "password-123", "")
	if err != nil {
		t.Fatalf("begin enrollment: %v", err)
	}
	confirmationCode, err := totp.GenerateCode(enrollment.Secret, time.Now())
	if err != nil {
		t.Fatalf("generate confirmation code: %v", err)
	}
	recoveryCodes, err := svc.ConfirmTOTPEnrollment(ctx, u.ID, confirmationCode)
	if err != nil {
		t.Fatalf("confirm enrollment: %v", err)
	}

	oauthUser, challenge, err := svc.OAuthLogin(ctx, domain.OAuthLoginCmd{
		Provider:       oauthKey[0],
		ProviderUserID: oauthKey[1],
		Username:       "refreshed-oauth-name",
	})
	if err != nil {
		t.Fatalf("start OAuth MFA login: %v", err)
	}
	if oauthUser == nil || oauthUser.ID != u.ID || !challenge.MFARequired || challenge.MFAChallenge == "" || challenge.Value != "" {
		t.Fatalf("OAuth MFA challenge = user:%+v token:%+v", oauthUser, challenge)
	}
	if stored := repo.users[u.ID]; stored.LastLoginAt != nil {
		t.Fatalf("community last login changed before MFA completion: %v", stored.LastLoginAt)
	}
	if account := repo.oauthAccount[oauthKey]; account.Username != "refreshed-oauth-name" || account.LastLoginAt == nil {
		t.Fatalf("OAuth account was not refreshed after provider authentication: %+v", account)
	}

	completedUser, token, err := svc.CompleteMFALogin(ctx, challenge.MFAChallenge, recoveryCodes[0])
	if err != nil {
		t.Fatalf("complete OAuth MFA login: %v", err)
	}
	if completedUser == nil || completedUser.LastLoginAt == nil || token.Value == "" || token.MFARequired {
		t.Fatalf("completed OAuth MFA login = user:%+v token:%+v", completedUser, token)
	}
}

type mfaMemoryRepo struct {
	*memoryRepo
	mu         sync.Mutex
	states     map[int64]domain.MFAState
	recovery   map[int64]map[string]bool
	challenges map[string]domain.MFALoginChallenge
}

func newMFAMemoryRepo() *mfaMemoryRepo {
	return &mfaMemoryRepo{
		memoryRepo: newMemoryRepo(),
		states:     map[int64]domain.MFAState{},
		recovery:   map[int64]map[string]bool{},
		challenges: map[string]domain.MFALoginChallenge{},
	}
}

func (r *mfaMemoryRepo) GetMFAState(_ context.Context, userID int64) (domain.MFAState, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	state, ok := r.states[userID]
	if !ok {
		return domain.MFAState{}, domain.ErrNotFound
	}
	remaining := int64(0)
	for _, used := range r.recovery[userID] {
		if !used {
			remaining++
		}
	}
	state.RecoveryCodesRemaining = remaining
	return state, nil
}

func (r *mfaMemoryRepo) SavePendingTOTP(_ context.Context, userID int64, ciphertext string, now time.Time) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	state := r.states[userID]
	state.UserID = userID
	state.PendingSecretCiphertext = ciphertext
	state.UpdatedAt = now
	if state.EnabledAt == nil {
		state.LastTOTPStep = -1
	}
	r.states[userID] = state
	return nil
}

func (r *mfaMemoryRepo) EnableTOTP(_ context.Context, userID int64, expected string, hashes []string, now time.Time) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	state, ok := r.states[userID]
	if !ok || state.PendingSecretCiphertext != expected {
		return domain.ErrMFAEnrollmentNotStarted
	}
	enabledAt := now
	state.SecretCiphertext = expected
	state.PendingSecretCiphertext = ""
	state.EnabledAt = &enabledAt
	state.LastTOTPStep = -1
	state.UpdatedAt = now
	r.states[userID] = state
	r.recovery[userID] = recoveryCodeMap(hashes)
	return nil
}

func (r *mfaMemoryRepo) ReplaceMFARecoveryCodes(_ context.Context, userID int64, hashes []string, _ time.Time) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	state, ok := r.states[userID]
	if !ok || !state.Enabled() {
		return domain.ErrMFANotEnabled
	}
	r.recovery[userID] = recoveryCodeMap(hashes)
	return nil
}

func (r *mfaMemoryRepo) DisableTOTP(_ context.Context, userID int64) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	state, ok := r.states[userID]
	if !ok || !state.Enabled() {
		return domain.ErrMFANotEnabled
	}
	delete(r.states, userID)
	delete(r.recovery, userID)
	for token, challenge := range r.challenges {
		if challenge.UserID == userID && challenge.UsedAt == nil {
			delete(r.challenges, token)
		}
	}
	return nil
}

func (r *mfaMemoryRepo) UseTOTPStep(_ context.Context, userID int64, step int64, now time.Time) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	state, ok := r.states[userID]
	if !ok || !state.Enabled() || state.LastTOTPStep >= step {
		return domain.ErrMFACodeReplayed
	}
	state.LastTOTPStep = step
	state.UpdatedAt = now
	r.states[userID] = state
	return nil
}

func (r *mfaMemoryRepo) UseMFARecoveryCode(_ context.Context, userID int64, hash string, _ time.Time) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.useRecoveryCodeLocked(userID, hash)
}

func (r *mfaMemoryRepo) CreateMFALoginChallenge(_ context.Context, challenge domain.MFALoginChallenge) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, exists := r.challenges[challenge.TokenHash]; exists {
		return domain.ErrMFAChallengeInvalid
	}
	r.challenges[challenge.TokenHash] = challenge
	return nil
}

func (r *mfaMemoryRepo) GetMFALoginChallenge(_ context.Context, tokenHash string, now time.Time) (domain.MFALoginChallenge, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	challenge, ok := r.challenges[tokenHash]
	if !ok {
		return domain.MFALoginChallenge{}, domain.ErrMFAChallengeInvalid
	}
	if err := validateMemoryChallenge(challenge, now); err != nil {
		return domain.MFALoginChallenge{}, err
	}
	return challenge, nil
}

func (r *mfaMemoryRepo) RecordMFALoginFailure(_ context.Context, tokenHash string, now time.Time) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	challenge, ok := r.challenges[tokenHash]
	if !ok {
		return domain.ErrMFAChallengeInvalid
	}
	if err := validateMemoryChallenge(challenge, now); err != nil {
		return err
	}
	challenge.Attempts++
	if challenge.Attempts >= domain.MFAMaxLoginAttempts {
		usedAt := now
		challenge.UsedAt = &usedAt
	}
	r.challenges[tokenHash] = challenge
	return nil
}

func (r *mfaMemoryRepo) CompleteMFALoginWithTOTP(_ context.Context, tokenHash string, userID int64, step int64, now time.Time) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	challenge, ok := r.challenges[tokenHash]
	if !ok || challenge.UserID != userID {
		return domain.ErrMFAChallengeInvalid
	}
	if err := validateMemoryChallenge(challenge, now); err != nil {
		return err
	}
	state, ok := r.states[userID]
	if !ok || !state.Enabled() || state.LastTOTPStep >= step {
		return domain.ErrMFACodeReplayed
	}
	state.LastTOTPStep = step
	state.UpdatedAt = now
	r.states[userID] = state
	usedAt := now
	challenge.UsedAt = &usedAt
	r.challenges[tokenHash] = challenge
	return nil
}

func (r *mfaMemoryRepo) CompleteMFALoginWithRecoveryCode(_ context.Context, tokenHash string, userID int64, hash string, now time.Time) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	challenge, ok := r.challenges[tokenHash]
	if !ok || challenge.UserID != userID {
		return domain.ErrMFAChallengeInvalid
	}
	if err := validateMemoryChallenge(challenge, now); err != nil {
		return err
	}
	if err := r.useRecoveryCodeLocked(userID, hash); err != nil {
		return err
	}
	usedAt := now
	challenge.UsedAt = &usedAt
	r.challenges[tokenHash] = challenge
	return nil
}

func (r *mfaMemoryRepo) useRecoveryCodeLocked(userID int64, hash string) error {
	used, ok := r.recovery[userID][hash]
	if !ok || used {
		return domain.ErrMFACodeInvalid
	}
	r.recovery[userID][hash] = true
	return nil
}

func recoveryCodeMap(hashes []string) map[string]bool {
	result := make(map[string]bool, len(hashes))
	for _, hash := range hashes {
		result[hash] = false
	}
	return result
}

func validateMemoryChallenge(challenge domain.MFALoginChallenge, now time.Time) error {
	if !challenge.ExpiresAt.After(now) {
		return domain.ErrMFAChallengeExpired
	}
	if challenge.Attempts >= domain.MFAMaxLoginAttempts {
		return domain.ErrMFAChallengeAttemptsExceeded
	}
	if challenge.UsedAt != nil {
		return domain.ErrMFAChallengeInvalid
	}
	return nil
}
