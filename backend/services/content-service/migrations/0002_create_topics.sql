CREATE TABLE IF NOT EXISTS topics (
  id BIGINT PRIMARY KEY,
  slug VARCHAR(128) NOT NULL UNIQUE,
  type VARCHAR(16) NOT NULL DEFAULT 'topic',
  title VARCHAR(180) NOT NULL DEFAULT '',
  body TEXT NOT NULL,
  tags JSONB NOT NULL DEFAULT '[]',
  author_id BIGINT NOT NULL,
  status SMALLINT NOT NULL DEFAULT 1,
  created_at TIMESTAMPTZ NOT NULL,
  updated_at TIMESTAMPTZ NOT NULL,
  published_at TIMESTAMPTZ
);

CREATE INDEX IF NOT EXISTS idx_topics_status_published_at
  ON topics(status, published_at DESC);

CREATE INDEX IF NOT EXISTS idx_topics_type_status_published
  ON topics(type, status, published_at DESC);

CREATE INDEX IF NOT EXISTS idx_topics_author_status_created
  ON topics(author_id, status, created_at DESC);

CREATE INDEX IF NOT EXISTS idx_topics_created_at
  ON topics(created_at DESC);

CREATE INDEX IF NOT EXISTS idx_topics_tags_gin
  ON topics USING GIN(tags);
