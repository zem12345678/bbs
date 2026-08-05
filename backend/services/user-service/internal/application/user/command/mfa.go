package command

import (
	"context"
	"errors"
	"strings"
	"time"

	domain "user-service/internal/domain/user"

	"golang.org/x/crypto/bcrypt"
)

const (
	mfaLoginChallengeTTL = 5 * time.Minute
	mfaRecoveryCodeCount = 10
)

type MFAManager interface {
	NewTOTP(account string) (domain.TOTPEnrollment, error)
	EncryptSecret(secret string) (string, error)
	DecryptSecret(ciphertext string) (string, error)
	VerifyTOTP(secret string, code string, at time.Time) (int64, bool)
	NewRecoveryCodes(count int) ([]string, []string, error)
	NewChallenge() (string, string, error)
	HashChallenge(token string) string
	HashRecoveryCode(code string) string
}

type MFAStatusResult struct {
	Enabled                bool
	RecoveryCodesRemaining int64
	EnabledAt              time.Time
}

func (s *Service) MFAStatus(ctx context.Context, userID int64) (MFAStatusResult, error) {
	repo, err := s.mfaRepository()
	if err != nil {
		return MFAStatusResult{}, err
	}
	state, err := repo.GetMFAState(ctx, userID)
	if errors.Is(err, domain.ErrNotFound) {
		return MFAStatusResult{}, nil
	}
	if err != nil {
		return MFAStatusResult{}, err
	}
	result := MFAStatusResult{Enabled: state.Enabled(), RecoveryCodesRemaining: state.RecoveryCodesRemaining}
	if state.EnabledAt != nil {
		result.EnabledAt = *state.EnabledAt
	}
	return result, nil
}

func (s *Service) BeginTOTPEnrollment(ctx context.Context, userID int64, password string, currentCode string) (domain.TOTPEnrollment, error) {
	repo, err := s.mfaRepository()
	if err != nil {
		return domain.TOTPEnrollment{}, err
	}
	u, err := s.authenticatePassword(ctx, userID, password)
	if err != nil {
		return domain.TOTPEnrollment{}, err
	}
	state, stateErr := repo.GetMFAState(ctx, userID)
	if stateErr != nil && !errors.Is(stateErr, domain.ErrNotFound) {
		return domain.TOTPEnrollment{}, stateErr
	}
	if stateErr == nil && state.Enabled() {
		if strings.TrimSpace(currentCode) == "" {
			return domain.TOTPEnrollment{}, domain.ErrMFACodeInvalid
		}
		if err := s.verifyAndUseMFA(ctx, repo, state, currentCode, time.Now()); err != nil {
			return domain.TOTPEnrollment{}, err
		}
	}
	if s.mfa == nil {
		return domain.TOTPEnrollment{}, domain.ErrMFAEncryptionUnavailable
	}
	enrollment, err := s.mfa.NewTOTP(u.Username)
	if err != nil {
		return domain.TOTPEnrollment{}, err
	}
	ciphertext, err := s.mfa.EncryptSecret(enrollment.Secret)
	if err != nil {
		return domain.TOTPEnrollment{}, domain.ErrMFAEncryptionUnavailable
	}
	if err := repo.SavePendingTOTP(ctx, userID, ciphertext, time.Now()); err != nil {
		return domain.TOTPEnrollment{}, err
	}
	return enrollment, nil
}

func (s *Service) ConfirmTOTPEnrollment(ctx context.Context, userID int64, code string) ([]string, error) {
	repo, err := s.mfaRepository()
	if err != nil {
		return nil, err
	}
	if s.mfa == nil {
		return nil, domain.ErrMFAEncryptionUnavailable
	}
	state, err := repo.GetMFAState(ctx, userID)
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			return nil, domain.ErrMFAEnrollmentNotStarted
		}
		return nil, err
	}
	if strings.TrimSpace(state.PendingSecretCiphertext) == "" {
		return nil, domain.ErrMFAEnrollmentNotStarted
	}
	secret, err := s.mfa.DecryptSecret(state.PendingSecretCiphertext)
	if err != nil {
		return nil, domain.ErrMFAEncryptionUnavailable
	}
	if _, valid := s.mfa.VerifyTOTP(secret, code, time.Now()); !valid {
		return nil, domain.ErrMFACodeInvalid
	}
	codes, hashes, err := s.mfa.NewRecoveryCodes(mfaRecoveryCodeCount)
	if err != nil {
		return nil, err
	}
	if err := repo.EnableTOTP(ctx, userID, state.PendingSecretCiphertext, hashes, time.Now()); err != nil {
		return nil, err
	}
	return codes, nil
}

func (s *Service) RegenerateMFARecoveryCodes(ctx context.Context, userID int64, password string, code string) ([]string, error) {
	repo, err := s.mfaRepository()
	if err != nil {
		return nil, err
	}
	if _, err := s.authenticatePassword(ctx, userID, password); err != nil {
		return nil, err
	}
	state, err := repo.GetMFAState(ctx, userID)
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			return nil, domain.ErrMFANotEnabled
		}
		return nil, err
	}
	if !state.Enabled() {
		return nil, domain.ErrMFANotEnabled
	}
	if err := s.verifyAndUseMFA(ctx, repo, state, code, time.Now()); err != nil {
		return nil, err
	}
	codes, hashes, err := s.mfa.NewRecoveryCodes(mfaRecoveryCodeCount)
	if err != nil {
		return nil, err
	}
	if err := repo.ReplaceMFARecoveryCodes(ctx, userID, hashes, time.Now()); err != nil {
		return nil, err
	}
	return codes, nil
}

func (s *Service) DisableTOTP(ctx context.Context, userID int64, password string, code string) error {
	repo, err := s.mfaRepository()
	if err != nil {
		return err
	}
	if _, err := s.authenticatePassword(ctx, userID, password); err != nil {
		return err
	}
	state, err := repo.GetMFAState(ctx, userID)
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			return domain.ErrMFANotEnabled
		}
		return err
	}
	if !state.Enabled() {
		return domain.ErrMFANotEnabled
	}
	if err := s.verifyAndUseMFA(ctx, repo, state, code, time.Now()); err != nil {
		return err
	}
	return repo.DisableTOTP(ctx, userID)
}

func (s *Service) CompleteMFALogin(ctx context.Context, challengeToken string, code string) (*domain.User, AuthToken, error) {
	repo, err := s.mfaRepository()
	if err != nil {
		return nil, AuthToken{}, err
	}
	if s.mfa == nil {
		return nil, AuthToken{}, domain.ErrMFAEncryptionUnavailable
	}
	now := time.Now()
	tokenHash := s.mfa.HashChallenge(challengeToken)
	challenge, err := repo.GetMFALoginChallenge(ctx, tokenHash, now)
	if err != nil {
		return nil, AuthToken{}, err
	}
	state, err := repo.GetMFAState(ctx, challenge.UserID)
	if err != nil || !state.Enabled() {
		if errors.Is(err, domain.ErrNotFound) || (err == nil && !state.Enabled()) {
			return nil, AuthToken{}, domain.ErrMFANotEnabled
		}
		return nil, AuthToken{}, err
	}
	secret, err := s.mfa.DecryptSecret(state.SecretCiphertext)
	if err != nil {
		return nil, AuthToken{}, domain.ErrMFAEncryptionUnavailable
	}
	if step, valid := s.mfa.VerifyTOTP(secret, code, now); valid {
		err = repo.CompleteMFALoginWithTOTP(ctx, tokenHash, challenge.UserID, step, now)
	} else {
		err = repo.CompleteMFALoginWithRecoveryCode(ctx, tokenHash, challenge.UserID, s.mfa.HashRecoveryCode(code), now)
	}
	if err != nil {
		if errors.Is(err, domain.ErrMFACodeInvalid) {
			if failureErr := repo.RecordMFALoginFailure(ctx, tokenHash, now); failureErr != nil {
				return nil, AuthToken{}, failureErr
			}
		}
		return nil, AuthToken{}, err
	}
	u, err := s.repo.FindByID(ctx, challenge.UserID)
	if err != nil {
		return nil, AuthToken{}, err
	}
	if err := u.EnsureActive(); err != nil {
		return nil, AuthToken{}, err
	}
	u.TouchLogin(now)
	if err := s.repo.UpdateLastLogin(ctx, u); err != nil {
		return nil, AuthToken{}, err
	}
	token, err := s.issueToken(ctx, u, LoginMethodMFA)
	if err != nil {
		return nil, AuthToken{}, err
	}
	return s.profileForAuthResponse(ctx, u), token, nil
}

func (s *Service) beginMFALoginIfEnabled(ctx context.Context, u *domain.User) (AuthToken, bool, error) {
	repo, ok := s.repo.(domain.MFARepository)
	if !ok {
		return AuthToken{}, false, nil
	}
	state, err := repo.GetMFAState(ctx, u.ID)
	if errors.Is(err, domain.ErrNotFound) {
		return AuthToken{}, false, nil
	}
	if err != nil {
		return AuthToken{}, false, err
	}
	if !state.Enabled() {
		return AuthToken{}, false, nil
	}
	if s.mfa == nil {
		return AuthToken{}, false, domain.ErrMFAEncryptionUnavailable
	}
	rawToken, tokenHash, err := s.mfa.NewChallenge()
	if err != nil {
		return AuthToken{}, false, err
	}
	now := time.Now()
	expiresAt := now.Add(mfaLoginChallengeTTL)
	if err := repo.CreateMFALoginChallenge(ctx, domain.MFALoginChallenge{
		TokenHash: tokenHash,
		UserID:    u.ID,
		ExpiresAt: expiresAt,
		CreatedAt: now,
	}); err != nil {
		return AuthToken{}, false, err
	}
	return AuthToken{MFARequired: true, MFAChallenge: rawToken, MFAChallengeExpiry: expiresAt}, true, nil
}

func (s *Service) authenticatePassword(ctx context.Context, userID int64, password string) (*domain.User, error) {
	if userID <= 0 {
		return nil, domain.ErrInvalidID
	}
	u, err := s.repo.FindByID(ctx, userID)
	if err != nil {
		return nil, err
	}
	if err := u.EnsureActive(); err != nil {
		return nil, err
	}
	if bcrypt.CompareHashAndPassword([]byte(u.PasswordHash), []byte(password)) != nil {
		return nil, domain.ErrInvalidPassword
	}
	return u, nil
}

func (s *Service) verifyAndUseMFA(ctx context.Context, repo domain.MFARepository, state domain.MFAState, code string, now time.Time) error {
	if s.mfa == nil {
		return domain.ErrMFAEncryptionUnavailable
	}
	secret, err := s.mfa.DecryptSecret(state.SecretCiphertext)
	if err != nil {
		return domain.ErrMFAEncryptionUnavailable
	}
	if step, valid := s.mfa.VerifyTOTP(secret, code, now); valid {
		return repo.UseTOTPStep(ctx, state.UserID, step, now)
	}
	return repo.UseMFARecoveryCode(ctx, state.UserID, s.mfa.HashRecoveryCode(code), now)
}

func (s *Service) mfaRepository() (domain.MFARepository, error) {
	if s == nil || s.repo == nil {
		return nil, domain.ErrMFARepositoryUnavailable
	}
	repo, ok := s.repo.(domain.MFARepository)
	if !ok {
		return nil, domain.ErrMFARepositoryUnavailable
	}
	return repo, nil
}
