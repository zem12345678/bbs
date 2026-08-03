package command

import (
	"context"
	"errors"
	"strings"
	"time"

	domain "user-service/internal/domain/user"
)

type PasskeyManager interface {
	NewChallenge() (string, string, error)
	HashChallenge(token string) string
	BeginRegistration(user domain.PasskeyUser) (domain.PasskeyCeremony, error)
	FinishRegistration(user domain.PasskeyUser, sessionCiphertext string, responseJSON string) (domain.PasskeyCredential, error)
	BeginLogin(user domain.PasskeyUser) (domain.PasskeyCeremony, error)
	FinishLogin(user domain.PasskeyUser, sessionCiphertext string, responseJSON string) (domain.PasskeyCredential, error)
	BeginPasswordlessLogin() (domain.PasskeyCeremony, error)
	FinishPasswordlessLogin(sessionCiphertext string, responseJSON string, lookup func(credentialID string, userID int64) (domain.PasskeyUser, error)) (int64, domain.PasskeyCredential, error)
}

type PasskeyListResult struct {
	PasswordlessEnabled bool
	Credentials         []domain.PasskeyCredential
}

type PasskeyOptionsResult struct {
	Challenge   string
	OptionsJSON string
	ExpiresAt   time.Time
}

func (s *Service) ListPasskeys(ctx context.Context, userID int64) (PasskeyListResult, error) {
	repo, err := s.passkeyRepository()
	if err != nil {
		return PasskeyListResult{}, err
	}
	state, err := repo.GetPasskeyState(ctx, userID)
	if errors.Is(err, domain.ErrMFANotEnabled) {
		return PasskeyListResult{}, nil
	}
	if err != nil {
		return PasskeyListResult{}, err
	}
	return PasskeyListResult{PasswordlessEnabled: state.PasswordlessEnabled, Credentials: state.Credentials}, nil
}

func (s *Service) BeginPasskeyRegistration(ctx context.Context, userID int64, name string, password string, code string) (PasskeyOptionsResult, error) {
	name, err := validatePasskeyName(name)
	if err != nil {
		return PasskeyOptionsResult{}, err
	}
	passkeyRepo, err := s.passkeyRepository()
	if err != nil {
		return PasskeyOptionsResult{}, err
	}
	mfaRepo, err := s.mfaRepository()
	if err != nil {
		return PasskeyOptionsResult{}, err
	}
	u, err := s.authenticatePassword(ctx, userID, password)
	if err != nil {
		return PasskeyOptionsResult{}, err
	}
	mfaState, err := mfaRepo.GetMFAState(ctx, userID)
	if errors.Is(err, domain.ErrNotFound) || (err == nil && !mfaState.Enabled()) {
		return PasskeyOptionsResult{}, domain.ErrMFANotEnabled
	}
	if err != nil {
		return PasskeyOptionsResult{}, err
	}
	state, err := passkeyRepo.GetPasskeyState(ctx, userID)
	if err != nil {
		return PasskeyOptionsResult{}, err
	}
	if len(state.Credentials) >= domain.PasskeyMaxCredentials {
		return PasskeyOptionsResult{}, domain.ErrPasskeyLimitReached
	}
	if err := s.verifyAndUseMFA(ctx, mfaRepo, mfaState, code, time.Now()); err != nil {
		return PasskeyOptionsResult{}, err
	}
	manager, err := s.passkeyManager()
	if err != nil {
		return PasskeyOptionsResult{}, err
	}
	ceremony, err := manager.BeginRegistration(toPasskeyUser(u, state.Credentials))
	if err != nil {
		return PasskeyOptionsResult{}, domain.ErrPasskeyVerificationFailed
	}
	rawToken, tokenHash, err := manager.NewChallenge()
	if err != nil {
		return PasskeyOptionsResult{}, err
	}
	now := time.Now()
	if err := passkeyRepo.CreatePasskeyChallenge(ctx, domain.PasskeyChallenge{
		TokenHash:         tokenHash,
		Ceremony:          domain.PasskeyCeremonyRegistration,
		UserID:            userID,
		PasskeyName:       name,
		SessionCiphertext: ceremony.SessionCiphertext,
		ExpiresAt:         ceremony.ExpiresAt,
		CreatedAt:         now,
	}); err != nil {
		return PasskeyOptionsResult{}, err
	}
	return PasskeyOptionsResult{Challenge: rawToken, OptionsJSON: ceremony.OptionsJSON, ExpiresAt: ceremony.ExpiresAt}, nil
}

func (s *Service) FinishPasskeyRegistration(ctx context.Context, userID int64, challengeToken string, responseJSON string) (domain.PasskeyCredential, error) {
	repo, err := s.passkeyRepository()
	if err != nil {
		return domain.PasskeyCredential{}, err
	}
	manager, err := s.passkeyManager()
	if err != nil {
		return domain.PasskeyCredential{}, err
	}
	now := time.Now()
	tokenHash := manager.HashChallenge(challengeToken)
	challenge, err := repo.GetPasskeyChallenge(ctx, tokenHash, now)
	if err != nil {
		return domain.PasskeyCredential{}, err
	}
	if challenge.Ceremony != domain.PasskeyCeremonyRegistration || challenge.UserID != userID {
		return domain.PasskeyCredential{}, domain.ErrPasskeyChallengeInvalid
	}
	state, err := repo.GetPasskeyState(ctx, userID)
	if err != nil {
		return domain.PasskeyCredential{}, err
	}
	u, err := s.repo.FindByID(ctx, userID)
	if err != nil {
		return domain.PasskeyCredential{}, err
	}
	credential, err := manager.FinishRegistration(toPasskeyUser(u, state.Credentials), challenge.SessionCiphertext, responseJSON)
	if err != nil {
		return domain.PasskeyCredential{}, s.recordPasskeyVerificationFailure(ctx, repo, tokenHash, now)
	}
	if err := repo.CreatePasskeyFromChallenge(ctx, tokenHash, userID, credential, now); err != nil {
		return domain.PasskeyCredential{}, err
	}
	updated, err := repo.GetPasskeyState(ctx, userID)
	if err != nil {
		return domain.PasskeyCredential{}, err
	}
	for _, stored := range updated.Credentials {
		if stored.CredentialID == credential.CredentialID {
			return stored, nil
		}
	}
	return domain.PasskeyCredential{}, domain.ErrPasskeyNotFound
}

func (s *Service) UpdatePasskeyName(ctx context.Context, userID int64, credentialID string, name string) (domain.PasskeyCredential, error) {
	name, err := validatePasskeyName(name)
	if err != nil {
		return domain.PasskeyCredential{}, err
	}
	repo, err := s.passkeyRepository()
	if err != nil {
		return domain.PasskeyCredential{}, err
	}
	return repo.UpdatePasskeyName(ctx, userID, credentialID, name, time.Now())
}

func (s *Service) DeletePasskey(ctx context.Context, userID int64, credentialID string, password string, code string) error {
	passkeyRepo, err := s.passkeyRepository()
	if err != nil {
		return err
	}
	mfaRepo, err := s.mfaRepository()
	if err != nil {
		return err
	}
	if _, err := s.authenticatePassword(ctx, userID, password); err != nil {
		return err
	}
	state, err := passkeyRepo.GetPasskeyState(ctx, userID)
	if err != nil {
		return err
	}
	found := false
	for _, credential := range state.Credentials {
		if credential.CredentialID == strings.TrimSpace(credentialID) {
			found = true
			break
		}
	}
	if !found {
		return domain.ErrPasskeyNotFound
	}
	mfaState, err := mfaRepo.GetMFAState(ctx, userID)
	if err != nil || !mfaState.Enabled() {
		if errors.Is(err, domain.ErrNotFound) || (err == nil && !mfaState.Enabled()) {
			return domain.ErrMFANotEnabled
		}
		return err
	}
	if err := s.verifyAndUseMFA(ctx, mfaRepo, mfaState, code, time.Now()); err != nil {
		return err
	}
	return passkeyRepo.DeletePasskey(ctx, userID, credentialID, time.Now())
}

func (s *Service) SetPasskeyPasswordless(ctx context.Context, userID int64, enabled bool, password string, code string) error {
	passkeyRepo, err := s.passkeyRepository()
	if err != nil {
		return err
	}
	mfaRepo, err := s.mfaRepository()
	if err != nil {
		return err
	}
	if _, err := s.authenticatePassword(ctx, userID, password); err != nil {
		return err
	}
	state, err := passkeyRepo.GetPasskeyState(ctx, userID)
	if err != nil {
		return err
	}
	if enabled && len(state.Credentials) == 0 {
		return domain.ErrPasskeyPasswordlessUnavailable
	}
	mfaState, err := mfaRepo.GetMFAState(ctx, userID)
	if err != nil || !mfaState.Enabled() {
		if errors.Is(err, domain.ErrNotFound) || (err == nil && !mfaState.Enabled()) {
			return domain.ErrMFANotEnabled
		}
		return err
	}
	if err := s.verifyAndUseMFA(ctx, mfaRepo, mfaState, code, time.Now()); err != nil {
		return err
	}
	return passkeyRepo.SetPasskeyPasswordless(ctx, userID, enabled, time.Now())
}

func (s *Service) BeginPasskeyMFALogin(ctx context.Context, mfaChallengeToken string) (PasskeyOptionsResult, error) {
	passkeyRepo, err := s.passkeyRepository()
	if err != nil {
		return PasskeyOptionsResult{}, err
	}
	mfaRepo, err := s.mfaRepository()
	if err != nil {
		return PasskeyOptionsResult{}, err
	}
	manager, err := s.passkeyManager()
	if err != nil {
		return PasskeyOptionsResult{}, err
	}
	if s.mfa == nil {
		return PasskeyOptionsResult{}, domain.ErrMFAEncryptionUnavailable
	}
	now := time.Now()
	mfaTokenHash := s.mfa.HashChallenge(mfaChallengeToken)
	mfaChallenge, err := mfaRepo.GetMFALoginChallenge(ctx, mfaTokenHash, now)
	if err != nil {
		return PasskeyOptionsResult{}, err
	}
	state, err := passkeyRepo.GetPasskeyState(ctx, mfaChallenge.UserID)
	if err != nil {
		return PasskeyOptionsResult{}, err
	}
	if len(state.Credentials) == 0 {
		return PasskeyOptionsResult{}, domain.ErrPasskeyNotFound
	}
	u, err := s.repo.FindByID(ctx, mfaChallenge.UserID)
	if err != nil {
		return PasskeyOptionsResult{}, err
	}
	ceremony, err := manager.BeginLogin(toPasskeyUser(u, state.Credentials))
	if err != nil {
		return PasskeyOptionsResult{}, domain.ErrPasskeyVerificationFailed
	}
	rawToken, tokenHash, err := manager.NewChallenge()
	if err != nil {
		return PasskeyOptionsResult{}, err
	}
	if err := passkeyRepo.CreatePasskeyChallenge(ctx, domain.PasskeyChallenge{
		TokenHash:         tokenHash,
		Ceremony:          domain.PasskeyCeremonyMFA,
		UserID:            mfaChallenge.UserID,
		MFATokenHash:      mfaTokenHash,
		SessionCiphertext: ceremony.SessionCiphertext,
		ExpiresAt:         ceremony.ExpiresAt,
		CreatedAt:         now,
	}); err != nil {
		return PasskeyOptionsResult{}, err
	}
	return PasskeyOptionsResult{Challenge: rawToken, OptionsJSON: ceremony.OptionsJSON, ExpiresAt: ceremony.ExpiresAt}, nil
}

func (s *Service) CompletePasskeyMFALogin(ctx context.Context, challengeToken string, responseJSON string) (*domain.User, AuthToken, error) {
	repo, err := s.passkeyRepository()
	if err != nil {
		return nil, AuthToken{}, err
	}
	manager, err := s.passkeyManager()
	if err != nil {
		return nil, AuthToken{}, err
	}
	now := time.Now()
	tokenHash := manager.HashChallenge(challengeToken)
	challenge, err := repo.GetPasskeyChallenge(ctx, tokenHash, now)
	if err != nil {
		return nil, AuthToken{}, err
	}
	if challenge.Ceremony != domain.PasskeyCeremonyMFA || challenge.UserID <= 0 || challenge.MFATokenHash == "" {
		return nil, AuthToken{}, domain.ErrPasskeyChallengeInvalid
	}
	state, err := repo.GetPasskeyState(ctx, challenge.UserID)
	if err != nil {
		return nil, AuthToken{}, err
	}
	u, err := s.repo.FindByID(ctx, challenge.UserID)
	if err != nil {
		return nil, AuthToken{}, err
	}
	credential, err := manager.FinishLogin(toPasskeyUser(u, state.Credentials), challenge.SessionCiphertext, responseJSON)
	if err != nil {
		return nil, AuthToken{}, s.recordPasskeyVerificationFailure(ctx, repo, tokenHash, now)
	}
	if err := repo.CompletePasskeyMFALogin(ctx, tokenHash, challenge.MFATokenHash, challenge.UserID, credential, credential.Version, now); err != nil {
		return nil, AuthToken{}, err
	}
	return s.issuePasskeyLogin(ctx, challenge.UserID, now)
}

func (s *Service) BeginPasswordlessPasskeyLogin(ctx context.Context) (PasskeyOptionsResult, error) {
	repo, err := s.passkeyRepository()
	if err != nil {
		return PasskeyOptionsResult{}, err
	}
	manager, err := s.passkeyManager()
	if err != nil {
		return PasskeyOptionsResult{}, err
	}
	ceremony, err := manager.BeginPasswordlessLogin()
	if err != nil {
		return PasskeyOptionsResult{}, domain.ErrPasskeyVerificationFailed
	}
	rawToken, tokenHash, err := manager.NewChallenge()
	if err != nil {
		return PasskeyOptionsResult{}, err
	}
	now := time.Now()
	if err := repo.CreatePasskeyChallenge(ctx, domain.PasskeyChallenge{
		TokenHash:         tokenHash,
		Ceremony:          domain.PasskeyCeremonyPasswordless,
		SessionCiphertext: ceremony.SessionCiphertext,
		ExpiresAt:         ceremony.ExpiresAt,
		CreatedAt:         now,
	}); err != nil {
		return PasskeyOptionsResult{}, err
	}
	return PasskeyOptionsResult{Challenge: rawToken, OptionsJSON: ceremony.OptionsJSON, ExpiresAt: ceremony.ExpiresAt}, nil
}

func (s *Service) CompletePasswordlessPasskeyLogin(ctx context.Context, challengeToken string, responseJSON string) (*domain.User, AuthToken, error) {
	repo, err := s.passkeyRepository()
	if err != nil {
		return nil, AuthToken{}, err
	}
	manager, err := s.passkeyManager()
	if err != nil {
		return nil, AuthToken{}, err
	}
	now := time.Now()
	tokenHash := manager.HashChallenge(challengeToken)
	challenge, err := repo.GetPasskeyChallenge(ctx, tokenHash, now)
	if err != nil {
		return nil, AuthToken{}, err
	}
	if challenge.Ceremony != domain.PasskeyCeremonyPasswordless {
		return nil, AuthToken{}, domain.ErrPasskeyChallengeInvalid
	}
	userID, credential, err := manager.FinishPasswordlessLogin(challenge.SessionCiphertext, responseJSON, func(credentialID string, handleUserID int64) (domain.PasskeyUser, error) {
		stored, lookupErr := repo.GetPasskeyByCredentialID(ctx, credentialID)
		if lookupErr != nil || stored.UserID != handleUserID {
			return domain.PasskeyUser{}, domain.ErrPasskeyVerificationFailed
		}
		state, lookupErr := repo.GetPasskeyState(ctx, handleUserID)
		if lookupErr != nil || !state.PasswordlessEnabled {
			return domain.PasskeyUser{}, domain.ErrPasskeyPasswordlessUnavailable
		}
		u, lookupErr := s.repo.FindByID(ctx, handleUserID)
		if lookupErr != nil {
			return domain.PasskeyUser{}, lookupErr
		}
		if lookupErr = u.EnsureActive(); lookupErr != nil {
			return domain.PasskeyUser{}, lookupErr
		}
		return toPasskeyUser(u, state.Credentials), nil
	})
	if err != nil {
		return nil, AuthToken{}, s.recordPasskeyVerificationFailure(ctx, repo, tokenHash, now)
	}
	if err := repo.CompletePasskeyPasswordlessLogin(ctx, tokenHash, userID, credential, credential.Version, now); err != nil {
		return nil, AuthToken{}, err
	}
	return s.issuePasskeyLogin(ctx, userID, now)
}

func (s *Service) issuePasskeyLogin(ctx context.Context, userID int64, now time.Time) (*domain.User, AuthToken, error) {
	u, err := s.repo.FindByID(ctx, userID)
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
	token, err := s.issueToken(u)
	if err != nil {
		return nil, AuthToken{}, err
	}
	return s.profileForAuthResponse(ctx, u), token, nil
}

func (s *Service) recordPasskeyVerificationFailure(ctx context.Context, repo domain.PasskeyRepository, tokenHash string, now time.Time) error {
	if err := repo.RecordPasskeyChallengeFailure(ctx, tokenHash, now); err != nil {
		return err
	}
	return domain.ErrPasskeyVerificationFailed
}

func (s *Service) passkeyRepository() (domain.PasskeyRepository, error) {
	if s == nil || s.repo == nil {
		return nil, domain.ErrPasskeyRepositoryUnavailable
	}
	repo, ok := s.repo.(domain.PasskeyRepository)
	if !ok {
		return nil, domain.ErrPasskeyRepositoryUnavailable
	}
	return repo, nil
}

func (s *Service) passkeyManager() (PasskeyManager, error) {
	if s == nil || s.passkeys == nil {
		return nil, domain.ErrPasskeyManagerUnavailable
	}
	return s.passkeys, nil
}

func toPasskeyUser(u *domain.User, credentials []domain.PasskeyCredential) domain.PasskeyUser {
	if u == nil {
		return domain.PasskeyUser{}
	}
	return domain.PasskeyUser{ID: u.ID, Username: u.Username, DisplayName: u.Nickname, Credentials: credentials}
}

func validatePasskeyName(name string) (string, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return "", domain.ErrPasskeyNameRequired
	}
	if len([]rune(name)) > domain.PasskeyNameMaxRunes {
		return "", domain.ErrPasskeyNameTooLong
	}
	return name, nil
}
