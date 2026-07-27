package persistence

import (
	"context"
	"strings"
	"time"

	domain "admin/internal/domain/admin"
	"admin/internal/infrastructure/persistence/po"

	"gorm.io/gorm/clause"
)

const maxAdminSessionIDLength = 32

// CreateAdminSession creates or replaces the active session for an admin.
func (r *Repository) CreateAdminSession(ctx context.Context, userID int64, sessionID string, expiresAt time.Time) error {
	now := time.Now()
	sessionID, err := validateAdminSession(userID, sessionID, expiresAt, now)
	if err != nil {
		return err
	}

	return r.db.WithContext(ctx).Clauses(clause.OnConflict{
		Columns: []clause.Column{{Name: "user_id"}},
		DoUpdates: clause.Assignments(map[string]any{
			"token":       sessionID,
			"expire_time": expiresAt,
			"update_time": now,
		}),
	}).Create(&po.UserToken{
		UserId:     int(userID),
		Token:      sessionID,
		UpdateTime: now,
		ExpireTime: expiresAt,
	}).Error
}

// IsAdminSessionActive verifies that the supplied session is still current and unexpired.
func (r *Repository) IsAdminSessionActive(ctx context.Context, userID int64, sessionID string) (bool, error) {
	sessionID, err := validateAdminSessionID(userID, sessionID)
	if err != nil {
		return false, err
	}

	var count int64
	err = r.db.WithContext(ctx).Model(&po.UserToken{}).
		Where("user_id = ? AND token = ? AND expire_time > ?", userID, sessionID, time.Now()).
		Count(&count).Error
	if err != nil {
		return false, err
	}
	if count != 1 {
		return false, domain.ErrInvalidToken
	}
	return true, nil
}

// RotateAdminSession atomically replaces an active session. A stale refresh token
// cannot pass the old-session predicate after another refresh succeeds.
func (r *Repository) RotateAdminSession(ctx context.Context, userID int64, oldSessionID string, newSessionID string, expiresAt time.Time) error {
	now := time.Now()
	oldSessionID, err := validateAdminSessionID(userID, oldSessionID)
	if err != nil {
		return err
	}
	newSessionID, err = validateAdminSession(userID, newSessionID, expiresAt, now)
	if err != nil {
		return err
	}
	if oldSessionID == newSessionID {
		return domain.ErrInvalidToken
	}

	result := r.db.WithContext(ctx).Model(&po.UserToken{}).
		Where("user_id = ? AND token = ? AND expire_time > ?", userID, oldSessionID, now).
		Updates(map[string]any{
			"token":       newSessionID,
			"expire_time": expiresAt,
			"update_time": now,
		})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected != 1 {
		return domain.ErrInvalidToken
	}
	return nil
}

// RevokeAdminSession removes the current active session.
func (r *Repository) RevokeAdminSession(ctx context.Context, userID int64, sessionID string) error {
	sessionID, err := validateAdminSessionID(userID, sessionID)
	if err != nil {
		return err
	}

	result := r.db.WithContext(ctx).Where("user_id = ? AND token = ? AND expire_time > ?", userID, sessionID, time.Now()).Delete(&po.UserToken{})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected != 1 {
		return domain.ErrInvalidToken
	}
	return nil
}

func validateAdminSession(userID int64, sessionID string, expiresAt time.Time, now time.Time) (string, error) {
	sessionID, err := validateAdminSessionID(userID, sessionID)
	if err != nil {
		return "", err
	}
	if !expiresAt.After(now) {
		return "", domain.ErrInvalidToken
	}
	return sessionID, nil
}

func validateAdminSessionID(userID int64, sessionID string) (string, error) {
	sessionID = strings.TrimSpace(sessionID)
	if userID <= 0 || int64(int(userID)) != userID || sessionID == "" || len(sessionID) > maxAdminSessionIDLength {
		return "", domain.ErrInvalidToken
	}
	return sessionID, nil
}
