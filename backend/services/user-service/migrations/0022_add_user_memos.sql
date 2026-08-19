CREATE TABLE IF NOT EXISTS user_memos (
  user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  target_user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  memo TEXT NOT NULL,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  PRIMARY KEY (user_id, target_user_id),
  CONSTRAINT user_memos_memo_length CHECK (char_length(memo) <= 2048)
);

CREATE INDEX IF NOT EXISTS idx_user_memos_target ON user_memos(target_user_id);
