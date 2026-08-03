CREATE TABLE IF NOT EXISTS content_erased_users (
  user_id BIGINT PRIMARY KEY,
  job_id BIGINT NOT NULL,
  policy_version INTEGER NOT NULL,
  archived_articles BIGINT NOT NULL DEFAULT 0,
  archived_topics BIGINT NOT NULL DEFAULT 0,
  deleted_poll_ballots BIGINT NOT NULL DEFAULT 0,
  erased_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  CONSTRAINT chk_content_erased_users_user_id CHECK (user_id > 0),
  CONSTRAINT chk_content_erased_users_job_id CHECK (job_id > 0),
  CONSTRAINT chk_content_erased_users_policy_version CHECK (policy_version > 0),
  CONSTRAINT chk_content_erased_users_counts CHECK (
    archived_articles >= 0 AND archived_topics >= 0 AND deleted_poll_ballots >= 0
  )
);

CREATE INDEX IF NOT EXISTS idx_content_erased_users_job
  ON content_erased_users(job_id);
