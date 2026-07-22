CREATE INDEX IF NOT EXISTS idx_credit_balances_leaderboard
  ON credit_balances(total DESC, user_id DESC)
  WHERE total > 0;

CREATE TABLE IF NOT EXISTS credit_leaderboard_state (
  id BOOLEAN PRIMARY KEY DEFAULT TRUE CHECK (id),
  revision BIGINT NOT NULL DEFAULT 0
);

INSERT INTO credit_leaderboard_state(id, revision)
VALUES(TRUE, 0)
ON CONFLICT(id) DO NOTHING;
