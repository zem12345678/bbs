package command

import (
	"context"
	"errors"
	"time"

	domain "user-service/internal/domain/user"
	"user-service/pkg/logger"
)

// Login methods recorded on each issued session.
const (
	LoginMethodRegister    = "register"
	LoginMethodPassword    = "password"
	LoginMethodOAuth       = "oauth"
	LoginMethodWebmaster   = "webmaster"
	LoginMethodMFA         = "mfa"
	LoginMethodPasskeyMFA  = "passkey_mfa"
	LoginMethodPasskeyless = "passkey"
)

// Login failure reasons recorded against existing accounts.
const LoginFailureInvalidPassword = "invalid_password"

type sessionClientContextKey struct{}

// WithSessionClient carries request-scoped client metadata (IP, user agent) so
// session records can attribute a login to a device without threading the
// values through every command signature.
func WithSessionClient(ctx context.Context, info domain.SessionClientInfo) context.Context {
	return context.WithValue(ctx, sessionClientContextKey{}, domain.NormalizeSessionClientInfo(info))
}

func sessionClientFromContext(ctx context.Context) domain.SessionClientInfo {
	if ctx == nil {
		return domain.SessionClientInfo{}
	}
	info, ok := ctx.Value(sessionClientContextKey{}).(domain.SessionClientInfo)
	if !ok {
		return domain.SessionClientInfo{}
	}
	return info
}

func (s *Service) sessionRepository() (domain.SessionRepository, error) {
	repo, ok := s.repo.(domain.SessionRepository)
	if !ok {
		return nil, domain.ErrSessionRepositoryUnavailable
	}
	return repo, nil
}

// recordSession persists the session backing a freshly-issued token. The token
// is already signed at this point, so a storage failure must not fail the
// login; it is logged and the session simply stays untracked.
func (s *Service) recordSession(ctx context.Context, userID int64, sessionID string, loginMethod string, issuedAt, expiresAt time.Time) {
	repo, err := s.sessionRepository()
	if err != nil {
		if s.log != nil {
			s.log.Warn("session repository unavailable", logger.Int64("user_id", userID))
		}
		return
	}
	client := sessionClientFromContext(ctx)
	session := domain.UserSession{
		SessionID:   sessionID,
		UserID:      userID,
		IPAddress:   client.IPAddress,
		UserAgent:   client.UserAgent,
		LoginMethod: domain.NormalizeLoginMethod(loginMethod),
		CreatedAt:   issuedAt,
		ExpiresAt:   expiresAt,
	}
	event := domain.LoginEvent{
		ID:        s.idgen.Generate(),
		UserID:    userID,
		SessionID: sessionID,
		IPAddress: client.IPAddress,
		UserAgent: client.UserAgent,
		Success:   true,
		CreatedAt: issuedAt,
	}
	if err := repo.RecordSession(ctx, session, event); err != nil && s.log != nil {
		s.log.Warn("record user session failed", logger.Int64("user_id", userID), logger.Error(err))
	}
}

// recordLoginFailure appends a failed attempt for an existing account. Attempts
// against unknown accounts are not recorded because they have no owner to show
// them to.
func (s *Service) recordLoginFailure(ctx context.Context, userID int64, reason string) {
	if userID <= 0 {
		return
	}
	repo, err := s.sessionRepository()
	if err != nil {
		return
	}
	client := sessionClientFromContext(ctx)
	event := domain.LoginEvent{
		ID:            s.idgen.Generate(),
		UserID:        userID,
		IPAddress:     client.IPAddress,
		UserAgent:     client.UserAgent,
		Success:       false,
		FailureReason: domain.NormalizeLoginFailureReason(reason),
		CreatedAt:     time.Now(),
	}
	if err := repo.RecordLoginEvent(ctx, event); err != nil && s.log != nil {
		s.log.Warn("record login failure failed", logger.Int64("user_id", userID), logger.Error(err))
	}
}

// ListSessions returns the caller's most recent sessions, newest first.
func (s *Service) ListSessions(ctx context.Context, userID int64, limit int) ([]domain.UserSession, error) {
	if userID <= 0 {
		return nil, domain.ErrInvalidID
	}
	repo, err := s.sessionRepository()
	if err != nil {
		return nil, err
	}
	return repo.ListSessions(ctx, userID, domain.SessionListLimit(limit))
}

// GetSession returns a single session owned by the caller.
func (s *Service) GetSession(ctx context.Context, userID int64, sessionID string) (domain.UserSession, error) {
	if userID <= 0 {
		return domain.UserSession{}, domain.ErrInvalidID
	}
	if !domain.ValidSessionID(sessionID) {
		return domain.UserSession{}, domain.ErrSessionIDInvalid
	}
	repo, err := s.sessionRepository()
	if err != nil {
		return domain.UserSession{}, err
	}
	return repo.GetSession(ctx, userID, sessionID)
}

// RevokeSession marks one of the caller's sessions revoked so the gateway stops
// honouring tokens that carry it.
func (s *Service) RevokeSession(ctx context.Context, userID int64, sessionID string) (domain.UserSession, error) {
	if userID <= 0 {
		return domain.UserSession{}, domain.ErrInvalidID
	}
	if !domain.ValidSessionID(sessionID) {
		return domain.UserSession{}, domain.ErrSessionIDInvalid
	}
	repo, err := s.sessionRepository()
	if err != nil {
		return domain.UserSession{}, err
	}
	session, err := repo.RevokeSession(ctx, userID, sessionID, time.Now())
	if err != nil {
		if errors.Is(err, domain.ErrSessionNotFound) {
			return domain.UserSession{}, domain.ErrSessionNotFound
		}
		return domain.UserSession{}, err
	}
	return session, nil
}

// ListLoginEvents returns the caller's login history, newest first.
func (s *Service) ListLoginEvents(ctx context.Context, userID int64, limit int) ([]domain.LoginEvent, error) {
	if userID <= 0 {
		return nil, domain.ErrInvalidID
	}
	repo, err := s.sessionRepository()
	if err != nil {
		return nil, err
	}
	return repo.ListLoginEvents(ctx, userID, domain.SessionListLimit(limit))
}
