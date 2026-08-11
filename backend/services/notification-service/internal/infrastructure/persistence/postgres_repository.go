package persistence

import (
	"context"
	"errors"
	"sort"
	"strconv"
	"time"

	domain "notification-service/internal/domain/notification"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type PostgresRepository struct {
	pool *pgxpool.Pool
}

func NewPostgresRepository(pool *pgxpool.Pool) *PostgresRepository {
	return &PostgresRepository{pool: pool}
}

func (r *PostgresRepository) EnsureSchema(ctx context.Context) error {
	_, err := r.pool.Exec(ctx, `
CREATE TABLE IF NOT EXISTS article_authors (
  article_id BIGINT PRIMARY KEY,
  author_id BIGINT NOT NULL,
  title TEXT NOT NULL DEFAULT '',
  published_at TIMESTAMPTZ,
  updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS content_refs (
  entity_type VARCHAR(32) NOT NULL,
  entity_id BIGINT NOT NULL,
  author_id BIGINT NOT NULL,
  title TEXT NOT NULL DEFAULT '',
  published_at TIMESTAMPTZ,
  updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  PRIMARY KEY(entity_type, entity_id)
);

CREATE TABLE IF NOT EXISTS comment_refs (
  comment_id BIGINT PRIMARY KEY,
  entity_type VARCHAR(32) NOT NULL,
  entity_id BIGINT NOT NULL,
  author_id BIGINT NOT NULL,
  parent_id BIGINT NOT NULL DEFAULT 0,
  created_at TIMESTAMPTZ,
  updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS notifications (
  id BIGSERIAL PRIMARY KEY,
  user_id BIGINT NOT NULL,
  type VARCHAR(64) NOT NULL,
  title TEXT NOT NULL,
  content TEXT NOT NULL DEFAULT '',
  actor_id BIGINT NOT NULL DEFAULT 0,
  entity_type VARCHAR(32) NOT NULL DEFAULT '',
  entity_id BIGINT NOT NULL DEFAULT 0,
  source_id BIGINT NOT NULL DEFAULT 0,
  source_event_id VARCHAR(128) NOT NULL,
  read_at TIMESTAMPTZ,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  UNIQUE (user_id, source_event_id)
);

CREATE TABLE IF NOT EXISTS notification_preferences (
  user_id BIGINT NOT NULL,
  notification_type VARCHAR(64) NOT NULL,
  enabled BOOLEAN NOT NULL DEFAULT TRUE,
  updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  PRIMARY KEY (user_id, notification_type),
  CONSTRAINT chk_notification_preferences_user_id CHECK (user_id > 0),
  CONSTRAINT chk_notification_preferences_type CHECK (notification_type <> '')
);

CREATE TABLE IF NOT EXISTS web_push_subscriptions (
  id BIGSERIAL PRIMARY KEY,
  user_id BIGINT NOT NULL,
  endpoint TEXT NOT NULL UNIQUE,
  auth TEXT NOT NULL,
  public_key TEXT NOT NULL,
  state VARCHAR(32) NOT NULL DEFAULT 'active',
  send_read_message BOOLEAN NOT NULL DEFAULT FALSE,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  CONSTRAINT chk_web_push_subscriptions_user_id CHECK (user_id > 0),
  CONSTRAINT chk_web_push_subscriptions_endpoint CHECK (endpoint <> '' AND octet_length(endpoint) <= 2048),
  CONSTRAINT chk_web_push_subscriptions_auth CHECK (auth <> '' AND octet_length(auth) <= 512),
  CONSTRAINT chk_web_push_subscriptions_public_key CHECK (public_key <> '' AND octet_length(public_key) <= 512),
  CONSTRAINT chk_web_push_subscriptions_state CHECK (state IN ('active'))
);

CREATE TABLE IF NOT EXISTS web_push_outbox (
  id BIGSERIAL PRIMARY KEY,
  notification_id BIGINT NOT NULL REFERENCES notifications(id) ON DELETE CASCADE,
  subscription_id BIGINT NOT NULL REFERENCES web_push_subscriptions(id) ON DELETE CASCADE,
  attempt_count INTEGER NOT NULL DEFAULT 0,
  available_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  locked_at TIMESTAMPTZ,
  completed_at TIMESTAMPTZ,
  result VARCHAR(32) NOT NULL DEFAULT 'pending',
  last_error TEXT NOT NULL DEFAULT '',
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  UNIQUE(notification_id, subscription_id),
  CONSTRAINT chk_web_push_outbox_attempt_count CHECK (attempt_count >= 0),
  CONSTRAINT chk_web_push_outbox_result CHECK (result IN ('pending', 'delivered', 'gone', 'failed'))
);

CREATE TABLE IF NOT EXISTS pending_article_notifications (
  event_id VARCHAR(128) PRIMARY KEY,
  type VARCHAR(64) NOT NULL,
  article_id BIGINT NOT NULL,
  actor_id BIGINT NOT NULL,
  source_id BIGINT NOT NULL DEFAULT 0,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS pending_content_notifications (
  event_id VARCHAR(128) PRIMARY KEY,
  type VARCHAR(64) NOT NULL,
  entity_type VARCHAR(32) NOT NULL,
  entity_id BIGINT NOT NULL,
  actor_id BIGINT NOT NULL,
  source_id BIGINT NOT NULL DEFAULT 0,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS pending_reply_notifications (
  event_id VARCHAR(128) PRIMARY KEY,
  parent_comment_id BIGINT NOT NULL,
  comment_id BIGINT NOT NULL,
  entity_type VARCHAR(32) NOT NULL,
  entity_id BIGINT NOT NULL,
  actor_id BIGINT NOT NULL,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS notification_erased_users (
  user_id BIGINT PRIMARY KEY,
  job_id BIGINT NOT NULL,
  policy_version INTEGER NOT NULL,
  erased_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  CONSTRAINT chk_notification_erased_users_user_id CHECK (user_id > 0),
  CONSTRAINT chk_notification_erased_users_job_id CHECK (job_id > 0),
  CONSTRAINT chk_notification_erased_users_policy_version CHECK (policy_version > 0)
);

CREATE TABLE IF NOT EXISTS notification_erased_content (
  entity_type VARCHAR(32) NOT NULL,
  entity_id BIGINT NOT NULL,
  owner_user_id BIGINT NOT NULL,
  job_id BIGINT NOT NULL,
  policy_version INTEGER NOT NULL,
  erased_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  PRIMARY KEY(entity_type, entity_id),
  CONSTRAINT chk_notification_erased_content_entity_id CHECK (entity_id > 0),
  CONSTRAINT chk_notification_erased_content_owner_user_id CHECK (owner_user_id > 0),
  CONSTRAINT chk_notification_erased_content_job_id CHECK (job_id > 0),
  CONSTRAINT chk_notification_erased_content_policy_version CHECK (policy_version > 0)
);

CREATE TABLE IF NOT EXISTS notification_erased_comments (
  comment_id BIGINT PRIMARY KEY,
  author_user_id BIGINT NOT NULL,
  job_id BIGINT NOT NULL,
  policy_version INTEGER NOT NULL,
  erased_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  CONSTRAINT chk_notification_erased_comments_comment_id CHECK (comment_id > 0),
  CONSTRAINT chk_notification_erased_comments_author_user_id CHECK (author_user_id > 0),
  CONSTRAINT chk_notification_erased_comments_job_id CHECK (job_id > 0),
  CONSTRAINT chk_notification_erased_comments_policy_version CHECK (policy_version > 0)
);

ALTER TABLE notification_erased_users
  ADD COLUMN IF NOT EXISTS policy_version INTEGER NOT NULL DEFAULT 1;

CREATE INDEX IF NOT EXISTS idx_pending_article_notifications_article
  ON pending_article_notifications(article_id, created_at);

CREATE INDEX IF NOT EXISTS idx_pending_content_notifications_entity
  ON pending_content_notifications(entity_type, entity_id, created_at);

CREATE INDEX IF NOT EXISTS idx_comment_refs_entity
  ON comment_refs(entity_type, entity_id, created_at);

CREATE INDEX IF NOT EXISTS idx_pending_reply_notifications_parent
  ON pending_reply_notifications(parent_comment_id, created_at);

CREATE INDEX IF NOT EXISTS idx_notifications_user_created
  ON notifications(user_id, created_at DESC);

CREATE INDEX IF NOT EXISTS idx_notifications_user_unread
  ON notifications(user_id, created_at DESC)
  WHERE read_at IS NULL;

CREATE INDEX IF NOT EXISTS idx_notification_preferences_user
  ON notification_preferences(user_id);

CREATE INDEX IF NOT EXISTS idx_web_push_subscriptions_user
  ON web_push_subscriptions(user_id, id);

CREATE INDEX IF NOT EXISTS idx_web_push_outbox_pending
  ON web_push_outbox(available_at, id)
  WHERE completed_at IS NULL;

CREATE INDEX IF NOT EXISTS idx_web_push_outbox_completed
  ON web_push_outbox(completed_at, id)
  WHERE completed_at IS NOT NULL;

CREATE INDEX IF NOT EXISTS idx_notification_erased_users_job
  ON notification_erased_users(job_id);

CREATE INDEX IF NOT EXISTS idx_notification_erased_content_job
  ON notification_erased_content(job_id);

CREATE INDEX IF NOT EXISTS idx_notification_erased_comments_job
  ON notification_erased_comments(job_id);

CREATE OR REPLACE FUNCTION enqueue_web_push_notification()
RETURNS TRIGGER AS $$
BEGIN
  INSERT INTO web_push_outbox(notification_id, subscription_id)
  SELECT NEW.id, subscriptions.id
  FROM web_push_subscriptions subscriptions
  WHERE subscriptions.user_id = NEW.user_id
    AND subscriptions.state = 'active'
  ON CONFLICT(notification_id, subscription_id) DO NOTHING;
  RETURN NEW;
END;
$$ LANGUAGE plpgsql;

DROP TRIGGER IF EXISTS notifications_enqueue_web_push ON notifications;

CREATE TRIGGER notifications_enqueue_web_push
AFTER INSERT ON notifications
FOR EACH ROW EXECUTE FUNCTION enqueue_web_push_notification();
`)
	if err != nil {
		return err
	}
	return r.ensureWebhookSchema(ctx)
}

func (r *PostgresRepository) EraseUserData(ctx context.Context, userID, deletionJobID int64, policyVersion int32) error {
	return r.withUserLocks(ctx, []int64{userID}, func(tx pgx.Tx) error {
		if err := lockUserErasureSubjects(ctx, tx, userID); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `
INSERT INTO notification_erased_users(user_id, job_id, policy_version)
VALUES($1, $2, $3)
ON CONFLICT(user_id) DO UPDATE SET
  job_id = EXCLUDED.job_id,
  policy_version = EXCLUDED.policy_version,
  erased_at = NOW()
WHERE EXCLUDED.policy_version > notification_erased_users.policy_version
`, userID, deletionJobID, policyVersion); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `
INSERT INTO notification_erased_content(entity_type, entity_id, owner_user_id, job_id, policy_version)
SELECT entity_type, entity_id, author_id, $2, $3
FROM content_refs
WHERE author_id = $1
ON CONFLICT(entity_type, entity_id) DO UPDATE SET
  owner_user_id = EXCLUDED.owner_user_id,
  job_id = EXCLUDED.job_id,
  policy_version = EXCLUDED.policy_version,
  erased_at = NOW()
WHERE EXCLUDED.policy_version > notification_erased_content.policy_version
`, userID, deletionJobID, policyVersion); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `
INSERT INTO notification_erased_content(entity_type, entity_id, owner_user_id, job_id, policy_version)
SELECT 'article', article_id, author_id, $2, $3
FROM article_authors
WHERE author_id = $1
ON CONFLICT(entity_type, entity_id) DO UPDATE SET
  owner_user_id = EXCLUDED.owner_user_id,
  job_id = EXCLUDED.job_id,
  policy_version = EXCLUDED.policy_version,
  erased_at = NOW()
WHERE EXCLUDED.policy_version > notification_erased_content.policy_version
`, userID, deletionJobID, policyVersion); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `
INSERT INTO notification_erased_comments(comment_id, author_user_id, job_id, policy_version)
SELECT comment_id, author_id, $2, $3
FROM comment_refs
WHERE author_id = $1
ON CONFLICT(comment_id) DO UPDATE SET
  author_user_id = EXCLUDED.author_user_id,
  job_id = EXCLUDED.job_id,
  policy_version = EXCLUDED.policy_version,
  erased_at = NOW()
WHERE EXCLUDED.policy_version > notification_erased_comments.policy_version
`, userID, deletionJobID, policyVersion); err != nil {
			return err
		}

		statements := []string{
			`DELETE FROM notification_webhook_outbox WHERE user_id = $1`,
			`DELETE FROM notification_webhooks WHERE user_id = $1`,
			`DELETE FROM web_push_outbox
WHERE subscription_id IN (SELECT id FROM web_push_subscriptions WHERE user_id = $1)
   OR notification_id IN (SELECT id FROM notifications WHERE user_id = $1 OR actor_id = $1)`,
			`DELETE FROM web_push_subscriptions WHERE user_id = $1`,
			`DELETE FROM notifications WHERE user_id = $1 OR actor_id = $1`,
			`DELETE FROM notification_preferences WHERE user_id = $1`,
			`DELETE FROM pending_article_notifications p
WHERE p.actor_id = $1 OR EXISTS (
  SELECT 1 FROM article_authors a WHERE a.article_id = p.article_id AND a.author_id = $1
)`,
			`DELETE FROM pending_content_notifications p
WHERE p.actor_id = $1 OR EXISTS (
  SELECT 1 FROM content_refs c
  WHERE c.entity_type = p.entity_type AND c.entity_id = p.entity_id AND c.author_id = $1
)`,
			`DELETE FROM pending_reply_notifications p
WHERE p.actor_id = $1
   OR EXISTS (SELECT 1 FROM comment_refs c WHERE c.comment_id = p.parent_comment_id AND c.author_id = $1)
   OR EXISTS (SELECT 1 FROM comment_refs c WHERE c.comment_id = p.comment_id AND c.author_id = $1)
   OR EXISTS (
     SELECT 1 FROM content_refs c
     WHERE c.entity_type = p.entity_type AND c.entity_id = p.entity_id AND c.author_id = $1
   )`,
			`DELETE FROM article_authors WHERE author_id = $1`,
			`DELETE FROM content_refs WHERE author_id = $1`,
			`DELETE FROM comment_refs WHERE author_id = $1`,
		}
		for _, statement := range statements {
			if _, err := tx.Exec(ctx, statement, userID); err != nil {
				return err
			}
		}
		return nil
	})
}

func (r *PostgresRepository) SaveContent(ctx context.Context, content domain.ContentRef, publishedAt time.Time) error {
	if content.ID <= 0 || content.AuthorID <= 0 || !supportedContentType(content.EntityType) {
		return nil
	}
	return r.withUserAndSubjectLocks(ctx, []int64{content.AuthorID}, []string{contentLockSubject(content.EntityType, content.ID)}, func(tx pgx.Tx) error {
		_, err := tx.Exec(ctx, `
INSERT INTO content_refs(entity_type, entity_id, author_id, title, published_at, updated_at)
SELECT $1::VARCHAR(32), $2, $3, $4::TEXT, $5, NOW()
WHERE NOT EXISTS (SELECT 1 FROM notification_erased_users WHERE user_id = $3)
  AND NOT EXISTS (
    SELECT 1 FROM notification_erased_content
    WHERE entity_type = $1 AND entity_id = $2
  )
ON CONFLICT(entity_type, entity_id) DO UPDATE SET
  author_id = EXCLUDED.author_id,
  title = EXCLUDED.title,
  published_at = EXCLUDED.published_at,
  updated_at = NOW()
`, content.EntityType, content.ID, content.AuthorID, content.Title, nullableTime(publishedAt))
		return err
	})
}

func (r *PostgresRepository) GetContent(ctx context.Context, entityType string, id int64) (domain.ContentRef, error) {
	if id <= 0 || !supportedContentType(entityType) {
		return domain.ContentRef{}, nil
	}
	var content domain.ContentRef
	err := r.pool.QueryRow(ctx, `
SELECT entity_type, entity_id, author_id, title
FROM content_refs
WHERE entity_type = $1 AND entity_id = $2
`, entityType, id).Scan(&content.EntityType, &content.ID, &content.AuthorID, &content.Title)
	if errors.Is(err, pgx.ErrNoRows) {
		if entityType == "article" {
			article, articleErr := r.GetArticle(ctx, id)
			if articleErr != nil || article.ID <= 0 {
				return domain.ContentRef{}, articleErr
			}
			return domain.ContentRef{EntityType: "article", ID: article.ID, AuthorID: article.AuthorID, Title: article.Title}, nil
		}
		return domain.ContentRef{}, nil
	}
	return content, err
}

func (r *PostgresRepository) SavePendingContentNotification(ctx context.Context, eventID, notificationType, entityType string, entityID, actorID, sourceID int64, createdAt time.Time) error {
	if eventID == "" || notificationType == "" || entityID <= 0 || actorID <= 0 || !supportedContentType(entityType) {
		return nil
	}
	if createdAt.IsZero() {
		createdAt = time.Now()
	}
	subjects := []string{contentLockSubject(entityType, entityID)}
	if notificationType == "comment" && sourceID > 0 {
		subjects = append(subjects, commentLockSubject(sourceID))
	}
	return r.withUserAndSubjectLocks(ctx, []int64{actorID}, subjects, func(tx pgx.Tx) error {
		_, err := tx.Exec(ctx, `
INSERT INTO pending_content_notifications(event_id, type, entity_type, entity_id, actor_id, source_id, created_at)
SELECT $1::VARCHAR(128), $2::VARCHAR(64), $3::VARCHAR(32), $4, $5, $6, $7
WHERE NOT EXISTS (SELECT 1 FROM notification_erased_users WHERE user_id = $5)
  AND NOT EXISTS (
    SELECT 1 FROM notification_erased_content
    WHERE entity_type = $3 AND entity_id = $4
  )
  AND (
    $2::VARCHAR(64) <> 'comment' OR $6::BIGINT = 0 OR NOT EXISTS (
      SELECT 1 FROM notification_erased_comments WHERE comment_id = $6
    )
  )
ON CONFLICT(event_id) DO NOTHING
`, eventID, notificationType, entityType, entityID, actorID, sourceID, createdAt)
		return err
	})
}

func (r *PostgresRepository) FlushPendingContentNotifications(ctx context.Context, content domain.ContentRef) error {
	if content.ID <= 0 || content.AuthorID <= 0 || !supportedContentType(content.EntityType) {
		return nil
	}
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	eventIDs, actorIDs, err := loadPendingSubjects(ctx, tx, `
SELECT event_id, actor_id
FROM pending_content_notifications
WHERE entity_type = $1 AND entity_id = $2
`, content.EntityType, content.ID)
	if err != nil {
		return err
	}
	if len(eventIDs) == 0 {
		return tx.Commit(ctx)
	}
	if err := lockNotificationUsers(ctx, tx, append(actorIDs, content.AuthorID)); err != nil {
		return err
	}
	if err := lockNotificationSubjects(ctx, tx, []string{contentLockSubject(content.EntityType, content.ID)}); err != nil {
		return err
	}

	_, err = tx.Exec(ctx, `
INSERT INTO notifications(user_id, type, title, content, actor_id, entity_type, entity_id, source_id, source_event_id, created_at)
SELECT
  $1,
  p.type,
  CASE
    WHEN p.type = 'comment' THEN CONCAT(CASE WHEN p.entity_type = 'topic' THEN '话题' ELSE '文章' END, '收到新评论')
    WHEN p.type = 'like' THEN CONCAT(CASE WHEN p.entity_type = 'topic' THEN '话题' ELSE '文章' END, '被点赞')
    WHEN p.type = 'favorite' THEN CONCAT(CASE WHEN p.entity_type = 'topic' THEN '话题' ELSE '文章' END, '被收藏')
    ELSE CONCAT(CASE WHEN p.entity_type = 'topic' THEN '话题' ELSE '文章' END, '收到互动')
  END,
  CASE
    WHEN p.type = 'comment' THEN CONCAT('用户 #', p.actor_id, ' 评论了《', $2::text, '》')
    WHEN p.type = 'like' THEN CONCAT('用户 #', p.actor_id, ' 点赞了《', $2::text, '》')
    WHEN p.type = 'favorite' THEN CONCAT('用户 #', p.actor_id, ' 收藏了《', $2::text, '》')
    ELSE CONCAT('用户 #', p.actor_id, ' 与《', $2::text, '》发生了互动')
  END,
  p.actor_id,
  p.entity_type,
  p.entity_id,
  p.source_id,
  p.event_id,
  p.created_at
FROM pending_content_notifications p
WHERE p.event_id = ANY($3::TEXT[])
  AND p.actor_id <> $1
  AND NOT EXISTS (SELECT 1 FROM notification_erased_users WHERE user_id = $1)
  AND NOT EXISTS (SELECT 1 FROM notification_erased_users WHERE user_id = p.actor_id)
  AND NOT EXISTS (
    SELECT 1 FROM notification_erased_content e
    WHERE e.entity_type = p.entity_type AND e.entity_id = p.entity_id
  )
  AND (
    p.type <> 'comment' OR p.source_id = 0 OR NOT EXISTS (
      SELECT 1 FROM notification_erased_comments WHERE comment_id = p.source_id
    )
  )
  AND COALESCE((SELECT enabled FROM notification_preferences WHERE user_id = $1 AND notification_type = p.type), TRUE)
ON CONFLICT(user_id, source_event_id) DO NOTHING
`, content.AuthorID, content.Title, eventIDs)
	if err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `DELETE FROM pending_content_notifications WHERE event_id = ANY($1::TEXT[])`, eventIDs); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func (r *PostgresRepository) SaveComment(ctx context.Context, comment domain.CommentRef, createdAt time.Time) error {
	if comment.ID <= 0 || comment.AuthorID <= 0 || comment.EntityID <= 0 || !supportedContentType(comment.EntityType) {
		return nil
	}
	subjects := []string{
		contentLockSubject(comment.EntityType, comment.EntityID),
		commentLockSubject(comment.ID),
	}
	return r.withUserAndSubjectLocks(ctx, []int64{comment.AuthorID}, subjects, func(tx pgx.Tx) error {
		_, err := tx.Exec(ctx, `
INSERT INTO comment_refs(comment_id, entity_type, entity_id, author_id, parent_id, created_at, updated_at)
SELECT $1, $2::VARCHAR(32), $3, $4, $5, $6, NOW()
WHERE NOT EXISTS (SELECT 1 FROM notification_erased_users WHERE user_id = $4)
  AND NOT EXISTS (SELECT 1 FROM notification_erased_comments WHERE comment_id = $1)
  AND NOT EXISTS (
    SELECT 1 FROM notification_erased_content
    WHERE entity_type = $2 AND entity_id = $3
  )
ON CONFLICT(comment_id) DO UPDATE SET
  entity_type = EXCLUDED.entity_type,
  entity_id = EXCLUDED.entity_id,
  author_id = EXCLUDED.author_id,
  parent_id = EXCLUDED.parent_id,
  created_at = COALESCE(comment_refs.created_at, EXCLUDED.created_at),
  updated_at = NOW()
`, comment.ID, comment.EntityType, comment.EntityID, comment.AuthorID, comment.ParentID, nullableTime(createdAt))
		return err
	})
}

func (r *PostgresRepository) GetComment(ctx context.Context, id int64) (domain.CommentRef, error) {
	if id <= 0 {
		return domain.CommentRef{}, nil
	}
	var comment domain.CommentRef
	err := r.pool.QueryRow(ctx, `
SELECT comment_id, entity_type, entity_id, author_id, parent_id
FROM comment_refs
WHERE comment_id = $1
`, id).Scan(&comment.ID, &comment.EntityType, &comment.EntityID, &comment.AuthorID, &comment.ParentID)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.CommentRef{}, nil
	}
	return comment, err
}

func (r *PostgresRepository) SavePendingReplyNotification(ctx context.Context, eventID string, parentCommentID, commentID int64, entityType string, entityID, actorID int64, createdAt time.Time) error {
	if eventID == "" || parentCommentID <= 0 || commentID <= 0 || entityID <= 0 || actorID <= 0 || !supportedContentType(entityType) {
		return nil
	}
	if createdAt.IsZero() {
		createdAt = time.Now()
	}
	subjects := []string{
		contentLockSubject(entityType, entityID),
		commentLockSubject(parentCommentID),
		commentLockSubject(commentID),
	}
	return r.withUserAndSubjectLocks(ctx, []int64{actorID}, subjects, func(tx pgx.Tx) error {
		_, err := tx.Exec(ctx, `
INSERT INTO pending_reply_notifications(event_id, parent_comment_id, comment_id, entity_type, entity_id, actor_id, created_at)
SELECT $1::VARCHAR(128), $2, $3, $4::VARCHAR(32), $5, $6, $7
WHERE NOT EXISTS (SELECT 1 FROM notification_erased_users WHERE user_id = $6)
  AND NOT EXISTS (
    SELECT 1 FROM notification_erased_content
    WHERE entity_type = $4 AND entity_id = $5
  )
  AND NOT EXISTS (
    SELECT 1 FROM notification_erased_comments
    WHERE comment_id IN ($2, $3)
  )
ON CONFLICT(event_id) DO NOTHING
`, eventID, parentCommentID, commentID, entityType, entityID, actorID, createdAt)
		return err
	})
}

func (r *PostgresRepository) FlushPendingReplyNotifications(ctx context.Context, parent domain.CommentRef) error {
	if parent.ID <= 0 || parent.AuthorID <= 0 {
		return nil
	}
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	eventIDs, actorIDs, commentIDs, err := loadPendingReplySubjects(ctx, tx, `
SELECT event_id, actor_id, comment_id
FROM pending_reply_notifications
WHERE parent_comment_id = $1
`, parent.ID)
	if err != nil {
		return err
	}
	if len(eventIDs) == 0 {
		return tx.Commit(ctx)
	}
	if err := lockNotificationUsers(ctx, tx, append(actorIDs, parent.AuthorID)); err != nil {
		return err
	}
	subjects := []string{commentLockSubject(parent.ID)}
	if supportedContentType(parent.EntityType) && parent.EntityID > 0 {
		subjects = append(subjects, contentLockSubject(parent.EntityType, parent.EntityID))
	}
	for _, commentID := range commentIDs {
		subjects = append(subjects, commentLockSubject(commentID))
	}
	if err := lockNotificationSubjects(ctx, tx, subjects); err != nil {
		return err
	}

	_, err = tx.Exec(ctx, `
INSERT INTO notifications(user_id, type, title, content, actor_id, entity_type, entity_id, source_id, source_event_id, created_at)
SELECT
  $1,
  'reply',
  '评论收到回复',
  CONCAT('用户 #', p.actor_id, ' 回复了你的评论'),
  p.actor_id,
  p.entity_type,
  p.entity_id,
  p.comment_id,
  p.event_id,
  p.created_at
FROM pending_reply_notifications p
WHERE p.event_id = ANY($2::TEXT[])
  AND p.actor_id <> $1
  AND NOT EXISTS (SELECT 1 FROM notification_erased_users WHERE user_id = $1)
  AND NOT EXISTS (SELECT 1 FROM notification_erased_users WHERE user_id = p.actor_id)
  AND NOT EXISTS (
    SELECT 1 FROM notification_erased_content e
    WHERE e.entity_type = p.entity_type AND e.entity_id = p.entity_id
  )
  AND NOT EXISTS (
    SELECT 1 FROM notification_erased_comments e
    WHERE e.comment_id IN (p.parent_comment_id, p.comment_id)
  )
  AND COALESCE((SELECT enabled FROM notification_preferences WHERE user_id = $1 AND notification_type = 'reply'), TRUE)
ON CONFLICT(user_id, source_event_id) DO NOTHING
`, parent.AuthorID, eventIDs)
	if err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `DELETE FROM pending_reply_notifications WHERE event_id = ANY($1::TEXT[])`, eventIDs); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func (r *PostgresRepository) SaveArticle(ctx context.Context, article domain.ArticleRef, publishedAt time.Time) error {
	if article.ID <= 0 || article.AuthorID <= 0 {
		return nil
	}
	if err := r.SaveContent(ctx, domain.ContentRef{EntityType: "article", ID: article.ID, AuthorID: article.AuthorID, Title: article.Title}, publishedAt); err != nil {
		return err
	}
	return r.withUserAndSubjectLocks(ctx, []int64{article.AuthorID}, []string{contentLockSubject("article", article.ID)}, func(tx pgx.Tx) error {
		_, err := tx.Exec(ctx, `
INSERT INTO article_authors(article_id, author_id, title, published_at, updated_at)
SELECT $1, $2, $3, $4, NOW()
WHERE NOT EXISTS (SELECT 1 FROM notification_erased_users WHERE user_id = $2)
  AND NOT EXISTS (
    SELECT 1 FROM notification_erased_content
    WHERE entity_type = 'article' AND entity_id = $1
  )
ON CONFLICT(article_id) DO UPDATE SET
  author_id = EXCLUDED.author_id,
  title = EXCLUDED.title,
  published_at = EXCLUDED.published_at,
  updated_at = NOW()
`, article.ID, article.AuthorID, article.Title, nullableTime(publishedAt))
		return err
	})
}

func (r *PostgresRepository) GetArticle(ctx context.Context, id int64) (domain.ArticleRef, error) {
	var article domain.ArticleRef
	err := r.pool.QueryRow(ctx, `SELECT article_id, author_id, title FROM article_authors WHERE article_id = $1`, id).Scan(&article.ID, &article.AuthorID, &article.Title)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.ArticleRef{}, nil
	}
	return article, err
}

func (r *PostgresRepository) SavePendingArticleNotification(ctx context.Context, eventID, notificationType string, articleID, actorID, sourceID int64, createdAt time.Time) error {
	if eventID == "" || notificationType == "" || articleID <= 0 || actorID <= 0 {
		return nil
	}
	if createdAt.IsZero() {
		createdAt = time.Now()
	}
	subjects := []string{contentLockSubject("article", articleID)}
	if notificationType == "comment" && sourceID > 0 {
		subjects = append(subjects, commentLockSubject(sourceID))
	}
	return r.withUserAndSubjectLocks(ctx, []int64{actorID}, subjects, func(tx pgx.Tx) error {
		_, err := tx.Exec(ctx, `
INSERT INTO pending_article_notifications(event_id, type, article_id, actor_id, source_id, created_at)
SELECT $1::VARCHAR(128), $2::VARCHAR(64), $3, $4, $5, $6
WHERE NOT EXISTS (SELECT 1 FROM notification_erased_users WHERE user_id = $4)
  AND NOT EXISTS (
    SELECT 1 FROM notification_erased_content
    WHERE entity_type = 'article' AND entity_id = $3
  )
  AND (
    $2::VARCHAR(64) <> 'comment' OR $5::BIGINT = 0 OR NOT EXISTS (
      SELECT 1 FROM notification_erased_comments WHERE comment_id = $5
    )
  )
ON CONFLICT(event_id) DO NOTHING
`, eventID, notificationType, articleID, actorID, sourceID, createdAt)
		return err
	})
}

func (r *PostgresRepository) FlushPendingArticleNotifications(ctx context.Context, article domain.ArticleRef) error {
	if article.ID <= 0 || article.AuthorID <= 0 {
		return nil
	}
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	eventIDs, actorIDs, err := loadPendingSubjects(ctx, tx, `
SELECT event_id, actor_id
FROM pending_article_notifications
WHERE article_id = $1
`, article.ID)
	if err != nil {
		return err
	}
	if len(eventIDs) == 0 {
		return tx.Commit(ctx)
	}
	if err := lockNotificationUsers(ctx, tx, append(actorIDs, article.AuthorID)); err != nil {
		return err
	}
	if err := lockNotificationSubjects(ctx, tx, []string{contentLockSubject("article", article.ID)}); err != nil {
		return err
	}

	_, err = tx.Exec(ctx, `
INSERT INTO notifications(user_id, type, title, content, actor_id, entity_type, entity_id, source_id, source_event_id, created_at)
SELECT
  $1,
  p.type,
  CASE
    WHEN p.type = 'comment' THEN '文章收到新评论'
    WHEN p.type = 'like' THEN '文章被点赞'
    WHEN p.type = 'favorite' THEN '文章被收藏'
    ELSE '文章收到互动'
  END,
  CASE
    WHEN p.type = 'comment' THEN CONCAT('用户 #', p.actor_id, ' 评论了《', $2::text, '》')
    WHEN p.type = 'like' THEN CONCAT('用户 #', p.actor_id, ' 点赞了《', $2::text, '》')
    WHEN p.type = 'favorite' THEN CONCAT('用户 #', p.actor_id, ' 收藏了《', $2::text, '》')
    ELSE CONCAT('用户 #', p.actor_id, ' 与《', $2::text, '》发生了互动')
  END,
  p.actor_id,
  'article',
  p.article_id,
  p.source_id,
  p.event_id,
  p.created_at
FROM pending_article_notifications p
WHERE p.event_id = ANY($3::TEXT[])
  AND p.actor_id <> $1
  AND NOT EXISTS (SELECT 1 FROM notification_erased_users WHERE user_id = $1)
  AND NOT EXISTS (SELECT 1 FROM notification_erased_users WHERE user_id = p.actor_id)
  AND NOT EXISTS (
    SELECT 1 FROM notification_erased_content e
    WHERE e.entity_type = 'article' AND e.entity_id = p.article_id
  )
  AND (
    p.type <> 'comment' OR p.source_id = 0 OR NOT EXISTS (
      SELECT 1 FROM notification_erased_comments WHERE comment_id = p.source_id
    )
  )
  AND COALESCE((SELECT enabled FROM notification_preferences WHERE user_id = $1 AND notification_type = p.type), TRUE)
ON CONFLICT(user_id, source_event_id) DO NOTHING
`, article.AuthorID, article.Title, eventIDs)
	if err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `DELETE FROM pending_article_notifications WHERE event_id = ANY($1::TEXT[])`, eventIDs); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func (r *PostgresRepository) Create(ctx context.Context, item domain.Notification, sourceEventID string, createdAt time.Time) error {
	if item.UserID <= 0 || sourceEventID == "" {
		return nil
	}
	if createdAt.IsZero() {
		createdAt = time.Now()
	}
	subjects := make([]string, 0, 2)
	if supportedContentType(item.EntityType) && item.EntityID > 0 {
		subjects = append(subjects, contentLockSubject(item.EntityType, item.EntityID))
	}
	if commentNotificationType(item.Type) && item.SourceID > 0 {
		subjects = append(subjects, commentLockSubject(item.SourceID))
	}
	return r.withUserAndSubjectLocks(ctx, []int64{item.UserID, item.ActorID}, subjects, func(tx pgx.Tx) error {
		_, err := tx.Exec(ctx, `
INSERT INTO notifications(user_id, type, title, content, actor_id, entity_type, entity_id, source_id, source_event_id, created_at)
SELECT $1::BIGINT, $2::VARCHAR(64), $3::TEXT, $4::TEXT, $5::BIGINT, $6::VARCHAR(32), $7::BIGINT, $8::BIGINT, $9::VARCHAR(128), $10
WHERE NOT EXISTS (SELECT 1 FROM notification_erased_users WHERE user_id = $1::BIGINT)
  AND ($5::BIGINT = 0 OR NOT EXISTS (SELECT 1 FROM notification_erased_users WHERE user_id = $5))
  AND (
    $6::VARCHAR(32) NOT IN ('article', 'topic') OR $7::BIGINT = 0 OR NOT EXISTS (
      SELECT 1 FROM notification_erased_content
      WHERE entity_type = $6 AND entity_id = $7
    )
  )
  AND (
    $2::VARCHAR(64) NOT IN ('comment', 'reply', 'qa_answer_accepted') OR $8::BIGINT = 0 OR NOT EXISTS (
      SELECT 1 FROM notification_erased_comments WHERE comment_id = $8
    )
  )
  AND COALESCE((SELECT enabled FROM notification_preferences WHERE user_id = $1 AND notification_type = $2), TRUE)
ON CONFLICT(user_id, source_event_id) DO NOTHING
`, item.UserID, item.Type, item.Title, item.Content, item.ActorID, item.EntityType, item.EntityID, item.SourceID, sourceEventID, createdAt)
		return err
	})
}

func (r *PostgresRepository) CreateSystemNotifications(ctx context.Context, command domain.SystemNotificationCommand, createdAt time.Time) (int32, error) {
	if createdAt.IsZero() {
		createdAt = time.Now()
	}
	userIDs := append([]int64{command.ActorID}, command.RecipientIDs...)
	var delivered int32
	err := r.withUserLocks(ctx, userIDs, func(tx pgx.Tx) error {
		tag, err := tx.Exec(ctx, `
INSERT INTO notifications(user_id, type, title, content, actor_id, entity_type, entity_id, source_id, source_event_id, created_at)
SELECT recipients.user_id, $2, $3, $4, $5, 'system', 0, 0, $6, $7
FROM unnest($1::bigint[]) AS recipients(user_id)
WHERE NOT EXISTS (SELECT 1 FROM notification_erased_users WHERE user_id = recipients.user_id)
  AND NOT EXISTS (SELECT 1 FROM notification_erased_users WHERE user_id = $5)
  AND COALESCE((SELECT enabled FROM notification_preferences WHERE user_id = recipients.user_id AND notification_type = $2::VARCHAR(64)), TRUE)
ON CONFLICT(user_id, source_event_id) DO NOTHING
`, command.RecipientIDs, domain.SystemNotificationType, command.Title, command.Content, command.ActorID, systemNotificationSourceEventID(command), createdAt)
		if err != nil {
			return err
		}
		delivered = int32(tag.RowsAffected())
		return nil
	})
	return delivered, err
}

func (r *PostgresRepository) ListPreferences(ctx context.Context, userID int64) ([]domain.NotificationPreference, error) {
	if userID <= 0 {
		return nil, domain.ErrInvalidNotificationPreferences
	}
	rows, err := r.pool.Query(ctx, `
SELECT notification_type, enabled
FROM notification_preferences
WHERE user_id = $1
ORDER BY notification_type
`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := make([]domain.NotificationPreference, 0)
	for rows.Next() {
		var item domain.NotificationPreference
		if err := rows.Scan(&item.Type, &item.Enabled); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return items, nil
}

func (r *PostgresRepository) ReplacePreferences(ctx context.Context, userID int64, preferences []domain.NotificationPreference) error {
	if userID <= 0 {
		return domain.ErrInvalidNotificationPreferences
	}
	preferences, err := domain.NormalizeNotificationPreferences(preferences)
	if err != nil {
		return err
	}
	return r.withUserLocks(ctx, []int64{userID}, func(tx pgx.Tx) error {
		if _, err := tx.Exec(ctx, `DELETE FROM notification_preferences WHERE user_id = $1`, userID); err != nil {
			return err
		}
		for _, preference := range preferences {
			if _, err := tx.Exec(ctx, `
INSERT INTO notification_preferences(user_id, notification_type, enabled)
SELECT $1, $2, $3
WHERE NOT EXISTS (SELECT 1 FROM notification_erased_users WHERE user_id = $1)
`, userID, preference.Type, preference.Enabled); err != nil {
				return err
			}
		}
		return nil
	})
}

func (r *PostgresRepository) UpsertWebPushSubscription(ctx context.Context, subscription domain.WebPushSubscription, maxPerUser int) (domain.WebPushSubscription, error) {
	if maxPerUser <= 0 {
		maxPerUser = domain.WebPushMaxSubscriptionsPerUser
	}
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return domain.WebPushSubscription{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if err := lockNotificationSubjects(ctx, tx, []string{"web_push_endpoint:" + subscription.Endpoint}); err != nil {
		return domain.WebPushSubscription{}, err
	}
	var previousUserID int64
	lookupErr := tx.QueryRow(ctx, `SELECT user_id FROM web_push_subscriptions WHERE endpoint = $1`, subscription.Endpoint).Scan(&previousUserID)
	if lookupErr != nil && !errors.Is(lookupErr, pgx.ErrNoRows) {
		return domain.WebPushSubscription{}, lookupErr
	}
	userIDs := []int64{subscription.UserID}
	if previousUserID > 0 {
		userIDs = append(userIDs, previousUserID)
	}
	if err := lockNotificationUsers(ctx, tx, userIDs); err != nil {
		return domain.WebPushSubscription{}, err
	}
	var previousSubscriptionID int64
	previousUserID = 0
	lookupErr = tx.QueryRow(ctx, `
SELECT id, user_id
FROM web_push_subscriptions
WHERE endpoint = $1
FOR UPDATE
`, subscription.Endpoint).Scan(&previousSubscriptionID, &previousUserID)
	if lookupErr != nil && !errors.Is(lookupErr, pgx.ErrNoRows) {
		return domain.WebPushSubscription{}, lookupErr
	}
	if previousSubscriptionID > 0 && previousUserID != subscription.UserID {
		if _, err := tx.Exec(ctx, `DELETE FROM web_push_outbox WHERE subscription_id = $1`, previousSubscriptionID); err != nil {
			return domain.WebPushSubscription{}, err
		}
	}
	alreadySubscribed := previousSubscriptionID > 0 && previousUserID == subscription.UserID
	var count int
	if err := tx.QueryRow(ctx, `
SELECT COUNT(*)
FROM web_push_subscriptions
WHERE user_id = $1 AND endpoint <> $2
`, subscription.UserID, subscription.Endpoint).Scan(&count); err != nil {
		return domain.WebPushSubscription{}, err
	}
	if count >= maxPerUser {
		return domain.WebPushSubscription{}, domain.ErrWebPushSubscriptionLimit
	}

	var stored domain.WebPushSubscription
	err = tx.QueryRow(ctx, `
INSERT INTO web_push_subscriptions(user_id, endpoint, auth, public_key, state, send_read_message)
SELECT $1, $2, $3, $4, $5, $6
WHERE NOT EXISTS (SELECT 1 FROM notification_erased_users WHERE user_id = $1)
ON CONFLICT(endpoint) DO UPDATE SET
  user_id = EXCLUDED.user_id,
  auth = EXCLUDED.auth,
  public_key = EXCLUDED.public_key,
  state = EXCLUDED.state,
  send_read_message = EXCLUDED.send_read_message,
  updated_at = NOW()
RETURNING id, user_id, endpoint, auth, public_key, state, send_read_message, created_at, updated_at
`, subscription.UserID, subscription.Endpoint, subscription.Auth, subscription.PublicKey, domain.WebPushSubscriptionStateActive, subscription.SendReadMessage).Scan(
		&stored.ID,
		&stored.UserID,
		&stored.Endpoint,
		&stored.Auth,
		&stored.PublicKey,
		&stored.State,
		&stored.SendReadMessage,
		&stored.CreatedAt,
		&stored.UpdatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.WebPushSubscription{}, domain.ErrInvalidWebPushSubscription
	}
	if err != nil {
		return domain.WebPushSubscription{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return domain.WebPushSubscription{}, err
	}
	if alreadySubscribed {
		stored.RegistrationState = "already-subscribed"
	} else {
		stored.RegistrationState = "subscribed"
	}
	return stored, nil
}

func (r *PostgresRepository) GetWebPushSubscription(ctx context.Context, userID int64, endpoint string) (domain.WebPushSubscription, error) {
	var subscription domain.WebPushSubscription
	err := r.pool.QueryRow(ctx, `
SELECT id, user_id, endpoint, auth, public_key, state, send_read_message, created_at, updated_at
FROM web_push_subscriptions
WHERE user_id = $1 AND endpoint = $2
`, userID, endpoint).Scan(
		&subscription.ID,
		&subscription.UserID,
		&subscription.Endpoint,
		&subscription.Auth,
		&subscription.PublicKey,
		&subscription.State,
		&subscription.SendReadMessage,
		&subscription.CreatedAt,
		&subscription.UpdatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.WebPushSubscription{}, nil
	}
	return subscription, err
}

func (r *PostgresRepository) DeleteWebPushSubscription(ctx context.Context, userID int64, endpoint string) error {
	return r.withUserLocks(ctx, []int64{userID}, func(tx pgx.Tx) error {
		_, err := tx.Exec(ctx, `DELETE FROM web_push_subscriptions WHERE user_id = $1 AND endpoint = $2`, userID, endpoint)
		return err
	})
}

func (r *PostgresRepository) ClaimWebPushDeliveries(ctx context.Context, limit int, now time.Time, lockTimeout time.Duration) ([]domain.WebPushDelivery, error) {
	if limit <= 0 {
		return nil, nil
	}
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	rows, err := tx.Query(ctx, `
SELECT
  outbox.id,
  outbox.notification_id,
  outbox.subscription_id,
  outbox.attempt_count,
  subscriptions.endpoint,
  subscriptions.auth,
  subscriptions.public_key,
  notifications.user_id,
  notifications.type,
  notifications.title,
  notifications.content,
  notifications.created_at
FROM web_push_outbox outbox
JOIN web_push_subscriptions subscriptions ON subscriptions.id = outbox.subscription_id
JOIN notifications ON notifications.id = outbox.notification_id
WHERE outbox.completed_at IS NULL
  AND outbox.available_at <= $1
  AND (outbox.locked_at IS NULL OR outbox.locked_at < $2)
  AND subscriptions.state = 'active'
ORDER BY outbox.available_at, outbox.id
FOR UPDATE OF outbox SKIP LOCKED
LIMIT $3
`, now, now.Add(-lockTimeout), limit)
	if err != nil {
		return nil, err
	}
	deliveries := make([]domain.WebPushDelivery, 0, limit)
	ids := make([]int64, 0, limit)
	for rows.Next() {
		var delivery domain.WebPushDelivery
		if err := rows.Scan(
			&delivery.ID,
			&delivery.NotificationID,
			&delivery.SubscriptionID,
			&delivery.AttemptCount,
			&delivery.Endpoint,
			&delivery.Auth,
			&delivery.PublicKey,
			&delivery.Notification.UserID,
			&delivery.Notification.Type,
			&delivery.Notification.Title,
			&delivery.Notification.Content,
			&delivery.Notification.CreatedAt,
		); err != nil {
			rows.Close()
			return nil, err
		}
		delivery.Notification.ID = delivery.NotificationID
		deliveries = append(deliveries, delivery)
		ids = append(ids, delivery.ID)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return nil, err
	}
	rows.Close()
	if len(ids) > 0 {
		if _, err := tx.Exec(ctx, `UPDATE web_push_outbox SET locked_at = $2, updated_at = $2 WHERE id = ANY($1::BIGINT[])`, ids, now); err != nil {
			return nil, err
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	return deliveries, nil
}

func (r *PostgresRepository) ReleaseWebPushDeliveries(ctx context.Context, deliveryIDs []int64) error {
	if len(deliveryIDs) == 0 {
		return nil
	}
	_, err := r.pool.Exec(ctx, `
UPDATE web_push_outbox
SET locked_at = NULL,
    updated_at = NOW()
WHERE id = ANY($1::BIGINT[])
  AND completed_at IS NULL
`, deliveryIDs)
	return err
}

func (r *PostgresRepository) CompleteWebPushDelivery(ctx context.Context, deliveryID int64, now time.Time) error {
	_, err := r.pool.Exec(ctx, `
UPDATE web_push_outbox
SET attempt_count = attempt_count + 1,
    completed_at = $2,
    locked_at = NULL,
    result = 'delivered',
    last_error = '',
    updated_at = $2
WHERE id = $1
`, deliveryID, now)
	return err
}

func (r *PostgresRepository) ExpireWebPushSubscription(ctx context.Context, deliveryID, subscriptionID int64, now time.Time) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if _, err := tx.Exec(ctx, `
UPDATE web_push_outbox
SET attempt_count = attempt_count + 1,
    completed_at = $2,
    locked_at = NULL,
    result = 'gone',
    updated_at = $2
WHERE id = $1
`, deliveryID, now); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `DELETE FROM web_push_subscriptions WHERE id = $1`, subscriptionID); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func (r *PostgresRepository) RetryWebPushDelivery(ctx context.Context, deliveryID int64, attemptCount int32, nextAttempt time.Time, message string, exhausted bool) error {
	if exhausted {
		_, err := r.pool.Exec(ctx, `
UPDATE web_push_outbox
SET attempt_count = $2,
    completed_at = NOW(),
    locked_at = NULL,
    result = 'failed',
    last_error = $3,
    updated_at = NOW()
WHERE id = $1
`, deliveryID, attemptCount, message)
		return err
	}
	_, err := r.pool.Exec(ctx, `
UPDATE web_push_outbox
SET attempt_count = $2,
    available_at = $3,
    locked_at = NULL,
    result = 'pending',
    last_error = $4,
    updated_at = NOW()
WHERE id = $1
`, deliveryID, attemptCount, nextAttempt, message)
	return err
}

func (r *PostgresRepository) CleanupCompletedWebPushDeliveries(ctx context.Context, before time.Time, limit int) (int64, error) {
	if limit <= 0 {
		return 0, nil
	}
	var deleted int64
	err := r.pool.QueryRow(ctx, `
WITH candidates AS (
  SELECT id
  FROM web_push_outbox
  WHERE completed_at IS NOT NULL
    AND completed_at < $1
  ORDER BY completed_at, id
  FOR UPDATE SKIP LOCKED
  LIMIT $2
), deleted AS (
  DELETE FROM web_push_outbox outbox
  USING candidates
  WHERE outbox.id = candidates.id
  RETURNING 1
)
SELECT COUNT(*) FROM deleted
`, before, limit).Scan(&deleted)
	return deleted, err
}

func systemNotificationSourceEventID(command domain.SystemNotificationCommand) string {
	return "admin_system:" + strconv.FormatInt(command.ActorID, 10) + ":" + command.IdempotencyKey
}

func (r *PostgresRepository) List(ctx context.Context, userID int64, limit, offset int32, unreadOnly bool) ([]domain.Notification, int64, int64, error) {
	if limit <= 0 {
		limit = 20
	}
	if limit > 100 {
		limit = 100
	}
	if offset < 0 {
		offset = 0
	}

	filter := `user_id = $1`
	if unreadOnly {
		filter += ` AND read_at IS NULL`
	}
	var total int64
	if err := r.pool.QueryRow(ctx, `SELECT COUNT(*) FROM notifications WHERE `+filter, userID).Scan(&total); err != nil {
		return nil, 0, 0, err
	}
	unread, err := r.UnreadCount(ctx, userID)
	if err != nil {
		return nil, 0, 0, err
	}

	rows, err := r.pool.Query(ctx, `
SELECT id, user_id, type, title, content, actor_id, entity_type, entity_id, source_id, read_at, created_at
FROM notifications
WHERE `+filter+`
ORDER BY created_at DESC, id DESC
LIMIT $2 OFFSET $3
`, userID, limit, offset)
	if err != nil {
		return nil, 0, 0, err
	}
	defer rows.Close()

	items := make([]domain.Notification, 0, limit)
	for rows.Next() {
		var item domain.Notification
		if err := rows.Scan(&item.ID, &item.UserID, &item.Type, &item.Title, &item.Content, &item.ActorID, &item.EntityType, &item.EntityID, &item.SourceID, &item.ReadAt, &item.CreatedAt); err != nil {
			return nil, 0, 0, err
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, 0, err
	}
	return items, total, unread, nil
}

func (r *PostgresRepository) UnreadCount(ctx context.Context, userID int64) (int64, error) {
	var total int64
	err := r.pool.QueryRow(ctx, `SELECT COUNT(*) FROM notifications WHERE user_id = $1 AND read_at IS NULL`, userID).Scan(&total)
	return total, err
}

func (r *PostgresRepository) MarkRead(ctx context.Context, userID, id int64) error {
	_, err := r.pool.Exec(ctx, `UPDATE notifications SET read_at = COALESCE(read_at, NOW()) WHERE user_id = $1 AND id = $2`, userID, id)
	return err
}

func (r *PostgresRepository) MarkAllRead(ctx context.Context, userID int64) error {
	_, err := r.pool.Exec(ctx, `UPDATE notifications SET read_at = COALESCE(read_at, NOW()) WHERE user_id = $1 AND read_at IS NULL`, userID)
	return err
}

func nullableTime(t time.Time) any {
	if t.IsZero() {
		return nil
	}
	return t
}

func supportedContentType(entityType string) bool {
	return entityType == "article" || entityType == "topic"
}

func commentNotificationType(notificationType string) bool {
	return notificationType == "comment" || notificationType == "reply" || notificationType == "qa_answer_accepted"
}

func contentLockSubject(entityType string, entityID int64) string {
	return "bbs-notification-content:" + entityType + ":" + strconv.FormatInt(entityID, 10)
}

func commentLockSubject(commentID int64) string {
	return "bbs-notification-comment:" + strconv.FormatInt(commentID, 10)
}

func (r *PostgresRepository) withUserLocks(ctx context.Context, userIDs []int64, operation func(pgx.Tx) error) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if err := lockNotificationUsers(ctx, tx, userIDs); err != nil {
		return err
	}
	if err := operation(tx); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func (r *PostgresRepository) withUserAndSubjectLocks(ctx context.Context, userIDs []int64, subjects []string, operation func(pgx.Tx) error) error {
	return r.withUserLocks(ctx, userIDs, func(tx pgx.Tx) error {
		if err := lockNotificationSubjects(ctx, tx, subjects); err != nil {
			return err
		}
		return operation(tx)
	})
}

func lockNotificationUsers(ctx context.Context, tx pgx.Tx, userIDs []int64) error {
	userIDs = normalizedUserIDs(userIDs)
	if len(userIDs) == 0 {
		return nil
	}
	rows, err := tx.Query(ctx, `
SELECT pg_advisory_xact_lock(hashtextextended('bbs-notification-user:' || user_id::text, 0))
FROM unnest($1::BIGINT[]) AS subjects(user_id)
ORDER BY user_id
`, userIDs)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
	}
	return rows.Err()
}

func lockNotificationSubjects(ctx context.Context, tx pgx.Tx, subjects []string) error {
	subjects = normalizedSubjects(subjects)
	if len(subjects) == 0 {
		return nil
	}
	rows, err := tx.Query(ctx, `
SELECT pg_advisory_xact_lock(hashtextextended(subject, 0))
FROM unnest($1::TEXT[]) AS lock_subjects(subject)
ORDER BY subject
`, subjects)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
	}
	return rows.Err()
}

func lockUserErasureSubjects(ctx context.Context, tx pgx.Tx, userID int64) error {
	rows, err := tx.Query(ctx, `
SELECT pg_advisory_xact_lock(hashtextextended(subject, 0))
FROM (
  SELECT 'bbs-notification-content:' || entity_type || ':' || entity_id::TEXT AS subject
  FROM content_refs
  WHERE author_id = $1
  UNION
  SELECT 'bbs-notification-content:article:' || article_id::TEXT
  FROM article_authors
  WHERE author_id = $1
  UNION
  SELECT 'bbs-notification-comment:' || comment_id::TEXT
  FROM comment_refs
  WHERE author_id = $1
) AS erasure_subjects
ORDER BY subject
`, userID)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
	}
	return rows.Err()
}

func loadPendingSubjects(ctx context.Context, tx pgx.Tx, query string, args ...any) ([]string, []int64, error) {
	rows, err := tx.Query(ctx, query, args...)
	if err != nil {
		return nil, nil, err
	}
	defer rows.Close()
	eventIDs := make([]string, 0)
	actorIDs := make([]int64, 0)
	for rows.Next() {
		var eventID string
		var actorID int64
		if err := rows.Scan(&eventID, &actorID); err != nil {
			return nil, nil, err
		}
		eventIDs = append(eventIDs, eventID)
		actorIDs = append(actorIDs, actorID)
	}
	if err := rows.Err(); err != nil {
		return nil, nil, err
	}
	return eventIDs, actorIDs, nil
}

func loadPendingReplySubjects(ctx context.Context, tx pgx.Tx, query string, args ...any) ([]string, []int64, []int64, error) {
	rows, err := tx.Query(ctx, query, args...)
	if err != nil {
		return nil, nil, nil, err
	}
	defer rows.Close()
	eventIDs := make([]string, 0)
	actorIDs := make([]int64, 0)
	commentIDs := make([]int64, 0)
	for rows.Next() {
		var eventID string
		var actorID int64
		var commentID int64
		if err := rows.Scan(&eventID, &actorID, &commentID); err != nil {
			return nil, nil, nil, err
		}
		eventIDs = append(eventIDs, eventID)
		actorIDs = append(actorIDs, actorID)
		commentIDs = append(commentIDs, commentID)
	}
	if err := rows.Err(); err != nil {
		return nil, nil, nil, err
	}
	return eventIDs, actorIDs, commentIDs, nil
}

func normalizedUserIDs(userIDs []int64) []int64 {
	unique := make(map[int64]struct{}, len(userIDs))
	for _, userID := range userIDs {
		if userID > 0 {
			unique[userID] = struct{}{}
		}
	}
	normalized := make([]int64, 0, len(unique))
	for userID := range unique {
		normalized = append(normalized, userID)
	}
	sort.Slice(normalized, func(left, right int) bool { return normalized[left] < normalized[right] })
	return normalized
}

func normalizedSubjects(subjects []string) []string {
	unique := make(map[string]struct{}, len(subjects))
	for _, subject := range subjects {
		if subject != "" {
			unique[subject] = struct{}{}
		}
	}
	normalized := make([]string, 0, len(unique))
	for subject := range unique {
		normalized = append(normalized, subject)
	}
	sort.Strings(normalized)
	return normalized
}
