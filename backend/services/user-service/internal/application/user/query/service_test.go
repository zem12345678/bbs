package query

import (
	"context"
	"errors"
	"testing"
	"time"

	domain "user-service/internal/domain/user"
)

func TestGetDemotesPremiumProfileWithoutActiveEntitlements(t *testing.T) {
	repo := &repoStub{users: map[int64]*domain.User{
		42: premiumUser(42),
	}}
	entitlements := &fakeProfileEntitlements{}
	svc := NewService(repo, entitlements)

	got, err := svc.Get(context.Background(), 42)
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}

	if got.BackgroundURL != "" {
		t.Fatalf("background url = %q, want hidden", got.BackgroundURL)
	}
	if got.ProfileTheme != domain.ProfileThemeDefault {
		t.Fatalf("profile theme = %q, want default", got.ProfileTheme)
	}
	stored := repo.users[42]
	if stored.BackgroundURL == "" || stored.ProfileTheme != domain.ProfileThemePro {
		t.Fatalf("stored user was mutated: %+v", stored)
	}
	if entitlements.membershipCalls != 1 || entitlements.themeCalls != 1 {
		t.Fatalf("entitlement calls = membership:%d theme:%d, want 1/1", entitlements.membershipCalls, entitlements.themeCalls)
	}
}

func TestGetKeepsPremiumProfileWithActiveEntitlements(t *testing.T) {
	repo := &repoStub{users: map[int64]*domain.User{
		42: premiumUser(42),
	}}
	entitlements := &fakeProfileEntitlements{membershipAllowed: true, themeAllowed: true}
	svc := NewService(repo, entitlements)

	got, err := svc.GetByUsername(context.Background(), "alice")
	if err != nil {
		t.Fatalf("GetByUsername() error = %v", err)
	}

	if got.BackgroundURL != "https://example.com/background.webp" {
		t.Fatalf("background url = %q, want premium background", got.BackgroundURL)
	}
	if got.ProfileTheme != domain.ProfileThemePro {
		t.Fatalf("profile theme = %q, want theme-pro", got.ProfileTheme)
	}
	if entitlements.membershipUserID != 42 || entitlements.themeUserID != 42 || entitlements.theme != domain.ProfileThemePro {
		t.Fatalf("entitlement lookup = membership user:%d theme user:%d theme:%q", entitlements.membershipUserID, entitlements.themeUserID, entitlements.theme)
	}
}

func TestListUsersDemotesPremiumProfileWhenEntitlementCheckFails(t *testing.T) {
	repo := &repoStub{
		users: map[int64]*domain.User{
			42: premiumUser(42),
			43: basicUser(43, "bob"),
		},
		listed: []*domain.User{premiumUser(42), basicUser(43, "bob")},
		total:  2,
	}
	entitlements := &fakeProfileEntitlements{
		membershipErr: errors.New("mall unavailable"),
		themeErr:      errors.New("mall unavailable"),
	}
	svc := NewService(repo, entitlements)

	result, err := svc.ListUsers(context.Background(), domain.UserListQuery{})
	if err != nil {
		t.Fatalf("ListUsers() error = %v", err)
	}

	if result.Total != 2 || len(result.Items) != 2 {
		t.Fatalf("result total=%d len=%d, want 2/2", result.Total, len(result.Items))
	}
	if result.Items[0].BackgroundURL != "" || result.Items[0].ProfileTheme != domain.ProfileThemeDefault {
		t.Fatalf("premium user response = %+v, want demoted profile", result.Items[0])
	}
	if result.Items[1].ProfileTheme != domain.ProfileThemeDefault {
		t.Fatalf("basic user response = %+v, want unchanged default profile", result.Items[1])
	}
	if entitlements.membershipCalls != 0 || entitlements.themeCalls != 0 {
		t.Fatalf("single entitlement calls = membership:%d theme:%d, want 0/0", entitlements.membershipCalls, entitlements.themeCalls)
	}
	if entitlements.membershipBatchCalls != 1 || entitlements.themeBatchCalls != 1 {
		t.Fatalf("batch entitlement calls = membership:%d theme:%d, want 1/1", entitlements.membershipBatchCalls, entitlements.themeBatchCalls)
	}
}

func TestListUsersUsesBatchEntitlementChecks(t *testing.T) {
	repo := &repoStub{
		users: map[int64]*domain.User{
			42: premiumUser(42),
			43: premiumUser(43),
		},
		listed: []*domain.User{premiumUser(42), premiumUser(43)},
		total:  2,
	}
	entitlements := &fakeProfileEntitlements{
		membershipActiveUserIDs: map[int64]bool{42: true},
		themeActiveUserIDs:      map[int64]bool{43: true},
	}
	svc := NewService(repo, entitlements)

	result, err := svc.ListUsers(context.Background(), domain.UserListQuery{})
	if err != nil {
		t.Fatalf("ListUsers() error = %v", err)
	}

	if got := entitlements.membershipBatchUserIDs; len(got) != 2 || got[0] != 42 || got[1] != 43 {
		t.Fatalf("membership batch user ids = %v, want [42 43]", got)
	}
	if got := entitlements.themeBatchUserIDs; len(got) != 2 || got[0] != 42 || got[1] != 43 {
		t.Fatalf("theme batch user ids = %v, want [42 43]", got)
	}
	if entitlements.membershipCalls != 0 || entitlements.themeCalls != 0 {
		t.Fatalf("single entitlement calls = membership:%d theme:%d, want 0/0", entitlements.membershipCalls, entitlements.themeCalls)
	}
	if result.Items[0].BackgroundURL == "" || result.Items[0].ProfileTheme != domain.ProfileThemeDefault {
		t.Fatalf("first premium profile = %+v, want membership background only", result.Items[0])
	}
	if result.Items[1].BackgroundURL != "" || result.Items[1].ProfileTheme != domain.ProfileThemePro {
		t.Fatalf("second premium profile = %+v, want theme only", result.Items[1])
	}
}

type repoStub struct {
	domain.Repository
	users  map[int64]*domain.User
	listed []*domain.User
	total  int64
}

func (r *repoStub) FindByID(_ context.Context, id int64) (*domain.User, error) {
	user, ok := r.users[id]
	if !ok {
		return nil, domain.ErrNotFound
	}
	return user, nil
}

func (r *repoStub) FindByUsername(_ context.Context, username string) (*domain.User, error) {
	for _, user := range r.users {
		if user.Username == username {
			return user, nil
		}
	}
	return nil, domain.ErrNotFound
}

func (r *repoStub) ListUsers(context.Context, domain.UserListQuery) ([]*domain.User, int64, error) {
	return r.listed, r.total, nil
}

type fakeProfileEntitlements struct {
	membershipAllowed       bool
	themeAllowed            bool
	membershipErr           error
	themeErr                error
	membershipCalls         int
	themeCalls              int
	membershipBatchCalls    int
	themeBatchCalls         int
	membershipUserID        int64
	themeUserID             int64
	membershipBatchUserIDs  []int64
	themeBatchUserIDs       []int64
	membershipActiveUserIDs map[int64]bool
	themeActiveUserIDs      map[int64]bool
	theme                   string
}

func (f *fakeProfileEntitlements) HasActiveProfileTheme(_ context.Context, userID int64, theme string) (bool, error) {
	f.themeCalls++
	f.themeUserID = userID
	f.theme = theme
	if f.themeErr != nil {
		return false, f.themeErr
	}
	return f.themeAllowed, nil
}

func (f *fakeProfileEntitlements) HasActiveMembership(_ context.Context, userID int64) (bool, error) {
	f.membershipCalls++
	f.membershipUserID = userID
	if f.membershipErr != nil {
		return false, f.membershipErr
	}
	return f.membershipAllowed, nil
}

func (f *fakeProfileEntitlements) ListActiveProfileThemeUserIDs(_ context.Context, userIDs []int64, theme string) (map[int64]bool, error) {
	f.themeBatchCalls++
	f.themeBatchUserIDs = append([]int64(nil), userIDs...)
	f.theme = theme
	if f.themeErr != nil {
		return nil, f.themeErr
	}
	return f.activeUserIDs(userIDs, f.themeActiveUserIDs, f.themeAllowed), nil
}

func (f *fakeProfileEntitlements) ListActiveMembershipUserIDs(_ context.Context, userIDs []int64) (map[int64]bool, error) {
	f.membershipBatchCalls++
	f.membershipBatchUserIDs = append([]int64(nil), userIDs...)
	if f.membershipErr != nil {
		return nil, f.membershipErr
	}
	return f.activeUserIDs(userIDs, f.membershipActiveUserIDs, f.membershipAllowed), nil
}

func (f *fakeProfileEntitlements) activeUserIDs(userIDs []int64, configured map[int64]bool, allowAll bool) map[int64]bool {
	active := make(map[int64]bool)
	for _, userID := range userIDs {
		if (configured != nil && configured[userID]) || (configured == nil && allowAll) {
			active[userID] = true
		}
	}
	return active
}

func premiumUser(id int64) *domain.User {
	user := basicUser(id, "alice")
	user.BackgroundURL = "https://example.com/background.webp"
	user.ProfileTheme = domain.ProfileThemePro
	return user
}

func basicUser(id int64, username string) *domain.User {
	now := time.Now()
	return &domain.User{
		ID:           id,
		Username:     username,
		Email:        username + "@example.com",
		PasswordHash: "hash",
		Nickname:     username,
		ProfileTheme: domain.ProfileThemeDefault,
		Status:       domain.StatusActive,
		CreatedAt:    now,
		UpdatedAt:    now,
	}
}
