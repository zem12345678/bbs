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
	svc := NewService(repo, idgen, nil, nil, "test-secret", 0, 8, nil)
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
	svc := NewService(repo, idgen, nil, nil, "test-secret", 0, 8, nil)
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

func TestServiceLoginHidesPremiumProfileWithoutActiveEntitlements(t *testing.T) {
	repo := newMemoryRepo()
	idgen := &fakeIDGen{next: 220}
	entitlements := &fakeProfileThemeEntitlements{}
	svc := NewService(repo, idgen, nil, nil, "test-secret", 0, 8, entitlements)
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
	seedPremiumProfile(t, repo, alice.ID)

	loggedIn, token, err := svc.Login(ctx, "alice", "password123")
	if err != nil {
		t.Fatalf("login: %v", err)
	}
	if token.Value == "" {
		t.Fatal("login token is empty")
	}
	if loggedIn.BackgroundURL != "" {
		t.Fatalf("login background url = %q, want hidden", loggedIn.BackgroundURL)
	}
	if loggedIn.ProfileTheme != domain.ProfileThemeDefault {
		t.Fatalf("login profile theme = %q, want default", loggedIn.ProfileTheme)
	}
	stored, err := repo.FindByID(ctx, alice.ID)
	if err != nil {
		t.Fatalf("find stored user: %v", err)
	}
	if stored.BackgroundURL == "" || stored.ProfileTheme != domain.ProfileThemePro {
		t.Fatalf("stored premium profile = background:%q theme:%q, want preserved", stored.BackgroundURL, stored.ProfileTheme)
	}
	if entitlements.membershipCalls != 1 || entitlements.calls != 1 {
		t.Fatalf("entitlement checks = membership:%d theme:%d, want 1/1", entitlements.membershipCalls, entitlements.calls)
	}
}

func TestServiceOAuthLoginHidesPremiumProfileWithoutActiveEntitlements(t *testing.T) {
	repo := newMemoryRepo()
	idgen := &fakeIDGen{next: 230}
	entitlements := &fakeProfileThemeEntitlements{}
	svc := NewService(repo, idgen, nil, nil, "test-secret", 0, 8, entitlements)
	ctx := context.Background()

	oauthUser, _, err := svc.OAuthLogin(ctx, domain.OAuthLoginCmd{
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
	seedPremiumProfile(t, repo, oauthUser.ID)

	again, token, err := svc.OAuthLogin(ctx, domain.OAuthLoginCmd{
		Provider:       "github",
		ProviderUserID: "12345",
		Username:       "renamed",
	})
	if err != nil {
		t.Fatalf("oauth login again: %v", err)
	}
	if token.Value == "" {
		t.Fatal("oauth token is empty")
	}
	if again.ID != oauthUser.ID {
		t.Fatalf("expected same oauth user id, got %d want %d", again.ID, oauthUser.ID)
	}
	if again.BackgroundURL != "" {
		t.Fatalf("oauth background url = %q, want hidden", again.BackgroundURL)
	}
	if again.ProfileTheme != domain.ProfileThemeDefault {
		t.Fatalf("oauth profile theme = %q, want default", again.ProfileTheme)
	}
	stored, err := repo.FindByID(ctx, oauthUser.ID)
	if err != nil {
		t.Fatalf("find stored user: %v", err)
	}
	if stored.BackgroundURL == "" || stored.ProfileTheme != domain.ProfileThemePro {
		t.Fatalf("stored premium profile = background:%q theme:%q, want preserved", stored.BackgroundURL, stored.ProfileTheme)
	}
	if entitlements.membershipCalls != 1 || entitlements.calls != 1 {
		t.Fatalf("entitlement checks = membership:%d theme:%d, want 1/1", entitlements.membershipCalls, entitlements.calls)
	}
}

func TestServiceWebmasterLoginHidesPremiumProfileWithoutActiveEntitlements(t *testing.T) {
	repo := newMemoryRepo()
	idgen := &fakeIDGen{next: 240}
	entitlements := &fakeProfileThemeEntitlements{}
	svc := NewService(repo, idgen, nil, nil, "test-secret", 0, 8, entitlements)
	ctx := context.Background()

	webmaster, _, err := svc.WebmasterLogin(ctx, domain.WebmasterLoginCmd{
		Username: "webmaster",
		Password: "password123",
		Email:    "webmaster@example.com",
		Nickname: "Webmaster",
	})
	if err != nil {
		t.Fatalf("webmaster login: %v", err)
	}
	seedPremiumProfile(t, repo, webmaster.ID)

	again, token, err := svc.WebmasterLogin(ctx, domain.WebmasterLoginCmd{
		Username: "webmaster",
		Password: "password123",
		Email:    "webmaster@example.com",
		Nickname: "Webmaster",
	})
	if err != nil {
		t.Fatalf("webmaster login again: %v", err)
	}
	if token.Value == "" {
		t.Fatal("webmaster token is empty")
	}
	if again.BackgroundURL != "" {
		t.Fatalf("webmaster background url = %q, want hidden", again.BackgroundURL)
	}
	if again.ProfileTheme != domain.ProfileThemeDefault {
		t.Fatalf("webmaster profile theme = %q, want default", again.ProfileTheme)
	}
	stored, err := repo.FindByID(ctx, webmaster.ID)
	if err != nil {
		t.Fatalf("find stored user: %v", err)
	}
	if stored.BackgroundURL == "" || stored.ProfileTheme != domain.ProfileThemePro {
		t.Fatalf("stored premium profile = background:%q theme:%q, want preserved", stored.BackgroundURL, stored.ProfileTheme)
	}
	if entitlements.membershipCalls != 1 || entitlements.calls != 1 {
		t.Fatalf("entitlement checks = membership:%d theme:%d, want 1/1", entitlements.membershipCalls, entitlements.calls)
	}
}

func TestServiceUpdateProfileSavesBackgroundURL(t *testing.T) {
	repo := newMemoryRepo()
	idgen := &fakeIDGen{next: 250}
	entitlements := &fakeProfileThemeEntitlements{allowed: true, membershipAllowed: true}
	svc := NewService(repo, idgen, nil, nil, "test-secret", 0, 8, entitlements)
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
		ProfileTheme:  " theme-pro ",
		Bio:           " Building things ",
	})
	if err != nil {
		t.Fatalf("update profile: %v", err)
	}
	if updated.Nickname != "Alice Dev" || updated.AvatarURL != "https://example.com/avatar.png" || updated.BackgroundURL != "https://example.com/background.webp" || updated.ProfileTheme != domain.ProfileThemePro || updated.Bio != "Building things" {
		t.Fatalf("unexpected updated user=%+v", updated)
	}

	stored, err := repo.FindByID(ctx, alice.ID)
	if err != nil {
		t.Fatalf("find updated user: %v", err)
	}
	if stored.BackgroundURL != "https://example.com/background.webp" {
		t.Fatalf("background url = %q", stored.BackgroundURL)
	}
	if stored.ProfileTheme != domain.ProfileThemePro {
		t.Fatalf("profile theme = %q", stored.ProfileTheme)
	}
	if entitlements.calls != 1 || entitlements.userID != alice.ID || entitlements.theme != domain.ProfileThemePro {
		t.Fatalf("theme entitlement check calls=%d user_id=%d theme=%q", entitlements.calls, entitlements.userID, entitlements.theme)
	}
	if entitlements.membershipCalls != 1 || entitlements.membershipUserID != alice.ID {
		t.Fatalf("membership entitlement check calls=%d user_id=%d", entitlements.membershipCalls, entitlements.membershipUserID)
	}
}

func TestServiceUpdateProfileRejectsBackgroundWithoutMembership(t *testing.T) {
	repo := newMemoryRepo()
	idgen := &fakeIDGen{next: 254}
	entitlements := &fakeProfileThemeEntitlements{}
	svc := NewService(repo, idgen, nil, nil, "test-secret", 0, 8, entitlements)
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

	_, err = svc.UpdateProfile(ctx, alice.ID, domain.UpdateProfileCmd{
		Nickname:      "Alice",
		BackgroundURL: "https://example.com/background.webp",
	})
	if !errors.Is(err, domain.ErrProfileBackgroundEntitlementRequired) {
		t.Fatalf("expected profile background entitlement error, got %v", err)
	}
	stored, err := repo.FindByID(ctx, alice.ID)
	if err != nil {
		t.Fatalf("find user: %v", err)
	}
	if stored.BackgroundURL != "" {
		t.Fatalf("stored background url = %q, want blank", stored.BackgroundURL)
	}
	if entitlements.membershipCalls != 1 || entitlements.membershipUserID != alice.ID {
		t.Fatalf("membership entitlement check calls=%d user_id=%d", entitlements.membershipCalls, entitlements.membershipUserID)
	}
}

func TestServiceUpdateProfileRejectsProfileThemeWithoutEntitlement(t *testing.T) {
	repo := newMemoryRepo()
	idgen := &fakeIDGen{next: 252}
	entitlements := &fakeProfileThemeEntitlements{}
	svc := NewService(repo, idgen, nil, nil, "test-secret", 0, 8, entitlements)
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

	_, err = svc.UpdateProfile(ctx, alice.ID, domain.UpdateProfileCmd{Nickname: "Alice", ProfileTheme: "theme-pro"})
	if !errors.Is(err, domain.ErrProfileThemeEntitlementRequired) {
		t.Fatalf("expected profile theme entitlement error, got %v", err)
	}
	stored, err := repo.FindByID(ctx, alice.ID)
	if err != nil {
		t.Fatalf("find user: %v", err)
	}
	if stored.ProfileTheme != domain.ProfileThemeDefault {
		t.Fatalf("stored profile theme = %q, want default", stored.ProfileTheme)
	}
	if entitlements.calls != 1 || entitlements.userID != alice.ID || entitlements.theme != domain.ProfileThemePro {
		t.Fatalf("entitlement check calls=%d user_id=%d theme=%q", entitlements.calls, entitlements.userID, entitlements.theme)
	}
}

func TestServiceUpdateProfileDemotesUnchangedThemeWithoutEntitlement(t *testing.T) {
	repo := newMemoryRepo()
	idgen := &fakeIDGen{next: 253}
	entitlements := &fakeProfileThemeEntitlements{allowed: true}
	svc := NewService(repo, idgen, nil, nil, "test-secret", 0, 8, entitlements)
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
	if _, err := svc.UpdateProfile(ctx, alice.ID, domain.UpdateProfileCmd{Nickname: "Alice", ProfileTheme: "theme-pro"}); err != nil {
		t.Fatalf("set profile theme: %v", err)
	}
	if entitlements.calls != 1 {
		t.Fatalf("entitlement check calls = %d, want 1 after theme update", entitlements.calls)
	}

	entitlements.allowed = false
	updated, err := svc.UpdateProfile(ctx, alice.ID, domain.UpdateProfileCmd{Nickname: "Alice Dev"})
	if err != nil {
		t.Fatalf("update profile without theme: %v", err)
	}
	if updated.Nickname != "Alice Dev" || updated.ProfileTheme != domain.ProfileThemeDefault {
		t.Fatalf("updated user = %+v, want nickname change and default profile theme", updated)
	}
	stored, err := repo.FindByID(ctx, alice.ID)
	if err != nil {
		t.Fatalf("find updated user: %v", err)
	}
	if stored.ProfileTheme != domain.ProfileThemeDefault {
		t.Fatalf("stored profile theme = %q, want default", stored.ProfileTheme)
	}
	if entitlements.calls != 2 {
		t.Fatalf("entitlement check calls = %d, want 2 after stale theme recheck", entitlements.calls)
	}
}

func TestServiceUpdateProfileRejectsInvalidTheme(t *testing.T) {
	repo := newMemoryRepo()
	idgen := &fakeIDGen{next: 251}
	svc := NewService(repo, idgen, nil, nil, "test-secret", 0, 8, nil)
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

	_, err = svc.UpdateProfile(ctx, alice.ID, domain.UpdateProfileCmd{Nickname: "Alice", ProfileTheme: "vip-gold"})
	if !errors.Is(err, domain.ErrInvalidProfileTheme) {
		t.Fatalf("expected invalid profile theme error, got %v", err)
	}
}

func TestServicePasswordReset(t *testing.T) {
	repo := newMemoryRepo()
	idgen := &fakeIDGen{next: 300}
	svc := NewService(repo, idgen, nil, nil, "test-secret", 0, 8, nil)
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
	svc := NewService(repo, idgen, nil, nil, "test-secret", 0, 8, nil)
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

func TestServiceVerifyEmailHidesPremiumProfileWithoutActiveEntitlements(t *testing.T) {
	repo := newMemoryRepo()
	idgen := &fakeIDGen{next: 410}
	entitlements := &fakeProfileThemeEntitlements{}
	svc := NewService(repo, idgen, nil, nil, "test-secret", 0, 8, entitlements)
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
	seedPremiumProfile(t, repo, alice.ID)
	result, err := svc.RequestEmailVerification(ctx, alice.ID)
	if err != nil {
		t.Fatalf("request email verification: %v", err)
	}

	verified, err := svc.VerifyEmail(ctx, result.VerificationToken)
	if err != nil {
		t.Fatalf("verify email: %v", err)
	}
	if verified.BackgroundURL != "" {
		t.Fatalf("verified background url = %q, want hidden", verified.BackgroundURL)
	}
	if verified.ProfileTheme != domain.ProfileThemeDefault {
		t.Fatalf("verified profile theme = %q, want default", verified.ProfileTheme)
	}
	stored, err := repo.FindByID(ctx, alice.ID)
	if err != nil {
		t.Fatalf("find stored user: %v", err)
	}
	if stored.BackgroundURL == "" || stored.ProfileTheme != domain.ProfileThemePro {
		t.Fatalf("stored premium profile = background:%q theme:%q, want preserved", stored.BackgroundURL, stored.ProfileTheme)
	}
	if entitlements.membershipCalls != 1 || entitlements.calls != 1 {
		t.Fatalf("entitlement checks = membership:%d theme:%d, want 1/1", entitlements.membershipCalls, entitlements.calls)
	}
}

func TestServiceUpdateStatusHidesPremiumProfileWithoutActiveEntitlements(t *testing.T) {
	repo := newMemoryRepo()
	idgen := &fakeIDGen{next: 420}
	entitlements := &fakeProfileThemeEntitlements{}
	svc := NewService(repo, idgen, nil, nil, "test-secret", 0, 8, entitlements)
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
	seedPremiumProfile(t, repo, alice.ID)

	updated, err := svc.UpdateStatus(ctx, alice.ID, domain.StatusMuted)
	if err != nil {
		t.Fatalf("update status: %v", err)
	}
	if updated.BackgroundURL != "" {
		t.Fatalf("updated background url = %q, want hidden", updated.BackgroundURL)
	}
	if updated.ProfileTheme != domain.ProfileThemeDefault {
		t.Fatalf("updated profile theme = %q, want default", updated.ProfileTheme)
	}
	stored, err := repo.FindByID(ctx, alice.ID)
	if err != nil {
		t.Fatalf("find stored user: %v", err)
	}
	if stored.BackgroundURL == "" || stored.ProfileTheme != domain.ProfileThemePro {
		t.Fatalf("stored premium profile = background:%q theme:%q, want preserved", stored.BackgroundURL, stored.ProfileTheme)
	}
	if stored.Status != domain.StatusMuted {
		t.Fatalf("stored status = %d, want muted", stored.Status)
	}
	if entitlements.membershipCalls != 1 || entitlements.calls != 1 {
		t.Fatalf("entitlement checks = membership:%d theme:%d, want 1/1", entitlements.membershipCalls, entitlements.calls)
	}
}

type fakeIDGen struct {
	next int64
}

func (g *fakeIDGen) Generate() int64 {
	g.next++
	return g.next
}

type fakeProfileThemeEntitlements struct {
	allowed           bool
	membershipAllowed bool
	err               error
	membershipErr     error
	calls             int
	membershipCalls   int
	userID            int64
	membershipUserID  int64
	theme             string
}

func (f *fakeProfileThemeEntitlements) HasActiveProfileTheme(_ context.Context, userID int64, theme string) (bool, error) {
	f.calls++
	f.userID = userID
	f.theme = theme
	if f.err != nil {
		return false, f.err
	}
	return f.allowed, nil
}

func (f *fakeProfileThemeEntitlements) HasActiveMembership(_ context.Context, userID int64) (bool, error) {
	f.membershipCalls++
	f.membershipUserID = userID
	if f.membershipErr != nil {
		return false, f.membershipErr
	}
	return f.membershipAllowed, nil
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
			updated := cloneUser(existing)
			updated.Email = u.Email
			updated.PasswordHash = u.PasswordHash
			updated.Nickname = u.Nickname
			updated.Status = domain.StatusActive
			updated.LastLoginAt = u.LastLoginAt
			updated.UpdatedAt = u.UpdatedAt
			r.users[id] = cloneUser(updated)
			*u = *cloneUser(updated)
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

func seedPremiumProfile(t *testing.T, repo *memoryRepo, userID int64) {
	t.Helper()
	user, err := repo.FindByID(context.Background(), userID)
	if err != nil {
		t.Fatalf("find user for premium seed: %v", err)
	}
	user.BackgroundURL = "https://example.com/background.webp"
	user.ProfileTheme = domain.ProfileThemePro
	if err := repo.UpdateProfile(context.Background(), user); err != nil {
		t.Fatalf("seed premium profile: %v", err)
	}
}
