package command

import (
	"context"
	"fmt"
	"time"

	domain "user-service/internal/domain/user"
	"user-service/internal/infrastructure/messaging"
	"user-service/pkg/logger"

	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/bcrypt"
)

type IDGenerator interface {
	Generate() int64
}

type AuthToken struct {
	Value     string
	ExpiresAt time.Time
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
