CREATE TABLE IF NOT EXISTS content_lifecycle_outbox (
  event_id VARCHAR(160) PRIMARY KEY,
  message_key VARCHAR(64) NOT NULL,
  event_type VARCHAR(80) NOT NULL,
  payload JSONB NOT NULL,
  status VARCHAR(16) NOT NULL DEFAULT 'PENDING',
  attempts INTEGER NOT NULL DEFAULT 0,
  lease_owner VARCHAR(160) NOT NULL DEFAULT '',
  lease_expires_at TIMESTAMPTZ,
  last_error TEXT NOT NULL DEFAULT '',
  next_attempt_at TIMESTAMPTZ,
  published_at TIMESTAMPTZ,
  created_at TIMESTAMPTZ NOT NULL,
  updated_at TIMESTAMPTZ NOT NULL,
  CONSTRAINT content_lifecycle_outbox_status_check CHECK (status IN ('PENDING', 'PUBLISHING', 'FAILED', 'PUBLISHED'))
);

CREATE INDEX IF NOT EXISTS idx_content_lifecycle_outbox_dispatch
  ON content_lifecycle_outbox(status, next_attempt_at, lease_expires_at, created_at);
