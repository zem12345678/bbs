package command

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"hash/fnv"
	"net/mail"
	"strings"
	"time"

	domain "user-service/internal/domain/user"
	"user-service/internal/infrastructure/messaging"
	"user-service/pkg/logger"

	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/bcrypt"
)

const (
	passwordResetTokenTTL     = 30 * time.Minute
	emailVerificationTokenTTL = 24 * time.Hour
	eventPublishTimeout       = 2 * time.Second
	credentialVersionClaim    = "cv"
	credentialVersionInitial  = domain.InitialCredentialVersion
)

type IDGenerator interface {
	Generate() int64
}

type ProfileThemeEntitlementReader interface {
	HasActiveProfileTheme(ctx context.Context, userID int64, theme string) (bool, error)
	HasActiveMembership(ctx context.Context, userID int64) (bool, error)
}

type SecurityEmailSender interface {
	Ready() bool
	SendPasswordReset(ctx context.Context, recipient, token string, expiresAt time.Time) error
	SendEmailVerification(ctx context.Context, recipient, token string, expiresAt time.Time) error
}

// CredentialVersionCache mirrors PostgreSQL-authoritative credential versions
// to the Redis key consumed by api-gateway.
type CredentialVersionCache interface {
	SetCurrent(ctx context.Context, userID int64, version string) error
	Delete(ctx context.Context, userID int64) error
}

type AuthToken struct {
	Value              string
	ExpiresAt          time.Time
	MFARequired        bool
	MFAChallenge       string
	MFAChallengeExpiry time.Time
}

type PasswordResetResult struct {
	Accepted   bool
	ResetToken string
	ExpiresAt  time.Time
}

type EmailVerificationResult struct {
	Accepted          bool
	VerificationToken string
	ExpiresAt         time.Time
	AlreadyVerified   bool
}

type Service struct {
	repo               domain.Repository
	idgen              IDGenerator
	publisher          messaging.EventPublisher
	log                logger.Logger
	themeEntitlements  ProfileThemeEntitlementReader
	securityEmails     SecurityEmailSender
	credentialVersions CredentialVersionCache
	mfa                MFAManager
	passkeys           PasskeyManager
	jwtSecret          []byte
	jwtTTL             time.Duration
	passwordMinLength  int
}

func NewServiceWithPasskeys(repo domain.Repository, idgen IDGenerator, publisher messaging.EventPublisher, log logger.Logger, jwtSecret string, jwtTTL time.Duration, passwordMinLength int, themeEntitlements ProfileThemeEntitlementReader, securityEmails SecurityEmailSender, credentialVersions CredentialVersionCache, mfa MFAManager, passkeys PasskeyManager) *Service {
	service := NewService(repo, idgen, publisher, log, jwtSecret, jwtTTL, passwordMinLength, themeEntitlements, securityEmails, credentialVersions, mfa)
	service.passkeys = passkeys
	return service
}

func NewService(repo domain.Repository, idgen IDGenerator, publisher messaging.EventPublisher, log logger.Logger, jwtSecret string, jwtTTL time.Duration, passwordMinLength int, themeEntitlements ProfileThemeEntitlementReader, securityEmails SecurityEmailSender, credentialVersions CredentialVersionCache, mfa ...MFAManager) *Service {
	if jwtTTL <= 0 {
		jwtTTL = 7 * 24 * time.Hour
	}
	if passwordMinLength <= 0 {
		passwordMinLength = 8
	}
	var mfaManager MFAManager
	if len(mfa) > 0 {
		mfaManager = mfa[0]
	}
	return &Service{
		repo:               repo,
		idgen:              idgen,
		publisher:          publisher,
		log:                log,
		themeEntitlements:  themeEntitlements,
		securityEmails:     securityEmails,
		credentialVersions: credentialVersions,
		mfa:                mfaManager,
		jwtSecret:          []byte(jwtSecret),
		jwtTTL:             jwtTTL,
		passwordMinLength:  passwordMinLength,
	}
}

func (s *Service) Register(ctx context.Context, cmd domain.RegisterCmd) (*domain.User, AuthToken, error) {
	if err := s.validatePassword(cmd.Password); err != nil {
		return nil, AuthToken{}, err
	}
	passwordHash, err := hashPassword(cmd.Password)
	if err != nil {
		return nil, AuthToken{}, err
	}
	u, err := domain.New(s.idgen.Generate(), cmd, passwordHash)
	if err != nil {
		return nil, AuthToken{}, err
	}
	code := domain.NormalizeInviteCode(cmd.InviteCode)
	if cmd.RequireInvite && code == "" {
		return nil, AuthToken{}, domain.ErrInviteCodeRequired
	}
	inviteRepo, supportsInvites := s.repo.(domain.InviteRepository)
	if (cmd.RequireInvite || code != "") && !supportsInvites {
		return nil, AuthToken{}, domain.ErrInviteRepositoryUnavailable
	}
	// Sign before persistence so a token-generation failure cannot leave a
	// newly-created user (or consumed invite) behind.
	token, err := s.issueToken(ctx, u, LoginMethodRegister)
	if err != nil {
		return nil, AuthToken{}, err
	}
	if supportsInvites && (cmd.RequireInvite || code != "") {
		if err := inviteRepo.CreateWithInvite(ctx, u, code, cmd.RequireInvite); err != nil {
			return nil, AuthToken{}, err
		}
	} else if err := s.repo.Create(ctx, u); err != nil {
		return nil, AuthToken{}, err
	}
	s.publishEvents(ctx, u.Events()...)
	return u, token, nil
}

func (s *Service) Login(ctx context.Context, account string, password string) (*domain.User, AuthToken, error) {
	u, err := s.repo.FindByAccount(ctx, account)
	if err != nil {
		return nil, AuthToken{}, err
	}
	if bcrypt.CompareHashAndPassword([]byte(u.PasswordHash), []byte(password)) != nil {
		s.recordLoginFailure(ctx, u.ID, LoginFailureInvalidPassword)
		return nil, AuthToken{}, domain.ErrInvalidPassword
	}
	// Reveal lifecycle state only after credentials are proven so callers
	// cannot enumerate suspended or deleting accounts by username/email.
	if err := u.EnsureActive(); err != nil {
		return nil, AuthToken{}, err
	}
	challenge, required, err := s.beginMFALoginIfEnabled(ctx, u)
	if err != nil {
		return nil, AuthToken{}, err
	}
	if required {
		return s.profileForAuthResponse(ctx, u), challenge, nil
	}
	u.TouchLogin(time.Now())
	if err := s.repo.UpdateLastLogin(ctx, u); err != nil {
		return nil, AuthToken{}, err
	}
	token, err := s.issueToken(ctx, u, LoginMethodPassword)
	if err != nil {
		return nil, AuthToken{}, err
	}
	return s.profileForAuthResponse(ctx, u), token, nil
}

func (s *Service) OAuthLogin(ctx context.Context, cmd domain.OAuthLoginCmd) (*domain.User, AuthToken, error) {
	provider := domain.NormalizeProvider(cmd.Provider)
	providerUserID := strings.TrimSpace(cmd.ProviderUserID)
	if !domain.ValidOAuthProvider(provider) || providerUserID == "" {
		return nil, AuthToken{}, domain.ErrInvalidOAuth
	}
	now := time.Now()
	account := domain.OAuthAccount{
		Provider:       provider,
		ProviderUserID: providerUserID,
		Username:       strings.TrimSpace(cmd.Username),
		Email:          domain.NormalizeEmail(cmd.Email),
		Nickname:       strings.TrimSpace(cmd.Nickname),
		AvatarURL:      strings.TrimSpace(cmd.AvatarURL),
		UpdatedAt:      now,
		LastLoginAt:    &now,
	}

	u, err := s.repo.FindByOAuth(ctx, provider, providerUserID)
	if err == nil {
		if err := u.EnsureActive(); err != nil {
			return nil, AuthToken{}, err
		}
		account.UserID = u.ID
		challenge, required, err := s.beginMFALoginIfEnabled(ctx, u)
		if err != nil {
			return nil, AuthToken{}, err
		}
		if required {
			if err := s.repo.UpdateOAuthLogin(ctx, u, account); err != nil {
				return nil, AuthToken{}, err
			}
			return s.profileForAuthResponse(ctx, u), challenge, nil
		}
		u.TouchLogin(now)
		if err := s.repo.UpdateOAuthLogin(ctx, u, account); err != nil {
			return nil, AuthToken{}, err
		}
		token, err := s.issueToken(ctx, u, LoginMethodOAuth)
		if err != nil {
			return nil, AuthToken{}, err
		}
		return s.profileForAuthResponse(ctx, u), token, nil
	}
	if !errors.Is(err, domain.ErrNotFound) {
		return nil, AuthToken{}, err
	}
	if cmd.ExistingOnly {
		return nil, AuthToken{}, domain.ErrOAuthSignupDisabled
	}

	username, err := s.availableOAuthUsername(ctx, provider, cmd.Username, providerUserID)
	if err != nil {
		return nil, AuthToken{}, err
	}
	email := s.availableOAuthEmail(ctx, provider, providerUserID, cmd.Email)
	passwordHash, err := randomPasswordHash()
	if err != nil {
		return nil, AuthToken{}, err
	}
	nickname := strings.TrimSpace(cmd.Nickname)
	if nickname == "" {
		nickname = username
	}
	u, err = domain.New(s.idgen.Generate(), domain.RegisterCmd{
		Username: username,
		Email:    email,
		Password: "oauth",
		Nickname: nickname,
	}, passwordHash)
	if err != nil {
		return nil, AuthToken{}, err
	}
	u.AvatarURL = strings.TrimSpace(cmd.AvatarURL)
	u.TouchLogin(now)
	account.UserID = u.ID
	account.CreatedAt = now
	account.UpdatedAt = now
	if err := s.repo.CreateWithOAuth(ctx, u, account); err != nil {
		return nil, AuthToken{}, err
	}
	token, err := s.issueToken(ctx, u, LoginMethodOAuth)
	if err != nil {
		return nil, AuthToken{}, err
	}
	s.publishEvents(ctx, u.Events()...)
	return u, token, nil
}

// CreateInviteCodes issues a bounded batch of cryptographically-random codes.
func (s *Service) CreateInviteCodes(ctx context.Context, actorID, count int64, expiresAt *time.Time) ([]domain.InviteCode, error) {
	if actorID <= 0 {
		return nil, domain.ErrInvalidID
	}
	if count < 1 || count > 100 {
		return nil, domain.ErrInviteCountInvalid
	}
	if expiresAt != nil && !expiresAt.After(time.Now()) {
		return nil, domain.ErrInviteExpiryInvalid
	}
	repo, ok := s.repo.(domain.InviteRepository)
	if !ok {
		return nil, domain.ErrInviteRepositoryUnavailable
	}
	codes := make([]domain.InviteCode, 0, count)
	seen := make(map[string]struct{}, count)
	now := time.Now()
	for i := int64(0); i < count; i++ {
		code, err := randomInviteCode()
		if err != nil {
			return nil, err
		}
		if _, exists := seen[code]; exists {
			i--
			continue
		}
		seen[code] = struct{}{}
		codes = append(codes, domain.InviteCode{
			ID:               s.idgen.Generate(),
			Code:             code,
			CreatedByAdminID: actorID,
			ExpiresAt:        expiresAt,
			CreatedAt:        now,
		})
	}
	if err := repo.CreateInviteCodes(ctx, codes); err != nil {
		return nil, err
	}
	return codes, nil
}

func (s *Service) ListInviteCodes(ctx context.Context, q domain.InviteCodeListQuery) ([]domain.InviteCode, int64, error) {
	if !domain.ValidInviteStatus(q.Status) {
		return nil, 0, domain.ErrInviteStatusInvalid
	}
	repo, ok := s.repo.(domain.InviteRepository)
	if !ok {
		return nil, 0, domain.ErrInviteRepositoryUnavailable
	}
	return repo.ListInviteCodes(ctx, q)
}

func (s *Service) RevokeInviteCode(ctx context.Context, actorID, id int64) error {
	if actorID <= 0 || id <= 0 {
		return domain.ErrInvalidID
	}
	repo, ok := s.repo.(domain.InviteRepository)
	if !ok {
		return domain.ErrInviteRepositoryUnavailable
	}
	return repo.RevokeInviteCode(ctx, id, actorID)
}

func (s *Service) profileForAuthResponse(ctx context.Context, u *domain.User) *domain.User {
	if u == nil {
		return nil
	}
	cp := *u
	if strings.TrimSpace(cp.BackgroundURL) != "" {
		active, err := s.hasActiveMembershipEntitlement(ctx, cp.ID)
		if err != nil || !active {
			cp.BackgroundURL = ""
		}
	}
	if domain.NormalizeProfileTheme(cp.ProfileTheme) == domain.ProfileThemePro {
		active, err := s.hasActiveProfileThemeEntitlement(ctx, cp.ID)
		if err != nil || !active {
			cp.ProfileTheme = domain.ProfileThemeDefault
		}
	}
	return &cp
}

func (s *Service) WebmasterLogin(ctx context.Context, cmd domain.WebmasterLoginCmd) (*domain.User, AuthToken, error) {
	if err := s.validatePassword(cmd.Password); err != nil {
		return nil, AuthToken{}, err
	}
	username := domain.NormalizeUsername(cmd.Username)
	if !domain.ValidUsername(username) {
		return nil, AuthToken{}, domain.ErrUsernameInvalid
	}
	email := domain.NormalizeEmail(cmd.Email)
	if !validEmail(email) {
		email = username + "@webmaster.local"
	}
	nickname := strings.TrimSpace(cmd.Nickname)
	if nickname == "" {
		nickname = "Webmaster"
	}
	passwordHash, err := hashPassword(cmd.Password)
	if err != nil {
		return nil, AuthToken{}, err
	}
	u, err := domain.New(s.idgen.Generate(), domain.RegisterCmd{
		Username: username,
		Email:    email,
		Password: cmd.Password,
		Nickname: nickname,
	}, passwordHash)
	if err != nil {
		return nil, AuthToken{}, err
	}
	u.ProtectedAccount = true
	u.TouchLogin(time.Now())
	if err := s.repo.EnsureWebmaster(ctx, u); err != nil {
		return nil, AuthToken{}, err
	}
	token, err := s.issueToken(ctx, u, LoginMethodWebmaster)
	if err != nil {
		return nil, AuthToken{}, err
	}
	s.publishEvents(ctx, u.Events()...)
	return s.profileForAuthResponse(ctx, u), token, nil
}

func (s *Service) UpdateProfile(ctx context.Context, id int64, cmd domain.UpdateProfileCmd) (*domain.User, error) {
	u, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}
	profileThemeRequested := strings.TrimSpace(cmd.ProfileTheme) != ""
	if err := u.UpdateProfile(cmd); err != nil {
		return nil, err
	}
	if strings.TrimSpace(u.BackgroundURL) != "" {
		active, err := s.hasActiveMembershipEntitlement(ctx, u.ID)
		if err != nil {
			return nil, err
		}
		if !active {
			return nil, domain.ErrProfileBackgroundEntitlementRequired
		}
	}
	if domain.NormalizeProfileTheme(u.ProfileTheme) == domain.ProfileThemePro {
		active, err := s.hasActiveProfileThemeEntitlement(ctx, u.ID)
		if err != nil {
			return nil, err
		}
		if !active {
			if profileThemeRequested {
				return nil, domain.ErrProfileThemeEntitlementRequired
			}
			u.ProfileTheme = domain.ProfileThemeDefault
		}
	}
	if err := s.repo.UpdateProfile(ctx, u); err != nil {
		return nil, err
	}
	s.publishEvents(ctx, u.Events()...)
	return u, nil
}

// UpdateUserMemo creates, replaces, or deletes the caller's private memo for
// a target user. The target lookup keeps the API's no-such-user behavior
// independent from the memo table's foreign-key implementation.
func (s *Service) UpdateUserMemo(ctx context.Context, userID, targetUserID int64, memo string) error {
	if userID <= 0 || targetUserID <= 0 {
		return domain.ErrInvalidID
	}
	if _, err := s.repo.FindByID(ctx, targetUserID); err != nil {
		return err
	}
	memo = domain.NormalizeUserMemo(memo)
	if !domain.ValidUserMemo(memo) {
		return domain.ErrUserMemoTooLong
	}
	repo, ok := s.repo.(domain.UserMemoRepository)
	if !ok {
		return domain.ErrUserMemoRepositoryUnavailable
	}
	return repo.UpdateUserMemo(ctx, userID, targetUserID, memo)
}

func (s *Service) GetUserMemo(ctx context.Context, userID, targetUserID int64) (string, error) {
	if userID <= 0 || targetUserID <= 0 {
		return "", domain.ErrInvalidID
	}
	if _, err := s.repo.FindByID(ctx, targetUserID); err != nil {
		return "", err
	}
	repo, ok := s.repo.(domain.UserMemoRepository)
	if !ok {
		return "", domain.ErrUserMemoRepositoryUnavailable
	}
	return repo.GetUserMemo(ctx, userID, targetUserID)
}

func (s *Service) ChangePassword(ctx context.Context, id int64, oldPassword string, newPassword string) error {
	if err := s.validatePassword(newPassword); err != nil {
		return err
	}
	u, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return err
	}
	if bcrypt.CompareHashAndPassword([]byte(u.PasswordHash), []byte(oldPassword)) != nil {
		return domain.ErrInvalidPassword
	}
	passwordHash, err := hashPassword(newPassword)
	if err != nil {
		return err
	}
	previousPasswordHash := u.PasswordHash
	credentialVersion, err := newCredentialVersion()
	if err != nil {
		return err
	}
	if err := u.ChangePasswordHash(passwordHash); err != nil {
		return err
	}
	u.CredentialVersion = credentialVersion
	// The conditional update writes both password fields together. If another
	// password rotation won after the read above, it fails rather than silently
	// overwriting that newer credential state.
	if err := s.repo.UpdatePasswordAndCredentialVersion(ctx, u, previousPasswordHash); err != nil {
		return err
	}
	s.refreshCredentialVersionCache(ctx, u.ID, credentialVersion)
	s.publishEvents(ctx, u.Events()...)
	return nil
}

func (s *Service) RequestPasswordReset(ctx context.Context, email string) (PasswordResetResult, error) {
	email = domain.NormalizeEmail(email)
	if !validEmail(email) {
		return PasswordResetResult{}, domain.ErrEmailInvalid
	}
	if err := s.ensureSecurityEmailDelivery(); err != nil {
		return PasswordResetResult{}, err
	}
	u, err := s.repo.FindByEmail(ctx, email)
	if errors.Is(err, domain.ErrNotFound) {
		return PasswordResetResult{Accepted: true}, nil
	}
	if err != nil {
		return PasswordResetResult{}, err
	}
	if err := u.EnsureActive(); err != nil {
		return PasswordResetResult{}, err
	}
	rawToken, err := randomToken()
	if err != nil {
		return PasswordResetResult{}, err
	}
	now := time.Now()
	expiresAt := now.Add(passwordResetTokenTTL)
	if err := s.repo.CreatePasswordResetToken(ctx, domain.PasswordResetToken{
		TokenHash: passwordResetTokenHash(rawToken),
		UserID:    u.ID,
		Email:     u.Email,
		ExpiresAt: expiresAt,
		CreatedAt: now,
	}); err != nil {
		return PasswordResetResult{}, err
	}
	if err := s.securityEmails.SendPasswordReset(ctx, u.Email, rawToken, expiresAt); err != nil {
		s.logSecurityEmailFailure("send password reset email failed", u.ID, err)
		return PasswordResetResult{}, domain.ErrSecurityEmailDeliveryUnavailable
	}
	return PasswordResetResult{Accepted: true, ResetToken: rawToken, ExpiresAt: expiresAt}, nil
}

func (s *Service) ResetPassword(ctx context.Context, token string, newPassword string) error {
	if err := s.validatePassword(newPassword); err != nil {
		return err
	}
	token = strings.TrimSpace(token)
	if token == "" {
		return domain.ErrResetTokenInvalid
	}
	now := time.Now()
	tokenHash := passwordResetTokenHash(token)
	passwordHash, err := hashPassword(newPassword)
	if err != nil {
		return err
	}
	credentialVersion, err := newCredentialVersion()
	if err != nil {
		return err
	}
	// Token consumption, password replacement, and credential rotation happen
	// in the same PostgreSQL transaction in the repository.
	u, err := s.repo.ResetPasswordWithToken(ctx, tokenHash, passwordHash, credentialVersion, now)
	if err != nil {
		return err
	}
	s.refreshCredentialVersionCache(ctx, u.ID, credentialVersion)
	u.AddEvent(domain.NewUpdatedEvent(u))
	s.publishEvents(ctx, u.Events()...)
	return nil
}

func (s *Service) RequestEmailVerification(ctx context.Context, userID int64) (EmailVerificationResult, error) {
	if userID <= 0 {
		return EmailVerificationResult{}, domain.ErrInvalidID
	}
	u, err := s.repo.FindByID(ctx, userID)
	if err != nil {
		return EmailVerificationResult{}, err
	}
	if err := u.EnsureActive(); err != nil {
		return EmailVerificationResult{}, err
	}
	if u.EmailVerifiedAt != nil {
		return EmailVerificationResult{Accepted: true, AlreadyVerified: true}, nil
	}
	if err := s.ensureSecurityEmailDelivery(); err != nil {
		return EmailVerificationResult{}, err
	}
	rawToken, err := randomToken()
	if err != nil {
		return EmailVerificationResult{}, err
	}
	now := time.Now()
	expiresAt := now.Add(emailVerificationTokenTTL)
	if err := s.repo.CreateEmailVerificationToken(ctx, domain.EmailVerificationToken{
		TokenHash: emailVerificationTokenHash(rawToken),
		UserID:    u.ID,
		Email:     u.Email,
		ExpiresAt: expiresAt,
		CreatedAt: now,
	}); err != nil {
		return EmailVerificationResult{}, err
	}
	if err := s.securityEmails.SendEmailVerification(ctx, u.Email, rawToken, expiresAt); err != nil {
		s.logSecurityEmailFailure("send email verification email failed", u.ID, err)
		return EmailVerificationResult{}, domain.ErrSecurityEmailDeliveryUnavailable
	}
	return EmailVerificationResult{Accepted: true, VerificationToken: rawToken, ExpiresAt: expiresAt}, nil
}

func (s *Service) VerifyEmail(ctx context.Context, token string) (*domain.User, error) {
	token = strings.TrimSpace(token)
	if token == "" {
		return nil, domain.ErrEmailVerificationTokenInvalid
	}
	u, err := s.repo.VerifyEmailWithToken(ctx, emailVerificationTokenHash(token), time.Now())
	if err != nil {
		return nil, err
	}
	u.AddEvent(domain.NewUpdatedEvent(u))
	s.publishEvents(ctx, u.Events()...)
	return s.profileForAuthResponse(ctx, u), nil
}

func (s *Service) UpdateStatus(ctx context.Context, id int64, status domain.Status) (*domain.User, error) {
	u, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if err := u.UpdateStatus(status); err != nil {
		return nil, err
	}
	if err := s.repo.UpdateStatus(ctx, u); err != nil {
		return nil, err
	}
	s.publishEvents(ctx, u.Events()...)
	return s.profileForAuthResponse(ctx, u), nil
}

// Follow either creates the relation immediately or, when the target account
// requires approval, records a pending request. The boolean reports whether the
// caller ended up with a pending request instead of a live follow.
func (s *Service) Follow(ctx context.Context, followerID, followeeID int64) (bool, error) {
	if followerID <= 0 || followeeID <= 0 {
		return false, domain.ErrInvalidID
	}
	if followerID == followeeID {
		return false, domain.ErrCannotFollowSelf
	}
	follower, err := s.repo.FindByID(ctx, followerID)
	if err != nil {
		return false, err
	}
	if err := follower.EnsureActive(); err != nil {
		return false, err
	}
	followee, err := s.repo.FindByID(ctx, followeeID)
	if err != nil {
		return false, err
	}
	if err := followee.EnsureActive(); err != nil {
		return false, err
	}
	safetyRepo, ok := s.repo.(domain.SafetyRepository)
	if !ok {
		return false, domain.ErrSafetyRepositoryUnavailable
	}
	if requests, ok := s.repo.(domain.FollowRequestRepository); ok {
		pending, created, err := requests.FollowOrRequest(ctx, s.idgen.Generate(), followerID, followeeID)
		if err != nil {
			return false, err
		}
		if pending {
			if created {
				s.publishEvents(ctx, domain.NewFollowRequestedEvent(followerID, followeeID))
			}
			return true, nil
		}
		s.publishEvents(ctx, domain.NewFollowedEvent(followerID, followeeID))
		return false, nil
	}
	// Repositories predating private-account support retain the original public
	// follow behavior; production repos implement FollowOrRequest above.
	relation, err := safetyRepo.GetSafetyRelation(ctx, followerID, followeeID)
	if err != nil {
		return false, err
	}
	if relation.Blocked || relation.BlockedBy {
		return false, domain.ErrFollowBlocked
	}
	if followee.FollowApprovalRequired {
		return false, domain.ErrFollowRequestRepositoryUnavailable
	}
	if err := s.repo.Follow(ctx, followerID, followeeID); err != nil {
		return false, err
	}
	s.publishEvents(ctx, domain.NewFollowedEvent(followerID, followeeID))
	return false, nil
}
func (s *Service) Block(ctx context.Context, actorID, targetID int64) error {
	repo, err := s.safetyRepository(actorID, targetID, true)
	if err != nil {
		return err
	}
	actor, err := s.repo.FindByID(ctx, actorID)
	if err != nil {
		return err
	}
	if err := actor.EnsureActive(); err != nil {
		return err
	}
	target, err := s.repo.FindByID(ctx, targetID)
	if err != nil {
		return err
	}
	if err := target.EnsureActive(); err != nil {
		return err
	}
	return repo.Block(ctx, actorID, targetID)
}

func (s *Service) Unblock(ctx context.Context, actorID, targetID int64) error {
	repo, err := s.safetyRepository(actorID, targetID, true)
	if err != nil {
		return err
	}
	if _, err := s.repo.FindByID(ctx, actorID); err != nil {
		return err
	}
	if _, err := s.repo.FindByID(ctx, targetID); err != nil {
		return err
	}
	return repo.Unblock(ctx, actorID, targetID)
}

func (s *Service) Mute(ctx context.Context, actorID, targetID int64) error {
	repo, err := s.safetyRepository(actorID, targetID, true)
	if err != nil {
		return err
	}
	if _, err := s.repo.FindByID(ctx, actorID); err != nil {
		return err
	}
	if _, err := s.repo.FindByID(ctx, targetID); err != nil {
		return err
	}
	return repo.Mute(ctx, actorID, targetID)
}

func (s *Service) Unmute(ctx context.Context, actorID, targetID int64) error {
	repo, err := s.safetyRepository(actorID, targetID, true)
	if err != nil {
		return err
	}
	if _, err := s.repo.FindByID(ctx, actorID); err != nil {
		return err
	}
	if _, err := s.repo.FindByID(ctx, targetID); err != nil {
		return err
	}
	return repo.Unmute(ctx, actorID, targetID)
}

func (s *Service) safetyRepository(actorID, targetID int64, rejectSelf bool) (domain.SafetyRepository, error) {
	if actorID <= 0 || targetID <= 0 {
		return nil, domain.ErrInvalidID
	}
	if rejectSelf && actorID == targetID {
		return nil, domain.ErrCannotRelateSelf
	}
	repo, ok := s.repo.(domain.SafetyRepository)
	if !ok {
		return nil, domain.ErrSafetyRepositoryUnavailable
	}
	return repo, nil
}

func (s *Service) Unfollow(ctx context.Context, followerID, followeeID int64) error {
	if followerID <= 0 || followeeID <= 0 {
		return domain.ErrInvalidID
	}
	if followerID == followeeID {
		return domain.ErrCannotFollowSelf
	}
	if err := s.repo.Unfollow(ctx, followerID, followeeID); err != nil {
		return err
	}
	s.publishEvents(ctx, domain.NewUnfollowedEvent(followerID, followeeID))
	return nil
}

func (s *Service) validatePassword(password string) error {
	if password == "" {
		return domain.ErrPasswordRequired
	}
	if len([]rune(password)) < s.passwordMinLength {
		return domain.ErrPasswordTooShort
	}
	return nil
}

func (s *Service) ensureSecurityEmailDelivery() error {
	if s.securityEmails == nil || !s.securityEmails.Ready() {
		return domain.ErrSecurityEmailDeliveryUnavailable
	}
	return nil
}

func (s *Service) logSecurityEmailFailure(message string, userID int64, err error) {
	if s.log != nil {
		s.log.Warn(message, logger.Int64("user_id", userID), logger.Error(err))
	}
}

func (s *Service) hasActiveProfileThemeEntitlement(ctx context.Context, userID int64) (bool, error) {
	if s.themeEntitlements == nil {
		return false, domain.ErrProfileThemeEntitlementRequired
	}
	return s.themeEntitlements.HasActiveProfileTheme(ctx, userID, domain.ProfileThemePro)
}

func (s *Service) hasActiveMembershipEntitlement(ctx context.Context, userID int64) (bool, error) {
	if s.themeEntitlements == nil {
		return false, domain.ErrProfileBackgroundEntitlementRequired
	}
	return s.themeEntitlements.HasActiveMembership(ctx, userID)
}

// issueToken signs an access token and records the session it represents. The
// JWT id doubles as the session id so the gateway can revoke a single device by
// rejecting that claim.
func (s *Service) issueToken(ctx context.Context, u *domain.User, loginMethod string) (AuthToken, error) {
	if len(s.jwtSecret) == 0 {
		return AuthToken{}, fmt.Errorf("jwt secret required")
	}
	if u == nil || u.ID <= 0 {
		return AuthToken{}, domain.ErrInvalidID
	}
	credentialVersion := strings.TrimSpace(u.CredentialVersion)
	if credentialVersion == "" {
		credentialVersion = credentialVersionInitial
	}
	jti, err := randomToken()
	if err != nil {
		return AuthToken{}, fmt.Errorf("generate jwt id: %w", err)
	}
	issuedAt := time.Now()
	expiresAt := issuedAt.Add(s.jwtTTL)
	claims := jwt.MapClaims{
		"sub":                  fmt.Sprintf("%d", u.ID),
		"user_id":              u.ID,
		"username":             u.Username,
		"jti":                  jti,
		credentialVersionClaim: credentialVersion,
		"exp":                  expiresAt.Unix(),
		"iat":                  issuedAt.Unix(),
	}
	value, err := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString(s.jwtSecret)
	if err != nil {
		return AuthToken{}, err
	}
	s.recordSession(ctx, u.ID, jti, loginMethod, issuedAt, expiresAt)
	return AuthToken{Value: value, ExpiresAt: expiresAt}, nil
}

func newCredentialVersion() (string, error) {
	version, err := randomToken()
	if err != nil {
		return "", fmt.Errorf("generate credential version: %w", err)
	}
	return version, nil
}

func (s *Service) refreshCredentialVersionCache(ctx context.Context, userID int64, version string) {
	if s.credentialVersions == nil {
		if s.log != nil {
			s.log.Warn("credential version cache is not configured", logger.Int64("user_id", userID))
		}
		return
	}
	if err := s.credentialVersions.SetCurrent(ctx, userID, version); err != nil {
		if s.log != nil {
			s.log.Warn("refresh credential version cache failed", logger.Int64("user_id", userID), logger.Error(err))
		}
		// A failed SET can leave an older no-TTL cache value in place. Removing
		// it forces api-gateway to read the durable PostgreSQL version instead.
		if deleteErr := s.credentialVersions.Delete(ctx, userID); deleteErr != nil && s.log != nil {
			s.log.Warn("delete stale credential version cache failed", logger.Int64("user_id", userID), logger.Error(deleteErr))
		}
	}
}

func (s *Service) publishEvents(ctx context.Context, events ...domain.DomainEvent) {
	if s.publisher == nil || len(events) == 0 {
		return
	}
	publishCtx, cancel := context.WithTimeout(ctx, eventPublishTimeout)
	defer cancel()
	if err := s.publisher.PublishDomainEvents(publishCtx, events); err != nil && s.log != nil {
		s.log.Warn("publish user events failed", logger.Error(err))
	}
}

func hashPassword(password string) (string, error) {
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return "", err
	}
	return string(hash), nil
}

func (s *Service) availableOAuthUsername(ctx context.Context, provider string, preferred string, providerUserID string) (string, error) {
	base := sanitizeUsername(preferred)
	if !domain.ValidUsername(base) {
		base = sanitizeUsername(provider + "_" + providerUserID)
	}
	if !domain.ValidUsername(base) {
		base = sanitizeUsername(provider + "_" + stableSuffix(providerUserID))
	}
	if !domain.ValidUsername(base) {
		return "", domain.ErrUsernameInvalid
	}
	for i := 0; i < 20; i++ {
		candidate := base
		if i > 0 {
			suffix := fmt.Sprintf("_%d", i+1)
			limit := domain.MaxUsernameLen - len(suffix)
			if len(candidate) > limit {
				candidate = candidate[:limit]
			}
			candidate += suffix
		}
		_, err := s.repo.FindByUsername(ctx, candidate)
		if errors.Is(err, domain.ErrNotFound) {
			return candidate, nil
		}
		if err != nil {
			return "", err
		}
	}
	return "", domain.ErrUsernameExists
}

func (s *Service) availableOAuthEmail(ctx context.Context, provider string, providerUserID string, preferred string) string {
	email := domain.NormalizeEmail(preferred)
	if !validEmail(email) {
		return syntheticOAuthEmail(provider, providerUserID)
	}
	if _, err := s.repo.FindByEmail(ctx, email); errors.Is(err, domain.ErrNotFound) {
		return email
	}
	return syntheticOAuthEmail(provider, providerUserID)
}

func sanitizeUsername(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	var b strings.Builder
	lastUnderscore := false
	for _, r := range value {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9':
			b.WriteRune(r)
			lastUnderscore = false
		case r == '_' || r == '-' || r == '.':
			if !lastUnderscore {
				b.WriteByte('_')
				lastUnderscore = true
			}
		}
		if b.Len() >= domain.MaxUsernameLen {
			break
		}
	}
	return strings.Trim(b.String(), "_")
}

func syntheticOAuthEmail(provider string, providerUserID string) string {
	return fmt.Sprintf("%s_%s@oauth.local", domain.NormalizeProvider(provider), stableSuffix(providerUserID))
}

func stableSuffix(value string) string {
	h := fnv.New64a()
	_, _ = h.Write([]byte(value))
	return fmt.Sprintf("%x", h.Sum64())
}

func validEmail(email string) bool {
	if strings.TrimSpace(email) == "" {
		return false
	}
	_, err := mail.ParseAddress(email)
	return err == nil
}

func randomPasswordHash() (string, error) {
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return hashPassword("oauth:" + hex.EncodeToString(buf))
}

func randomToken() (string, error) {
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return hex.EncodeToString(buf), nil
}

func randomInviteCode() (string, error) {
	var buf [10]byte
	if _, err := rand.Read(buf[:]); err != nil {
		return "", fmt.Errorf("generate invite code: %w", err)
	}
	return strings.ToUpper(hex.EncodeToString(buf[:])), nil
}

func passwordResetTokenHash(token string) string {
	sum := sha256.Sum256([]byte(strings.TrimSpace(token)))
	return hex.EncodeToString(sum[:])
}

func emailVerificationTokenHash(token string) string {
	sum := sha256.Sum256([]byte(strings.TrimSpace(token)))
	return hex.EncodeToString(sum[:])
}
