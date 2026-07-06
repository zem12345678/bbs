package query

import (
	"context"

	domain "user-service/internal/domain/user"
)

type UserListResult struct {
	Items []*domain.User
	Total int64
}

type Service struct {
	repo domain.Repository
}

func NewService(repo domain.Repository) *Service {
	return &Service{repo: repo}
}

func (s *Service) Get(ctx context.Context, id int64) (*domain.User, error) {
	if id <= 0 {
		return nil, domain.ErrInvalidID
	}
	return s.repo.FindByID(ctx, id)
}

func (s *Service) GetByUsername(ctx context.Context, username string) (*domain.User, error) {
	username = domain.NormalizeUsername(username)
	if !domain.ValidUsername(username) {
		return nil, domain.ErrUsernameInvalid
	}
	return s.repo.FindByUsername(ctx, username)
}

func (s *Service) IsFollowing(ctx context.Context, followerID, followeeID int64) (bool, error) {
	if followerID <= 0 || followeeID <= 0 {
		return false, domain.ErrInvalidID
	}
	if followerID == followeeID {
		return false, nil
	}
	return s.repo.IsFollowing(ctx, followerID, followeeID)
}

func (s *Service) ListUsers(ctx context.Context, q domain.UserListQuery) (UserListResult, error) {
	items, total, err := s.repo.ListUsers(ctx, q)
	if err != nil {
		return UserListResult{}, err
	}
	return UserListResult{Items: items, Total: total}, nil
}

func (s *Service) ListFollowers(ctx context.Context, q domain.FollowListQuery) (UserListResult, error) {
	if q.UserID <= 0 {
		return UserListResult{}, domain.ErrInvalidID
	}
	items, total, err := s.repo.ListFollowers(ctx, q)
	if err != nil {
		return UserListResult{}, err
	}
	return UserListResult{Items: items, Total: total}, nil
}

func (s *Service) ListFollowing(ctx context.Context, q domain.FollowListQuery) (UserListResult, error) {
	if q.UserID <= 0 {
		return UserListResult{}, domain.ErrInvalidID
	}
	items, total, err := s.repo.ListFollowing(ctx, q)
	if err != nil {
		return UserListResult{}, err
	}
	return UserListResult{Items: items, Total: total}, nil
}
