CREATE TABLE IF NOT EXISTS user_follow_lifecycles (
  id BIGSERIAL PRIMARY KEY,
  follower_id BIGINT NOT NULL,
  followee_id BIGINT NOT NULL,
  followed_at TIMESTAMPTZ NOT NULL,
  unfollowed_at TIMESTAMPTZ,
  CONSTRAINT chk_user_follow_lifecycles_not_self CHECK (follower_id <> followee_id),
  CONSTRAINT chk_user_follow_lifecycles_order CHECK (unfollowed_at IS NULL OR unfollowed_at >= followed_at)
);

CREATE UNIQUE INDEX IF NOT EXISTS uk_user_follow_lifecycles_open_pair
  ON user_follow_lifecycles (follower_id, followee_id)
  WHERE unfollowed_at IS NULL;

CREATE INDEX IF NOT EXISTS idx_user_follow_lifecycles_follower_followed
  ON user_follow_lifecycles (follower_id, followed_at);

CREATE INDEX IF NOT EXISTS idx_user_follow_lifecycles_follower_unfollowed
  ON user_follow_lifecycles (follower_id, unfollowed_at)
  WHERE unfollowed_at IS NOT NULL;

CREATE INDEX IF NOT EXISTS idx_user_follow_lifecycles_followee_followed
  ON user_follow_lifecycles (followee_id, followed_at);

CREATE INDEX IF NOT EXISTS idx_user_follow_lifecycles_followee_unfollowed
  ON user_follow_lifecycles (followee_id, unfollowed_at)
  WHERE unfollowed_at IS NOT NULL;

INSERT INTO user_follow_lifecycles (follower_id, followee_id, followed_at)
SELECT f.follower_id, f.followee_id, f.created_at
FROM user_follows f
WHERE NOT EXISTS (
  SELECT 1
  FROM user_follow_lifecycles l
  WHERE l.follower_id = f.follower_id
    AND l.followee_id = f.followee_id
    AND l.unfollowed_at IS NULL
);
