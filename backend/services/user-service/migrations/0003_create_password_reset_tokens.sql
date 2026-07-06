CREATE TABLE IF NOT EXISTS user_password_reset_tokens (
  token_hash TEXT PRIMARY KEY,
  user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  email VARCHAR(255) NOT NULL,
  expires_at TIMESTAMPTZ NOT NULL,
  used_at TIMESTAMPTZ NULL,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_user_password_reset_tokens_user_id ON user_password_reset_tokens(user_id);
CREATE INDEX IF NOT EXISTS idx_user_password_reset_tokens_expires_at ON user_password_reset_tokens(expires_at);
