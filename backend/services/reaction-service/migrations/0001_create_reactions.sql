CREATE TABLE IF NOT EXISTS user_likes (
  id BIGSERIAL PRIMARY KEY,
  user_id BIGINT NOT NULL,
  entity_type VARCHAR(32) NOT NULL,
  entity_id BIGINT NOT NULL,
  status SMALLINT NOT NULL DEFAULT 1,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  UNIQUE(user_id, entity_type, entity_id)
);

CREATE INDEX IF NOT EXISTS idx_user_likes_entity_status
  ON user_likes(entity_type, entity_id, status);

CREATE INDEX IF NOT EXISTS idx_user_likes_user_status_created
  ON user_likes(user_id, status, created_at DESC);

CREATE TABLE IF NOT EXISTS favorites (
  id BIGSERIAL PRIMARY KEY,
  user_id BIGINT NOT NULL,
  entity_type VARCHAR(32) NOT NULL,
  entity_id BIGINT NOT NULL,
  deleted_at TIMESTAMPTZ,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  UNIQUE(user_id, entity_type, entity_id)
);

CREATE INDEX IF NOT EXISTS idx_favorites_user_deleted_created
  ON favorites(user_id, deleted_at, created_at DESC);

CREATE INDEX IF NOT EXISTS idx_favorites_entity_deleted
  ON favorites(entity_type, entity_id, deleted_at);

CREATE TABLE IF NOT EXISTS user_reports (
  id BIGSERIAL PRIMARY KEY,
  entity_type VARCHAR(32) NOT NULL,
  entity_id BIGINT NOT NULL,
  reporter_id BIGINT NOT NULL,
  reason VARCHAR(64) NOT NULL,
  description TEXT NOT NULL DEFAULT '',
  status SMALLINT NOT NULL DEFAULT 1,
  handled_by BIGINT NOT NULL DEFAULT 0,
  handled_at TIMESTAMPTZ,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  UNIQUE(entity_type, entity_id, reporter_id, status)
);

CREATE INDEX IF NOT EXISTS idx_user_reports_status_created
  ON user_reports(status, created_at DESC);

CREATE INDEX IF NOT EXISTS idx_user_reports_entity_status
  ON user_reports(entity_type, entity_id, status);

CREATE INDEX IF NOT EXISTS idx_user_reports_reporter_created
  ON user_reports(reporter_id, created_at DESC);
