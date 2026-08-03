package persistence

import (
	"context"
	"errors"
	"strings"
	"time"

	articleDomain "content-service/internal/domain/article"
	outboxDomain "content-service/internal/domain/outbox"
	topicDomain "content-service/internal/domain/topic"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type contentLifecycleOutboxPO struct {
	EventID        string     `gorm:"primaryKey;size:160"`
	MessageKey     string     `gorm:"size:64;not null"`
	EventType      string     `gorm:"size:80;not null"`
	Payload        string     `gorm:"type:jsonb;not null"`
	Status         string     `gorm:"size:16;not null;index"`
	Attempts       int        `gorm:"not null;default:0"`
	LeaseOwner     string     `gorm:"size:160;not null;default:''"`
	LeaseExpiresAt *time.Time `gorm:"index"`
	LastError      string     `gorm:"type:text;not null;default:''"`
	NextAttemptAt  *time.Time `gorm:"index"`
	PublishedAt    *time.Time `gorm:"index"`
	CreatedAt      time.Time  `gorm:"index"`
	UpdatedAt      time.Time
}

func (contentLifecycleOutboxPO) TableName() string {
	return "content_lifecycle_outbox"
}

type ContentLifecycleOutboxRepo struct {
	db *gorm.DB
}

func NewContentLifecycleOutboxRepo(db *gorm.DB) *ContentLifecycleOutboxRepo {
	return &ContentLifecycleOutboxRepo{db: db}
}

// UpdateStatusWithOutbox writes the visibility state and its compensating
// search/feed removal event in one transaction.
func (r *Repo) UpdateStatusWithOutbox(ctx context.Context, id int64, status articleDomain.Status, publishedAt *time.Time, updatedAt time.Time, event outboxDomain.LifecycleEvent) error {
	if updatedAt.IsZero() {
		updatedAt = time.Now()
	}
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		updates := map[string]any{
			"status":     int32(status),
			"updated_at": updatedAt,
		}
		if publishedAt != nil {
			updates["published_at"] = publishedAt
		}
		res := tx.Model(&articlePO{}).
			Where("id = ?", id).
			Where("NOT EXISTS (SELECT 1 FROM content_erased_users WHERE user_id = articles.author_id)").
			Updates(updates)
		if res.Error != nil {
			return res.Error
		}
		if res.RowsAffected == 0 {
			return articleDomain.ErrNotFound
		}
		return insertContentLifecycleOutboxEvent(tx, event)
	})
}

// UpdateStatusWithOutbox writes a topic visibility state and its lifecycle
// event atomically, including the intermediate QA ARCHIVING state.
func (r *TopicRepo) UpdateStatusWithOutbox(ctx context.Context, id int64, status topicDomain.Status, publishedAt *time.Time, updatedAt time.Time, event outboxDomain.LifecycleEvent) error {
	if updatedAt.IsZero() {
		updatedAt = time.Now()
	}
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		updates := map[string]any{
			"status":     int32(status),
			"updated_at": updatedAt,
		}
		if publishedAt != nil {
			updates["published_at"] = publishedAt
		}
		res := tx.Model(&topicPO{}).
			Where("id = ?", id).
			Where("NOT EXISTS (SELECT 1 FROM content_erased_users WHERE user_id = topics.author_id)").
			Updates(updates)
		if res.Error != nil {
			return res.Error
		}
		if res.RowsAffected == 0 {
			return topicDomain.ErrNotFound
		}
		return insertContentLifecycleOutboxEvent(tx, event)
	})
}

func insertContentLifecycleOutboxEvent(tx *gorm.DB, event outboxDomain.LifecycleEvent) error {
	event.EventID = strings.TrimSpace(event.EventID)
	event.MessageKey = strings.TrimSpace(event.MessageKey)
	event.EventType = strings.TrimSpace(event.EventType)
	if event.EventID == "" || event.MessageKey == "" || event.EventType == "" || len(event.Payload) == 0 {
		return errors.New("invalid content lifecycle outbox event")
	}
	now := time.Now().UTC()
	return tx.Clauses(clause.OnConflict{Columns: []clause.Column{{Name: "event_id"}}, DoNothing: true}).Create(&contentLifecycleOutboxPO{
		EventID:    event.EventID,
		MessageKey: event.MessageKey,
		EventType:  event.EventType,
		Payload:    string(event.Payload),
		Status:     "PENDING",
		CreatedAt:  now,
		UpdatedAt:  now,
	}).Error
}

func (r *ContentLifecycleOutboxRepo) ClaimPendingLifecycleEvents(ctx context.Context, owner string, limit int, leaseDuration time.Duration) ([]outboxDomain.LifecycleEvent, error) {
	if r == nil || r.db == nil {
		return nil, errors.New("content lifecycle outbox repository is unavailable")
	}
	owner = lifecycleOwner(owner)
	limit = lifecycleLimit(limit)
	leaseExpiresAt := time.Now().UTC().Add(lifecycleLeaseDuration(leaseDuration))
	rows := make([]contentLifecycleOutboxPO, 0, limit)
	err := r.db.WithContext(ctx).Raw(`
WITH candidates AS (
  SELECT event_id
  FROM content_lifecycle_outbox AS candidate
  WHERE (
      candidate.status = 'PENDING'
      OR (candidate.status = 'FAILED' AND (candidate.next_attempt_at IS NULL OR candidate.next_attempt_at <= NOW()))
      OR (candidate.status = 'PUBLISHING' AND candidate.lease_expires_at <= NOW())
  )
    -- A later visibility transition for the same aggregate must wait for its
    -- predecessor. This preserves Kafka order after a failed hide/archive is
    -- retried following a re-publish.
    AND NOT EXISTS (
      SELECT 1
      FROM content_lifecycle_outbox AS prior
      WHERE prior.message_key = candidate.message_key
        AND prior.status <> 'PUBLISHED'
        AND prior.event_sequence < candidate.event_sequence
    )
  ORDER BY candidate.event_sequence ASC
  LIMIT $3
  FOR UPDATE SKIP LOCKED
)
UPDATE content_lifecycle_outbox AS outbox
SET status = 'PUBLISHING',
    attempts = outbox.attempts + 1,
    lease_owner = $1,
    lease_expires_at = $2,
    last_error = '',
    updated_at = NOW()
FROM candidates
WHERE outbox.event_id = candidates.event_id
RETURNING outbox.event_id, outbox.message_key, outbox.event_type, outbox.payload, outbox.attempts`, owner, leaseExpiresAt, limit).Scan(&rows).Error
	if err != nil {
		return nil, err
	}
	return lifecycleEvents(rows), nil
}

func (r *ContentLifecycleOutboxRepo) ClaimLifecycleEvent(ctx context.Context, eventID, owner string, leaseDuration time.Duration) (*outboxDomain.LifecycleEvent, error) {
	if r == nil || r.db == nil {
		return nil, errors.New("content lifecycle outbox repository is unavailable")
	}
	eventID = strings.TrimSpace(eventID)
	if eventID == "" {
		return nil, nil
	}
	owner = lifecycleOwner(owner)
	leaseExpiresAt := time.Now().UTC().Add(lifecycleLeaseDuration(leaseDuration))
	rows := make([]contentLifecycleOutboxPO, 0, 1)
	err := r.db.WithContext(ctx).Raw(`
WITH candidate AS (
  SELECT event_id
  FROM content_lifecycle_outbox AS candidate
  WHERE candidate.event_id = $1
    AND (
      candidate.status = 'PENDING'
      OR (candidate.status = 'FAILED' AND (candidate.next_attempt_at IS NULL OR candidate.next_attempt_at <= NOW()))
      OR (candidate.status = 'PUBLISHING' AND candidate.lease_expires_at <= NOW())
    )
    AND NOT EXISTS (
      SELECT 1
      FROM content_lifecycle_outbox AS prior
      WHERE prior.message_key = candidate.message_key
        AND prior.status <> 'PUBLISHED'
        AND prior.event_sequence < candidate.event_sequence
    )
  FOR UPDATE SKIP LOCKED
)
UPDATE content_lifecycle_outbox AS outbox
SET status = 'PUBLISHING',
    attempts = outbox.attempts + 1,
    lease_owner = $2,
    lease_expires_at = $3,
    last_error = '',
    updated_at = NOW()
FROM candidate
WHERE outbox.event_id = candidate.event_id
RETURNING outbox.event_id, outbox.message_key, outbox.event_type, outbox.payload, outbox.attempts`, eventID, owner, leaseExpiresAt).Scan(&rows).Error
	if err != nil {
		return nil, err
	}
	if len(rows) == 0 {
		return nil, nil
	}
	events := lifecycleEvents(rows)
	return &events[0], nil
}

func (r *ContentLifecycleOutboxRepo) MarkLifecycleEventPublished(ctx context.Context, eventID, owner string, attempt int) error {
	now := time.Now().UTC()
	res := r.db.WithContext(ctx).Model(&contentLifecycleOutboxPO{}).
		Where("event_id = ? AND status = ? AND lease_owner = ? AND attempts = ?", eventID, "PUBLISHING", owner, attempt).
		Updates(map[string]any{
			"status":           "PUBLISHED",
			"lease_owner":      "",
			"lease_expires_at": nil,
			"last_error":       "",
			"next_attempt_at":  nil,
			"published_at":     now,
			"updated_at":       now,
		})
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return outboxDomain.ErrLeaseLost
	}
	return nil
}

func (r *ContentLifecycleOutboxRepo) MarkLifecycleEventFailed(ctx context.Context, eventID, owner string, attempt int, message string, nextAttemptAt time.Time) error {
	if nextAttemptAt.IsZero() {
		nextAttemptAt = time.Now().UTC()
	}
	res := r.db.WithContext(ctx).Model(&contentLifecycleOutboxPO{}).
		Where("event_id = ? AND status = ? AND lease_owner = ? AND attempts = ?", eventID, "PUBLISHING", owner, attempt).
		Updates(map[string]any{
			"status":           "FAILED",
			"lease_owner":      "",
			"lease_expires_at": nil,
			"last_error":       strings.TrimSpace(message),
			"next_attempt_at":  nextAttemptAt.UTC(),
			"updated_at":       time.Now().UTC(),
		})
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return outboxDomain.ErrLeaseLost
	}
	return nil
}

func lifecycleEvents(rows []contentLifecycleOutboxPO) []outboxDomain.LifecycleEvent {
	events := make([]outboxDomain.LifecycleEvent, 0, len(rows))
	for _, row := range rows {
		events = append(events, outboxDomain.LifecycleEvent{
			EventID:    row.EventID,
			MessageKey: row.MessageKey,
			EventType:  row.EventType,
			Payload:    []byte(row.Payload),
			Attempt:    row.Attempts,
		})
	}
	return events
}

func lifecycleOwner(owner string) string {
	owner = strings.TrimSpace(owner)
	if owner == "" {
		return "content-service"
	}
	return owner
}

func lifecycleLimit(limit int) int {
	if limit <= 0 {
		return 20
	}
	if limit > 100 {
		return 100
	}
	return limit
}

func lifecycleLeaseDuration(duration time.Duration) time.Duration {
	if duration <= 0 {
		return 30 * time.Second
	}
	return duration
}

var _ outboxDomain.LifecycleRepository = (*ContentLifecycleOutboxRepo)(nil)
