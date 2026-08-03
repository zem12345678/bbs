ALTER TABLE user_mfa_totp
  ADD COLUMN IF NOT EXISTS passwordless_enabled BOOLEAN NOT NULL DEFAULT FALSE;

CREATE TABLE IF NOT EXISTS user_passkeys (
  credential_id TEXT PRIMARY KEY,
  user_id BIGINT NOT NULL REFERENCES user_mfa_totp(user_id) ON DELETE CASCADE,
  name VARCHAR(120) NOT NULL,
  credential_ciphertext TEXT NOT NULL,
  version BIGINT NOT NULL DEFAULT 1,
  backup_eligible BOOLEAN NOT NULL DEFAULT FALSE,
  backup_state BOOLEAN NOT NULL DEFAULT FALSE,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  last_used_at TIMESTAMPTZ,
  CONSTRAINT chk_user_passkeys_credential_id CHECK (char_length(credential_id) BETWEEN 1 AND 2048),
  CONSTRAINT chk_user_passkeys_name CHECK (char_length(btrim(name)) BETWEEN 1 AND 30),
  CONSTRAINT chk_user_passkeys_ciphertext CHECK (credential_ciphertext <> ''),
  CONSTRAINT chk_user_passkeys_version CHECK (version > 0)
);

CREATE INDEX IF NOT EXISTS idx_user_passkeys_user_created
  ON user_passkeys (user_id, created_at, credential_id);

CREATE TABLE IF NOT EXISTS user_passkey_challenges (
  token_hash CHAR(64) PRIMARY KEY,
  ceremony VARCHAR(20) NOT NULL,
  user_id BIGINT REFERENCES users(id) ON DELETE CASCADE,
  mfa_token_hash CHAR(64) REFERENCES user_mfa_login_challenges(token_hash) ON DELETE CASCADE,
  passkey_name VARCHAR(120) NOT NULL DEFAULT '',
  session_ciphertext TEXT NOT NULL,
  expires_at TIMESTAMPTZ NOT NULL,
  attempts SMALLINT NOT NULL DEFAULT 0,
  used_at TIMESTAMPTZ,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  CONSTRAINT chk_user_passkey_challenges_ceremony CHECK (ceremony IN ('registration', 'mfa', 'passwordless')),
  CONSTRAINT chk_user_passkey_challenges_attempts CHECK (attempts BETWEEN 0 AND 5),
  CONSTRAINT chk_user_passkey_challenges_session CHECK (session_ciphertext <> ''),
  CONSTRAINT chk_user_passkey_challenges_shape CHECK (
    (ceremony = 'registration' AND user_id IS NOT NULL AND mfa_token_hash IS NULL AND char_length(btrim(passkey_name)) BETWEEN 1 AND 30) OR
    (ceremony = 'mfa' AND user_id IS NOT NULL AND mfa_token_hash IS NOT NULL AND passkey_name = '') OR
    (ceremony = 'passwordless' AND user_id IS NULL AND mfa_token_hash IS NULL AND passkey_name = '')
  )
);

CREATE INDEX IF NOT EXISTS idx_user_passkey_challenges_user_created
  ON user_passkey_challenges (user_id, created_at DESC)
  WHERE user_id IS NOT NULL;
CREATE INDEX IF NOT EXISTS idx_user_passkey_challenges_expiry
  ON user_passkey_challenges (expires_at)
  WHERE used_at IS NULL;
CREATE INDEX IF NOT EXISTS idx_user_passkey_challenges_expiry_all
  ON user_passkey_challenges (expires_at);
