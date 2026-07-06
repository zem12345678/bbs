package user

import "errors"

var (
	ErrNotFound                      = errors.New("user not found")
	ErrInvalidID                     = errors.New("invalid user id")
	ErrUsernameRequired              = errors.New("username required")
	ErrUsernameInvalid               = errors.New("username invalid")
	ErrEmailRequired                 = errors.New("email required")
	ErrEmailInvalid                  = errors.New("email invalid")
	ErrPasswordRequired              = errors.New("password required")
	ErrPasswordTooShort              = errors.New("password too short")
	ErrResetTokenInvalid             = errors.New("password reset token invalid")
	ErrResetTokenExpired             = errors.New("password reset token expired")
	ErrEmailVerificationTokenInvalid = errors.New("email verification token invalid")
	ErrEmailVerificationTokenExpired = errors.New("email verification token expired")
	ErrNicknameRequired              = errors.New("nickname required")
	ErrUsernameExists                = errors.New("username already exists")
	ErrEmailExists                   = errors.New("email already exists")
	ErrInvalidPassword               = errors.New("invalid password")
	ErrInvalidOAuth                  = errors.New("invalid oauth identity")
	ErrMuted                         = errors.New("user muted")
	ErrInvalidStatus                 = errors.New("invalid user status")
	ErrCannotFollowSelf              = errors.New("cannot follow self")
	ErrAlreadyFollowing              = errors.New("already following user")
	ErrNotFollowing                  = errors.New("not following user")
)
