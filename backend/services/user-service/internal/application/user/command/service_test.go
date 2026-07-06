package command

import (
	"context"
	"errors"
	"strings"
	"testing"

	domain "user-service/internal/domain/user"
)

func TestServiceRegisterLoginAndFollow(t *testing.T) {
	repo := newMemoryRepo()
	idgen := &fakeIDGen{next: 100}
	svc := NewService(repo, idgen, nil, nil, "test-secret", 0, 8)
	ctx := context.Background()

	alice, token, err := svc.Register(ctx, domain.RegisterCmd{
		Username: "alice",
		Email:    "alice@example.com",
		Password: "password123",
		Nickname: "Alice",
	})
	if err != nil {
		t.Fatalf("register alice: %v", err)
	}
	if token.Value == "" || token.ExpiresAt.IsZero() {
		t.Fatalf("expected issued token")
	}
	bob, _, err := svc.Register(ctx, domain.RegisterCmd{
		Username: "bob",
		Email:    "bob@example.com",
		Password: "password123",
		Nickname: "Bob",
	})
	if err != nil {
		t.Fatalf("register bob: %v", err)
	}

	loggedIn, loginToken, err := svc.Login(ctx, "alice", "password123")
	if err != nil {
		t.Fatalf("login: %v", err)
	}
	if loggedIn.ID != alice.ID || loginToken.Value == "" {
		t.Fatalf("unexpected login result")
	}
	if _, _, err := svc.Login(ctx, "alice", "wrong-password"); !errors.Is(err, domain.ErrInvalidPassword) {
		t.Fatalf("wrong password error = %v", err)
	}

	if err := svc.Follow(ctx, alice.ID, bob.ID); err != nil {
		t.Fatalf("follow: %v", err)
	}
	ok, err := repo.IsFollowing(ctx, alice.ID, bob.ID)
	if err != nil || !ok {
		t.Fatalf("is following ok=%v err=%v", ok, err)
	}
	if err := svc.Unfollow(ctx, alice.ID, bob.ID); err != nil {
		t.Fatalf("unfollow: %v", err)
	}
}

type fakeIDGen struct {
	next int64
}

func (g *fakeIDGen) Generate() int64 {
	g.next++
	return g.next
}

type memoryRepo struct {
	users   map[int64]*domain.User
	follows map[[2]int64]struct{}
}

func newMemoryRepo() *memoryRepo {
	return &memoryRepo{users: map[int64]*domain.User{}, follows: map[[2]int64]struct{}{}}
}

func (r *memoryRepo) Create(_ context.Context, u *domain.User) error {
	for _, existing := range r.users {
		if existing.Username == u.Username {
			return domain.ErrUsernameExists
		}
		if existing.Email == u.Email {
			return domain.ErrEmailExists
		}
	}
	r.users[u.ID] = cloneUser(u)
	return nil
}

func (r *memoryRepo) UpdateProfile(_ context.Context, u *domain.User) error {
	if _, ok := r.users[u.ID]; !ok {
		return domain.ErrNotFound
	}
	r.users[u.ID] = cloneUser(u)
	return nil
}

func (r *memoryRepo) UpdatePassword(_ context.Context, u *domain.User) error {
	return r.UpdateProfile(context.Background(), u)
}

func (r *memoryRepo) UpdateStatus(_ context.Context, u *domain.User) error {
	return r.UpdateProfile(context.Background(), u)
}

func (r *memoryRepo) UpdateLastLogin(_ context.Context, u *domain.User) error {
	return r.UpdateProfile(context.Background(), u)
}

func (r *memoryRepo) FindByID(_ context.Context, id int64) (*domain.User, error) {
	u, ok := r.users[id]
	if !ok {
		return nil, domain.ErrNotFound
	}
	return cloneUser(u), nil
}

func (r *memoryRepo) FindByUsername(_ context.Context, username string) (*domain.User, error) {
	username = domain.NormalizeUsername(username)
	for _, u := range r.users {
		if u.Username == username {
			return cloneUser(u), nil
		}
	}
	return nil, domain.ErrNotFound
}

func (r *memoryRepo) FindByEmail(_ context.Context, email string) (*domain.User, error) {
	email = domain.NormalizeEmail(email)
	for _, u := range r.users {
		if u.Email == email {
			return cloneUser(u), nil
		}
	}
	return nil, domain.ErrNotFound
}

func (r *memoryRepo) FindByAccount(ctx context.Context, account string) (*domain.User, error) {
	if strings.Contains(account, "@") {
		return r.FindByEmail(ctx, account)
	}
	return r.FindByUsername(ctx, account)
}

func (r *memoryRepo) Follow(_ context.Context, followerID, followeeID int64) error {
	key := [2]int64{followerID, followeeID}
	if _, ok := r.follows[key]; ok {
		return domain.ErrAlreadyFollowing
	}
	r.follows[key] = struct{}{}
	return nil
}

func (r *memoryRepo) Unfollow(_ context.Context, followerID, followeeID int64) error {
	key := [2]int64{followerID, followeeID}
	if _, ok := r.follows[key]; !ok {
		return domain.ErrNotFollowing
	}
	delete(r.follows, key)
	return nil
}

func (r *memoryRepo) IsFollowing(_ context.Context, followerID, followeeID int64) (bool, error) {
	_, ok := r.follows[[2]int64{followerID, followeeID}]
	return ok, nil
}

func (r *memoryRepo) ListUsers(_ context.Context, q domain.UserListQuery) ([]*domain.User, int64, error) {
	items := make([]*domain.User, 0, len(r.users))
	for _, user := range r.users {
		items = append(items, cloneUser(user))
	}
	return items, int64(len(items)), nil
}

func (r *memoryRepo) ListFollowers(_ context.Context, q domain.FollowListQuery) ([]*domain.User, int64, error) {
	return nil, 0, nil
}

func (r *memoryRepo) ListFollowing(_ context.Context, q domain.FollowListQuery) ([]*domain.User, int64, error) {
	return nil, 0, nil
}

func cloneUser(u *domain.User) *domain.User {
	cp := *u
	cp.Events()
	return &cp
}
