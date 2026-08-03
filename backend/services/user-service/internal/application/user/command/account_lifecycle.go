package command

import (
	"context"
	"errors"
	"time"

	domain "user-service/internal/domain/user"
)

const accountDeletionPolicyVersion int32 = 1

func (s *Service) GetAccountLifecycle(ctx context.Context, userID int64) (domain.AccountLifecycle, error) {
	repo, err := s.accountLifecycleRepository()
	if err != nil {
		return domain.AccountLifecycle{}, err
	}
	return repo.GetAccountLifecycle(ctx, userID)
}

func (s *Service) RequestAccountDeletion(ctx context.Context, userID int64, password, code string) (domain.AccountLifecycle, error) {
	repo, err := s.accountLifecycleRepository()
	if err != nil {
		return domain.AccountLifecycle{}, err
	}
	u, err := s.authenticatePassword(ctx, userID, password)
	if err != nil {
		return domain.AccountLifecycle{}, err
	}
	lifecycle, err := repo.GetAccountLifecycle(ctx, userID)
	if err != nil {
		return domain.AccountLifecycle{}, err
	}
	if lifecycle.Protected {
		return domain.AccountLifecycle{}, domain.ErrAccountProtected
	}
	if lifecycle.State != domain.AccountStateActive {
		return domain.AccountLifecycle{}, accountLifecycleStateError(lifecycle.State)
	}
	if err := s.verifyMFAIfEnabled(ctx, userID, code); err != nil {
		return domain.AccountLifecycle{}, err
	}
	if s.idgen == nil {
		return domain.AccountLifecycle{}, domain.ErrAccountLifecycleRepositoryUnavailable
	}
	credentialVersion, err := newCredentialVersion()
	if err != nil {
		return domain.AccountLifecycle{}, err
	}
	requestedAt := time.Now()
	result, err := repo.RequestAccountDeletion(ctx, domain.AccountDeletionRequest{
		JobID: s.idgen.Generate(), UserID: userID, ActorUserID: userID,
		ExpectedCredentialVersion: domain.NormalizeCredentialVersion(u.CredentialVersion), CredentialVersion: credentialVersion,
		RequestedAt: requestedAt, PolicyVersion: accountDeletionPolicyVersion, Steps: domain.AccountDeletionSteps(),
	})
	if err != nil {
		return domain.AccountLifecycle{}, err
	}
	s.refreshCredentialVersionCache(ctx, userID, result.CredentialVersion)
	return result, nil
}

func (s *Service) verifyMFAIfEnabled(ctx context.Context, userID int64, code string) error {
	repo, err := s.mfaRepository()
	if err != nil {
		return err
	}
	state, err := repo.GetMFAState(ctx, userID)
	if errors.Is(err, domain.ErrNotFound) {
		return nil
	}
	if err != nil {
		return err
	}
	if !state.Enabled() {
		return nil
	}
	return s.verifyAndUseMFA(ctx, repo, state, code, time.Now())
}

func (s *Service) accountLifecycleRepository() (domain.AccountLifecycleRepository, error) {
	if s == nil || s.repo == nil {
		return nil, domain.ErrAccountLifecycleRepositoryUnavailable
	}
	repo, ok := s.repo.(domain.AccountLifecycleRepository)
	if !ok {
		return nil, domain.ErrAccountLifecycleRepositoryUnavailable
	}
	return repo, nil
}

func accountLifecycleStateError(state domain.AccountState) error {
	switch domain.NormalizeAccountState(state) {
	case domain.AccountStateSuspended:
		return domain.ErrAccountSuspended
	case domain.AccountStateDeletionPending:
		return domain.ErrAccountDeletionPending
	case domain.AccountStateAnonymized:
		return domain.ErrAccountAnonymized
	default:
		return domain.ErrInvalidAccountState
	}
}
