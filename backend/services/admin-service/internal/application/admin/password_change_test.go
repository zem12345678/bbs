package admin

import (
	"context"
	"errors"
	"testing"
	"time"

	domain "admin/internal/domain/admin"
)

func TestChangePasswordUpdatesCurrentAdminPassword(t *testing.T) {
	store := &passwordChangeAuthStore{
		user: domain.AdminUser{
			ID:           7,
			Username:     "admin",
			Status:       domain.AdminStatusActive,
			PasswordHash: "stored-hash",
		},
	}
	service := &Service{
		authStore: store,
		passwords: passwordChangeVerifier{},
		hasher:    passwordChangeHasher{},
	}

	profile, err := service.ChangePassword(context.Background(), domain.Actor{ID: 7, Username: "admin"}, "Old123!", "New1234!")
	if err != nil {
		t.Fatalf("ChangePassword() error = %v", err)
	}
	if !store.updateCalled {
		t.Fatalf("password update was not persisted")
	}
	if store.updatedHash != "hashed:New1234!" {
		t.Fatalf("updated hash = %q, want hashed new password", store.updatedHash)
	}
	if profile.User.ID != 7 {
		t.Fatalf("profile user id = %d, want 7", profile.User.ID)
	}
	if len(profile.Roles) != 1 || profile.Roles[0] != "admin" {
		t.Fatalf("roles = %v, want [admin]", profile.Roles)
	}
}

func TestChangePasswordRejectsWrongOldPassword(t *testing.T) {
	store := &passwordChangeAuthStore{
		user: domain.AdminUser{
			ID:           7,
			Username:     "admin",
			Status:       domain.AdminStatusActive,
			PasswordHash: "stored-hash",
		},
	}
	service := &Service{
		authStore: store,
		passwords: passwordChangeVerifier{err: errors.New("mismatch")},
		hasher:    passwordChangeHasher{},
	}

	_, err := service.ChangePassword(context.Background(), domain.Actor{ID: 7, Username: "admin"}, "Wrong123!", "New1234!")
	if !errors.Is(err, domain.ErrInvalidCredentials) {
		t.Fatalf("ChangePassword() error = %v, want ErrInvalidCredentials", err)
	}
	if store.updateCalled {
		t.Fatalf("password update should not run after wrong old password")
	}
}

func TestChangePasswordRejectsWeakNewPassword(t *testing.T) {
	store := &passwordChangeAuthStore{
		user: domain.AdminUser{
			ID:           7,
			Username:     "admin",
			Status:       domain.AdminStatusActive,
			PasswordHash: "stored-hash",
		},
	}
	service := &Service{
		authStore: store,
		passwords: passwordChangeVerifier{},
		hasher:    passwordChangeHasher{},
	}

	_, err := service.ChangePassword(context.Background(), domain.Actor{ID: 7, Username: "admin"}, "Old123!", "weak")
	if !errors.Is(err, domain.ErrInvalidPassword) {
		t.Fatalf("ChangePassword() error = %v, want ErrInvalidPassword", err)
	}
	if store.updateCalled {
		t.Fatalf("password update should not run for weak new password")
	}
}

type passwordChangeAuthStore struct {
	user         domain.AdminUser
	updateCalled bool
	updatedHash  string
}

func (s *passwordChangeAuthStore) FindAdminUserByAccount(context.Context, string) (domain.AdminUser, error) {
	return domain.AdminUser{}, domain.ErrInvalidCredentials
}

func (s *passwordChangeAuthStore) FindAdminUserByID(_ context.Context, id int64) (domain.AdminUser, error) {
	if id != s.user.ID {
		return domain.AdminUser{}, domain.ErrInvalidCredentials
	}
	return s.user, nil
}

func (s *passwordChangeAuthStore) UpdateAdminProfile(context.Context, domain.UpdateAdminProfileCommand) (domain.AdminUser, error) {
	return domain.AdminUser{}, domain.ErrInvalidAdminProfile
}

func (s *passwordChangeAuthStore) UpdateAdminPassword(_ context.Context, userID int64, passwordHash string) (domain.AdminUser, error) {
	if userID != s.user.ID {
		return domain.AdminUser{}, domain.ErrInvalidAdminUserID
	}
	s.updateCalled = true
	s.updatedHash = passwordHash
	s.user.PasswordHash = passwordHash
	return s.user, nil
}

func (s *passwordChangeAuthStore) RoleKeysByUserID(context.Context, int64) ([]string, error) {
	return []string{"admin"}, nil
}

func (s *passwordChangeAuthStore) PermissionsByRoleKeys(context.Context, []string) ([]string, error) {
	return []string{"*:*"}, nil
}

func (s *passwordChangeAuthStore) UpdateAdminLastLogin(context.Context, int64, string) error {
	return nil
}

func (s *passwordChangeAuthStore) CreateAdminSession(context.Context, int64, string, time.Time) error {
	return nil
}

func (s *passwordChangeAuthStore) IsAdminSessionActive(context.Context, int64, string) (bool, error) {
	return true, nil
}

func (s *passwordChangeAuthStore) RotateAdminSession(context.Context, int64, string, string, time.Time) error {
	return nil
}

func (s *passwordChangeAuthStore) RevokeAdminSession(context.Context, int64, string) error {
	return nil
}

type passwordChangeVerifier struct {
	err error
}

func (v passwordChangeVerifier) Verify(string, string) error {
	return v.err
}

type passwordChangeHasher struct{}

func (passwordChangeHasher) Hash(password string) (string, error) {
	return "hashed:" + password, nil
}
