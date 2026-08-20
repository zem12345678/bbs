package query

import (
	"context"

	domain "user-service/internal/domain/user"
)

func (s *Service) GetFollowing(ctx context.Context, followerID, followeeID int64) (*domain.Following, error) {
	if followerID <= 0 || followeeID <= 0 {
		return nil, domain.ErrInvalidID
	}
	if followerID == followeeID {
		return nil, domain.ErrCannotFollowSelf
	}
	repo, err := s.followingRepository()
	if err != nil {
		return nil, err
	}
	edge, err := repo.GetFollowing(ctx, followerID, followeeID)
	if err != nil {
		return nil, err
	}
	s.shapeFollowingProfiles(ctx, []*domain.Following{edge})
	return edge, nil
}

func (s *Service) ListFollowingEdges(ctx context.Context, input domain.FollowingQuery) ([]*domain.Following, error) {
	if err := input.Normalize(); err != nil {
		return nil, err
	}
	repo, err := s.followingRepository()
	if err != nil {
		return nil, err
	}
	edges, err := repo.ListFollowingEdges(ctx, input)
	if err != nil {
		return nil, err
	}
	s.shapeFollowingProfiles(ctx, edges)
	return edges, nil
}

func (s *Service) ListFollowerEdges(ctx context.Context, input domain.FollowingQuery) ([]*domain.Following, error) {
	if err := input.Normalize(); err != nil {
		return nil, err
	}
	repo, err := s.followingRepository()
	if err != nil {
		return nil, err
	}
	edges, err := repo.ListFollowerEdges(ctx, input)
	if err != nil {
		return nil, err
	}
	s.shapeFollowingProfiles(ctx, edges)
	return edges, nil
}

func (s *Service) followingRepository() (domain.FollowingRepository, error) {
	repo, ok := s.repo.(domain.FollowingRepository)
	if !ok {
		return nil, domain.ErrFollowingRepositoryUnavailable
	}
	return repo, nil
}

func (s *Service) shapeFollowingProfiles(ctx context.Context, edges []*domain.Following) {
	for _, edge := range edges {
		if edge == nil {
			continue
		}
		if edge.Follower != nil {
			edge.Follower = s.profileForResponse(ctx, edge.Follower)
		}
		if edge.Followee != nil {
			edge.Followee = s.profileForResponse(ctx, edge.Followee)
		}
	}
}
