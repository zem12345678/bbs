CREATE TABLE IF NOT EXISTS user_blocks (
  actor_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  target_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  PRIMARY KEY (actor_id, target_id),
  CONSTRAINT chk_user_blocks_not_self CHECK (actor_id <> target_id)
);

CREATE INDEX IF NOT EXISTS idx_user_blocks_actor_created ON user_blocks (actor_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_user_blocks_target_actor ON user_blocks (target_id, actor_id);

CREATE TABLE IF NOT EXISTS user_mutes (
  actor_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  target_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  PRIMARY KEY (actor_id, target_id),
  CONSTRAINT chk_user_mutes_not_self CHECK (actor_id <> target_id)
);

CREATE INDEX IF NOT EXISTS idx_user_mutes_actor_created ON user_mutes (actor_id, created_at DESC);
