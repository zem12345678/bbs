package admin

import (
	"context"
	"errors"
	"testing"
	"time"

	domain "admin/internal/domain/admin"
	adminauth "admin/internal/infrastructure/auth"
)

func TestAdminSessionRefreshRotatesAndLogoutRevokesTokens(t *testing.T) {
	store := &sessionAuthStore{user: domain.AdminUser{
		ID:           7,
		Username:     "admin",
		Status:       domain.AdminStatusActive,
		PasswordHash: "stored-hash",
	}}
	service := &Service{
		authStore: store,
		passwords: sessionPasswordVerifier{},
		tokens:    adminauth.NewTokenManager("test-secret", time.Hour, 24*time.Hour),
	}

	first, err := service.Login(context.Background(), "admin", "Admin123!", "127.0.0.1", "test")
	if err != nil {
		t.Fatalf("Login() error = %v", err)
	}
	if _, err := service.GetProfile(context.Background(), first.Token.AccessToken); err != nil {
		t.Fatalf("GetProfile(first access token) error = %v", err)
	}

	second, err := service.Refresh(context.Background(), first.Token.RefreshToken)
	if err != nil {
		t.Fatalf("Refresh() error = %v", err)
	}
	if second.Token.AccessToken == first.Token.AccessToken || second.Token.RefreshToken == first.Token.RefreshToken {
		t.Fatal("Refresh() did not rotate both tokens")
	}
	if _, err := service.GetProfile(context.Background(), first.Token.AccessToken); !errors.Is(err, domain.ErrInvalidToken) {
		t.Fatalf("GetProfile(old access token) error = %v, want ErrInvalidToken", err)
	}
	if _, err := service.Refresh(context.Background(), first.Token.RefreshToken); !errors.Is(err, domain.ErrInvalidToken) {
		t.Fatalf("Refresh(old refresh token) error = %v, want ErrInvalidToken", err)
	}
	if _, err := service.GetProfile(context.Background(), second.Token.AccessToken); err != nil {
		t.Fatalf("GetProfile(refreshed access token) error = %v", err)
	}

	if err := service.Logout(context.Background(), second.Token.AccessToken); err != nil {
		t.Fatalf("Logout() error = %v", err)
	}
	if _, err := service.GetProfile(context.Background(), second.Token.AccessToken); !errors.Is(err, domain.ErrInvalidToken) {
		t.Fatalf("GetProfile(logged-out access token) error = %v, want ErrInvalidToken", err)
	}
}

type sessionAuthStore struct {
	user      domain.AdminUser
	sessionID string
	expiresAt time.Time
	lastLogin string
}

func (s *sessionAuthStore) FindAdminUserByAccount(_ context.Context, account string) (domain.AdminUser, error) {
	if account != s.user.Username {
		return domain.AdminUser{}, domain.ErrInvalidCredentials
	}
	return s.user, nil
}

func (s *sessionAuthStore) FindAdminUserByID(_ context.Context, id int64) (domain.AdminUser, error) {
	if id != s.user.ID {
		return domain.AdminUser{}, domain.ErrInvalidCredentials
	}
	return s.user, nil
}

func (s *sessionAuthStore) UpdateAdminProfile(context.Context, domain.UpdateAdminProfileCommand) (domain.AdminUser, error) {
	return s.user, nil
}

func (s *sessionAuthStore) UpdateAdminPassword(context.Context, int64, string) (domain.AdminUser, error) {
	return s.user, nil
}

func (s *sessionAuthStore) RoleKeysByUserID(context.Context, int64) ([]string, error) {
	return []string{"admin"}, nil
}

func (s *sessionAuthStore) PermissionsByRoleKeys(context.Context, []string) ([]string, error) {
	return []string{"*:*"}, nil
}

func (s *sessionAuthStore) UpdateAdminLastLogin(_ context.Context, _ int64, loginIP string) error {
	s.lastLogin = loginIP
	return nil
}

func (s *sessionAuthStore) CreateAdminSession(_ context.Context, userID int64, sessionID string, expiresAt time.Time) error {
	if userID != s.user.ID {
		return domain.ErrInvalidToken
	}
	s.sessionID = sessionID
	s.expiresAt = expiresAt
	return nil
}

func (s *sessionAuthStore) IsAdminSessionActive(_ context.Context, userID int64, sessionID string) (bool, error) {
	return userID == s.user.ID && sessionID == s.sessionID && s.expiresAt.After(time.Now()), nil
}

func (s *sessionAuthStore) RotateAdminSession(_ context.Context, userID int64, oldSessionID string, newSessionID string, expiresAt time.Time) error {
	if userID != s.user.ID || oldSessionID != s.sessionID || !s.expiresAt.After(time.Now()) {
		return domain.ErrInvalidToken
	}
	s.sessionID = newSessionID
	s.expiresAt = expiresAt
	return nil
}

func (s *sessionAuthStore) RevokeAdminSession(_ context.Context, userID int64, sessionID string) error {
	if userID != s.user.ID || sessionID != s.sessionID {
		return domain.ErrInvalidToken
	}
	s.sessionID = ""
	s.expiresAt = time.Time{}
	return nil
}

type sessionPasswordVerifier struct{}

func (sessionPasswordVerifier) Verify(string, string) error { return nil }
