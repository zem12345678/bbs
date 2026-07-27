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
	ListActiveProfileThemeUserIDs(ctx context.Context, userIDs []int64, theme string) (map[int64]bool, error)
	ListActiveMembershipUserIDs(ctx context.Context, userIDs []int64) (map[int64]bool, error)
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

// GetCredentialVersion returns the opaque, PostgreSQL-authoritative version
// used to validate a user's signed credentials. It deliberately bypasses
// profile shaping because it is consumed by trusted internal callers only.
func (s *Service) GetCredentialVersion(ctx context.Context, userID int64) (string, error) {
	if userID <= 0 {
		return "", domain.ErrInvalidID
	}
	return s.repo.GetCredentialVersion(ctx, userID)
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
	membershipUserIDs := make([]int64, 0, len(items))
	themeUserIDs := make([]int64, 0, len(items))
	membershipSeen := make(map[int64]bool, len(items))
	themeSeen := make(map[int64]bool, len(items))
	for _, item := range items {
		if item == nil {
			out = append(out, nil)
			continue
		}
		profile := *item
		profile.ProfileTheme = domain.NormalizeProfileTheme(profile.ProfileTheme)
		if !domain.ValidProfileTheme(profile.ProfileTheme) {
			profile.ProfileTheme = domain.ProfileThemeDefault
		}
		if profile.BackgroundURL != "" && profile.ID > 0 && !membershipSeen[profile.ID] {
			membershipSeen[profile.ID] = true
			membershipUserIDs = append(membershipUserIDs, profile.ID)
		}
		if profile.ProfileTheme == domain.ProfileThemePro && profile.ID > 0 && !themeSeen[profile.ID] {
			themeSeen[profile.ID] = true
			themeUserIDs = append(themeUserIDs, profile.ID)
		}
		out = append(out, &profile)
	}
	membershipUsers, membershipAvailable := s.activeMembershipUsers(ctx, membershipUserIDs)
	themeUsers, themeAvailable := s.activeThemeUsers(ctx, themeUserIDs)
	for _, profile := range out {
		if profile == nil {
			continue
		}
		if profile.BackgroundURL != "" && (!membershipAvailable || !membershipUsers[profile.ID]) {
			profile.BackgroundURL = ""
		}
		if profile.ProfileTheme == domain.ProfileThemePro && (!themeAvailable || !themeUsers[profile.ID]) {
			profile.ProfileTheme = domain.ProfileThemeDefault
		}
	}
	return out
}

func (s *Service) activeMembershipUsers(ctx context.Context, userIDs []int64) (map[int64]bool, bool) {
	if len(userIDs) == 0 {
		return map[int64]bool{}, true
	}
	if s.entitlements == nil {
		return nil, false
	}
	active, err := s.entitlements.ListActiveMembershipUserIDs(ctx, userIDs)
	return active, err == nil
}

func (s *Service) activeThemeUsers(ctx context.Context, userIDs []int64) (map[int64]bool, bool) {
	if len(userIDs) == 0 {
		return map[int64]bool{}, true
	}
	if s.entitlements == nil {
		return nil, false
	}
	active, err := s.entitlements.ListActiveProfileThemeUserIDs(ctx, userIDs, domain.ProfileThemePro)
	return active, err == nil
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
