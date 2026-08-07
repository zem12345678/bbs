package command

import (
	"context"

	domain "user-service/internal/domain/user"
)

// AcceptFollowRequest approves a pending request addressed to targetID.
func (s *Service) AcceptFollowRequest(ctx context.Context, targetID, requesterID int64) error {
	repo, err := s.followRequestPair(targetID, requesterID)
	if err != nil {
		return err
	}
	if err := s.ensureActiveUser(ctx, targetID); err != nil {
		return err
	}
	if err := s.ensureActiveUser(ctx, requesterID); err != nil {
		return err
	}
	safetyRepo, ok := s.repo.(domain.SafetyRepository)
	if !ok {
		return domain.ErrSafetyRepositoryUnavailable
	}
	relation, err := safetyRepo.GetSafetyRelation(ctx, requesterID, targetID)
	if err != nil {
		return err
	}
	if relation.Blocked || relation.BlockedBy {
		return domain.ErrFollowBlocked
	}
	followCreated, err := repo.AcceptFollowRequest(ctx, requesterID, targetID)
	if err != nil {
		return err
	}
	if !followCreated {
		return nil
	}
	s.publishEvents(ctx,
		domain.NewFollowRequestAcceptedEvent(requesterID, targetID),
		domain.NewFollowedEvent(requesterID, targetID),
	)
	return nil
}

// RejectFollowRequest drops a pending request addressed to targetID.
func (s *Service) RejectFollowRequest(ctx context.Context, targetID, requesterID int64) error {
	repo, err := s.followRequestPair(targetID, requesterID)
	if err != nil {
		return err
	}
	return repo.DeleteFollowRequest(ctx, requesterID, targetID)
}

// CancelFollowRequest withdraws a request the actor sent earlier.
func (s *Service) CancelFollowRequest(ctx context.Context, requesterID, targetID int64) error {
	repo, err := s.followRequestPair(requesterID, targetID)
	if err != nil {
		return err
	}
	return repo.DeleteFollowRequest(ctx, requesterID, targetID)
}

// SetFollowApprovalRequired flips the private-account switch for userID.
func (s *Service) SetFollowApprovalRequired(ctx context.Context, userID int64, required bool) error {
	if userID <= 0 {
		return domain.ErrInvalidID
	}
	repo, err := s.followRequestRepository()
	if err != nil {
		return err
	}
	if err := s.ensureActiveUser(ctx, userID); err != nil {
		return err
	}
	return repo.SetFollowApprovalRequired(ctx, userID, required)
}

func (s *Service) ensureActiveUser(ctx context.Context, userID int64) error {
	u, err := s.repo.FindByID(ctx, userID)
	if err != nil {
		return err
	}
	return u.EnsureActive()
}

func (s *Service) followRequestPair(actorID, otherID int64) (domain.FollowRequestRepository, error) {
	if actorID <= 0 || otherID <= 0 {
		return nil, domain.ErrInvalidID
	}
	if actorID == otherID {
		return nil, domain.ErrCannotFollowSelf
	}
	return s.followRequestRepository()
}

func (s *Service) followRequestRepository() (domain.FollowRequestRepository, error) {
	repo, ok := s.repo.(domain.FollowRequestRepository)
	if !ok {
		return nil, domain.ErrFollowRequestRepositoryUnavailable
	}
	return repo, nil
}
