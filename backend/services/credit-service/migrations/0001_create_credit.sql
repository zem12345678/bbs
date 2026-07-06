CREATE TABLE IF NOT EXISTS credit_balances (
  user_id BIGINT PRIMARY KEY,
  total BIGINT NOT NULL DEFAULT 0,
  updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS credit_ledger (
  id BIGSERIAL PRIMARY KEY,
  user_id BIGINT NOT NULL,
  delta BIGINT NOT NULL,
  balance_after BIGINT NOT NULL,
  reason VARCHAR(64) NOT NULL,
  description TEXT NOT NULL DEFAULT '',
  source_event_id VARCHAR(128) NOT NULL,
  source_type VARCHAR(64) NOT NULL DEFAULT '',
  source_id BIGINT NOT NULL DEFAULT 0,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  UNIQUE(user_id, source_event_id, reason)
);

CREATE INDEX IF NOT EXISTS idx_credit_ledger_user_created
  ON credit_ledger(user_id, created_at DESC, id DESC);

CREATE TABLE IF NOT EXISTS article_authors (
  article_id BIGINT PRIMARY KEY,
  author_id BIGINT NOT NULL,
  title TEXT NOT NULL DEFAULT '',
  published_at TIMESTAMPTZ,
  updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS pending_article_credits (
  event_id VARCHAR(128) NOT NULL,
  reason VARCHAR(64) NOT NULL,
  article_id BIGINT NOT NULL,
  actor_id BIGINT NOT NULL,
  delta BIGINT NOT NULL,
  source_type VARCHAR(64) NOT NULL DEFAULT '',
  source_id BIGINT NOT NULL DEFAULT 0,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  PRIMARY KEY(event_id, reason)
);

CREATE INDEX IF NOT EXISTS idx_pending_article_credits_article
  ON pending_article_credits(article_id, created_at);
