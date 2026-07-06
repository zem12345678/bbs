CREATE TABLE IF NOT EXISTS users (
  id BIGINT PRIMARY KEY,
  username VARCHAR(32) NOT NULL,
  email VARCHAR(255) NOT NULL,
  password_hash TEXT NOT NULL,
  nickname VARCHAR(64) NOT NULL,
  avatar_url TEXT NOT NULL DEFAULT '',
  bio TEXT NOT NULL DEFAULT '',
  status SMALLINT NOT NULL DEFAULT 1,
  follower_count BIGINT NOT NULL DEFAULT 0,
  following_count BIGINT NOT NULL DEFAULT 0,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  last_login_at TIMESTAMPTZ,
  CONSTRAINT chk_users_status CHECK (status IN (1, 2)),
  CONSTRAINT chk_users_counts CHECK (follower_count >= 0 AND following_count >= 0)
);

CREATE UNIQUE INDEX IF NOT EXISTS uk_users_username ON users (username);
CREATE UNIQUE INDEX IF NOT EXISTS uk_users_email ON users (email);
CREATE INDEX IF NOT EXISTS idx_users_status_created ON users (status, created_at DESC);

CREATE TABLE IF NOT EXISTS user_follows (
  follower_id BIGINT NOT NULL,
  followee_id BIGINT NOT NULL,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  PRIMARY KEY (follower_id, followee_id),
  CONSTRAINT chk_user_follows_not_self CHECK (follower_id <> followee_id)
);

CREATE INDEX IF NOT EXISTS idx_user_follows_followee_created ON user_follows (followee_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_user_follows_follower_created ON user_follows (follower_id, created_at DESC);

