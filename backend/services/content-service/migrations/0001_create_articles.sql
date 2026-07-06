CREATE TABLE IF NOT EXISTS articles (
  id BIGINT PRIMARY KEY,
  slug VARCHAR(128) NOT NULL UNIQUE,
  title VARCHAR(180) NOT NULL,
  summary TEXT,
  body TEXT NOT NULL,
  cover_url VARCHAR(1024),
  tags JSONB NOT NULL DEFAULT '[]',
  author_id BIGINT NOT NULL,
  status SMALLINT NOT NULL DEFAULT 1,
  created_at TIMESTAMPTZ NOT NULL,
  updated_at TIMESTAMPTZ NOT NULL,
  published_at TIMESTAMPTZ
);

CREATE INDEX IF NOT EXISTS idx_articles_status_published_at
  ON articles(status, published_at DESC);

CREATE INDEX IF NOT EXISTS idx_articles_author_status_created
  ON articles(author_id, status, created_at DESC);

CREATE INDEX IF NOT EXISTS idx_articles_created_at
  ON articles(created_at DESC);

CREATE INDEX IF NOT EXISTS idx_articles_tags_gin
  ON articles USING GIN(tags);

