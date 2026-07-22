package query

import (
	"context"
	"testing"

	domain "user-service/internal/domain/user"
)

func TestListUsersForwardsIDFilterToRepository(t *testing.T) {
	repo := &userIDsQueryRepo{}
	svc := NewService(repo, nil)

	_, err := svc.ListUsers(context.Background(), domain.UserListQuery{
		IDs:      []int64{42, 7},
		Status:   int32(domain.StatusActive),
		Page:     1,
		PageSize: 2,
	})
	if err != nil {
		t.Fatalf("ListUsers() error = %v", err)
	}
	if got := repo.query.IDs; len(got) != 2 || got[0] != 42 || got[1] != 7 {
		t.Fatalf("repository IDs = %v, want [42 7]", got)
	}
	if repo.query.Status != int32(domain.StatusActive) {
		t.Fatalf("repository status = %d, want active", repo.query.Status)
	}
}

type userIDsQueryRepo struct {
	domain.Repository
	query domain.UserListQuery
}

func (r *userIDsQueryRepo) ListUsers(_ context.Context, q domain.UserListQuery) ([]*domain.User, int64, error) {
	r.query = q
	return []*domain.User{{ID: 42, Username: "alice", Nickname: "Alice", ProfileTheme: domain.ProfileThemeDefault}}, 1, nil
}
