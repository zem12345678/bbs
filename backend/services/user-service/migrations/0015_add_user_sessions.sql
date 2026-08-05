CREATE TABLE IF NOT EXISTS user_sessions (
  session_id VARCHAR(128) PRIMARY KEY,
  user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  ip_address VARCHAR(64) NOT NULL DEFAULT '',
  user_agent VARCHAR(512) NOT NULL DEFAULT '',
  login_method VARCHAR(32) NOT NULL,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  expires_at TIMESTAMPTZ NOT NULL,
  revoked_at TIMESTAMPTZ,
  CONSTRAINT chk_user_sessions_session_id CHECK (char_length(btrim(session_id)) BETWEEN 16 AND 128),
  CONSTRAINT chk_user_sessions_login_method CHECK (char_length(btrim(login_method)) BETWEEN 1 AND 32),
  CONSTRAINT chk_user_sessions_expiry CHECK (expires_at > created_at),
  CONSTRAINT chk_user_sessions_revoked_at CHECK (revoked_at IS NULL OR revoked_at >= created_at)
);

CREATE INDEX IF NOT EXISTS idx_user_sessions_user_created
  ON user_sessions (user_id, created_at DESC, session_id);

CREATE INDEX IF NOT EXISTS idx_user_sessions_user_active
  ON user_sessions (user_id, expires_at DESC)
  WHERE revoked_at IS NULL;

CREATE TABLE IF NOT EXISTS user_login_events (
  id BIGINT PRIMARY KEY,
  user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  session_id VARCHAR(128) REFERENCES user_sessions(session_id) ON DELETE SET NULL,
  ip_address VARCHAR(64) NOT NULL DEFAULT '',
  user_agent VARCHAR(512) NOT NULL DEFAULT '',
  success BOOLEAN NOT NULL,
  failure_reason VARCHAR(64) NOT NULL DEFAULT '',
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  CONSTRAINT chk_user_login_events_shape CHECK (
    (success AND failure_reason = '') OR
    (NOT success AND session_id IS NULL AND char_length(btrim(failure_reason)) BETWEEN 1 AND 64)
  )
);

CREATE INDEX IF NOT EXISTS idx_user_login_events_user_created
  ON user_login_events (user_id, created_at DESC, id);
