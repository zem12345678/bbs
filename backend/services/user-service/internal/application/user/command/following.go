package command

import (
	"context"

	domain "user-service/internal/domain/user"
)

func (s *Service) UpdateFollowing(ctx context.Context, followerID, followeeID int64, patch domain.FollowingPatch) (*domain.Following, error) {
	if followerID <= 0 || followeeID <= 0 {
		return nil, domain.ErrInvalidID
	}
	if followerID == followeeID {
		return nil, domain.ErrCannotFollowSelf
	}
	if err := patch.Validate(); err != nil {
		return nil, err
	}
	if err := s.ensureActiveUser(ctx, followerID); err != nil {
		return nil, err
	}
	if err := s.ensureActiveUser(ctx, followeeID); err != nil {
		return nil, err
	}
	repo, err := s.followingRepository()
	if err != nil {
		return nil, err
	}
	return repo.UpdateFollowing(ctx, followerID, followeeID, patch)
}

func (s *Service) UpdateAllFollowings(ctx context.Context, followerID int64, patch domain.FollowingPatch) error {
	if followerID <= 0 {
		return domain.ErrInvalidID
	}
	if err := patch.Validate(); err != nil {
		return err
	}
	if err := s.ensureActiveUser(ctx, followerID); err != nil {
		return err
	}
	repo, err := s.followingRepository()
	if err != nil {
		return err
	}
	return repo.UpdateAllFollowings(ctx, followerID, patch)
}

func (s *Service) followingRepository() (domain.FollowingRepository, error) {
	repo, ok := s.repo.(domain.FollowingRepository)
	if !ok {
		return nil, domain.ErrFollowingRepositoryUnavailable
	}
	return repo, nil
}
