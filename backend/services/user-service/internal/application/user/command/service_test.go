package command

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

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

func TestServiceOAuthAndWebmasterLogin(t *testing.T) {
	repo := newMemoryRepo()
	idgen := &fakeIDGen{next: 200}
	svc := NewService(repo, idgen, nil, nil, "test-secret", 0, 8)
	ctx := context.Background()

	oauthUser, oauthToken, err := svc.OAuthLogin(ctx, domain.OAuthLoginCmd{
		Provider:       "github",
		ProviderUserID: "12345",
		Username:       "GitHub_User",
		Email:          "github@example.com",
		Nickname:       "GitHub User",
		AvatarURL:      "https://example.com/avatar.png",
	})
	if err != nil {
		t.Fatalf("oauth login: %v", err)
	}
	if oauthUser.Username != "github_user" || oauthUser.AvatarURL == "" || oauthToken.Value == "" {
		t.Fatalf("unexpected oauth user=%+v token=%q", oauthUser, oauthToken.Value)
	}
	again, _, err := svc.OAuthLogin(ctx, domain.OAuthLoginCmd{
		Provider:       "github",
		ProviderUserID: "12345",
		Username:       "renamed",
	})
	if err != nil {
		t.Fatalf("oauth login again: %v", err)
	}
	if again.ID != oauthUser.ID {
		t.Fatalf("expected same oauth user id, got %d want %d", again.ID, oauthUser.ID)
	}

	webmaster, webmasterToken, err := svc.WebmasterLogin(ctx, domain.WebmasterLoginCmd{
		Username: "webmaster",
		Password: "password123",
		Email:    "webmaster@example.com",
		Nickname: "Webmaster",
	})
	if err != nil {
		t.Fatalf("webmaster login: %v", err)
	}
	if webmaster.Username != "webmaster" || webmasterToken.Value == "" {
		t.Fatalf("unexpected webmaster user=%+v token=%q", webmaster, webmasterToken.Value)
	}
}

func TestServiceUpdateProfileSavesBackgroundURL(t *testing.T) {
	repo := newMemoryRepo()
	idgen := &fakeIDGen{next: 250}
	svc := NewService(repo, idgen, nil, nil, "test-secret", 0, 8)
	ctx := context.Background()

	alice, _, err := svc.Register(ctx, domain.RegisterCmd{
		Username: "alice",
		Email:    "alice@example.com",
		Password: "password123",
		Nickname: "Alice",
	})
	if err != nil {
		t.Fatalf("register alice: %v", err)
	}

	updated, err := svc.UpdateProfile(ctx, alice.ID, domain.UpdateProfileCmd{
		Nickname:      " Alice Dev ",
		AvatarURL:     " https://example.com/avatar.png ",
		BackgroundURL: " https://example.com/background.webp ",
		Bio:           " Building things ",
	})
	if err != nil {
		t.Fatalf("update profile: %v", err)
	}
	if updated.Nickname != "Alice Dev" || updated.AvatarURL != "https://example.com/avatar.png" || updated.BackgroundURL != "https://example.com/background.webp" || updated.Bio != "Building things" {
		t.Fatalf("unexpected updated user=%+v", updated)
	}

	stored, err := repo.FindByID(ctx, alice.ID)
	if err != nil {
		t.Fatalf("find updated user: %v", err)
	}
	if stored.BackgroundURL != "https://example.com/background.webp" {
		t.Fatalf("background url = %q", stored.BackgroundURL)
	}
}

func TestServicePasswordReset(t *testing.T) {
	repo := newMemoryRepo()
	idgen := &fakeIDGen{next: 300}
	svc := NewService(repo, idgen, nil, nil, "test-secret", 0, 8)
	ctx := context.Background()

	alice, _, err := svc.Register(ctx, domain.RegisterCmd{
		Username: "alice",
		Email:    "alice@example.com",
		Password: "password123",
		Nickname: "Alice",
	})
	if err != nil {
		t.Fatalf("register alice: %v", err)
	}
	result, err := svc.RequestPasswordReset(ctx, "ALICE@example.com")
	if err != nil {
		t.Fatalf("request password reset: %v", err)
	}
	if !result.Accepted || result.ResetToken == "" || !result.ExpiresAt.After(time.Now()) {
		t.Fatalf("unexpected password reset result=%+v", result)
	}
	missing, err := svc.RequestPasswordReset(ctx, "missing@example.com")
	if err != nil {
		t.Fatalf("request missing password reset: %v", err)
	}
	if !missing.Accepted || missing.ResetToken != "" {
		t.Fatalf("missing account leaked reset token: %+v", missing)
	}

	if err := svc.ResetPassword(ctx, result.ResetToken, "newpass123"); err != nil {
		t.Fatalf("reset password: %v", err)
	}
	if _, _, err := svc.Login(ctx, alice.Username, "password123"); !errors.Is(err, domain.ErrInvalidPassword) {
		t.Fatalf("old password login error = %v", err)
	}
	loggedIn, token, err := svc.Login(ctx, alice.Username, "newpass123")
	if err != nil {
		t.Fatalf("login with reset password: %v", err)
	}
	if loggedIn.ID != alice.ID || token.Value == "" {
		t.Fatalf("unexpected reset login user=%+v token=%q", loggedIn, token.Value)
	}
	if err := svc.ResetPassword(ctx, result.ResetToken, "another123"); !errors.Is(err, domain.ErrResetTokenInvalid) {
		t.Fatalf("reused reset token error = %v", err)
	}
}

func TestServiceEmailVerification(t *testing.T) {
	repo := newMemoryRepo()
	idgen := &fakeIDGen{next: 400}
	svc := NewService(repo, idgen, nil, nil, "test-secret", 0, 8)
	ctx := context.Background()

	alice, _, err := svc.Register(ctx, domain.RegisterCmd{
		Username: "alice",
		Email:    "alice@example.com",
		Password: "password123",
		Nickname: "Alice",
	})
	if err != nil {
		t.Fatalf("register alice: %v", err)
	}
	result, err := svc.RequestEmailVerification(ctx, alice.ID)
	if err != nil {
		t.Fatalf("request email verification: %v", err)
	}
	if !result.Accepted || result.VerificationToken == "" || !result.ExpiresAt.After(time.Now()) || result.AlreadyVerified {
		t.Fatalf("unexpected email verification result=%+v", result)
	}
	verified, err := svc.VerifyEmail(ctx, result.VerificationToken)
	if err != nil {
		t.Fatalf("verify email: %v", err)
	}
	if verified.EmailVerifiedAt == nil {
		t.Fatalf("expected verified timestamp")
	}
	if _, err := svc.VerifyEmail(ctx, result.VerificationToken); !errors.Is(err, domain.ErrEmailVerificationTokenInvalid) {
		t.Fatalf("reused verification token error = %v", err)
	}
	again, err := svc.RequestEmailVerification(ctx, alice.ID)
	if err != nil {
		t.Fatalf("request email verification again: %v", err)
	}
	if !again.Accepted || !again.AlreadyVerified || again.VerificationToken != "" {
		t.Fatalf("verified account should not receive another token: %+v", again)
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
	users        map[int64]*domain.User
	oauthByKey   map[[2]string]int64
	oauthAccount map[[2]string]domain.OAuthAccount
	follows      map[[2]int64]struct{}
	resetTokens  map[string]domain.PasswordResetToken
	emailTokens  map[string]domain.EmailVerificationToken
}

func newMemoryRepo() *memoryRepo {
	return &memoryRepo{
		users:        map[int64]*domain.User{},
		oauthByKey:   map[[2]string]int64{},
		oauthAccount: map[[2]string]domain.OAuthAccount{},
		follows:      map[[2]int64]struct{}{},
		resetTokens:  map[string]domain.PasswordResetToken{},
		emailTokens:  map[string]domain.EmailVerificationToken{},
	}
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

func (r *memoryRepo) UpdateOAuthLogin(_ context.Context, u *domain.User, account domain.OAuthAccount) error {
	if err := r.UpdateLastLogin(context.Background(), u); err != nil {
		return err
	}
	key := [2]string{domain.NormalizeProvider(account.Provider), strings.TrimSpace(account.ProviderUserID)}
	account.UserID = u.ID
	r.oauthByKey[key] = u.ID
	r.oauthAccount[key] = account
	return nil
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

func (r *memoryRepo) FindByOAuth(_ context.Context, provider string, providerUserID string) (*domain.User, error) {
	key := [2]string{domain.NormalizeProvider(provider), strings.TrimSpace(providerUserID)}
	userID, ok := r.oauthByKey[key]
	if !ok {
		return nil, domain.ErrNotFound
	}
	return r.FindByID(context.Background(), userID)
}

func (r *memoryRepo) CreateWithOAuth(ctx context.Context, u *domain.User, account domain.OAuthAccount) error {
	if err := r.Create(ctx, u); err != nil {
		return err
	}
	key := [2]string{domain.NormalizeProvider(account.Provider), strings.TrimSpace(account.ProviderUserID)}
	if _, ok := r.oauthByKey[key]; ok {
		return domain.ErrInvalidOAuth
	}
	account.UserID = u.ID
	r.oauthByKey[key] = u.ID
	r.oauthAccount[key] = account
	return nil
}

func (r *memoryRepo) EnsureWebmaster(_ context.Context, u *domain.User) error {
	for id, existing := range r.users {
		if existing.Username == u.Username {
			u.ID = id
			r.users[id] = cloneUser(u)
			return nil
		}
	}
	r.users[u.ID] = cloneUser(u)
	return nil
}

func (r *memoryRepo) CreatePasswordResetToken(_ context.Context, token domain.PasswordResetToken) error {
	for key, existing := range r.resetTokens {
		if existing.UserID == token.UserID && existing.UsedAt == nil {
			now := token.CreatedAt
			existing.UsedAt = &now
			r.resetTokens[key] = existing
		}
	}
	r.resetTokens[token.TokenHash] = token
	return nil
}

func (r *memoryRepo) ResetPasswordWithToken(_ context.Context, tokenHash string, passwordHash string, now time.Time) (*domain.User, error) {
	token, ok := r.resetTokens[tokenHash]
	if !ok || token.UsedAt != nil {
		return nil, domain.ErrResetTokenInvalid
	}
	if !token.ExpiresAt.After(now) {
		return nil, domain.ErrResetTokenExpired
	}
	u, ok := r.users[token.UserID]
	if !ok {
		return nil, domain.ErrNotFound
	}
	cp := cloneUser(u)
	cp.PasswordHash = passwordHash
	cp.UpdatedAt = now
	r.users[cp.ID] = cloneUser(cp)
	token.UsedAt = &now
	r.resetTokens[tokenHash] = token
	return cloneUser(cp), nil
}

func (r *memoryRepo) CreateEmailVerificationToken(_ context.Context, token domain.EmailVerificationToken) error {
	for key, existing := range r.emailTokens {
		if existing.UserID == token.UserID && existing.UsedAt == nil {
			now := token.CreatedAt
			existing.UsedAt = &now
			r.emailTokens[key] = existing
		}
	}
	r.emailTokens[token.TokenHash] = token
	return nil
}

func (r *memoryRepo) VerifyEmailWithToken(_ context.Context, tokenHash string, now time.Time) (*domain.User, error) {
	token, ok := r.emailTokens[tokenHash]
	if !ok || token.UsedAt != nil {
		return nil, domain.ErrEmailVerificationTokenInvalid
	}
	if !token.ExpiresAt.After(now) {
		return nil, domain.ErrEmailVerificationTokenExpired
	}
	u, ok := r.users[token.UserID]
	if !ok {
		return nil, domain.ErrNotFound
	}
	if domain.NormalizeEmail(u.Email) != token.Email {
		return nil, domain.ErrEmailVerificationTokenInvalid
	}
	cp := cloneUser(u)
	cp.EmailVerifiedAt = &now
	cp.UpdatedAt = now
	r.users[cp.ID] = cloneUser(cp)
	token.UsedAt = &now
	r.emailTokens[tokenHash] = token
	return cloneUser(cp), nil
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
