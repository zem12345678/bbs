package user

import "context"

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
}

type Repository interface {
	Create(ctx context.Context, u *User) error
	UpdateProfile(ctx context.Context, u *User) error
	UpdatePassword(ctx context.Context, u *User) error
	UpdateStatus(ctx context.Context, u *User) error
	UpdateLastLogin(ctx context.Context, u *User) error
	FindByID(ctx context.Context, id int64) (*User, error)
	FindByUsername(ctx context.Context, username string) (*User, error)
	FindByEmail(ctx context.Context, email string) (*User, error)
	FindByAccount(ctx context.Context, account string) (*User, error)
	Follow(ctx context.Context, followerID, followeeID int64) error
	Unfollow(ctx context.Context, followerID, followeeID int64) error
	IsFollowing(ctx context.Context, followerID, followeeID int64) (bool, error)
	ListUsers(ctx context.Context, q UserListQuery) ([]*User, int64, error)
	ListFollowers(ctx context.Context, q FollowListQuery) ([]*User, int64, error)
	ListFollowing(ctx context.Context, q FollowListQuery) ([]*User, int64, error)
}
