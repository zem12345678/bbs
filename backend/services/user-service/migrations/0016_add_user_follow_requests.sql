-- Private accounts gate new followers behind an approval step. The flag lives on
-- users so profile reads need no extra join, and pending requests live in their
-- own table so an approval can be replayed into user_follows transactionally.
ALTER TABLE users
  ADD COLUMN IF NOT EXISTS follow_approval_required BOOLEAN NOT NULL DEFAULT FALSE;

CREATE TABLE IF NOT EXISTS user_follow_requests (
  id BIGINT PRIMARY KEY,
  requester_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  target_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  CONSTRAINT chk_user_follow_requests_not_self CHECK (requester_id <> target_id)
);

-- A requester may only have one pending request per target. Accepting or
-- rejecting deletes the row, so uniqueness needs no status predicate.
CREATE UNIQUE INDEX IF NOT EXISTS uk_user_follow_requests_pair
  ON user_follow_requests (requester_id, target_id);
CREATE INDEX IF NOT EXISTS idx_user_follow_requests_target_created
  ON user_follow_requests (target_id, created_at DESC, id DESC);
CREATE INDEX IF NOT EXISTS idx_user_follow_requests_requester_created
  ON user_follow_requests (requester_id, created_at DESC, id DESC);