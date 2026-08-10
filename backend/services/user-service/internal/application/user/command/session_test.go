package command

import (
	"context"
	"errors"
	"sort"
	"strings"
	"testing"
	"time"

	domain "user-service/internal/domain/user"

	"github.com/golang-jwt/jwt/v5"
)

// sessionMemoryRepo augments memoryRepo with the SessionRepository behaviour so
// login flows can be observed end to end without PostgreSQL.
type sessionMemoryRepo struct {
	*memoryRepo
	sessions   []domain.UserSession
	events     []domain.LoginEvent
	recordErr  error
	recordCall int
}

func newSessionMemoryRepo() *sessionMemoryRepo {
	return &sessionMemoryRepo{memoryRepo: newMemoryRepo()}
}

func (r *sessionMemoryRepo) RecordSession(_ context.Context, session domain.UserSession, event domain.LoginEvent) error {
	r.recordCall++
	if r.recordErr != nil {
		return r.recordErr
	}
	if err := session.Validate(); err != nil {
		return err
	}
	if err := event.Validate(); err != nil {
		return err
	}
	r.sessions = append(r.sessions, session)
	r.events = append(r.events, event)
	return nil
}

func (r *sessionMemoryRepo) CreateAPIToken(_ context.Context, session domain.UserSession) error {
	r.recordCall++
	if r.recordErr != nil {
		return r.recordErr
	}
	if session.LoginMethod != LoginMethodAPIToken {
		return domain.ErrLoginMethodInvalid
	}
	if err := session.Validate(); err != nil {
		return err
	}
	r.sessions = append(r.sessions, session)
	return nil
}

func (r *sessionMemoryRepo) RecordLoginEvent(_ context.Context, event domain.LoginEvent) error {
	if err := event.Validate(); err != nil {
		return err
	}
	r.events = append(r.events, event)
	return nil
}

func (r *sessionMemoryRepo) ListSessions(_ context.Context, userID int64, limit int) ([]domain.UserSession, error) {
	out := make([]domain.UserSession, 0, len(r.sessions))
	for _, session := range r.sessions {
		if session.UserID == userID && session.LoginMethod != LoginMethodAPIToken {
			out = append(out, session)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].CreatedAt.After(out[j].CreatedAt) })
	if limit > 0 && len(out) > limit {
		out = out[:limit]
	}
	return out, nil
}

func (r *sessionMemoryRepo) ListAPITokens(_ context.Context, userID int64, limit int, offset int) ([]domain.UserSession, int64, error) {
	out := make([]domain.UserSession, 0, len(r.sessions))
	for _, session := range r.sessions {
		if session.UserID == userID && session.LoginMethod == LoginMethodAPIToken {
			out = append(out, session)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].CreatedAt.After(out[j].CreatedAt) })
	total := int64(len(out))
	if offset > len(out) {
		offset = len(out)
	}
	out = out[offset:]
	if limit > 0 && len(out) > limit {
		out = out[:limit]
	}
	return out, total, nil
}

func (r *sessionMemoryRepo) RevokeAPIToken(_ context.Context, userID int64, tokenID string, now time.Time) (domain.UserSession, error) {
	for i := range r.sessions {
		if r.sessions[i].UserID == userID && r.sessions[i].SessionID == tokenID && r.sessions[i].LoginMethod == LoginMethodAPIToken {
			if r.sessions[i].RevokedAt == nil {
				revoked := now
				r.sessions[i].RevokedAt = &revoked
			}
			return r.sessions[i], nil
		}
	}
	return domain.UserSession{}, domain.ErrAPITokenNotFound
}

func (r *sessionMemoryRepo) GetSession(_ context.Context, userID int64, sessionID string) (domain.UserSession, error) {
	for _, session := range r.sessions {
		if session.UserID == userID && session.SessionID == sessionID {
			return session, nil
		}
	}
	return domain.UserSession{}, domain.ErrSessionNotFound
}

func (r *sessionMemoryRepo) RevokeSession(_ context.Context, userID int64, sessionID string, now time.Time) (domain.UserSession, error) {
	for i := range r.sessions {
		if r.sessions[i].UserID == userID && r.sessions[i].SessionID == sessionID {
			if r.sessions[i].RevokedAt == nil {
				revoked := now
				r.sessions[i].RevokedAt = &revoked
			}
			return r.sessions[i], nil
		}
	}
	return domain.UserSession{}, domain.ErrSessionNotFound
}

func (r *sessionMemoryRepo) ListLoginEvents(_ context.Context, userID int64, limit int) ([]domain.LoginEvent, error) {
	out := make([]domain.LoginEvent, 0, len(r.events))
	for _, event := range r.events {
		if event.UserID == userID {
			out = append(out, event)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].CreatedAt.After(out[j].CreatedAt) })
	if limit > 0 && len(out) > limit {
		out = out[:limit]
	}
	return out, nil
}

func newSessionService(repo domain.Repository) *Service {
	return NewService(repo, &fakeIDGen{next: 1000}, nil, nil, "test-secret", time.Hour, 8, nil, nil, nil)
}

func registerSessionUser(t *testing.T, svc *Service, ctx context.Context, username string) *domain.User {
	t.Helper()
	u, token, err := svc.Register(ctx, domain.RegisterCmd{
		Username: username,
		Email:    username + "@example.com",
		Password: "password123",
		Nickname: username,
	})
	if err != nil {
		t.Fatalf("register %s: %v", username, err)
	}
	if token.Value == "" {
		t.Fatalf("register %s returned no token", username)
	}
	return u
}

func TestLoginRecordsSessionWithClientInfo(t *testing.T) {
	repo := newSessionMemoryRepo()
	svc := newSessionService(repo)
	ctx := WithSessionClient(context.Background(), domain.SessionClientInfo{
		IPAddress: "203.0.113.7",
		UserAgent: "Mozilla/5.0 (Macintosh)",
	})
	u := registerSessionUser(t, svc, ctx, "alice")

	if _, _, err := svc.Login(ctx, "alice", "password123"); err != nil {
		t.Fatalf("login: %v", err)
	}

	sessions, err := svc.ListSessions(ctx, u.ID, 0)
	if err != nil {
		t.Fatalf("list sessions: %v", err)
	}
	if len(sessions) != 2 {
		t.Fatalf("sessions = %d, want 2 (register + login)", len(sessions))
	}
	methods := map[string]bool{}
	for _, session := range sessions {
		methods[session.LoginMethod] = true
		if session.IPAddress != "203.0.113.7" {
			t.Errorf("ip = %q, want 203.0.113.7", session.IPAddress)
		}
		if session.UserAgent != "Mozilla/5.0 (Macintosh)" {
			t.Errorf("user agent = %q", session.UserAgent)
		}
		if !session.ExpiresAt.After(session.CreatedAt) {
			t.Errorf("expiry %v must be after creation %v", session.ExpiresAt, session.CreatedAt)
		}
	}
	if !methods[LoginMethodRegister] || !methods[LoginMethodPassword] {
		t.Errorf("login methods = %v, want register and password", methods)
	}
}

func TestSessionIDMatchesTokenJTI(t *testing.T) {
	repo := newSessionMemoryRepo()
	svc := newSessionService(repo)
	ctx := context.Background()
	u := registerSessionUser(t, svc, ctx, "bob")

	_, token, err := svc.Login(ctx, "bob", "password123")
	if err != nil {
		t.Fatalf("login: %v", err)
	}
	sessions, err := svc.ListSessions(ctx, u.ID, 0)
	if err != nil {
		t.Fatalf("list sessions: %v", err)
	}

	// The gateway revokes a device by rejecting the token's jti, so the stored
	// session id must be exactly that claim.
	found := false
	for _, session := range sessions {
		if session.LoginMethod == LoginMethodPassword {
			found = true
			if !domain.ValidSessionID(session.SessionID) {
				t.Errorf("session id %q is not a valid session id", session.SessionID)
			}
			if jti := tokenJTI(t, token.Value); jti != session.SessionID {
				t.Errorf("session id %q != token jti %q", session.SessionID, jti)
			}
		}
	}
	if !found {
		t.Fatal("no password session recorded")
	}
}

func TestCreateAPITokenPersistsScopedRevocableToken(t *testing.T) {
	repo := newSessionMemoryRepo()
	svc := newSessionService(repo)
	ctx := WithSessionClient(context.Background(), domain.SessionClientInfo{IPAddress: "203.0.113.8", UserAgent: "integration-client/1.0"})
	u := registerSessionUser(t, svc, ctx, "api_alice")
	loginEventCount := len(repo.events)

	session, token, err := svc.CreateAPIToken(ctx, u.ID, "  Deploy Bot  ", []string{"WRITE", " read ", "read"}, 0)
	if err != nil {
		t.Fatalf("create api token: %v", err)
	}
	if token.Value == "" || session.SessionID == "" {
		t.Fatal("create api token returned no secret or id")
	}
	if session.APITokenName != "Deploy Bot" {
		t.Fatalf("name = %q", session.APITokenName)
	}
	if got := strings.Join(session.APITokenScopes, ","); got != "read,write" {
		t.Fatalf("scopes = %q", got)
	}
	if !session.APITokenCredentialValid || session.APITokenCredentialVersion == "" {
		t.Fatalf("credential metadata = %+v", session)
	}
	if len(repo.events) != loginEventCount {
		t.Fatalf("api token creation added login event: %d -> %d", loginEventCount, len(repo.events))
	}
	if remaining := token.ExpiresAt.Sub(session.CreatedAt); remaining < 89*24*time.Hour || remaining > 91*24*time.Hour {
		t.Fatalf("default expiry duration = %v", remaining)
	}

	parsed, err := jwt.Parse(token.Value, func(*jwt.Token) (any, error) { return []byte("test-secret"), nil })
	if err != nil || !parsed.Valid {
		t.Fatalf("parse token: valid=%v err=%v", parsed != nil && parsed.Valid, err)
	}
	claims := parsed.Claims.(jwt.MapClaims)
	if claims["token_type"] != LoginMethodAPIToken {
		t.Fatalf("token_type = %#v", claims["token_type"])
	}
	if claims[credentialVersionClaim] != session.APITokenCredentialVersion {
		t.Fatalf("credential version = %#v", claims[credentialVersionClaim])
	}
	if jti, _ := claims["jti"].(string); jti != session.SessionID {
		t.Fatalf("jti = %q, id = %q", jti, session.SessionID)
	}
	rawScopes, ok := claims["scopes"].([]any)
	if !ok || len(rawScopes) != 2 || rawScopes[0] != "read" || rawScopes[1] != "write" {
		t.Fatalf("token scopes = %#v", claims["scopes"])
	}

	sessions, err := svc.ListSessions(ctx, u.ID, 20)
	if err != nil || len(sessions) != 1 {
		t.Fatalf("browser sessions = %d, err = %v", len(sessions), err)
	}
	tokens, total, err := svc.ListAPITokens(ctx, u.ID, 0, 0)
	if err != nil || total != 1 || len(tokens) != 1 || tokens[0].SessionID != session.SessionID {
		t.Fatalf("api tokens = %+v total=%d err=%v", tokens, total, err)
	}
}

func TestCreateAPITokenRequiresStrictPersistence(t *testing.T) {
	repo := newSessionMemoryRepo()
	svc := newSessionService(repo)
	u := registerSessionUser(t, svc, context.Background(), "api_strict")
	repo.recordErr = errors.New("database unavailable")

	session, token, err := svc.CreateAPIToken(context.Background(), u.ID, "CI", []string{"read"}, 30)
	if err == nil {
		t.Fatal("create api token succeeded despite persistence failure")
	}
	if session.SessionID != "" || token.Value != "" {
		t.Fatalf("failed create leaked token: session=%+v token=%q", session, token.Value)
	}
}

func TestAPITokenValidationCredentialInvalidationAndRevoke(t *testing.T) {
	repo := newSessionMemoryRepo()
	svc := newSessionService(repo)
	ctx := context.Background()
	u := registerSessionUser(t, svc, ctx, "api_security")

	invalidCases := []struct {
		name   string
		scopes []string
		days   int
		want   error
	}{
		{name: " ", scopes: []string{"read"}, days: 1, want: domain.ErrAPITokenNameRequired},
		{name: strings.Repeat("界", 129), scopes: []string{"read"}, days: 1, want: domain.ErrAPITokenNameTooLong},
		{name: "bad scope", scopes: []string{"read:account"}, days: 1, want: domain.ErrAPITokenScopeInvalid},
		{name: "bad expiry", scopes: []string{"read"}, days: 366, want: domain.ErrAPITokenExpiryInvalid},
	}
	for _, test := range invalidCases {
		if _, _, err := svc.CreateAPIToken(ctx, u.ID, test.name, test.scopes, test.days); !errors.Is(err, test.want) {
			t.Errorf("create %q error = %v, want %v", test.name, err, test.want)
		}
	}
	if _, _, err := svc.ListAPITokens(ctx, u.ID, 101, 0); !errors.Is(err, domain.ErrAPITokenListInvalid) {
		t.Fatalf("oversized list error = %v", err)
	}
	if _, _, err := svc.ListAPITokens(ctx, u.ID, 30, -1); !errors.Is(err, domain.ErrAPITokenListInvalid) {
		t.Fatalf("negative offset error = %v", err)
	}

	session, _, err := svc.CreateAPIToken(ctx, u.ID, "Read client", []string{"read"}, 365)
	if err != nil {
		t.Fatalf("create valid token: %v", err)
	}
	stored := cloneUser(repo.users[u.ID])
	stored.CredentialVersion = "rotated-version"
	repo.users[u.ID] = stored
	tokens, _, err := svc.ListAPITokens(ctx, u.ID, 30, 0)
	if err != nil || len(tokens) != 1 {
		t.Fatalf("list invalidated token: %+v err=%v", tokens, err)
	}
	if tokens[0].APITokenCredentialValid {
		t.Fatal("credential-rotated token still marked valid")
	}

	browserSessions, err := svc.ListSessions(ctx, u.ID, 20)
	if err != nil || len(browserSessions) != 1 {
		t.Fatalf("browser sessions: %+v err=%v", browserSessions, err)
	}
	if _, err := svc.RevokeAPIToken(ctx, u.ID, browserSessions[0].SessionID); !errors.Is(err, domain.ErrAPITokenNotFound) {
		t.Fatalf("revoke browser session as token error = %v", err)
	}
	revoked, err := svc.RevokeAPIToken(ctx, u.ID, session.SessionID)
	if err != nil || revoked.RevokedAt == nil {
		t.Fatalf("revoke api token: %+v err=%v", revoked, err)
	}
	first := *revoked.RevokedAt
	again, err := svc.RevokeAPIToken(ctx, u.ID, session.SessionID)
	if err != nil || again.RevokedAt == nil || !again.RevokedAt.Equal(first) {
		t.Fatalf("idempotent revoke: %+v err=%v", again, err)
	}
}

// tokenJTI extracts the jti claim, which doubles as the session id.
func tokenJTI(t *testing.T, raw string) string {
	t.Helper()
	parsed, err := jwt.Parse(raw, func(*jwt.Token) (any, error) { return []byte("test-secret"), nil })
	if err != nil || !parsed.Valid {
		t.Fatalf("parse token: valid=%v err=%v", parsed != nil && parsed.Valid, err)
	}
	claims, ok := parsed.Claims.(jwt.MapClaims)
	if !ok {
		t.Fatalf("claims type = %T", parsed.Claims)
	}
	jti, ok := claims["jti"].(string)
	if !ok {
		t.Fatalf("jti = %#v, want string", claims["jti"])
	}
	return jti
}

func TestFailedPasswordRecordsLoginEvent(t *testing.T) {
	repo := newSessionMemoryRepo()
	svc := newSessionService(repo)
	ctx := context.Background()
	u := registerSessionUser(t, svc, ctx, "carol")

	if _, _, err := svc.Login(ctx, "carol", "wrong-password"); !errors.Is(err, domain.ErrInvalidPassword) {
		t.Fatalf("login error = %v, want ErrInvalidPassword", err)
	}

	events, err := svc.ListLoginEvents(ctx, u.ID, 0)
	if err != nil {
		t.Fatalf("list login events: %v", err)
	}
	var failures int
	for _, event := range events {
		if event.Success {
			continue
		}
		failures++
		if event.FailureReason != LoginFailureInvalidPassword {
			t.Errorf("failure reason = %q, want %q", event.FailureReason, LoginFailureInvalidPassword)
		}
		if event.SessionID != "" {
			t.Errorf("failed attempt must not reference a session, got %q", event.SessionID)
		}
	}
	if failures != 1 {
		t.Fatalf("failure events = %d, want 1", failures)
	}
}

func TestRevokeSessionIsScopedAndIdempotent(t *testing.T) {
	repo := newSessionMemoryRepo()
	svc := newSessionService(repo)
	ctx := context.Background()
	alice := registerSessionUser(t, svc, ctx, "dave")
	mallory := registerSessionUser(t, svc, ctx, "erin")

	sessions, err := svc.ListSessions(ctx, alice.ID, 0)
	if err != nil || len(sessions) == 0 {
		t.Fatalf("list sessions: %v (%d)", err, len(sessions))
	}
	target := sessions[0].SessionID

	// Another user must not be able to revoke it.
	if _, err := svc.RevokeSession(ctx, mallory.ID, target); !errors.Is(err, domain.ErrSessionNotFound) {
		t.Fatalf("cross-user revoke error = %v, want ErrSessionNotFound", err)
	}

	revoked, err := svc.RevokeSession(ctx, alice.ID, target)
	if err != nil {
		t.Fatalf("revoke: %v", err)
	}
	if revoked.RevokedAt == nil {
		t.Fatal("revoked_at must be set")
	}
	first := *revoked.RevokedAt

	again, err := svc.RevokeSession(ctx, alice.ID, target)
	if err != nil {
		t.Fatalf("second revoke: %v", err)
	}
	if again.RevokedAt == nil || !again.RevokedAt.Equal(first) {
		t.Errorf("repeat revoke changed timestamp: %v -> %v", first, again.RevokedAt)
	}
}

func TestSessionOperationsRejectInvalidInput(t *testing.T) {
	svc := newSessionService(newSessionMemoryRepo())
	ctx := context.Background()

	if _, err := svc.ListSessions(ctx, 0, 10); !errors.Is(err, domain.ErrInvalidID) {
		t.Errorf("list with id 0 = %v, want ErrInvalidID", err)
	}
	if _, err := svc.RevokeSession(ctx, 5, "short"); !errors.Is(err, domain.ErrSessionIDInvalid) {
		t.Errorf("revoke with bad session id = %v, want ErrSessionIDInvalid", err)
	}
	if _, err := svc.GetSession(ctx, 5, "not a valid id!"); !errors.Is(err, domain.ErrSessionIDInvalid) {
		t.Errorf("get with bad session id = %v, want ErrSessionIDInvalid", err)
	}
}

func TestSessionRepositoryUnavailableDoesNotBreakLogin(t *testing.T) {
	// memoryRepo alone does not implement SessionRepository.
	svc := newSessionService(newMemoryRepo())
	ctx := context.Background()

	if _, _, err := svc.Register(ctx, domain.RegisterCmd{
		Username: "frank",
		Email:    "frank@example.com",
		Password: "password123",
		Nickname: "frank",
	}); err != nil {
		t.Fatalf("register must succeed without session tracking: %v", err)
	}
	if _, _, err := svc.Login(ctx, "frank", "password123"); err != nil {
		t.Fatalf("login must succeed without session tracking: %v", err)
	}
	if _, err := svc.ListSessions(ctx, 1, 10); !errors.Is(err, domain.ErrSessionRepositoryUnavailable) {
		t.Errorf("list sessions = %v, want ErrSessionRepositoryUnavailable", err)
	}
}

func TestRecordSessionFailureDoesNotFailLogin(t *testing.T) {
	repo := newSessionMemoryRepo()
	repo.recordErr = errors.New("database unreachable")
	svc := newSessionService(repo)
	ctx := context.Background()

	if _, _, err := svc.Register(ctx, domain.RegisterCmd{
		Username: "grace",
		Email:    "grace@example.com",
		Password: "password123",
		Nickname: "grace",
	}); err != nil {
		t.Fatalf("register must tolerate session write failure: %v", err)
	}
	if repo.recordCall == 0 {
		t.Fatal("expected a session write attempt")
	}
}
