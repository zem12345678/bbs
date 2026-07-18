CREATE TABLE IF NOT EXISTS qa_acceptance_outbox (
  event_id VARCHAR(160) PRIMARY KEY,
  topic_id BIGINT NOT NULL UNIQUE REFERENCES topics(id),
  message_key VARCHAR(64) NOT NULL,
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
  CONSTRAINT qa_acceptance_outbox_status_check CHECK (status IN ('PENDING', 'PUBLISHING', 'FAILED', 'PUBLISHED'))
);

CREATE INDEX IF NOT EXISTS idx_qa_acceptance_outbox_dispatch
  ON qa_acceptance_outbox(status, next_attempt_at, lease_expires_at, created_at);
