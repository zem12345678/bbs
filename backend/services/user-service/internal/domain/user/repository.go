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

type SafetyRelation struct {
	Blocked   bool
	BlockedBy bool
	Muted     bool
}

type SafetyRepository interface {
	Block(ctx context.Context, actorID, targetID int64) error
	Unblock(ctx context.Context, actorID, targetID int64) error
	Mute(ctx context.Context, actorID, targetID int64) error
	Unmute(ctx context.Context, actorID, targetID int64) error
	GetSafetyRelation(ctx context.Context, actorID, targetID int64) (SafetyRelation, error)
	ListBlockedUsers(ctx context.Context, q FollowListQuery) ([]*User, int64, error)
	ListMutedUsers(ctx context.Context, q FollowListQuery) ([]*User, int64, error)
}

// FollowRequestRepository stores approvals pending for private accounts.
type FollowRequestRepository interface {
	// FollowOrRequest makes the privacy decision while holding the same pair
	// lock used by blocks and approvals. created is false only when an identical
	// pending request already existed.
	FollowOrRequest(ctx context.Context, requestID, requesterID, targetID int64) (pending bool, created bool, err error)
	CreateFollowRequest(ctx context.Context, req *FollowRequest) error
	DeleteFollowRequest(ctx context.Context, requesterID, targetID int64) error
	// AcceptFollowRequest deletes the pending row and creates the follow in one
	// transaction so an approval can never leave a half-applied relation.
	// The boolean reports whether a new follow was created; stale requests that
	// already have a live relation are consumed without emitting follow events.
	AcceptFollowRequest(ctx context.Context, requesterID, targetID int64) (bool, error)
	GetFollowRequest(ctx context.Context, requesterID, targetID int64) (*FollowRequest, error)
	ListReceivedFollowRequests(ctx context.Context, q FollowRequestQuery) ([]*FollowRequest, int64, error)
	ListSentFollowRequests(ctx context.Context, q FollowRequestQuery) ([]*FollowRequest, int64, error)
	SetFollowApprovalRequired(ctx context.Context, userID int64, required bool) error
}
type InviteRepository interface {
	CreateWithInvite(ctx context.Context, u *User, code string, requireInvite bool) error
	CreateInviteCodes(ctx context.Context, codes []InviteCode) error
	ListInviteCodes(ctx context.Context, q InviteCodeListQuery) ([]InviteCode, int64, error)
	RevokeInviteCode(ctx context.Context, id, actorID int64) error
}

type UserListRepository interface {
	CreateUserList(ctx context.Context, list *UserList) error
	UpdateUserList(ctx context.Context, list *UserList) error
	DeleteUserList(ctx context.Context, ownerID, listID int64) error
	GetUserList(ctx context.Context, viewerID, listID int64) (*UserList, error)
	ListUserLists(ctx context.Context, q UserListsQuery) ([]*UserList, int64, error)
	ListFavoriteUserLists(ctx context.Context, q UserListFavoritesQuery) ([]*UserList, int64, error)
	AddUserListMember(ctx context.Context, ownerID int64, membership UserListMembership) error
	RemoveUserListMember(ctx context.Context, ownerID, listID, userID int64) error
	ListUserListMembers(ctx context.Context, q UserListMembersQuery) ([]*User, int64, error)
	CopyUserList(ctx context.Context, sourceListID int64, target *UserList) error
	FavoriteUserList(ctx context.Context, favorite UserListFavorite) error
	UnfavoriteUserList(ctx context.Context, userID, listID int64) error
}

type AntennaRepository interface {
	CreateAntenna(ctx context.Context, antenna *Antenna) error
	UpdateAntenna(ctx context.Context, antenna *Antenna) error
	DeleteAntenna(ctx context.Context, ownerID, antennaID int64) error
	GetAntenna(ctx context.Context, ownerID, antennaID int64) (*Antenna, error)
	ListAntennas(ctx context.Context, ownerID int64) ([]*Antenna, error)
}

type UserListQuery struct {
	Query    string
	Status   int32
	Page     int
	PageSize int
	IDs      []int64
}

const (
	UserChartSpanHour = "hour"
	UserChartSpanDay  = "day"

	DefaultUserChartLimit    = 30
	MaxUserChartLimit        = 500
	MaxUserChartOffsetMillis = int64(8640000000000000)
)

type UserChartQuery struct {
	Span   string
	Limit  int
	Offset *int64
}

type UserChartSeries struct {
	Total []int64
	Inc   []int64
	Dec   []int64
}

type UserChart struct {
	Local  UserChartSeries
	Remote UserChartSeries
}

type UserChartRepository interface {
	GetUserChart(ctx context.Context, q UserChartQuery) (UserChart, error)
}

type ActiveUsersChartBucket struct {
	ReadUserIDs            []int64
	RegisteredWithinWeek   int64
	RegisteredWithinMonth  int64
	RegisteredWithinYear   int64
	RegisteredOutsideWeek  int64
	RegisteredOutsideMonth int64
	RegisteredOutsideYear  int64
}

type ActiveUsersChart struct {
	Buckets []ActiveUsersChartBucket
}

type ActiveUsersChartRepository interface {
	GetActiveUsersChart(ctx context.Context, q UserChartQuery) (ActiveUsersChart, error)
}

type UserFollowingChartQuery struct {
	Span   string
	Limit  int
	Offset *int64
	UserID int64
}

type UserFollowingChartScope struct {
	Followings UserChartSeries
	Followers  UserChartSeries
}

type UserFollowingChart struct {
	Local  UserFollowingChartScope
	Remote UserFollowingChartScope
}

type UserFollowingChartRepository interface {
	GetUserFollowingChart(ctx context.Context, q UserFollowingChartQuery) (UserFollowingChart, error)
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
