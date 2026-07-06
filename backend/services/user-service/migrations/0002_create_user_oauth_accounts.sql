CREATE TABLE IF NOT EXISTS user_oauth_accounts (
  id BIGSERIAL PRIMARY KEY,
  provider VARCHAR(32) NOT NULL,
  provider_user_id VARCHAR(128) NOT NULL,
  user_id BIGINT NOT NULL,
  username VARCHAR(128) NOT NULL DEFAULT '',
  email VARCHAR(255) NOT NULL DEFAULT '',
  nickname VARCHAR(128) NOT NULL DEFAULT '',
  avatar_url TEXT NOT NULL DEFAULT '',
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  last_login_at TIMESTAMPTZ,
  CONSTRAINT fk_user_oauth_accounts_user
    FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE
);

CREATE UNIQUE INDEX IF NOT EXISTS uk_user_oauth_provider_user
  ON user_oauth_accounts (provider, provider_user_id);
CREATE INDEX IF NOT EXISTS idx_user_oauth_user
  ON user_oauth_accounts (user_id);
CREATE INDEX IF NOT EXISTS idx_user_oauth_last_login
  ON user_oauth_accounts (last_login_at DESC);
