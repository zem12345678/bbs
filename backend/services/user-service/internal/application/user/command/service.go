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

const passwordResetTokenTTL = 30 * time.Minute

type IDGenerator interface {
	Generate() int64
}

type AuthToken struct {
	Value     string
	ExpiresAt time.Time
}

type PasswordResetResult struct {
	Accepted   bool
	ResetToken string
	ExpiresAt  time.Time
}

type Service struct {
	repo              domain.Repository
	idgen             IDGenerator
	publisher         messaging.EventPublisher
	log               logger.Logger
	jwtSecret         []byte
	jwtTTL            time.Duration
	passwordMinLength int
}

func NewService(repo domain.Repository, idgen IDGenerator, publisher messaging.EventPublisher, log logger.Logger, jwtSecret string, jwtTTL time.Duration, passwordMinLength int) *Service {
	if jwtTTL <= 0 {
		jwtTTL = 7 * 24 * time.Hour
	}
	if passwordMinLength <= 0 {
		passwordMinLength = 8
	}
	return &Service{
		repo:              repo,
		idgen:             idgen,
		publisher:         publisher,
		log:               log,
		jwtSecret:         []byte(jwtSecret),
		jwtTTL:            jwtTTL,
		passwordMinLength: passwordMinLength,
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
	if err := s.repo.Create(ctx, u); err != nil {
		return nil, AuthToken{}, err
	}
	token, err := s.issueToken(u)
	if err != nil {
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
	if err := u.EnsureActive(); err != nil {
		return nil, AuthToken{}, err
	}
	if bcrypt.CompareHashAndPassword([]byte(u.PasswordHash), []byte(password)) != nil {
		return nil, AuthToken{}, domain.ErrInvalidPassword
	}
	u.TouchLogin(time.Now())
	if err := s.repo.UpdateLastLogin(ctx, u); err != nil {
		return nil, AuthToken{}, err
	}
	token, err := s.issueToken(u)
	if err != nil {
		return nil, AuthToken{}, err
	}
	return u, token, nil
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
		u.TouchLogin(now)
		account.UserID = u.ID
		if err := s.repo.UpdateOAuthLogin(ctx, u, account); err != nil {
			return nil, AuthToken{}, err
		}
		token, err := s.issueToken(u)
		if err != nil {
			return nil, AuthToken{}, err
		}
		return u, token, nil
	}
	if !errors.Is(err, domain.ErrNotFound) {
		return nil, AuthToken{}, err
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
	token, err := s.issueToken(u)
	if err != nil {
		return nil, AuthToken{}, err
	}
	s.publishEvents(ctx, u.Events()...)
	return u, token, nil
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
	u.TouchLogin(time.Now())
	if err := s.repo.EnsureWebmaster(ctx, u); err != nil {
		return nil, AuthToken{}, err
	}
	token, err := s.issueToken(u)
	if err != nil {
		return nil, AuthToken{}, err
	}
	s.publishEvents(ctx, u.Events()...)
	return u, token, nil
}

func (s *Service) UpdateProfile(ctx context.Context, id int64, cmd domain.UpdateProfileCmd) (*domain.User, error) {
	u, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if err := u.UpdateProfile(cmd); err != nil {
		return nil, err
	}
	if err := s.repo.UpdateProfile(ctx, u); err != nil {
		return nil, err
	}
	s.publishEvents(ctx, u.Events()...)
	return u, nil
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
	if err := u.ChangePasswordHash(passwordHash); err != nil {
		return err
	}
	if err := s.repo.UpdatePassword(ctx, u); err != nil {
		return err
	}
	s.publishEvents(ctx, u.Events()...)
	return nil
}

func (s *Service) RequestPasswordReset(ctx context.Context, email string) (PasswordResetResult, error) {
	email = domain.NormalizeEmail(email)
	if !validEmail(email) {
		return PasswordResetResult{}, domain.ErrEmailInvalid
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
	passwordHash, err := hashPassword(newPassword)
	if err != nil {
		return err
	}
	u, err := s.repo.ResetPasswordWithToken(ctx, passwordResetTokenHash(token), passwordHash, time.Now())
	if err != nil {
		return err
	}
	u.AddEvent(domain.NewUpdatedEvent(u))
	s.publishEvents(ctx, u.Events()...)
	return nil
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
	return u, nil
}

func (s *Service) Follow(ctx context.Context, followerID, followeeID int64) error {
	if followerID <= 0 || followeeID <= 0 {
		return domain.ErrInvalidID
	}
	if followerID == followeeID {
		return domain.ErrCannotFollowSelf
	}
	if _, err := s.repo.FindByID(ctx, followerID); err != nil {
		return err
	}
	if _, err := s.repo.FindByID(ctx, followeeID); err != nil {
		return err
	}
	if err := s.repo.Follow(ctx, followerID, followeeID); err != nil {
		return err
	}
	s.publishEvents(ctx, domain.NewFollowedEvent(followerID, followeeID))
	return nil
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

func (s *Service) issueToken(u *domain.User) (AuthToken, error) {
	if len(s.jwtSecret) == 0 {
		return AuthToken{}, fmt.Errorf("jwt secret required")
	}
	expiresAt := time.Now().Add(s.jwtTTL)
	claims := jwt.MapClaims{
		"sub":      fmt.Sprintf("%d", u.ID),
		"user_id":  u.ID,
		"username": u.Username,
		"exp":      expiresAt.Unix(),
		"iat":      time.Now().Unix(),
	}
	value, err := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString(s.jwtSecret)
	if err != nil {
		return AuthToken{}, err
	}
	return AuthToken{Value: value, ExpiresAt: expiresAt}, nil
}

func (s *Service) publishEvents(ctx context.Context, events ...domain.DomainEvent) {
	if s.publisher == nil || len(events) == 0 {
		return
	}
	if err := s.publisher.PublishDomainEvents(ctx, events); err != nil && s.log != nil {
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

func passwordResetTokenHash(token string) string {
	sum := sha256.Sum256([]byte(strings.TrimSpace(token)))
	return hex.EncodeToString(sum[:])
}
