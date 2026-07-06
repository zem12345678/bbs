CREATE TABLE IF NOT EXISTS article_authors (
  article_id BIGINT PRIMARY KEY,
  author_id BIGINT NOT NULL,
  title TEXT NOT NULL DEFAULT '',
  published_at TIMESTAMPTZ,
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

CREATE TABLE IF NOT EXISTS pending_article_notifications (
  event_id VARCHAR(128) PRIMARY KEY,
  type VARCHAR(64) NOT NULL,
  article_id BIGINT NOT NULL,
  actor_id BIGINT NOT NULL,
  source_id BIGINT NOT NULL DEFAULT 0,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_pending_article_notifications_article
  ON pending_article_notifications(article_id, created_at);

CREATE INDEX IF NOT EXISTS idx_notifications_user_created
  ON notifications(user_id, created_at DESC);

CREATE INDEX IF NOT EXISTS idx_notifications_user_unread
  ON notifications(user_id, created_at DESC)
  WHERE read_at IS NULL;
