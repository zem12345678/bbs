package persistence

import (
	"context"
	"errors"
	"strings"
	"time"

	domain "notification-service/internal/domain/notification"

	"github.com/jackc/pgx/v5"
)

type webhookScanner interface {
	Scan(dest ...any) error
}

func (r *PostgresRepository) CreateWebhook(ctx context.Context, item domain.Webhook, maxPerUser int) (domain.Webhook, error) {
	if err := item.Validate(); err != nil {
		return domain.Webhook{}, err
	}
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return domain.Webhook{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if _, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtextextended('bbs-notification-user:' || $1::BIGINT::TEXT, 0))`, item.UserID); err != nil {
		return domain.Webhook{}, err
	}
	var count int
	if err := tx.QueryRow(ctx, `SELECT COUNT(*) FROM notification_webhooks WHERE user_id = $1`, item.UserID).Scan(&count); err != nil {
		return domain.Webhook{}, err
	}
	if maxPerUser > 0 && count >= maxPerUser {
		return domain.Webhook{}, domain.ErrWebhookLimitReached
	}
	created := time.Now().UTC()
	var result domain.Webhook
	err = scanWebhook(tx.QueryRow(ctx, `
	INSERT INTO notification_webhooks(user_id, name, url, secret, events, active, created_at, updated_at)
	SELECT $1, $2, $3, $4, $5, TRUE, $6, $6
	WHERE NOT EXISTS (SELECT 1 FROM notification_erased_users WHERE user_id = $1)
	RETURNING id, user_id, name, url, secret, events, active, latest_sent_at, latest_status, created_at, updated_at
`, item.UserID, item.Name, item.URL, item.Secret, item.Events, created), &result)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.Webhook{}, domain.ErrInvalidWebhook
	}
	if err != nil {
		return domain.Webhook{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return domain.Webhook{}, err
	}
	return result, nil
}

func (r *PostgresRepository) ListWebhooks(ctx context.Context, userID int64) ([]domain.Webhook, error) {
	if userID <= 0 {
		return nil, domain.ErrInvalidWebhook
	}
	rows, err := r.pool.Query(ctx, `
SELECT id, user_id, name, url, secret, events, active, latest_sent_at, latest_status, created_at, updated_at
FROM notification_webhooks WHERE user_id = $1 ORDER BY created_at DESC, id DESC
`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := make([]domain.Webhook, 0)
	for rows.Next() {
		var item domain.Webhook
		if err := scanWebhook(rows, &item); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (r *PostgresRepository) GetWebhook(ctx context.Context, userID, webhookID int64) (domain.Webhook, error) {
	if userID <= 0 || webhookID <= 0 {
		return domain.Webhook{}, domain.ErrInvalidWebhook
	}
	var item domain.Webhook
	err := scanWebhook(r.pool.QueryRow(ctx, `
SELECT id, user_id, name, url, secret, events, active, latest_sent_at, latest_status, created_at, updated_at
FROM notification_webhooks WHERE user_id = $1 AND id = $2
`, userID, webhookID), &item)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.Webhook{}, domain.ErrWebhookNotFound
	}
	if err != nil {
		return domain.Webhook{}, err
	}
	return item, nil
}

func (r *PostgresRepository) UpdateWebhook(ctx context.Context, item domain.Webhook) (domain.Webhook, error) {
	if err := item.Validate(); err != nil {
		return domain.Webhook{}, err
	}
	var result domain.Webhook
	err := scanWebhook(r.pool.QueryRow(ctx, `
UPDATE notification_webhooks
SET name = $3, url = $4, secret = $5, events = $6, active = $7, updated_at = NOW()
WHERE user_id = $1 AND id = $2
RETURNING id, user_id, name, url, secret, events, active, latest_sent_at, latest_status, created_at, updated_at
`, item.UserID, item.ID, item.Name, item.URL, item.Secret, item.Events, item.Active), &result)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.Webhook{}, domain.ErrWebhookNotFound
	}
	if err != nil {
		return domain.Webhook{}, err
	}
	return result, nil
}

func (r *PostgresRepository) DeleteWebhook(ctx context.Context, userID, webhookID int64) error {
	if userID <= 0 || webhookID <= 0 {
		return domain.ErrInvalidWebhook
	}
	result, err := r.pool.Exec(ctx, `DELETE FROM notification_webhooks WHERE user_id = $1 AND id = $2`, userID, webhookID)
	if err != nil {
		return err
	}
	if result.RowsAffected() == 0 {
		return domain.ErrWebhookNotFound
	}
	return nil
}

func (r *PostgresRepository) EnqueueWebhookEvent(ctx context.Context, userID int64, eventType, eventID string, payload []byte, createdAt time.Time) error {
	eventType = strings.TrimSpace(eventType)
	eventID = strings.TrimSpace(eventID)
	if userID <= 0 || !domain.IsWebhookEventType(eventType) || eventID == "" || len(eventID) > domain.WebhookMaxEventIDBytes || len(payload) == 0 || len(payload) > domain.WebhookMaxPayloadBytes {
		return domain.ErrInvalidWebhook
	}
	if createdAt.IsZero() {
		createdAt = time.Now().UTC()
	}
	_, err := r.pool.Exec(ctx, `
INSERT INTO notification_webhook_outbox(webhook_id, user_id, event_id, event_type, url, secret, payload, created_at, updated_at)
SELECT id, $1::BIGINT, $3::VARCHAR(128), $2::TEXT, url, secret, $4::JSONB, $5, $5
FROM notification_webhooks
WHERE user_id = $1::BIGINT AND active AND $2::TEXT = ANY(events)
  AND NOT EXISTS (SELECT 1 FROM notification_erased_users erased WHERE erased.user_id = $1::BIGINT)
ON CONFLICT(webhook_id, event_id, event_type) DO NOTHING
`, userID, eventType, eventID, payload, createdAt)
	return err
}

func (r *PostgresRepository) EnqueueWebhookTest(ctx context.Context, item domain.Webhook, eventType, eventID string, payload []byte, createdAt time.Time) error {
	eventType = strings.TrimSpace(eventType)
	eventID = strings.TrimSpace(eventID)
	if item.ID <= 0 || item.UserID <= 0 || !domain.IsWebhookEventType(eventType) || eventID == "" || len(eventID) > domain.WebhookMaxEventIDBytes || len(payload) == 0 || len(payload) > domain.WebhookMaxPayloadBytes {
		return domain.ErrInvalidWebhook
	}
	if createdAt.IsZero() {
		createdAt = time.Now().UTC()
	}
	_, err := r.pool.Exec(ctx, `
INSERT INTO notification_webhook_outbox(webhook_id, user_id, event_id, event_type, url, secret, payload, created_at, updated_at)
	SELECT $1, $2, $3, $4, $5, $6, $7::JSONB, $8, $8
	WHERE EXISTS (SELECT 1 FROM notification_webhooks WHERE id = $1 AND user_id = $2)
  AND NOT EXISTS (SELECT 1 FROM notification_erased_users erased WHERE erased.user_id = $2)
	ON CONFLICT(webhook_id, event_id, event_type) DO NOTHING
`, item.ID, item.UserID, eventID, eventType, item.URL, item.Secret, payload, createdAt)
	return err
}

func (r *PostgresRepository) ClaimWebhookDeliveries(ctx context.Context, limit int, now time.Time, lockTimeout time.Duration) ([]domain.WebhookDelivery, error) {
	if limit <= 0 {
		return nil, nil
	}
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	claimAt := now.UTC().Truncate(time.Microsecond)
	rows, err := tx.Query(ctx, `
SELECT id, webhook_id, user_id, event_id, event_type, url, secret, payload, attempt_count, created_at
FROM notification_webhook_outbox
WHERE completed_at IS NULL AND available_at <= $1
  AND (locked_at IS NULL OR locked_at < $2)
ORDER BY id
FOR UPDATE SKIP LOCKED
LIMIT $3
	`, claimAt, claimAt.Add(-lockTimeout), limit)
	if err != nil {
		return nil, err
	}
	deliveries := make([]domain.WebhookDelivery, 0, limit)
	ids := make([]int64, 0, limit)
	for rows.Next() {
		var item domain.WebhookDelivery
		if err := rows.Scan(&item.ID, &item.WebhookID, &item.UserID, &item.EventID, &item.EventType, &item.URL, &item.Secret, &item.Payload, &item.AttemptCount, &item.CreatedAt); err != nil {
			rows.Close()
			return nil, err
		}
		item.LockedAt = claimAt
		deliveries = append(deliveries, item)
		ids = append(ids, item.ID)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if len(ids) > 0 {
		if _, err := tx.Exec(ctx, `UPDATE notification_webhook_outbox SET locked_at = $1, updated_at = $1 WHERE id = ANY($2::BIGINT[])`, claimAt, ids); err != nil {
			return nil, err
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	return deliveries, nil
}

func (r *PostgresRepository) ReleaseWebhookDeliveries(ctx context.Context, deliveries []domain.WebhookDelivery) error {
	if len(deliveries) == 0 {
		return nil
	}
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	for _, delivery := range deliveries {
		if delivery.ID <= 0 || delivery.LockedAt.IsZero() {
			continue
		}
		if _, err := tx.Exec(ctx, `
UPDATE notification_webhook_outbox
SET locked_at = NULL, updated_at = NOW()
WHERE id = $1 AND completed_at IS NULL AND locked_at = $2
`, delivery.ID, delivery.LockedAt); err != nil {
			return err
		}
	}
	return tx.Commit(ctx)
}

func (r *PostgresRepository) IsWebhookDeliveryActive(ctx context.Context, delivery domain.WebhookDelivery) (bool, error) {
	if delivery.ID <= 0 || delivery.WebhookID <= 0 || delivery.UserID <= 0 || delivery.LockedAt.IsZero() {
		return false, nil
	}
	var active bool
	err := r.pool.QueryRow(ctx, `
SELECT EXISTS (
  SELECT 1
  FROM notification_webhook_outbox outbox
  JOIN notification_webhooks webhooks ON webhooks.id = outbox.webhook_id
  WHERE outbox.id = $1
    AND outbox.webhook_id = $2
    AND outbox.user_id = $3
    AND outbox.completed_at IS NULL
    AND outbox.locked_at = $4
    AND webhooks.active
    AND NOT EXISTS (
      SELECT 1 FROM notification_erased_users erased WHERE erased.user_id = outbox.user_id
    )
)
`, delivery.ID, delivery.WebhookID, delivery.UserID, delivery.LockedAt).Scan(&active)
	return active, err
}

func (r *PostgresRepository) CompleteWebhookDelivery(ctx context.Context, delivery domain.WebhookDelivery, statusCode int32, sentAt time.Time) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	result, err := tx.Exec(ctx, `
UPDATE notification_webhook_outbox SET completed_at = $2, locked_at = NULL, result = 'delivered', last_status = $3, updated_at = $2
	WHERE id = $1 AND completed_at IS NULL AND locked_at = $4
	`, delivery.ID, sentAt, statusCode, delivery.LockedAt)
	if err != nil {
		return err
	}
	if result.RowsAffected() > 0 {
		if _, err := tx.Exec(ctx, `
UPDATE notification_webhooks
SET latest_sent_at = $2, latest_status = $3, updated_at = $2
WHERE id = $1 AND (latest_sent_at IS NULL OR latest_sent_at <= $2)
`, delivery.WebhookID, sentAt, statusCode); err != nil {
			return err
		}
	}
	return tx.Commit(ctx)
}

func (r *PostgresRepository) RetryWebhookDelivery(ctx context.Context, delivery domain.WebhookDelivery, statusCode int32, attemptCount int32, nextAttempt time.Time, message string, exhausted bool, sentAt time.Time) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	var rowsAffected int64
	if exhausted {
		result, execErr := tx.Exec(ctx, `
UPDATE notification_webhook_outbox
SET attempt_count = $2, available_at = $3, locked_at = NULL, completed_at = $4,
    result = 'failed', last_status = $5, last_error = $6, updated_at = $4
		WHERE id = $1 AND completed_at IS NULL AND locked_at = $7
		`, delivery.ID, attemptCount, nextAttempt, sentAt, statusCode, message, delivery.LockedAt)
		err = execErr
		rowsAffected = result.RowsAffected()
	} else {
		result, execErr := tx.Exec(ctx, `
UPDATE notification_webhook_outbox
SET attempt_count = $2, available_at = $3, locked_at = NULL,
    result = 'pending', last_status = $4, last_error = $5, updated_at = $6
		WHERE id = $1 AND completed_at IS NULL AND locked_at = $7
		`, delivery.ID, attemptCount, nextAttempt, statusCode, message, sentAt, delivery.LockedAt)
		err = execErr
		rowsAffected = result.RowsAffected()
	}
	if err != nil {
		return err
	}
	if rowsAffected > 0 {
		if _, err := tx.Exec(ctx, `
UPDATE notification_webhooks
SET latest_sent_at = $2, latest_status = $3, updated_at = $2
WHERE id = $1 AND (latest_sent_at IS NULL OR latest_sent_at <= $2)
`, delivery.WebhookID, sentAt, statusCode); err != nil {
			return err
		}
	}
	return tx.Commit(ctx)
}

func (r *PostgresRepository) CleanupCompletedWebhookDeliveries(ctx context.Context, before time.Time, limit int) (int64, error) {
	if limit <= 0 {
		return 0, nil
	}
	result, err := r.pool.Exec(ctx, `
DELETE FROM notification_webhook_outbox
WHERE id IN (
  SELECT id FROM notification_webhook_outbox
  WHERE completed_at IS NOT NULL AND completed_at < $1
  ORDER BY id LIMIT $2
)
`, before, limit)
	if err != nil {
		return 0, err
	}
	return result.RowsAffected(), nil
}

func scanWebhook(row webhookScanner, item *domain.Webhook) error {
	return row.Scan(&item.ID, &item.UserID, &item.Name, &item.URL, &item.Secret, &item.Events, &item.Active, &item.LatestSentAt, &item.LatestStatus, &item.CreatedAt, &item.UpdatedAt)
}

func (r *PostgresRepository) ensureWebhookSchema(ctx context.Context) error {
	_, err := r.pool.Exec(ctx, webhookSchemaSQL)
	return err
}

const webhookSchemaSQL = `
CREATE TABLE IF NOT EXISTS notification_webhooks (
  id BIGSERIAL PRIMARY KEY,
  user_id BIGINT NOT NULL,
  name VARCHAR(100) NOT NULL,
  url TEXT NOT NULL,
  secret TEXT NOT NULL DEFAULT '',
  events TEXT[] NOT NULL,
  active BOOLEAN NOT NULL DEFAULT TRUE,
  latest_sent_at TIMESTAMPTZ,
  latest_status INTEGER,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  CONSTRAINT chk_notification_webhooks_user_id CHECK (user_id > 0),
  CONSTRAINT chk_notification_webhooks_name CHECK (char_length(btrim(name)) BETWEEN 1 AND 100),
  CONSTRAINT chk_notification_webhooks_url CHECK (url <> '' AND octet_length(url) <= 1024),
  CONSTRAINT chk_notification_webhooks_secret CHECK (octet_length(secret) <= 1024),
  CONSTRAINT chk_notification_webhooks_events CHECK (
    cardinality(events) > 0 AND events <@ ARRAY['mention','unfollow','follow','followed','note','reply','renote','reaction','edited']::TEXT[]
  )
);

ALTER TABLE notification_webhooks
  DROP CONSTRAINT IF EXISTS chk_notification_webhooks_url;

ALTER TABLE notification_webhooks
  ADD CONSTRAINT chk_notification_webhooks_url CHECK (url <> '' AND octet_length(url) <= 1024);

CREATE TABLE IF NOT EXISTS notification_webhook_outbox (
  id BIGSERIAL PRIMARY KEY,
  webhook_id BIGINT NOT NULL REFERENCES notification_webhooks(id) ON DELETE CASCADE,
  user_id BIGINT NOT NULL,
  event_id VARCHAR(128) NOT NULL,
  event_type VARCHAR(32) NOT NULL,
  url TEXT NOT NULL,
  secret TEXT NOT NULL DEFAULT '',
  payload JSONB NOT NULL,
  attempt_count INTEGER NOT NULL DEFAULT 0,
  available_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  locked_at TIMESTAMPTZ,
  completed_at TIMESTAMPTZ,
  result VARCHAR(32) NOT NULL DEFAULT 'pending',
  last_status INTEGER,
  last_error TEXT NOT NULL DEFAULT '',
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  CONSTRAINT chk_notification_webhook_outbox_user_id CHECK (user_id > 0),
  CONSTRAINT chk_notification_webhook_outbox_event_id CHECK (event_id <> ''),
  CONSTRAINT chk_notification_webhook_outbox_event_type CHECK (
    event_type IN ('mention','unfollow','follow','followed','note','reply','renote','reaction','edited')
  ),
  CONSTRAINT chk_notification_webhook_outbox_attempt_count CHECK (attempt_count >= 0),
  CONSTRAINT chk_notification_webhook_outbox_result CHECK (result IN ('pending','delivered','failed')),
  UNIQUE(webhook_id, event_id, event_type)
);

CREATE INDEX IF NOT EXISTS idx_notification_webhooks_user
  ON notification_webhooks(user_id, id);

CREATE INDEX IF NOT EXISTS idx_notification_webhook_outbox_pending
  ON notification_webhook_outbox(available_at, id)
  WHERE completed_at IS NULL;

CREATE INDEX IF NOT EXISTS idx_notification_webhook_outbox_completed
  ON notification_webhook_outbox(completed_at, id)
  WHERE completed_at IS NOT NULL;

CREATE OR REPLACE FUNCTION enqueue_notification_webhooks()
RETURNS TRIGGER AS $$
DECLARE
  webhook_event_type TEXT;
BEGIN
  webhook_event_type := CASE NEW.type
    WHEN 'comment' THEN 'reply'
    WHEN 'reply' THEN 'reply'
    WHEN 'like' THEN 'reaction'
    WHEN 'favorite' THEN 'reaction'
    ELSE NULL
  END;
  IF webhook_event_type IS NULL THEN
    RETURN NEW;
  END IF;
  INSERT INTO notification_webhook_outbox(
    webhook_id, user_id, event_id, event_type, url, secret, payload,
    available_at, created_at, updated_at
  )
  SELECT webhook.id, NEW.user_id, NEW.source_event_id, webhook_event_type, webhook.url, webhook.secret,
    jsonb_build_object('notification', jsonb_build_object(
      'type', NEW.type, 'title', NEW.title, 'content', NEW.content,
      'actorId', NEW.actor_id, 'entityType', NEW.entity_type,
      'entityId', NEW.entity_id, 'sourceId', NEW.source_id
    )), NEW.created_at, NEW.created_at, NEW.created_at
  FROM notification_webhooks webhook
  WHERE webhook.user_id = NEW.user_id
    AND webhook.active
    AND webhook_event_type = ANY(webhook.events)
    AND NOT EXISTS (SELECT 1 FROM notification_erased_users erased WHERE erased.user_id = NEW.user_id)
  ON CONFLICT(webhook_id, event_id, event_type) DO NOTHING;
  RETURN NEW;
END;
$$ LANGUAGE plpgsql;

DROP TRIGGER IF EXISTS notifications_enqueue_user_webhooks ON notifications;

CREATE TRIGGER notifications_enqueue_user_webhooks
AFTER INSERT ON notifications
FOR EACH ROW EXECUTE FUNCTION enqueue_notification_webhooks();

`

var _ domain.WebhookRepository = (*PostgresRepository)(nil)
var _ domain.WebhookOutboxRepository = (*PostgresRepository)(nil)
