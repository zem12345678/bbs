package query

import (
	"context"

	domain "user-service/internal/domain/user"
)

type UserListResult struct {
	Items []*domain.User
	Total int64
}

type ProfileEntitlementReader interface {
	HasActiveProfileTheme(ctx context.Context, userID int64, theme string) (bool, error)
	HasActiveMembership(ctx context.Context, userID int64) (bool, error)
}

type Service struct {
	repo         domain.Repository
	entitlements ProfileEntitlementReader
}

func NewService(repo domain.Repository, entitlements ProfileEntitlementReader) *Service {
	return &Service{repo: repo, entitlements: entitlements}
}

func (s *Service) Get(ctx context.Context, id int64) (*domain.User, error) {
	if id <= 0 {
		return nil, domain.ErrInvalidID
	}
	u, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}
	return s.profileForResponse(ctx, u), nil
}

func (s *Service) GetByUsername(ctx context.Context, username string) (*domain.User, error) {
	username = domain.NormalizeUsername(username)
	if !domain.ValidUsername(username) {
		return nil, domain.ErrUsernameInvalid
	}
	u, err := s.repo.FindByUsername(ctx, username)
	if err != nil {
		return nil, err
	}
	return s.profileForResponse(ctx, u), nil
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
	return UserListResult{Items: s.profilesForResponse(ctx, items), Total: total}, nil
}

func (s *Service) ListFollowers(ctx context.Context, q domain.FollowListQuery) (UserListResult, error) {
	if q.UserID <= 0 {
		return UserListResult{}, domain.ErrInvalidID
	}
	items, total, err := s.repo.ListFollowers(ctx, q)
	if err != nil {
		return UserListResult{}, err
	}
	return UserListResult{Items: s.profilesForResponse(ctx, items), Total: total}, nil
}

func (s *Service) ListFollowing(ctx context.Context, q domain.FollowListQuery) (UserListResult, error) {
	if q.UserID <= 0 {
		return UserListResult{}, domain.ErrInvalidID
	}
	items, total, err := s.repo.ListFollowing(ctx, q)
	if err != nil {
		return UserListResult{}, err
	}
	return UserListResult{Items: s.profilesForResponse(ctx, items), Total: total}, nil
}

func (s *Service) profilesForResponse(ctx context.Context, items []*domain.User) []*domain.User {
	out := make([]*domain.User, 0, len(items))
	for _, item := range items {
		out = append(out, s.profileForResponse(ctx, item))
	}
	return out
}

func (s *Service) profileForResponse(ctx context.Context, u *domain.User) *domain.User {
	if u == nil {
		return nil
	}
	cp := *u
	if cp.BackgroundURL != "" && !s.hasActiveMembership(ctx, cp.ID) {
		cp.BackgroundURL = ""
	}
	if domain.NormalizeProfileTheme(cp.ProfileTheme) == domain.ProfileThemePro && !s.hasActiveProfileTheme(ctx, cp.ID) {
		cp.ProfileTheme = domain.ProfileThemeDefault
	}
	return &cp
}

func (s *Service) hasActiveProfileTheme(ctx context.Context, userID int64) bool {
	if s.entitlements == nil {
		return false
	}
	active, err := s.entitlements.HasActiveProfileTheme(ctx, userID, domain.ProfileThemePro)
	return err == nil && active
}

func (s *Service) hasActiveMembership(ctx context.Context, userID int64) bool {
	if s.entitlements == nil {
		return false
	}
	active, err := s.entitlements.HasActiveMembership(ctx, userID)
	return err == nil && active
}
