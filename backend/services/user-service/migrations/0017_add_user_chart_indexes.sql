CREATE INDEX IF NOT EXISTS idx_users_created_at
  ON users (created_at);

CREATE INDEX IF NOT EXISTS idx_users_deleted_at
  ON users (deleted_at)
  WHERE deleted_at IS NOT NULL;
