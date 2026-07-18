CREATE TABLE IF NOT EXISTS check_ins (
  id BIGSERIAL PRIMARY KEY,
  user_id BIGINT NOT NULL UNIQUE,
  latest_day DATE NOT NULL,
  consecutive_days INTEGER NOT NULL DEFAULT 1,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  CHECK (user_id > 0),
  CHECK (consecutive_days > 0)
);

CREATE INDEX IF NOT EXISTS idx_check_ins_latest_day
  ON check_ins(latest_day DESC, user_id ASC);
