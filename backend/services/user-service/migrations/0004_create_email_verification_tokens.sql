ALTER TABLE users
  ADD COLUMN IF NOT EXISTS email_verified_at TIMESTAMPTZ NULL;

CREATE INDEX IF NOT EXISTS idx_users_email_verified_at ON users(email_verified_at);

CREATE TABLE IF NOT EXISTS user_email_verification_tokens (
  token_hash TEXT PRIMARY KEY,
  user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  email VARCHAR(255) NOT NULL,
  expires_at TIMESTAMPTZ NOT NULL,
  used_at TIMESTAMPTZ NULL,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_user_email_verification_tokens_user_id ON user_email_verification_tokens(user_id);
CREATE INDEX IF NOT EXISTS idx_user_email_verification_tokens_expires_at ON user_email_verification_tokens(expires_at);
