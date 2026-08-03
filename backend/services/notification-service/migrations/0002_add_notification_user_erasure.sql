CREATE TABLE IF NOT EXISTS notification_erased_users (
  user_id BIGINT PRIMARY KEY,
  job_id BIGINT NOT NULL,
  policy_version INTEGER NOT NULL,
  erased_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  CONSTRAINT chk_notification_erased_users_user_id CHECK (user_id > 0),
  CONSTRAINT chk_notification_erased_users_job_id CHECK (job_id > 0),
  CONSTRAINT chk_notification_erased_users_policy_version CHECK (policy_version > 0)
);

CREATE TABLE IF NOT EXISTS notification_erased_content (
  entity_type VARCHAR(32) NOT NULL,
  entity_id BIGINT NOT NULL,
  owner_user_id BIGINT NOT NULL,
  job_id BIGINT NOT NULL,
  policy_version INTEGER NOT NULL,
  erased_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  PRIMARY KEY(entity_type, entity_id),
  CONSTRAINT chk_notification_erased_content_entity_id CHECK (entity_id > 0),
  CONSTRAINT chk_notification_erased_content_owner_user_id CHECK (owner_user_id > 0),
  CONSTRAINT chk_notification_erased_content_job_id CHECK (job_id > 0),
  CONSTRAINT chk_notification_erased_content_policy_version CHECK (policy_version > 0)
);

CREATE TABLE IF NOT EXISTS notification_erased_comments (
  comment_id BIGINT PRIMARY KEY,
  author_user_id BIGINT NOT NULL,
  job_id BIGINT NOT NULL,
  policy_version INTEGER NOT NULL,
  erased_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  CONSTRAINT chk_notification_erased_comments_comment_id CHECK (comment_id > 0),
  CONSTRAINT chk_notification_erased_comments_author_user_id CHECK (author_user_id > 0),
  CONSTRAINT chk_notification_erased_comments_job_id CHECK (job_id > 0),
  CONSTRAINT chk_notification_erased_comments_policy_version CHECK (policy_version > 0)
);

ALTER TABLE notification_erased_users
  ADD COLUMN IF NOT EXISTS policy_version INTEGER NOT NULL DEFAULT 1;

CREATE INDEX IF NOT EXISTS idx_notification_erased_users_job
  ON notification_erased_users(job_id);

CREATE INDEX IF NOT EXISTS idx_notification_erased_content_job
  ON notification_erased_content(job_id);

CREATE INDEX IF NOT EXISTS idx_notification_erased_comments_job
  ON notification_erased_comments(job_id);
