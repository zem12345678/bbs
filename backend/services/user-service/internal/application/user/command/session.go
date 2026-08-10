package command

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	domain "user-service/internal/domain/user"
	"user-service/pkg/logger"

	"github.com/golang-jwt/jwt/v5"
)

// Login methods recorded on each issued session.
const (
	LoginMethodRegister       = "register"
	LoginMethodPassword       = "password"
	LoginMethodOAuth          = "oauth"
	LoginMethodWebmaster      = "webmaster"
	LoginMethodMFA            = "mfa"
	LoginMethodPasskeyMFA     = "passkey_mfa"
	LoginMethodPasskeyless    = "passkey"
	LoginMethodAPIToken       = "api_token"
	DefaultAPITokenExpiryDays = 90
	MaxAPITokenExpiryDays     = 365
	DefaultAPITokenListLimit  = 30
	MaxAPITokenListLimit      = 100
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

// CreateAPIToken issues a scoped JWT and persists its revocable session before
// returning the secret to the caller.
func (s *Service) CreateAPIToken(ctx context.Context, userID int64, name string, scopes []string, expiresInDays int) (domain.UserSession, AuthToken, error) {
	if userID <= 0 {
		return domain.UserSession{}, AuthToken{}, domain.ErrInvalidID
	}
	name, err := domain.NormalizeAPITokenName(name)
	if err != nil {
		return domain.UserSession{}, AuthToken{}, err
	}
	scopes, err = domain.NormalizeAPITokenScopes(scopes)
	if err != nil {
		return domain.UserSession{}, AuthToken{}, err
	}
	if expiresInDays == 0 {
		expiresInDays = DefaultAPITokenExpiryDays
	}
	if expiresInDays < 1 || expiresInDays > MaxAPITokenExpiryDays {
		return domain.UserSession{}, AuthToken{}, domain.ErrAPITokenExpiryInvalid
	}
	if len(s.jwtSecret) == 0 {
		return domain.UserSession{}, AuthToken{}, fmt.Errorf("jwt secret required")
	}
	u, err := s.repo.FindByID(ctx, userID)
	if err != nil {
		return domain.UserSession{}, AuthToken{}, err
	}
	if err := u.EnsureActive(); err != nil {
		return domain.UserSession{}, AuthToken{}, err
	}
	jti, err := randomToken()
	if err != nil {
		return domain.UserSession{}, AuthToken{}, fmt.Errorf("generate jwt id: %w", err)
	}
	credentialVersion := strings.TrimSpace(u.CredentialVersion)
	if credentialVersion == "" {
		credentialVersion = credentialVersionInitial
	}
	issuedAt := time.Now()
	expiresAt := issuedAt.Add(time.Duration(expiresInDays) * 24 * time.Hour)
	claims := jwt.MapClaims{
		"sub":                  fmt.Sprintf("%d", u.ID),
		"user_id":              u.ID,
		"username":             u.Username,
		"jti":                  jti,
		"token_type":           LoginMethodAPIToken,
		"scopes":               scopes,
		credentialVersionClaim: credentialVersion,
		"exp":                  expiresAt.Unix(),
		"iat":                  issuedAt.Unix(),
	}
	value, err := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString(s.jwtSecret)
	if err != nil {
		return domain.UserSession{}, AuthToken{}, err
	}
	client := sessionClientFromContext(ctx)
	session := domain.UserSession{
		SessionID: jti, UserID: u.ID, IPAddress: client.IPAddress, UserAgent: client.UserAgent,
		LoginMethod: LoginMethodAPIToken, APITokenName: name, APITokenScopes: scopes,
		APITokenCredentialVersion: credentialVersion, APITokenCredentialValid: true,
		CreatedAt: issuedAt, ExpiresAt: expiresAt,
	}
	repo, err := s.sessionRepository()
	if err != nil {
		return domain.UserSession{}, AuthToken{}, err
	}
	if err := repo.CreateAPIToken(ctx, session); err != nil {
		return domain.UserSession{}, AuthToken{}, err
	}
	return session, AuthToken{Value: value, ExpiresAt: expiresAt}, nil
}

func (s *Service) ListAPITokens(ctx context.Context, userID int64, limit int, offset int) ([]domain.UserSession, int64, error) {
	if userID <= 0 {
		return nil, 0, domain.ErrInvalidID
	}
	if limit == 0 {
		limit = DefaultAPITokenListLimit
	}
	if limit < 1 || limit > MaxAPITokenListLimit || offset < 0 {
		return nil, 0, domain.ErrAPITokenListInvalid
	}
	repo, err := s.sessionRepository()
	if err != nil {
		return nil, 0, err
	}
	u, err := s.repo.FindByID(ctx, userID)
	if err != nil {
		return nil, 0, err
	}
	tokens, total, err := repo.ListAPITokens(ctx, userID, limit, offset)
	if err != nil {
		return nil, 0, err
	}
	currentCredentialVersion := domain.NormalizeCredentialVersion(u.CredentialVersion)
	for index := range tokens {
		tokens[index].APITokenCredentialValid = domain.NormalizeCredentialVersion(tokens[index].APITokenCredentialVersion) == currentCredentialVersion
	}
	return tokens, total, nil
}

func (s *Service) RevokeAPIToken(ctx context.Context, userID int64, tokenID string) (domain.UserSession, error) {
	if userID <= 0 {
		return domain.UserSession{}, domain.ErrInvalidID
	}
	if !domain.ValidSessionID(tokenID) {
		return domain.UserSession{}, domain.ErrSessionIDInvalid
	}
	repo, err := s.sessionRepository()
	if err != nil {
		return domain.UserSession{}, err
	}
	return repo.RevokeAPIToken(ctx, userID, tokenID, time.Now())
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
