package persistence

import (
	"context"
	"errors"
	"strings"
	"time"

	domain "user-service/internal/domain/user"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type userSessionPO struct {
	SessionID   string `gorm:"primaryKey;size:128"`
	UserID      int64
	IPAddress   string
	UserAgent   string
	LoginMethod string
	CreatedAt   time.Time
	ExpiresAt   time.Time
	RevokedAt   *time.Time
}

func (userSessionPO) TableName() string { return "user_sessions" }

type userLoginEventPO struct {
	ID            int64 `gorm:"primaryKey"`
	UserID        int64
	SessionID     *string `gorm:"size:128"`
	IPAddress     string
	UserAgent     string
	Success       bool
	FailureReason string
	CreatedAt     time.Time
}

func (userLoginEventPO) TableName() string { return "user_login_events" }

var _ domain.SessionRepository = (*Repo)(nil)

func (r *Repo) RecordSession(ctx context.Context, session domain.UserSession, event domain.LoginEvent) error {
	if err := session.Validate(); err != nil {
		return err
	}
	if !event.Success || event.SessionID != session.SessionID {
		return domain.ErrLoginEventInvalid
	}
	if err := event.Validate(); err != nil {
		return err
	}
	if event.UserID != session.UserID {
		return domain.ErrLoginEventInvalid
	}
	sessionRow := toUserSessionPO(session)
	eventRow := toUserLoginEventPO(event)
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(&sessionRow).Error; err != nil {
			return err
		}
		return tx.Create(&eventRow).Error
	})
}

func (r *Repo) RecordLoginEvent(ctx context.Context, event domain.LoginEvent) error {
	// A tracked session only exists for successful logins; failures are stored
	// without one so the schema's shape constraint stays satisfied.
	if event.Success {
		if event.SessionID == "" {
			return domain.ErrLoginEventInvalid
		}
	} else if event.SessionID != "" || domain.NormalizeLoginFailureReason(event.FailureReason) == "" {
		return domain.ErrLoginEventInvalid
	}
	if err := event.Validate(); err != nil {
		return err
	}
	row := toUserLoginEventPO(event)
	return r.db.WithContext(ctx).Create(&row).Error
}

func (r *Repo) ListSessions(ctx context.Context, userID int64, limit int) ([]domain.UserSession, error) {
	if userID <= 0 {
		return nil, domain.ErrInvalidID
	}
	var rows []userSessionPO
	if err := r.db.WithContext(ctx).
		Where("user_id = ?", userID).
		Order("created_at DESC, session_id").
		Limit(domain.SessionListLimit(limit)).
		Find(&rows).Error; err != nil {
		return nil, err
	}
	sessions := make([]domain.UserSession, 0, len(rows))
	for _, row := range rows {
		sessions = append(sessions, toUserSession(row))
	}
	return sessions, nil
}

func (r *Repo) GetSession(ctx context.Context, userID int64, sessionID string) (domain.UserSession, error) {
	if userID <= 0 {
		return domain.UserSession{}, domain.ErrInvalidID
	}
	sessionID = strings.TrimSpace(sessionID)
	if !domain.ValidSessionID(sessionID) {
		return domain.UserSession{}, domain.ErrSessionIDInvalid
	}
	var row userSessionPO
	if err := r.db.WithContext(ctx).
		First(&row, "user_id = ? AND session_id = ?", userID, sessionID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return domain.UserSession{}, domain.ErrSessionNotFound
		}
		return domain.UserSession{}, err
	}
	return toUserSession(row), nil
}

// RevokeSession is idempotent: an already-revoked session keeps its original
// revocation time instead of erroring, so a retried request still succeeds.
func (r *Repo) RevokeSession(ctx context.Context, userID int64, sessionID string, now time.Time) (domain.UserSession, error) {
	if userID <= 0 {
		return domain.UserSession{}, domain.ErrInvalidID
	}
	sessionID = strings.TrimSpace(sessionID)
	if !domain.ValidSessionID(sessionID) {
		return domain.UserSession{}, domain.ErrSessionIDInvalid
	}
	var result domain.UserSession
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var row userSessionPO
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			First(&row, "user_id = ? AND session_id = ?", userID, sessionID).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return domain.ErrSessionNotFound
			}
			return err
		}
		if row.RevokedAt == nil {
			revokedAt := now
			if revokedAt.Before(row.CreatedAt) {
				revokedAt = row.CreatedAt
			}
			if err := tx.Model(&userSessionPO{}).
				Where("session_id = ? AND revoked_at IS NULL", row.SessionID).
				Update("revoked_at", revokedAt).Error; err != nil {
				return err
			}
			row.RevokedAt = &revokedAt
		}
		result = toUserSession(row)
		return nil
	})
	if err != nil {
		return domain.UserSession{}, err
	}
	return result, nil
}

func (r *Repo) ListLoginEvents(ctx context.Context, userID int64, limit int) ([]domain.LoginEvent, error) {
	if userID <= 0 {
		return nil, domain.ErrInvalidID
	}
	var rows []userLoginEventPO
	if err := r.db.WithContext(ctx).
		Where("user_id = ?", userID).
		Order("created_at DESC, id DESC").
		Limit(domain.SessionListLimit(limit)).
		Find(&rows).Error; err != nil {
		return nil, err
	}
	events := make([]domain.LoginEvent, 0, len(rows))
	for _, row := range rows {
		events = append(events, toLoginEvent(row))
	}
	return events, nil
}

func toUserSessionPO(session domain.UserSession) userSessionPO {
	client := domain.NormalizeSessionClientInfo(domain.SessionClientInfo{
		IPAddress: session.IPAddress,
		UserAgent: session.UserAgent,
	})
	return userSessionPO{
		SessionID:   strings.TrimSpace(session.SessionID),
		UserID:      session.UserID,
		IPAddress:   client.IPAddress,
		UserAgent:   client.UserAgent,
		LoginMethod: domain.NormalizeLoginMethod(session.LoginMethod),
		CreatedAt:   session.CreatedAt,
		ExpiresAt:   session.ExpiresAt,
		RevokedAt:   session.RevokedAt,
	}
}

func toUserLoginEventPO(event domain.LoginEvent) userLoginEventPO {
	client := domain.NormalizeSessionClientInfo(domain.SessionClientInfo{
		IPAddress: event.IPAddress,
		UserAgent: event.UserAgent,
	})
	row := userLoginEventPO{
		ID:            event.ID,
		UserID:        event.UserID,
		IPAddress:     client.IPAddress,
		UserAgent:     client.UserAgent,
		Success:       event.Success,
		FailureReason: domain.NormalizeLoginFailureReason(event.FailureReason),
		CreatedAt:     event.CreatedAt,
	}
	if event.Success {
		row.FailureReason = ""
	}
	if sessionID := strings.TrimSpace(event.SessionID); sessionID != "" {
		row.SessionID = &sessionID
	}
	return row
}

func toUserSession(row userSessionPO) domain.UserSession {
	return domain.UserSession{
		SessionID:   row.SessionID,
		UserID:      row.UserID,
		IPAddress:   row.IPAddress,
		UserAgent:   row.UserAgent,
		LoginMethod: row.LoginMethod,
		CreatedAt:   row.CreatedAt,
		ExpiresAt:   row.ExpiresAt,
		RevokedAt:   row.RevokedAt,
	}
}

func toLoginEvent(row userLoginEventPO) domain.LoginEvent {
	event := domain.LoginEvent{
		ID:            row.ID,
		UserID:        row.UserID,
		IPAddress:     row.IPAddress,
		UserAgent:     row.UserAgent,
		Success:       row.Success,
		FailureReason: row.FailureReason,
		CreatedAt:     row.CreatedAt,
	}
	if row.SessionID != nil {
		event.SessionID = *row.SessionID
	}
	return event
}
