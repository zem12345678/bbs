package command

import (
	"context"
	"errors"
	"sort"
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
		if session.UserID == userID {
			out = append(out, session)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].CreatedAt.After(out[j].CreatedAt) })
	if limit > 0 && len(out) > limit {
		out = out[:limit]
	}
	return out, nil
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
