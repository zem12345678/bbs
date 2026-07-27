package user

import (
	"context"
	"time"
)

type FollowListQuery struct {
	UserID   int64
	Page     int
	PageSize int
}

type UserListQuery struct {
	Query    string
	Status   int32
	Page     int
	PageSize int
	IDs      []int64
}

type PasswordResetToken struct {
	TokenHash string
	UserID    int64
	Email     string
	ExpiresAt time.Time
	UsedAt    *time.Time
	CreatedAt time.Time
}

type EmailVerificationToken struct {
	TokenHash string
	UserID    int64
	Email     string
	ExpiresAt time.Time
	UsedAt    *time.Time
	CreatedAt time.Time
}

type Repository interface {
	Create(ctx context.Context, u *User) error
	UpdateProfile(ctx context.Context, u *User) error
	// UpdatePasswordAndCredentialVersion changes both password credential
	// fields in one durable operation. expectedPasswordHash provides an
	// optimistic guard against a concurrent password change.
	UpdatePasswordAndCredentialVersion(ctx context.Context, u *User, expectedPasswordHash string) error
	UpdateStatus(ctx context.Context, u *User) error
	UpdateLastLogin(ctx context.Context, u *User) error
	UpdateOAuthLogin(ctx context.Context, u *User, account OAuthAccount) error
	FindByID(ctx context.Context, id int64) (*User, error)
	FindByUsername(ctx context.Context, username string) (*User, error)
	FindByEmail(ctx context.Context, email string) (*User, error)
	FindByAccount(ctx context.Context, account string) (*User, error)
	FindByOAuth(ctx context.Context, provider string, providerUserID string) (*User, error)
	CreateWithOAuth(ctx context.Context, u *User, account OAuthAccount) error
	EnsureWebmaster(ctx context.Context, u *User) error
	CreatePasswordResetToken(ctx context.Context, token PasswordResetToken) error
	ResetPasswordWithToken(ctx context.Context, tokenHash string, passwordHash string, credentialVersion string, now time.Time) (*User, error)
	GetCredentialVersion(ctx context.Context, userID int64) (string, error)
	CreateEmailVerificationToken(ctx context.Context, token EmailVerificationToken) error
	VerifyEmailWithToken(ctx context.Context, tokenHash string, now time.Time) (*User, error)
	Follow(ctx context.Context, followerID, followeeID int64) error
	Unfollow(ctx context.Context, followerID, followeeID int64) error
	IsFollowing(ctx context.Context, followerID, followeeID int64) (bool, error)
	ListUsers(ctx context.Context, q UserListQuery) ([]*User, int64, error)
	ListFollowers(ctx context.Context, q FollowListQuery) ([]*User, int64, error)
	ListFollowing(ctx context.Context, q FollowListQuery) ([]*User, int64, error)
}
