CREATE TABLE IF NOT EXISTS user_mfa_totp (
  user_id BIGINT PRIMARY KEY REFERENCES users(id) ON DELETE CASCADE,
  secret_ciphertext TEXT NOT NULL DEFAULT '',
  pending_secret_ciphertext TEXT NOT NULL DEFAULT '',
  enabled_at TIMESTAMPTZ,
  last_totp_step BIGINT NOT NULL DEFAULT -1,
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  CONSTRAINT chk_user_mfa_totp_enabled_secret CHECK (
    (enabled_at IS NULL AND secret_ciphertext = '') OR
    (enabled_at IS NOT NULL AND secret_ciphertext <> '')
  )
);

CREATE TABLE IF NOT EXISTS user_mfa_recovery_codes (
  code_hash CHAR(64) PRIMARY KEY,
  user_id BIGINT NOT NULL REFERENCES user_mfa_totp(user_id) ON DELETE CASCADE,
  used_at TIMESTAMPTZ,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_user_mfa_recovery_codes_available
  ON user_mfa_recovery_codes (user_id, created_at)
  WHERE used_at IS NULL;

CREATE TABLE IF NOT EXISTS user_mfa_login_challenges (
  token_hash CHAR(64) PRIMARY KEY,
  user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  expires_at TIMESTAMPTZ NOT NULL,
  attempts SMALLINT NOT NULL DEFAULT 0,
  used_at TIMESTAMPTZ,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  CONSTRAINT chk_user_mfa_login_challenges_attempts CHECK (attempts BETWEEN 0 AND 5)
);

CREATE INDEX IF NOT EXISTS idx_user_mfa_login_challenges_user_created
  ON user_mfa_login_challenges (user_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_user_mfa_login_challenges_expiry
  ON user_mfa_login_challenges (expires_at)
  WHERE used_at IS NULL;
