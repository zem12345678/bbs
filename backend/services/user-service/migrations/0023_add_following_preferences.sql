ALTER TABLE user_follows
  ADD COLUMN IF NOT EXISTS id BIGINT;

WITH ranked_follows AS (
  SELECT
    follower_id,
    followee_id,
    ROW_NUMBER() OVER (ORDER BY created_at ASC, follower_id ASC, followee_id ASC) AS generated_id
  FROM user_follows
  WHERE id IS NULL
)
UPDATE user_follows AS follows
SET id = ranked_follows.generated_id
FROM ranked_follows
WHERE follows.follower_id = ranked_follows.follower_id
  AND follows.followee_id = ranked_follows.followee_id;

ALTER TABLE user_follows
  ALTER COLUMN id SET NOT NULL,
  ADD COLUMN IF NOT EXISTS with_replies BOOLEAN NOT NULL DEFAULT FALSE,
  ADD COLUMN IF NOT EXISTS notify VARCHAR(16) NOT NULL DEFAULT 'none';

ALTER TABLE user_follows
  ADD CONSTRAINT chk_user_follows_id_positive CHECK (id > 0),
  ADD CONSTRAINT chk_user_follows_notify CHECK (notify IN ('normal', 'none'));

CREATE UNIQUE INDEX IF NOT EXISTS uk_user_follows_id
  ON user_follows (id);
CREATE INDEX IF NOT EXISTS idx_user_follows_follower_id_cursor
  ON user_follows (follower_id, id DESC);
CREATE INDEX IF NOT EXISTS idx_user_follows_followee_id_cursor
  ON user_follows (followee_id, id DESC);

ALTER TABLE user_follow_requests
  ADD COLUMN IF NOT EXISTS with_replies BOOLEAN NOT NULL DEFAULT FALSE;

CREATE INDEX IF NOT EXISTS idx_user_follow_requests_target_id_cursor
  ON user_follow_requests (target_id, id DESC);
CREATE INDEX IF NOT EXISTS idx_user_follow_requests_requester_id_cursor
  ON user_follow_requests (requester_id, id DESC);
