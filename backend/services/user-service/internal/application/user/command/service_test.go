package command

import (
	"context"
	"errors"
	"sort"
	"strings"
	"sync"
	"testing"
	"time"

	domain "user-service/internal/domain/user"

	"github.com/golang-jwt/jwt/v5"
)

func TestRegisterInviteValidationAndAtomicConsumption(t *testing.T) {
	ctx := context.Background()
	repo := newInviteMemoryRepo()
	svc := NewService(repo, &fakeIDGen{next: 500}, nil, nil, "test-secret", time.Hour, 8, nil, nil, nil)

	if _, _, err := svc.Register(ctx, domain.RegisterCmd{
		Username: "alice", Email: "alice@example.com", Password: "short", RequireInvite: true,
	}); !errors.Is(err, domain.ErrPasswordTooShort) {
		t.Fatalf("short password error = %v, want ErrPasswordTooShort", err)
	}
	if _, _, err := svc.Register(ctx, domain.RegisterCmd{
		Username: "a!", Email: "alice@example.com", Password: "password123", RequireInvite: true,
	}); !errors.Is(err, domain.ErrUsernameInvalid) {
		t.Fatalf("invalid username error = %v, want ErrUsernameInvalid", err)
	}
	if _, _, err := svc.Register(ctx, domain.RegisterCmd{
		Username: "alice", Email: "alice@example.com", Password: "password123", RequireInvite: true,
	}); !errors.Is(err, domain.ErrInviteCodeRequired) {
		t.Fatalf("missing invite error = %v, want ErrInviteCodeRequired", err)
	}

	now := time.Now()
	repo.invites[1] = domain.InviteCode{ID: 1, Code: "VALID", CreatedByAdminID: 9, CreatedAt: now}
	alice, token, err := svc.Register(ctx, domain.RegisterCmd{
		Username: "alice", Email: "alice@example.com", Password: "password123", InviteCode: " valid ", RequireInvite: true,
	})
	if err != nil {
		t.Fatalf("register with invite: %v", err)
	}
	if alice.ID <= 0 || token.Value == "" {
		t.Fatalf("registration result = user:%+v token:%q", alice, token.Value)
	}
	consumed := repo.invites[1]
	if consumed.UsedByUserID == nil || *consumed.UsedByUserID != alice.ID || consumed.UsedAt == nil {
		t.Fatalf("consumed invite = %+v, want user %d", consumed, alice.ID)
	}

	repo.invites[2] = domain.InviteCode{ID: 2, Code: "CONFLICT", CreatedByAdminID: 9, CreatedAt: now}
	if _, _, err := svc.Register(ctx, domain.RegisterCmd{
		Username: "alice", Email: "other@example.com", Password: "password123", InviteCode: "CONFLICT", RequireInvite: true,
	}); !errors.Is(err, domain.ErrUsernameExists) {
		t.Fatalf("duplicate username error = %v, want ErrUsernameExists", err)
	}
	if invite := repo.invites[2]; invite.UsedAt != nil || invite.UsedByUserID != nil {
		t.Fatalf("failed registration consumed invite: %+v", invite)
	}
}

func TestRegisterTokenFailureDoesNotCreateUserOrConsumeInvite(t *testing.T) {
	ctx := context.Background()
	repo := newInviteMemoryRepo()
	repo.invites[1] = domain.InviteCode{ID: 1, Code: "TOKENFAIL", CreatedByAdminID: 9, CreatedAt: time.Now()}
	svc := NewService(repo, &fakeIDGen{next: 600}, nil, nil, "", time.Hour, 8, nil, nil, nil)

	if _, _, err := svc.Register(ctx, domain.RegisterCmd{
		Username: "alice", Email: "alice@example.com", Password: "password123", InviteCode: "TOKENFAIL", RequireInvite: true,
	}); err == nil || !strings.Contains(err.Error(), "jwt secret required") {
		t.Fatalf("token failure error = %v", err)
	}
	if len(repo.users) != 0 {
		t.Fatalf("users after token failure = %d, want 0", len(repo.users))
	}
	if invite := repo.invites[1]; invite.UsedAt != nil || invite.UsedByUserID != nil {
		t.Fatalf("token failure consumed invite: %+v", invite)
	}
}

func TestRegisterWithInviteFailsClosedWithoutInviteRepository(t *testing.T) {
	base := newMemoryRepo()
	repo := struct{ domain.Repository }{Repository: base}
	svc := NewService(repo, &fakeIDGen{next: 650}, nil, nil, "test-secret", time.Hour, 8, nil, nil, nil)

	if _, _, err := svc.Register(context.Background(), domain.RegisterCmd{
		Username: "alice", Email: "alice@example.com", Password: "password123", InviteCode: "INVITE", RequireInvite: true,
	}); !errors.Is(err, domain.ErrInviteRepositoryUnavailable) {
		t.Fatalf("register error = %v, want ErrInviteRepositoryUnavailable", err)
	}
	if len(base.users) != 0 {
		t.Fatalf("users = %d, want 0", len(base.users))
	}
}

func TestConcurrentRegistrationConsumesInviteOnce(t *testing.T) {
	ctx := context.Background()
	repo := newInviteMemoryRepo()
	repo.invites[1] = domain.InviteCode{ID: 1, Code: "ONCEONLY", CreatedByAdminID: 9, CreatedAt: time.Now()}
	services := []*Service{
		NewService(repo, &fakeIDGen{next: 700}, nil, nil, "test-secret", time.Hour, 8, nil, nil, nil),
		NewService(repo, &fakeIDGen{next: 800}, nil, nil, "test-secret", time.Hour, 8, nil, nil, nil),
	}
	commands := []domain.RegisterCmd{
		{Username: "alice", Email: "alice@example.com", Password: "password123", InviteCode: "ONCEONLY", RequireInvite: true},
		{Username: "bob", Email: "bob@example.com", Password: "password123", InviteCode: "ONCEONLY", RequireInvite: true},
	}

	start := make(chan struct{})
	errs := make(chan error, 2)
	var wg sync.WaitGroup
	for i := range services {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			<-start
			_, _, err := services[i].Register(ctx, commands[i])
			errs <- err
		}(i)
	}
	close(start)
	wg.Wait()
	close(errs)

	var successes, usedErrors int
	for err := range errs {
		switch {
		case err == nil:
			successes++
		case errors.Is(err, domain.ErrInviteCodeUsed):
			usedErrors++
		default:
			t.Fatalf("unexpected concurrent registration error: %v", err)
		}
	}
	if successes != 1 || usedErrors != 1 || len(repo.users) != 1 {
		t.Fatalf("concurrent result successes=%d used=%d users=%d, want 1/1/1", successes, usedErrors, len(repo.users))
	}
}

func TestInviteCodeAdministrationAndStatusFiltering(t *testing.T) {
	ctx := context.Background()
	repo := newInviteMemoryRepo()
	svc := NewService(repo, &fakeIDGen{next: 900}, nil, nil, "test-secret", time.Hour, 8, nil, nil, nil)

	if _, err := svc.CreateInviteCodes(ctx, 42, 0, nil); !errors.Is(err, domain.ErrInviteCountInvalid) {
		t.Fatalf("zero count error = %v", err)
	}
	past := time.Now().Add(-time.Minute)
	if _, err := svc.CreateInviteCodes(ctx, 42, 1, &past); !errors.Is(err, domain.ErrInviteExpiryInvalid) {
		t.Fatalf("past expiry error = %v", err)
	}
	future := time.Now().Add(time.Hour)
	created, err := svc.CreateInviteCodes(ctx, 42, 3, &future)
	if err != nil {
		t.Fatalf("create invite codes: %v", err)
	}
	seen := map[string]bool{}
	for _, invite := range created {
		if invite.CreatedByAdminID != 42 || len(invite.Code) != 20 || invite.Code != strings.ToUpper(invite.Code) || seen[invite.Code] {
			t.Fatalf("created invite = %+v", invite)
		}
		seen[invite.Code] = true
	}
	if err := svc.RevokeInviteCode(ctx, 77, created[0].ID); err != nil {
		t.Fatalf("revoke invite: %v", err)
	}
	revoked := repo.invites[created[0].ID]
	if revoked.RevokedAt == nil || revoked.RevokedByAdminID == nil || *revoked.RevokedByAdminID != 77 {
		t.Fatalf("revoked invite = %+v", revoked)
	}
	items, total, err := svc.ListInviteCodes(ctx, domain.InviteCodeListQuery{Status: domain.InviteStatusUnused, Page: 1, PageSize: 10})
	if err != nil || total != 2 || len(items) != 2 {
		t.Fatalf("unused invites total=%d len=%d err=%v", total, len(items), err)
	}
	items, total, err = svc.ListInviteCodes(ctx, domain.InviteCodeListQuery{Status: domain.InviteStatusRevoked, Page: 1, PageSize: 10})
	if err != nil || total != 1 || len(items) != 1 || items[0].ID != created[0].ID {
		t.Fatalf("revoked invites total=%d items=%+v err=%v", total, items, err)
	}
	if _, _, err := svc.ListInviteCodes(ctx, domain.InviteCodeListQuery{Status: "unknown"}); !errors.Is(err, domain.ErrInviteStatusInvalid) {
		t.Fatalf("invalid status error = %v", err)
	}
}

func TestOAuthExistingOnlyAllowsLoginButRejectsSignup(t *testing.T) {
	ctx := context.Background()
	repo := newMemoryRepo()
	svc := NewService(repo, &fakeIDGen{next: 1000}, nil, nil, "test-secret", time.Hour, 8, nil, nil, nil)

	if _, _, err := svc.OAuthLogin(ctx, domain.OAuthLoginCmd{
		Provider: "github", ProviderUserID: "missing", Username: "missing", ExistingOnly: true,
	}); !errors.Is(err, domain.ErrOAuthSignupDisabled) {
		t.Fatalf("existing-only signup error = %v", err)
	}
	created, _, err := svc.OAuthLogin(ctx, domain.OAuthLoginCmd{
		Provider: "github", ProviderUserID: "known", Username: "known", Email: "known@example.com",
	})
	if err != nil {
		t.Fatalf("create oauth user: %v", err)
	}
	loggedIn, token, err := svc.OAuthLogin(ctx, domain.OAuthLoginCmd{
		Provider: "github", ProviderUserID: "known", ExistingOnly: true,
	})
	if err != nil || loggedIn.ID != created.ID || token.Value == "" {
		t.Fatalf("existing-only login user=%+v token=%q err=%v", loggedIn, token.Value, err)
	}
}

func TestUserSafetyRelationsGuardFollowing(t *testing.T) {
	repo := newMemoryRepo()
	svc := NewService(repo, &fakeIDGen{next: 200}, nil, nil, "test-secret", time.Hour, 8, nil, nil, nil)
	ctx := context.Background()
	alice, _, err := svc.Register(ctx, domain.RegisterCmd{Username: "alice", Email: "alice@example.com", Password: "password1", Nickname: "Alice"})
	if err != nil {
		t.Fatalf("register alice: %v", err)
	}
	bob, _, err := svc.Register(ctx, domain.RegisterCmd{Username: "bob", Email: "bob@example.com", Password: "password1", Nickname: "Bob"})
	if err != nil {
		t.Fatalf("register bob: %v", err)
	}
	if _, err := svc.Follow(ctx, alice.ID, bob.ID); err != nil {
		t.Fatalf("follow bob: %v", err)
	}
	if _, err := svc.Follow(ctx, bob.ID, alice.ID); err != nil {
		t.Fatalf("follow alice: %v", err)
	}
	if err := svc.Block(ctx, alice.ID, bob.ID); err != nil {
		t.Fatalf("block bob: %v", err)
	}
	if _, ok := repo.follows[[2]int64{alice.ID, bob.ID}]; ok {
		t.Fatal("blocking must remove actor follow")
	}
	if _, ok := repo.follows[[2]int64{bob.ID, alice.ID}]; ok {
		t.Fatal("blocking must remove target follow")
	}
	relation, err := repo.GetSafetyRelation(ctx, alice.ID, bob.ID)
	if err != nil || !relation.Blocked || !relation.Muted {
		t.Fatalf("block relation = %+v, %v; want blocked and muted", relation, err)
	}
	if _, err := svc.Follow(ctx, alice.ID, bob.ID); !errors.Is(err, domain.ErrFollowBlocked) {
		t.Fatalf("actor follow after block = %v, want ErrFollowBlocked", err)
	}
	if _, err := svc.Follow(ctx, bob.ID, alice.ID); !errors.Is(err, domain.ErrFollowBlocked) {
		t.Fatalf("target follow after block = %v, want ErrFollowBlocked", err)
	}
	if err := svc.Unblock(ctx, alice.ID, bob.ID); err != nil {
		t.Fatalf("unblock bob: %v", err)
	}
	relation, err = repo.GetSafetyRelation(ctx, alice.ID, bob.ID)
	if err != nil || relation.Blocked || relation.Muted {
		t.Fatalf("unblock relation = %+v, %v; want neither blocked nor muted", relation, err)
	}
	if _, err := svc.Follow(ctx, alice.ID, bob.ID); err != nil {
		t.Fatalf("follow after unblock: %v", err)
	}
}

func TestBlockWithoutExistingFollowAndDuplicateSafetyOperations(t *testing.T) {
	repo := newMemoryRepo()
	svc := NewService(repo, &fakeIDGen{next: 250}, nil, nil, "test-secret", time.Hour, 8, nil, nil, nil)
	ctx := context.Background()
	alice, _, _ := svc.Register(ctx, domain.RegisterCmd{Username: "alice", Email: "alice@example.com", Password: "password1", Nickname: "Alice"})
	bob, _, _ := svc.Register(ctx, domain.RegisterCmd{Username: "bob", Email: "bob@example.com", Password: "password1", Nickname: "Bob"})

	if err := svc.Block(ctx, alice.ID, bob.ID); err != nil {
		t.Fatalf("block without follow: %v", err)
	}
	if err := svc.Block(ctx, alice.ID, bob.ID); !errors.Is(err, domain.ErrAlreadyBlocking) {
		t.Fatalf("duplicate block = %v, want ErrAlreadyBlocking", err)
	}
	if err := svc.Mute(ctx, bob.ID, alice.ID); err != nil {
		t.Fatalf("mute: %v", err)
	}
	if err := svc.Mute(ctx, bob.ID, alice.ID); !errors.Is(err, domain.ErrAlreadyMuted) {
		t.Fatalf("duplicate mute = %v, want ErrAlreadyMuted", err)
	}
	if err := svc.Unmute(ctx, bob.ID, alice.ID); err != nil {
		t.Fatalf("unmute: %v", err)
	}
	if err := svc.Unmute(ctx, bob.ID, alice.ID); !errors.Is(err, domain.ErrNotMuted) {
		t.Fatalf("duplicate unmute = %v, want ErrNotMuted", err)
	}
	if err := svc.Unblock(ctx, alice.ID, bob.ID); err != nil {
		t.Fatalf("unblock: %v", err)
	}
	if err := svc.Unblock(ctx, alice.ID, bob.ID); !errors.Is(err, domain.ErrNotBlocking) {
		t.Fatalf("duplicate unblock = %v, want ErrNotBlocking", err)
	}
}

func TestSafetyRelationsRejectSelfAndMissingUsers(t *testing.T) {
	repo := newMemoryRepo()
	svc := NewService(repo, &fakeIDGen{next: 280}, nil, nil, "test-secret", time.Hour, 8, nil, nil, nil)
	ctx := context.Background()
	alice, _, _ := svc.Register(ctx, domain.RegisterCmd{Username: "alice", Email: "alice@example.com", Password: "password1", Nickname: "Alice"})

	for name, action := range map[string]func() error{
		"block self":   func() error { return svc.Block(ctx, alice.ID, alice.ID) },
		"unblock self": func() error { return svc.Unblock(ctx, alice.ID, alice.ID) },
		"mute self":    func() error { return svc.Mute(ctx, alice.ID, alice.ID) },
		"unmute self":  func() error { return svc.Unmute(ctx, alice.ID, alice.ID) },
	} {
		t.Run(name, func(t *testing.T) {
			if err := action(); !errors.Is(err, domain.ErrCannotRelateSelf) {
				t.Fatalf("error = %v, want ErrCannotRelateSelf", err)
			}
		})
	}

	for name, action := range map[string]func() error{
		"block missing":   func() error { return svc.Block(ctx, alice.ID, 999) },
		"unblock missing": func() error { return svc.Unblock(ctx, alice.ID, 999) },
		"mute missing":    func() error { return svc.Mute(ctx, alice.ID, 999) },
		"unmute missing":  func() error { return svc.Unmute(ctx, alice.ID, 999) },
	} {
		t.Run(name, func(t *testing.T) {
			if err := action(); !errors.Is(err, domain.ErrNotFound) {
				t.Fatalf("error = %v, want ErrNotFound", err)
			}
		})
	}
}

func TestUserMuteCanBeToggledWithoutRemovingFollow(t *testing.T) {
	repo := newMemoryRepo()
	svc := NewService(repo, &fakeIDGen{next: 300}, nil, nil, "test-secret", time.Hour, 8, nil, nil, nil)
	ctx := context.Background()
	alice, _, _ := svc.Register(ctx, domain.RegisterCmd{Username: "alice", Email: "alice@example.com", Password: "password1", Nickname: "Alice"})
	bob, _, _ := svc.Register(ctx, domain.RegisterCmd{Username: "bob", Email: "bob@example.com", Password: "password1", Nickname: "Bob"})
	if _, err := svc.Follow(ctx, alice.ID, bob.ID); err != nil {
		t.Fatalf("follow bob: %v", err)
	}
	if err := svc.Mute(ctx, alice.ID, bob.ID); err != nil {
		t.Fatalf("mute bob: %v", err)
	}
	if _, ok := repo.follows[[2]int64{alice.ID, bob.ID}]; !ok {
		t.Fatal("muting must not remove follow")
	}
	relation, err := repo.GetSafetyRelation(ctx, alice.ID, bob.ID)
	if err != nil || !relation.Muted {
		t.Fatalf("mute relation = %+v, %v", relation, err)
	}
	if err := svc.Unmute(ctx, alice.ID, bob.ID); err != nil {
		t.Fatalf("unmute bob: %v", err)
	}
	relation, err = repo.GetSafetyRelation(ctx, alice.ID, bob.ID)
	if err != nil || relation.Muted {
		t.Fatalf("unmuted relation = %+v, %v", relation, err)
	}
}

func TestIssueTokenUsesUniqueJWTID(t *testing.T) {
	svc := NewService(newMemoryRepo(), &fakeIDGen{next: 1}, nil, nil, "test-secret", time.Hour, 8, nil, nil, nil)
	user := &domain.User{ID: 1, Username: "alice", CredentialVersion: domain.InitialCredentialVersion}

	first, err := svc.issueToken(context.Background(), user, LoginMethodPassword)
	if err != nil {
		t.Fatalf("issue first token: %v", err)
	}
	second, err := svc.issueToken(context.Background(), user, LoginMethodPassword)
	if err != nil {
		t.Fatalf("issue second token: %v", err)
	}
	if first.Value == second.Value {
		t.Fatal("consecutive tokens must differ")
	}

	parse := func(raw string) string {
		t.Helper()
		parsed, err := jwt.Parse(raw, func(token *jwt.Token) (any, error) {
			if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
				t.Fatalf("unexpected signing method: %v", token.Header["alg"])
			}
			return []byte("test-secret"), nil
		})
		if err != nil || !parsed.Valid {
			t.Fatalf("parse signed token: valid=%v err=%v", parsed != nil && parsed.Valid, err)
		}
		claims, ok := parsed.Claims.(jwt.MapClaims)
		if !ok {
			t.Fatalf("claims type = %T", parsed.Claims)
		}
		jti, ok := claims["jti"].(string)
		if !ok || jti == "" {
			t.Fatalf("jti = %#v, want non-empty string", claims["jti"])
		}
		if version, ok := claims[credentialVersionClaim].(string); !ok || version != credentialVersionInitial {
			t.Fatalf("credential version = %#v, want %q", claims[credentialVersionClaim], credentialVersionInitial)
		}
		return jti
	}

	if firstJTI, secondJTI := parse(first.Value), parse(second.Value); firstJTI == secondJTI {
		t.Fatal("consecutive tokens must have different jti values")
	}
}

func credentialVersionFromToken(t *testing.T, raw string) string {
	t.Helper()
	parsed, err := jwt.Parse(raw, func(token *jwt.Token) (any, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			t.Fatalf("unexpected signing method: %v", token.Header["alg"])
		}
		return []byte("test-secret"), nil
	})
	if err != nil || !parsed.Valid {
		t.Fatalf("parse token: valid=%v err=%v", parsed != nil && parsed.Valid, err)
	}
	claims, ok := parsed.Claims.(jwt.MapClaims)
	if !ok {
		t.Fatalf("claims type = %T", parsed.Claims)
	}
	version, ok := claims[credentialVersionClaim].(string)
	if !ok || version == "" {
		t.Fatalf("credential version = %#v, want non-empty string", claims[credentialVersionClaim])
	}
	return version
}

func TestServiceRegisterLoginAndFollow(t *testing.T) {
	repo := newMemoryRepo()
	idgen := &fakeIDGen{next: 100}
	svc := NewService(repo, idgen, nil, nil, "test-secret", 0, 8, nil, nil, nil)
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

	if _, err := svc.Follow(ctx, alice.ID, bob.ID); err != nil {
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

func TestPasswordChangeRotatesCredentialVersionForFutureJWTs(t *testing.T) {
	repo := newMFAMemoryRepo()
	cache := newCredentialVersionCacheStub()
	svc := NewService(repo, &fakeIDGen{next: 125}, nil, nil, "test-secret", time.Hour, 8, nil, nil, cache)
	ctx := context.Background()

	alice, before, err := svc.Register(ctx, domain.RegisterCmd{
		Username: "credential_alice", Email: "credential_alice@example.com", Password: "password123", Nickname: "Credential Alice",
	})
	if err != nil {
		t.Fatalf("register: %v", err)
	}
	if got := credentialVersionFromToken(t, before.Value); got != credentialVersionInitial {
		t.Fatalf("initial credential version = %q, want %q", got, credentialVersionInitial)
	}

	if err := svc.ChangePassword(ctx, alice.ID, "password123", "changedpass123", ""); err != nil {
		t.Fatalf("change password: %v", err)
	}
	stored, err := repo.FindByID(ctx, alice.ID)
	if err != nil {
		t.Fatalf("find changed user: %v", err)
	}
	rotated := stored.CredentialVersion
	if rotated == "" || rotated == credentialVersionInitial {
		t.Fatalf("rotated credential version = %q", rotated)
	}
	if got := cache.versions[alice.ID]; got != rotated {
		t.Fatalf("cached credential version = %q, want %q", got, rotated)
	}
	_, after, err := svc.Login(ctx, alice.Username, "changedpass123")
	if err != nil {
		t.Fatalf("login with changed password: %v", err)
	}
	if got := credentialVersionFromToken(t, after.Value); got != rotated {
		t.Fatalf("new token credential version = %q, want %q", got, rotated)
	}
}

func TestPasswordChangeRejectsAConcurrentCredentialRotation(t *testing.T) {
	repo := newMFAMemoryRepo()
	svc := NewService(repo, &fakeIDGen{next: 130}, nil, nil, "test-secret", time.Hour, 8, nil, nil, newCredentialVersionCacheStub())
	ctx := context.Background()

	alice, _, err := svc.Register(ctx, domain.RegisterCmd{
		Username: "credential_race", Email: "credential_race@example.com", Password: "password123", Nickname: "Credential Race",
	})
	if err != nil {
		t.Fatalf("register: %v", err)
	}
	repo.beforePasswordUpdate = func() {
		repo.beforePasswordUpdate = nil
		stored, findErr := repo.FindByID(ctx, alice.ID)
		if findErr != nil {
			t.Fatalf("find concurrent user: %v", findErr)
		}
		stored.PasswordHash = "other-password-hash"
		stored.CredentialVersion = "other-credential-version"
		repo.users[alice.ID] = cloneUser(stored)
	}
	if err := svc.ChangePassword(ctx, alice.ID, "password123", "changedpass123", ""); !errors.Is(err, domain.ErrInvalidPassword) {
		t.Fatalf("change password error = %v, want stale credential rejection", err)
	}
	stored, err := repo.FindByID(ctx, alice.ID)
	if err != nil {
		t.Fatalf("find race winner: %v", err)
	}
	if stored.PasswordHash != "other-password-hash" || stored.CredentialVersion != "other-credential-version" {
		t.Fatalf("concurrent credential rotation was overwritten: %+v", stored)
	}
}

func TestPasswordChangeSucceedsAndDeletesStaleCacheWhenRefreshFails(t *testing.T) {
	repo := newMFAMemoryRepo()
	cache := newCredentialVersionCacheStub()
	cache.setErr = errors.New("redis set unavailable")
	svc := NewService(repo, &fakeIDGen{next: 125}, nil, nil, "test-secret", time.Hour, 8, nil, nil, cache)
	ctx := context.Background()

	alice, _, err := svc.Register(ctx, domain.RegisterCmd{
		Username: "credential_cache_failure", Email: "credential_cache_failure@example.com", Password: "password123", Nickname: "Credential Cache Failure",
	})
	if err != nil {
		t.Fatalf("register: %v", err)
	}
	cache.versions[alice.ID] = "stale-version"
	if err := svc.ChangePassword(ctx, alice.ID, "password123", "changedpass123", ""); err != nil {
		t.Fatalf("change password: %v", err)
	}
	stored, err := repo.FindByID(ctx, alice.ID)
	if err != nil {
		t.Fatalf("find changed user: %v", err)
	}
	if stored.CredentialVersion == credentialVersionInitial {
		t.Fatalf("durable credential version was not rotated")
	}
	if _, ok := cache.versions[alice.ID]; ok {
		t.Fatalf("stale credential cache remained after failed refresh: %#v", cache.versions)
	}
	if cache.deleteCalls != 1 {
		t.Fatalf("cache delete calls = %d, want 1", cache.deleteCalls)
	}
}

func TestPasswordResetSucceedsAndDeletesStaleCacheWhenRefreshFails(t *testing.T) {
	repo := newMemoryRepo()
	emails := &securityEmailSenderStub{ready: true}
	cache := newCredentialVersionCacheStub()
	cache.setErr = errors.New("redis set unavailable")
	svc := NewService(repo, &fakeIDGen{next: 126}, nil, nil, "test-secret", time.Hour, 8, nil, emails, cache)
	ctx := context.Background()

	alice, _, err := svc.Register(ctx, domain.RegisterCmd{
		Username: "reset_cache_failure", Email: "reset_cache_failure@example.com", Password: "password123", Nickname: "Reset Cache Failure",
	})
	if err != nil {
		t.Fatalf("register: %v", err)
	}
	reset, err := svc.RequestPasswordReset(ctx, alice.Email)
	if err != nil {
		t.Fatalf("request password reset: %v", err)
	}
	cache.versions[alice.ID] = "stale-version"
	if err := svc.ResetPassword(ctx, reset.ResetToken, "changedpass123"); err != nil {
		t.Fatalf("reset password: %v", err)
	}
	stored, err := repo.FindByID(ctx, alice.ID)
	if err != nil {
		t.Fatalf("find reset user: %v", err)
	}
	if stored.CredentialVersion == credentialVersionInitial {
		t.Fatalf("durable credential version was not rotated")
	}
	if _, ok := cache.versions[alice.ID]; ok {
		t.Fatalf("stale credential cache remained after failed refresh: %#v", cache.versions)
	}
	if cache.deleteCalls != 1 {
		t.Fatalf("cache delete calls = %d, want 1", cache.deleteCalls)
	}
}

func TestLoginUsesTheCredentialVersionReadWithThePassword(t *testing.T) {
	repo := newMFAMemoryRepo()
	svc := NewService(repo, &fakeIDGen{next: 140}, nil, nil, "test-secret", time.Hour, 8, nil, nil, newCredentialVersionCacheStub())
	ctx := context.Background()

	alice, before, err := svc.Register(ctx, domain.RegisterCmd{
		Username: "credential_login_race", Email: "credential_login_race@example.com", Password: "password123", Nickname: "Credential Login Race",
	})
	if err != nil {
		t.Fatalf("register: %v", err)
	}
	oldVersion := credentialVersionFromToken(t, before.Value)
	var rotationErr error
	repo.beforeUpdateLastLogin = func() {
		rotationErr = svc.ChangePassword(ctx, alice.ID, "password123", "changedpass123", "")
	}

	_, token, err := svc.Login(ctx, alice.Username, "password123")
	if err != nil {
		t.Fatalf("concurrent login: %v", err)
	}
	if rotationErr != nil {
		t.Fatalf("concurrent password change: %v", rotationErr)
	}
	current, err := repo.FindByID(ctx, alice.ID)
	if err != nil {
		t.Fatalf("find changed user: %v", err)
	}
	if current.CredentialVersion == oldVersion {
		t.Fatalf("credential version was not rotated: %q", current.CredentialVersion)
	}
	if got := credentialVersionFromToken(t, token.Value); got != oldVersion {
		t.Fatalf("login token credential version = %q, want captured %q", got, oldVersion)
	}
}

func TestServiceRegisterBoundsEventPublishDeadline(t *testing.T) {
	publisher := &deadlinePublisher{}
	svc := NewService(newMemoryRepo(), &fakeIDGen{next: 150}, publisher, nil, "test-secret", 0, 8, nil, nil, nil)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if _, _, err := svc.Register(ctx, domain.RegisterCmd{
		Username: "deadline_user",
		Email:    "deadline@example.com",
		Password: "password123",
		Nickname: "Deadline User",
	}); err != nil {
		t.Fatalf("register user: %v", err)
	}
	if !publisher.hasDeadline {
		t.Fatal("event publisher context has no deadline")
	}
	if remaining := time.Until(publisher.deadline); remaining < eventPublishTimeout-250*time.Millisecond || remaining > eventPublishTimeout+100*time.Millisecond {
		t.Fatalf("event publisher deadline remaining = %s, want about %s", remaining, eventPublishTimeout)
	}
}

func TestServiceOAuthAndWebmasterLogin(t *testing.T) {
	repo := newMemoryRepo()
	idgen := &fakeIDGen{next: 200}
	svc := NewService(repo, idgen, nil, nil, "test-secret", 0, 8, nil, nil, nil)
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
	svc := NewService(repo, idgen, nil, nil, "test-secret", 0, 8, entitlements, nil, nil)
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
	svc := NewService(repo, idgen, nil, nil, "test-secret", 0, 8, entitlements, nil, nil)
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
	svc := NewService(repo, idgen, nil, nil, "test-secret", 0, 8, entitlements, nil, nil)
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
	svc := NewService(repo, idgen, nil, nil, "test-secret", 0, 8, entitlements, nil, nil)
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
	svc := NewService(repo, idgen, nil, nil, "test-secret", 0, 8, entitlements, nil, nil)
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

func TestServiceUpdateProfileFailsClosedWhenMembershipLookupUnavailable(t *testing.T) {
	repo := newMemoryRepo()
	idgen := &fakeIDGen{next: 255}
	membershipErr := errors.New("mall unavailable")
	entitlements := &fakeProfileThemeEntitlements{membershipErr: membershipErr}
	svc := NewService(repo, idgen, nil, nil, "test-secret", 0, 8, entitlements, nil, nil)
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
		Nickname:      "Alice Updated",
		BackgroundURL: "https://example.com/background.webp",
	})
	if !errors.Is(err, membershipErr) {
		t.Fatalf("err = %v, want membership lookup error", err)
	}
	stored, err := repo.FindByID(ctx, alice.ID)
	if err != nil {
		t.Fatalf("find user: %v", err)
	}
	if stored.Nickname != "Alice" || stored.BackgroundURL != "" {
		t.Fatalf("stored user = %+v, want unchanged profile", stored)
	}
	if entitlements.membershipCalls != 1 || entitlements.membershipUserID != alice.ID {
		t.Fatalf("membership entitlement check calls=%d user_id=%d", entitlements.membershipCalls, entitlements.membershipUserID)
	}
}

func TestServiceUpdateProfileRejectsProfileThemeWithoutEntitlement(t *testing.T) {
	repo := newMemoryRepo()
	idgen := &fakeIDGen{next: 252}
	entitlements := &fakeProfileThemeEntitlements{}
	svc := NewService(repo, idgen, nil, nil, "test-secret", 0, 8, entitlements, nil, nil)
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
	svc := NewService(repo, idgen, nil, nil, "test-secret", 0, 8, entitlements, nil, nil)
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
	svc := NewService(repo, idgen, nil, nil, "test-secret", 0, 8, nil, nil, nil)
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
	emails := &securityEmailSenderStub{ready: true}
	cache := newCredentialVersionCacheStub()
	svc := NewService(repo, idgen, nil, nil, "test-secret", 0, 8, nil, emails, cache)
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
	if len(emails.passwordResets) != 1 || emails.passwordResets[0].recipient != alice.Email || emails.passwordResets[0].token != result.ResetToken || !emails.passwordResets[0].expiresAt.Equal(result.ExpiresAt) {
		t.Fatalf("password reset email = %+v, want recipient=%q token=%q expiry=%s", emails.passwordResets, alice.Email, result.ResetToken, result.ExpiresAt)
	}
	missing, err := svc.RequestPasswordReset(ctx, "missing@example.com")
	if err != nil {
		t.Fatalf("request missing password reset: %v", err)
	}
	if !missing.Accepted || missing.ResetToken != "" {
		t.Fatalf("missing account leaked reset token: %+v", missing)
	}
	if len(emails.passwordResets) != 1 {
		t.Fatalf("password reset emails = %d, want 1", len(emails.passwordResets))
	}

	if err := svc.ResetPassword(ctx, result.ResetToken, "newpass123"); err != nil {
		t.Fatalf("reset password: %v", err)
	}
	stored, err := repo.FindByID(ctx, alice.ID)
	if err != nil {
		t.Fatalf("find reset user: %v", err)
	}
	if stored.CredentialVersion == "" || stored.CredentialVersion == credentialVersionInitial {
		t.Fatalf("password reset did not rotate credential version: %q", stored.CredentialVersion)
	}
	if got := cache.versions[alice.ID]; got != stored.CredentialVersion {
		t.Fatalf("cached reset credential version = %q, want %q", got, stored.CredentialVersion)
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
	if got := credentialVersionFromToken(t, token.Value); got != stored.CredentialVersion {
		t.Fatalf("reset login credential version = %q, want %q", got, stored.CredentialVersion)
	}
	if err := svc.ResetPassword(ctx, result.ResetToken, "another123"); !errors.Is(err, domain.ErrResetTokenInvalid) {
		t.Fatalf("reused reset token error = %v", err)
	}
}

func TestServiceEmailVerification(t *testing.T) {
	repo := newMemoryRepo()
	idgen := &fakeIDGen{next: 400}
	emails := &securityEmailSenderStub{ready: true}
	svc := NewService(repo, idgen, nil, nil, "test-secret", 0, 8, nil, emails, nil)
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
	if len(emails.emailVerifications) != 1 || emails.emailVerifications[0].recipient != alice.Email || emails.emailVerifications[0].token != result.VerificationToken || !emails.emailVerifications[0].expiresAt.Equal(result.ExpiresAt) {
		t.Fatalf("email verification delivery = %+v, want recipient=%q token=%q expiry=%s", emails.emailVerifications, alice.Email, result.VerificationToken, result.ExpiresAt)
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
	svc := NewService(repo, idgen, nil, nil, "test-secret", 0, 8, entitlements, &securityEmailSenderStub{ready: true}, nil)
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

func TestServiceRejectsSecurityEmailRequestsWhenDeliveryIsUnavailable(t *testing.T) {
	repo := newMemoryRepo()
	idgen := &fakeIDGen{next: 405}
	svc := NewService(repo, idgen, nil, nil, "test-secret", 0, 8, nil, &securityEmailSenderStub{}, nil)
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
	if _, err := svc.RequestPasswordReset(ctx, alice.Email); !errors.Is(err, domain.ErrSecurityEmailDeliveryUnavailable) {
		t.Fatalf("password reset error = %v, want security email unavailable", err)
	}
	if _, err := svc.RequestEmailVerification(ctx, alice.ID); !errors.Is(err, domain.ErrSecurityEmailDeliveryUnavailable) {
		t.Fatalf("email verification error = %v, want security email unavailable", err)
	}
	if len(repo.resetTokens) != 0 || len(repo.emailTokens) != 0 {
		t.Fatalf("security tokens persisted without delivery: reset=%d verification=%d", len(repo.resetTokens), len(repo.emailTokens))
	}
}

func TestServiceUpdateStatusHidesPremiumProfileWithoutActiveEntitlements(t *testing.T) {
	repo := newMemoryRepo()
	idgen := &fakeIDGen{next: 420}
	entitlements := &fakeProfileThemeEntitlements{}
	svc := NewService(repo, idgen, nil, nil, "test-secret", 0, 8, entitlements, nil, nil)
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

type deadlinePublisher struct {
	deadline    time.Time
	hasDeadline bool
}

func (p *deadlinePublisher) PublishDomainEvents(ctx context.Context, _ []domain.DomainEvent) error {
	p.deadline, p.hasDeadline = ctx.Deadline()
	return nil
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

type securityEmailDelivery struct {
	recipient string
	token     string
	expiresAt time.Time
}

type securityEmailSenderStub struct {
	ready              bool
	err                error
	passwordResets     []securityEmailDelivery
	emailVerifications []securityEmailDelivery
}

func (s *securityEmailSenderStub) Ready() bool {
	return s != nil && s.ready
}

func (s *securityEmailSenderStub) SendPasswordReset(_ context.Context, recipient, token string, expiresAt time.Time) error {
	if s.err != nil {
		return s.err
	}
	s.passwordResets = append(s.passwordResets, securityEmailDelivery{recipient: recipient, token: token, expiresAt: expiresAt})
	return nil
}

func (s *securityEmailSenderStub) SendEmailVerification(_ context.Context, recipient, token string, expiresAt time.Time) error {
	if s.err != nil {
		return s.err
	}
	s.emailVerifications = append(s.emailVerifications, securityEmailDelivery{recipient: recipient, token: token, expiresAt: expiresAt})
	return nil
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
	users                 map[int64]*domain.User
	oauthByKey            map[[2]string]int64
	oauthAccount          map[[2]string]domain.OAuthAccount
	follows               map[[2]int64]struct{}
	blocks                map[[2]int64]struct{}
	mutes                 map[[2]int64]struct{}
	resetTokens           map[string]domain.PasswordResetToken
	emailTokens           map[string]domain.EmailVerificationToken
	beforePasswordUpdate  func()
	beforeUpdateLastLogin func()
}

type inviteMemoryRepo struct {
	*memoryRepo
	mu      sync.Mutex
	invites map[int64]domain.InviteCode
}

func newInviteMemoryRepo() *inviteMemoryRepo {
	return &inviteMemoryRepo{memoryRepo: newMemoryRepo(), invites: map[int64]domain.InviteCode{}}
}

func (r *inviteMemoryRepo) CreateWithInvite(ctx context.Context, u *domain.User, code string, requireInvite bool) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	code = domain.NormalizeInviteCode(code)
	if code == "" && requireInvite {
		return domain.ErrInviteCodeRequired
	}
	if code == "" {
		return r.memoryRepo.Create(ctx, u)
	}
	var id int64
	var invite domain.InviteCode
	for candidateID, candidate := range r.invites {
		if domain.NormalizeInviteCode(candidate.Code) == code {
			id, invite = candidateID, candidate
			break
		}
	}
	if id == 0 {
		return domain.ErrInviteCodeInvalid
	}
	now := time.Now()
	switch invite.StatusAt(now) {
	case domain.InviteStatusUsed:
		return domain.ErrInviteCodeUsed
	case domain.InviteStatusExpired:
		return domain.ErrInviteCodeExpired
	case domain.InviteStatusRevoked:
		return domain.ErrInviteCodeRevoked
	}
	if err := r.memoryRepo.Create(ctx, u); err != nil {
		return err
	}
	userID := u.ID
	invite.UsedByUserID = &userID
	invite.UsedAt = &now
	r.invites[id] = invite
	return nil
}

func (r *inviteMemoryRepo) CreateInviteCodes(_ context.Context, codes []domain.InviteCode) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if len(codes) < 1 || len(codes) > 100 {
		return domain.ErrInviteCountInvalid
	}
	for _, code := range codes {
		for _, existing := range r.invites {
			if domain.NormalizeInviteCode(existing.Code) == domain.NormalizeInviteCode(code.Code) {
				return domain.ErrInviteCodeExists
			}
		}
	}
	for _, code := range codes {
		r.invites[code.ID] = code
	}
	return nil
}

func (r *inviteMemoryRepo) ListInviteCodes(_ context.Context, q domain.InviteCodeListQuery) ([]domain.InviteCode, int64, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	status := domain.NormalizeInviteStatus(q.Status)
	if !domain.ValidInviteStatus(status) {
		return nil, 0, domain.ErrInviteStatusInvalid
	}
	now := time.Now()
	items := make([]domain.InviteCode, 0, len(r.invites))
	for _, invite := range r.invites {
		if status == domain.InviteStatusAll || invite.StatusAt(now) == status {
			items = append(items, invite)
		}
	}
	sort.Slice(items, func(i, j int) bool {
		if items[i].CreatedAt.Equal(items[j].CreatedAt) {
			return items[i].ID > items[j].ID
		}
		return items[i].CreatedAt.After(items[j].CreatedAt)
	})
	total := int64(len(items))
	if q.Page <= 0 {
		q.Page = 1
	}
	if q.PageSize <= 0 {
		q.PageSize = 20
	}
	if q.PageSize > 100 {
		q.PageSize = 100
	}
	start := (q.Page - 1) * q.PageSize
	if start >= len(items) {
		return []domain.InviteCode{}, total, nil
	}
	end := start + q.PageSize
	if end > len(items) {
		end = len(items)
	}
	return append([]domain.InviteCode(nil), items[start:end]...), total, nil
}

func (r *inviteMemoryRepo) RevokeInviteCode(_ context.Context, id, actorID int64) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	invite, ok := r.invites[id]
	if !ok {
		return domain.ErrInviteCodeNotFound
	}
	if invite.UsedAt != nil || invite.UsedByUserID != nil {
		return domain.ErrInviteCodeUsed
	}
	if invite.RevokedAt != nil {
		return domain.ErrInviteCodeRevoked
	}
	now := time.Now()
	invite.RevokedAt = &now
	invite.RevokedByAdminID = &actorID
	r.invites[id] = invite
	return nil
}

func newMemoryRepo() *memoryRepo {
	return &memoryRepo{
		users:        map[int64]*domain.User{},
		oauthByKey:   map[[2]string]int64{},
		oauthAccount: map[[2]string]domain.OAuthAccount{},
		follows:      map[[2]int64]struct{}{},
		blocks:       map[[2]int64]struct{}{},
		mutes:        map[[2]int64]struct{}{},
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

func (r *memoryRepo) UpdatePasswordAndCredentialVersion(_ context.Context, u *domain.User, expectedPasswordHash string) error {
	if r.beforePasswordUpdate != nil {
		hook := r.beforePasswordUpdate
		r.beforePasswordUpdate = nil
		hook()
	}
	stored, ok := r.users[u.ID]
	if !ok {
		return domain.ErrNotFound
	}
	if stored.PasswordHash != expectedPasswordHash {
		return domain.ErrInvalidPassword
	}
	r.users[u.ID] = cloneUser(u)
	return nil
}

func (r *memoryRepo) UpdateStatus(_ context.Context, u *domain.User) error {
	return r.UpdateProfile(context.Background(), u)
}

func (r *memoryRepo) UpdateLastLogin(_ context.Context, u *domain.User) error {
	if r.beforeUpdateLastLogin != nil {
		hook := r.beforeUpdateLastLogin
		r.beforeUpdateLastLogin = nil
		hook()
	}
	stored, ok := r.users[u.ID]
	if !ok {
		return domain.ErrNotFound
	}
	stored = cloneUser(stored)
	stored.LastLoginAt = u.LastLoginAt
	stored.UpdatedAt = u.UpdatedAt
	r.users[u.ID] = stored
	return nil
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

func (r *memoryRepo) ResetPasswordWithToken(_ context.Context, tokenHash string, passwordHash string, credentialVersion string, now time.Time) (*domain.User, error) {
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
	cp.CredentialVersion = credentialVersion
	cp.UpdatedAt = now
	r.users[cp.ID] = cloneUser(cp)
	token.UsedAt = &now
	r.resetTokens[tokenHash] = token
	return cloneUser(cp), nil
}

func (r *memoryRepo) GetCredentialVersion(_ context.Context, userID int64) (string, error) {
	u, ok := r.users[userID]
	if !ok {
		return "", domain.ErrNotFound
	}
	return domain.NormalizeCredentialVersion(u.CredentialVersion), nil
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

func (r *memoryRepo) Block(_ context.Context, actorID, targetID int64) error {
	key := [2]int64{actorID, targetID}
	if _, ok := r.blocks[key]; ok {
		return domain.ErrAlreadyBlocking
	}
	r.blocks[key] = struct{}{}
	r.mutes[key] = struct{}{}
	delete(r.follows, [2]int64{actorID, targetID})
	delete(r.follows, [2]int64{targetID, actorID})
	return nil
}

func (r *memoryRepo) Unblock(_ context.Context, actorID, targetID int64) error {
	key := [2]int64{actorID, targetID}
	if _, ok := r.blocks[key]; !ok {
		return domain.ErrNotBlocking
	}
	delete(r.blocks, key)
	delete(r.mutes, key)
	return nil
}

func (r *memoryRepo) Mute(_ context.Context, actorID, targetID int64) error {
	key := [2]int64{actorID, targetID}
	if _, ok := r.mutes[key]; ok {
		return domain.ErrAlreadyMuted
	}
	r.mutes[key] = struct{}{}
	return nil
}

func (r *memoryRepo) Unmute(_ context.Context, actorID, targetID int64) error {
	key := [2]int64{actorID, targetID}
	if _, ok := r.mutes[key]; !ok {
		return domain.ErrNotMuted
	}
	delete(r.mutes, key)
	return nil
}

func (r *memoryRepo) GetSafetyRelation(_ context.Context, actorID, targetID int64) (domain.SafetyRelation, error) {
	_, blocked := r.blocks[[2]int64{actorID, targetID}]
	_, blockedBy := r.blocks[[2]int64{targetID, actorID}]
	_, muted := r.mutes[[2]int64{actorID, targetID}]
	return domain.SafetyRelation{Blocked: blocked, BlockedBy: blockedBy, Muted: muted}, nil
}

func (r *memoryRepo) ListBlockedUsers(_ context.Context, q domain.FollowListQuery) ([]*domain.User, int64, error) {
	items, total := r.listSafetyUsers(q.UserID, r.blocks)
	return items, total, nil
}

func (r *memoryRepo) ListMutedUsers(_ context.Context, q domain.FollowListQuery) ([]*domain.User, int64, error) {
	items, total := r.listSafetyUsers(q.UserID, r.mutes)
	return items, total, nil
}

func (r *memoryRepo) listSafetyUsers(actorID int64, relations map[[2]int64]struct{}) ([]*domain.User, int64) {
	ids := make([]int64, 0)
	for relation := range relations {
		if relation[0] == actorID {
			ids = append(ids, relation[1])
		}
	}
	sort.Slice(ids, func(i, j int) bool { return ids[i] < ids[j] })
	items := make([]*domain.User, 0, len(ids))
	for _, id := range ids {
		if user := r.users[id]; user != nil {
			items = append(items, cloneUser(user))
		}
	}
	return items, int64(len(items))
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

type credentialVersionCacheStub struct {
	versions    map[int64]string
	setErr      error
	deleteErr   error
	deleteCalls int
}

func newCredentialVersionCacheStub() *credentialVersionCacheStub {
	return &credentialVersionCacheStub{versions: make(map[int64]string)}
}

func (s *credentialVersionCacheStub) SetCurrent(_ context.Context, userID int64, version string) error {
	if s.setErr != nil {
		return s.setErr
	}
	s.versions[userID] = version
	return nil
}

func (s *credentialVersionCacheStub) Delete(_ context.Context, userID int64) error {
	s.deleteCalls++
	if s.deleteErr != nil {
		return s.deleteErr
	}
	delete(s.versions, userID)
	return nil
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
