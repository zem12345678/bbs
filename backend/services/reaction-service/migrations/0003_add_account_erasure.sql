CREATE TABLE IF NOT EXISTS reaction_erased_users (
  user_id BIGINT PRIMARY KEY,
  deletion_job_id BIGINT NOT NULL,
  policy_version INTEGER NOT NULL,
  deleted_likes BIGINT NOT NULL DEFAULT 0,
  deleted_favorites BIGINT NOT NULL DEFAULT 0,
  deleted_collections BIGINT NOT NULL DEFAULT 0,
  anonymized_reports BIGINT NOT NULL DEFAULT 0,
  anonymized_handled_reports BIGINT NOT NULL DEFAULT 0,
  erased_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_reaction_erased_users_job
  ON reaction_erased_users(deletion_job_id);

ALTER TABLE user_reports
  DROP CONSTRAINT IF EXISTS user_reports_entity_type_entity_id_reporter_id_status_key;

DROP INDEX IF EXISTS idx_reports_unique_open;

CREATE UNIQUE INDEX IF NOT EXISTS ux_user_reports_live_identity_status
  ON user_reports(entity_type, entity_id, reporter_id, status)
  WHERE reporter_id > 0;
