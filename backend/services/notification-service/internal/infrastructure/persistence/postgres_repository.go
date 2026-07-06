package persistence

import (
	"context"
	"errors"
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

CREATE INDEX IF NOT EXISTS idx_pending_article_notifications_article
  ON pending_article_notifications(article_id, created_at);

CREATE INDEX IF NOT EXISTS idx_pending_content_notifications_entity
  ON pending_content_notifications(entity_type, entity_id, created_at);

CREATE INDEX IF NOT EXISTS idx_notifications_user_created
  ON notifications(user_id, created_at DESC);

CREATE INDEX IF NOT EXISTS idx_notifications_user_unread
  ON notifications(user_id, created_at DESC)
  WHERE read_at IS NULL;
`)
	return err
}

func (r *PostgresRepository) SaveContent(ctx context.Context, content domain.ContentRef, publishedAt time.Time) error {
	if content.ID <= 0 || content.AuthorID <= 0 || !supportedContentType(content.EntityType) {
		return nil
	}
	_, err := r.pool.Exec(ctx, `
INSERT INTO content_refs(entity_type, entity_id, author_id, title, published_at, updated_at)
VALUES($1, $2, $3, $4, $5, NOW())
ON CONFLICT(entity_type, entity_id) DO UPDATE SET
  author_id = EXCLUDED.author_id,
  title = EXCLUDED.title,
  published_at = EXCLUDED.published_at,
  updated_at = NOW()
`, content.EntityType, content.ID, content.AuthorID, content.Title, nullableTime(publishedAt))
	return err
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
	_, err := r.pool.Exec(ctx, `
INSERT INTO pending_content_notifications(event_id, type, entity_type, entity_id, actor_id, source_id, created_at)
VALUES($1, $2, $3, $4, $5, $6, $7)
ON CONFLICT(event_id) DO NOTHING
`, eventID, notificationType, entityType, entityID, actorID, sourceID, createdAt)
	return err
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

	_, err = tx.Exec(ctx, `
INSERT INTO notifications(user_id, type, title, content, actor_id, entity_type, entity_id, source_id, source_event_id, created_at)
SELECT
  $3,
  p.type,
  CASE
    WHEN p.type = 'comment' THEN CONCAT(CASE WHEN p.entity_type = 'topic' THEN '话题' ELSE '文章' END, '收到新评论')
    WHEN p.type = 'like' THEN CONCAT(CASE WHEN p.entity_type = 'topic' THEN '话题' ELSE '文章' END, '被点赞')
    WHEN p.type = 'favorite' THEN CONCAT(CASE WHEN p.entity_type = 'topic' THEN '话题' ELSE '文章' END, '被收藏')
    ELSE CONCAT(CASE WHEN p.entity_type = 'topic' THEN '话题' ELSE '文章' END, '收到互动')
  END,
  CASE
    WHEN p.type = 'comment' THEN CONCAT('用户 #', p.actor_id, ' 评论了《', $4::text, '》')
    WHEN p.type = 'like' THEN CONCAT('用户 #', p.actor_id, ' 点赞了《', $4::text, '》')
    WHEN p.type = 'favorite' THEN CONCAT('用户 #', p.actor_id, ' 收藏了《', $4::text, '》')
    ELSE CONCAT('用户 #', p.actor_id, ' 与《', $4::text, '》发生了互动')
  END,
  p.actor_id,
  p.entity_type,
  p.entity_id,
  p.source_id,
  p.event_id,
  p.created_at
FROM pending_content_notifications p
WHERE p.entity_type = $1 AND p.entity_id = $2 AND p.actor_id <> $3
ON CONFLICT(user_id, source_event_id) DO NOTHING
`, content.EntityType, content.ID, content.AuthorID, content.Title)
	if err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `DELETE FROM pending_content_notifications WHERE entity_type = $1 AND entity_id = $2`, content.EntityType, content.ID); err != nil {
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
	_, err := r.pool.Exec(ctx, `
INSERT INTO article_authors(article_id, author_id, title, published_at, updated_at)
VALUES($1, $2, $3, $4, NOW())
ON CONFLICT(article_id) DO UPDATE SET
  author_id = EXCLUDED.author_id,
  title = EXCLUDED.title,
  published_at = EXCLUDED.published_at,
  updated_at = NOW()
`, article.ID, article.AuthorID, article.Title, nullableTime(publishedAt))
	return err
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
	_, err := r.pool.Exec(ctx, `
INSERT INTO pending_article_notifications(event_id, type, article_id, actor_id, source_id, created_at)
VALUES($1, $2, $3, $4, $5, $6)
ON CONFLICT(event_id) DO NOTHING
`, eventID, notificationType, articleID, actorID, sourceID, createdAt)
	return err
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

	_, err = tx.Exec(ctx, `
INSERT INTO notifications(user_id, type, title, content, actor_id, entity_type, entity_id, source_id, source_event_id, created_at)
SELECT
  $2,
  p.type,
  CASE
    WHEN p.type = 'comment' THEN '文章收到新评论'
    WHEN p.type = 'like' THEN '文章被点赞'
    WHEN p.type = 'favorite' THEN '文章被收藏'
    ELSE '文章收到互动'
  END,
  CASE
    WHEN p.type = 'comment' THEN CONCAT('用户 #', p.actor_id, ' 评论了《', $3::text, '》')
    WHEN p.type = 'like' THEN CONCAT('用户 #', p.actor_id, ' 点赞了《', $3::text, '》')
    WHEN p.type = 'favorite' THEN CONCAT('用户 #', p.actor_id, ' 收藏了《', $3::text, '》')
    ELSE CONCAT('用户 #', p.actor_id, ' 与《', $3::text, '》发生了互动')
  END,
  p.actor_id,
  'article',
  p.article_id,
  p.source_id,
  p.event_id,
  p.created_at
FROM pending_article_notifications p
WHERE p.article_id = $1 AND p.actor_id <> $2
ON CONFLICT(user_id, source_event_id) DO NOTHING
`, article.ID, article.AuthorID, article.Title)
	if err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `DELETE FROM pending_article_notifications WHERE article_id = $1`, article.ID); err != nil {
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
	_, err := r.pool.Exec(ctx, `
INSERT INTO notifications(user_id, type, title, content, actor_id, entity_type, entity_id, source_id, source_event_id, created_at)
VALUES($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
ON CONFLICT(user_id, source_event_id) DO NOTHING
`, item.UserID, item.Type, item.Title, item.Content, item.ActorID, item.EntityType, item.EntityID, item.SourceID, sourceEventID, createdAt)
	return err
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
