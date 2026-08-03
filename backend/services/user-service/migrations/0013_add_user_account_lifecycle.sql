ALTER TABLE users
  ADD COLUMN IF NOT EXISTS account_state VARCHAR(24) NOT NULL DEFAULT 'active',
  ADD COLUMN IF NOT EXISTS account_state_version BIGINT NOT NULL DEFAULT 1,
  ADD COLUMN IF NOT EXISTS protected_account BOOLEAN NOT NULL DEFAULT FALSE,
  ADD COLUMN IF NOT EXISTS deletion_requested_at TIMESTAMPTZ,
  ADD COLUMN IF NOT EXISTS deleted_at TIMESTAMPTZ;

ALTER TABLE users
  DROP CONSTRAINT IF EXISTS chk_users_account_state;

ALTER TABLE users
  ADD CONSTRAINT chk_users_account_state
  CHECK (account_state IN ('active', 'suspended', 'deletion_pending', 'anonymized'));

ALTER TABLE users
  DROP CONSTRAINT IF EXISTS chk_users_account_state_version;

ALTER TABLE users
  ADD CONSTRAINT chk_users_account_state_version
  CHECK (account_state_version > 0);

ALTER TABLE users
  DROP CONSTRAINT IF EXISTS chk_users_account_lifecycle_timestamps;

ALTER TABLE users
  ADD CONSTRAINT chk_users_account_lifecycle_timestamps
  CHECK (
    (account_state = 'active' AND deletion_requested_at IS NULL AND deleted_at IS NULL) OR
    (account_state = 'suspended' AND deletion_requested_at IS NULL AND deleted_at IS NULL) OR
    (account_state = 'deletion_pending' AND deletion_requested_at IS NOT NULL AND deleted_at IS NULL) OR
    (account_state = 'anonymized' AND deletion_requested_at IS NOT NULL AND deleted_at IS NOT NULL)
  );

CREATE INDEX IF NOT EXISTS idx_users_account_state_created
  ON users (account_state, created_at DESC);

CREATE TABLE IF NOT EXISTS user_account_jobs (
  id BIGINT PRIMARY KEY,
  user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE RESTRICT,
  kind VARCHAR(32) NOT NULL,
  status VARCHAR(20) NOT NULL DEFAULT 'pending',
  policy_version INTEGER NOT NULL DEFAULT 1,
  attempts SMALLINT NOT NULL DEFAULT 0,
  available_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  lease_owner TEXT,
  lease_expires_at TIMESTAMPTZ,
  last_error TEXT NOT NULL DEFAULT '',
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  started_at TIMESTAMPTZ,
  completed_at TIMESTAMPTZ,
  CONSTRAINT chk_user_account_jobs_kind CHECK (kind = 'delete_account'),
  CONSTRAINT chk_user_account_jobs_status CHECK (status IN ('pending', 'running', 'retry_wait', 'blocked', 'succeeded', 'failed')),
  CONSTRAINT chk_user_account_jobs_policy CHECK (policy_version > 0),
  CONSTRAINT chk_user_account_jobs_attempts CHECK (attempts BETWEEN 0 AND 100),
  CONSTRAINT chk_user_account_jobs_lease CHECK (
    (lease_owner IS NULL AND lease_expires_at IS NULL) OR
    (lease_owner IS NOT NULL AND length(btrim(lease_owner)) > 0 AND lease_expires_at IS NOT NULL)
  )
);

CREATE UNIQUE INDEX IF NOT EXISTS uk_user_account_jobs_active_deletion
  ON user_account_jobs (user_id)
  WHERE kind = 'delete_account' AND status IN ('pending', 'running', 'retry_wait', 'blocked');

CREATE INDEX IF NOT EXISTS idx_user_account_jobs_claim
  ON user_account_jobs (available_at, created_at, id)
  WHERE status IN ('pending', 'retry_wait');

CREATE TABLE IF NOT EXISTS user_account_job_steps (
  job_id BIGINT NOT NULL REFERENCES user_account_jobs(id) ON DELETE CASCADE,
  service VARCHAR(40) NOT NULL,
  status VARCHAR(20) NOT NULL DEFAULT 'pending',
  attempts SMALLINT NOT NULL DEFAULT 0,
  available_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  lease_owner TEXT,
  lease_expires_at TIMESTAMPTZ,
  last_error TEXT NOT NULL DEFAULT '',
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  completed_at TIMESTAMPTZ,
  PRIMARY KEY (job_id, service),
  CONSTRAINT chk_user_account_job_steps_service CHECK (length(btrim(service)) BETWEEN 1 AND 40),
  CONSTRAINT chk_user_account_job_steps_status CHECK (status IN ('pending', 'running', 'retry_wait', 'blocked', 'succeeded', 'failed')),
  CONSTRAINT chk_user_account_job_steps_attempts CHECK (attempts BETWEEN 0 AND 100),
  CONSTRAINT chk_user_account_job_steps_lease CHECK (
    (lease_owner IS NULL AND lease_expires_at IS NULL) OR
    (lease_owner IS NOT NULL AND length(btrim(lease_owner)) > 0 AND lease_expires_at IS NOT NULL)
  )
);

CREATE INDEX IF NOT EXISTS idx_user_account_job_steps_claim
  ON user_account_job_steps (available_at, created_at, job_id, service)
  WHERE status IN ('pending', 'retry_wait');

CREATE TABLE IF NOT EXISTS user_account_actions (
  id BIGSERIAL PRIMARY KEY,
  actor_user_id BIGINT NOT NULL,
  target_user_id BIGINT NOT NULL,
  action VARCHAR(40) NOT NULL,
  from_state VARCHAR(24) NOT NULL,
  to_state VARCHAR(24) NOT NULL,
  reason TEXT NOT NULL DEFAULT '',
  metadata JSONB NOT NULL DEFAULT '{}'::jsonb,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  CONSTRAINT chk_user_account_actions_ids CHECK (actor_user_id > 0 AND target_user_id > 0),
  CONSTRAINT chk_user_account_actions_action CHECK (length(btrim(action)) BETWEEN 1 AND 40),
  CONSTRAINT chk_user_account_actions_states CHECK (
    from_state IN ('active', 'suspended', 'deletion_pending', 'anonymized') AND
    to_state IN ('active', 'suspended', 'deletion_pending', 'anonymized')
  )
);

CREATE INDEX IF NOT EXISTS idx_user_account_actions_target_created
  ON user_account_actions (target_user_id, created_at DESC, id DESC);
