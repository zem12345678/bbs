CREATE TABLE IF NOT EXISTS user_account_deletion_outbox (
  event_id UUID PRIMARY KEY,
  job_id BIGINT NOT NULL UNIQUE REFERENCES user_account_jobs(id) ON DELETE CASCADE,
  aggregate_id BIGINT NOT NULL,
  event_type VARCHAR(80) NOT NULL,
  message_key TEXT NOT NULL,
  payload_json JSONB NOT NULL,
  status VARCHAR(20) NOT NULL DEFAULT 'pending',
  attempts INTEGER NOT NULL DEFAULT 0,
  available_at TIMESTAMPTZ NOT NULL,
  lease_owner TEXT,
  lease_expires_at TIMESTAMPTZ,
  last_error TEXT NOT NULL DEFAULT '',
  occurred_at TIMESTAMPTZ NOT NULL,
  created_at TIMESTAMPTZ NOT NULL,
  updated_at TIMESTAMPTZ NOT NULL,
  published_at TIMESTAMPTZ,
  CONSTRAINT chk_user_account_deletion_outbox_identity CHECK (
    aggregate_id > 0 AND length(btrim(event_type)) > 0 AND length(btrim(message_key)) > 0
  ),
  CONSTRAINT chk_user_account_deletion_outbox_attempts CHECK (attempts >= 0),
  CONSTRAINT chk_user_account_deletion_outbox_status CHECK (
    status IN ('pending', 'publishing', 'failed', 'published')
  ),
  CONSTRAINT chk_user_account_deletion_outbox_lifecycle CHECK (
    (status = 'pending' AND attempts = 0 AND lease_owner IS NULL AND lease_expires_at IS NULL AND last_error = '' AND published_at IS NULL) OR
    (status = 'publishing' AND attempts > 0 AND length(btrim(lease_owner)) > 0 AND lease_expires_at IS NOT NULL AND published_at IS NULL) OR
    (status = 'failed' AND attempts > 0 AND lease_owner IS NULL AND lease_expires_at IS NULL AND length(btrim(last_error)) > 0 AND published_at IS NULL) OR
    (status = 'published' AND attempts > 0 AND lease_owner IS NULL AND lease_expires_at IS NULL AND last_error = '' AND published_at IS NOT NULL)
  )
);

CREATE INDEX IF NOT EXISTS idx_user_account_deletion_outbox_claim
  ON user_account_deletion_outbox (available_at, created_at, event_id)
  WHERE status IN ('pending', 'failed', 'publishing');
