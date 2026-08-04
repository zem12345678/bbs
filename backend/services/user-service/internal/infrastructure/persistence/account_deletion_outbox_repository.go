package persistence

import (
	"context"
	"strings"
	"time"

	domain "user-service/internal/domain/user"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type accountDeletionOutboxPO struct {
	EventID        string `gorm:"type:uuid;primaryKey"`
	JobID          int64  `gorm:"not null;uniqueIndex"`
	AggregateID    int64  `gorm:"not null"`
	EventType      string `gorm:"size:80;not null"`
	MessageKey     string `gorm:"type:text;not null"`
	PayloadJSON    string `gorm:"type:jsonb;not null"`
	Status         string `gorm:"size:20;not null;index"`
	Attempts       int    `gorm:"not null"`
	AvailableAt    time.Time
	LeaseOwner     *string
	LeaseExpiresAt *time.Time
	LastError      string
	OccurredAt     time.Time
	CreatedAt      time.Time
	UpdatedAt      time.Time
	PublishedAt    *time.Time
}

func (accountDeletionOutboxPO) TableName() string { return "user_account_deletion_outbox" }

func (r *Repo) ClaimAccountDeletionOutboxEvents(ctx context.Context, leaseOwner string, limit int, now, leaseUntil time.Time) ([]domain.AccountDeletionOutboxEvent, error) {
	leaseOwner = strings.TrimSpace(leaseOwner)
	if leaseOwner == "" || limit <= 0 || now.IsZero() || !leaseUntil.After(now) {
		return nil, domain.ErrAccountDeletionOutboxLeaseLost
	}
	var claimed []domain.AccountDeletionOutboxEvent
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var rows []accountDeletionOutboxPO
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE", Options: "SKIP LOCKED"}).
			Where(`(
			  status IN ? AND available_at <= ?
			) OR (
			  status = ? AND lease_expires_at <= ?
			)`, []string{"pending", "failed"}, now, "publishing", now).
			Order("available_at ASC, created_at ASC, event_id ASC").
			Limit(limit).
			Find(&rows).Error; err != nil {
			return err
		}
		if len(rows) == 0 {
			return nil
		}
		ids := make([]string, 0, len(rows))
		for _, row := range rows {
			ids = append(ids, row.EventID)
		}
		result := tx.Model(&accountDeletionOutboxPO{}).
			Where("event_id IN ?", ids).
			Updates(map[string]any{
				"status": "publishing", "attempts": gorm.Expr("attempts + 1"),
				"lease_owner": leaseOwner, "lease_expires_at": leaseUntil,
				"last_error": "", "updated_at": now,
			})
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected != int64(len(rows)) {
			return domain.ErrAccountDeletionOutboxLeaseLost
		}
		claimed = make([]domain.AccountDeletionOutboxEvent, 0, len(rows))
		for _, row := range rows {
			claimed = append(claimed, accountDeletionOutboxFromPO(row, row.Attempts+1))
		}
		return nil
	})
	return claimed, err
}

func (r *Repo) MarkAccountDeletionOutboxFailed(ctx context.Context, eventID, leaseOwner, lastError string, failedAt, retryAt time.Time) error {
	eventID = strings.TrimSpace(eventID)
	leaseOwner = strings.TrimSpace(leaseOwner)
	lastError = truncateLifecycleError(lastError)
	if eventID == "" || leaseOwner == "" || lastError == "" || failedAt.IsZero() || retryAt.Before(failedAt) {
		return domain.ErrAccountDeletionOutboxLeaseLost
	}
	result := r.db.WithContext(ctx).Model(&accountDeletionOutboxPO{}).
		Where("event_id = ? AND status = ? AND lease_owner = ?", eventID, "publishing", leaseOwner).
		Updates(map[string]any{
			"status": "failed", "available_at": retryAt, "lease_owner": nil,
			"lease_expires_at": nil, "last_error": lastError, "updated_at": failedAt,
		})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected != 1 {
		return domain.ErrAccountDeletionOutboxLeaseLost
	}
	return nil
}

func (r *Repo) MarkAccountDeletionOutboxPublished(ctx context.Context, eventID, leaseOwner string, publishedAt time.Time) error {
	eventID = strings.TrimSpace(eventID)
	leaseOwner = strings.TrimSpace(leaseOwner)
	if eventID == "" || leaseOwner == "" || publishedAt.IsZero() {
		return domain.ErrAccountDeletionOutboxLeaseLost
	}
	result := r.db.WithContext(ctx).Model(&accountDeletionOutboxPO{}).
		Where("event_id = ? AND status = ? AND lease_owner = ?", eventID, "publishing", leaseOwner).
		Updates(map[string]any{
			"status": "published", "lease_owner": nil, "lease_expires_at": nil,
			"last_error": "", "published_at": publishedAt, "updated_at": publishedAt,
		})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected != 1 {
		return domain.ErrAccountDeletionOutboxLeaseLost
	}
	return nil
}

func accountDeletionOutboxFromPO(row accountDeletionOutboxPO, attempt int) domain.AccountDeletionOutboxEvent {
	return domain.AccountDeletionOutboxEvent{
		EventID: row.EventID, JobID: row.JobID, AggregateID: row.AggregateID,
		EventType: row.EventType, MessageKey: row.MessageKey, Payload: []byte(row.PayloadJSON),
		Attempt: attempt, OccurredAt: row.OccurredAt,
	}
}

var _ domain.AccountDeletionOutboxRepository = (*Repo)(nil)
