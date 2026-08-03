CREATE TABLE IF NOT EXISTS user_invite_codes (
  id BIGINT PRIMARY KEY,
  code VARCHAR(32) NOT NULL,
  created_by_admin_id BIGINT NOT NULL,
  used_by_user_id BIGINT NULL REFERENCES users(id) ON DELETE SET NULL,
  expires_at TIMESTAMPTZ NULL,
  used_at TIMESTAMPTZ NULL,
  revoked_at TIMESTAMPTZ NULL,
  revoked_by_admin_id BIGINT NULL,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE UNIQUE INDEX IF NOT EXISTS uk_user_invite_codes_code
  ON user_invite_codes (code);
CREATE UNIQUE INDEX IF NOT EXISTS uk_user_invite_codes_used_by
  ON user_invite_codes (used_by_user_id)
  WHERE used_by_user_id IS NOT NULL;
CREATE INDEX IF NOT EXISTS idx_user_invite_codes_created
  ON user_invite_codes (created_at DESC, id DESC);
CREATE INDEX IF NOT EXISTS idx_user_invite_codes_expires
  ON user_invite_codes (expires_at)
  WHERE used_at IS NULL AND revoked_at IS NULL;
CREATE INDEX IF NOT EXISTS idx_user_invite_codes_used
  ON user_invite_codes (used_at DESC)
  WHERE used_at IS NOT NULL;
CREATE INDEX IF NOT EXISTS idx_user_invite_codes_revoked
  ON user_invite_codes (revoked_at DESC)
  WHERE revoked_at IS NOT NULL;
