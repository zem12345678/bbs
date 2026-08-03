package user

import (
	"strings"
	"time"
)

const (
	InviteStatusAll     = "all"
	InviteStatusUnused  = "unused"
	InviteStatusUsed    = "used"
	InviteStatusExpired = "expired"
	InviteStatusRevoked = "revoked"
)

type InviteCode struct {
	ID               int64
	Code             string
	CreatedByAdminID int64
	UsedByUserID     *int64
	ExpiresAt        *time.Time
	UsedAt           *time.Time
	RevokedAt        *time.Time
	RevokedByAdminID *int64
	CreatedAt        time.Time
}

type InviteCodeListQuery struct {
	Status   string
	Page     int
	PageSize int
}

func NormalizeInviteCode(code string) string {
	return strings.ToUpper(strings.TrimSpace(code))
}

func NormalizeInviteStatus(status string) string {
	status = strings.ToLower(strings.TrimSpace(status))
	if status == "" {
		return InviteStatusAll
	}
	return status
}

func ValidInviteStatus(status string) bool {
	switch NormalizeInviteStatus(status) {
	case InviteStatusAll, InviteStatusUnused, InviteStatusUsed, InviteStatusExpired, InviteStatusRevoked:
		return true
	default:
		return false
	}
}

func (code InviteCode) StatusAt(now time.Time) string {
	switch {
	case code.UsedAt != nil:
		return InviteStatusUsed
	case code.RevokedAt != nil:
		return InviteStatusRevoked
	case code.ExpiresAt != nil && !code.ExpiresAt.After(now):
		return InviteStatusExpired
	default:
		return InviteStatusUnused
	}
}

func (code InviteCode) ValidateForCreate(now time.Time) error {
	if code.ID <= 0 || code.CreatedByAdminID <= 0 {
		return ErrInvalidID
	}
	if NormalizeInviteCode(code.Code) == "" {
		return ErrInviteCodeInvalid
	}
	if code.ExpiresAt != nil && !code.ExpiresAt.After(now) {
		return ErrInviteExpiryInvalid
	}
	return nil
}
