CREATE TABLE IF NOT EXISTS channels (
  id BIGINT PRIMARY KEY,
  owner_id BIGINT NOT NULL,
  category_id BIGINT REFERENCES categories(id) ON DELETE SET NULL,
  name VARCHAR(128) NOT NULL,
  description TEXT NOT NULL DEFAULT '',
  color VARCHAR(16) NOT NULL DEFAULT '#3b82f6',
  is_archived BOOLEAN NOT NULL DEFAULT FALSE,
  created_at TIMESTAMPTZ NOT NULL,
  updated_at TIMESTAMPTZ NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_channels_owner_archived_updated
  ON channels(owner_id, is_archived, updated_at DESC);

CREATE INDEX IF NOT EXISTS idx_channels_category_archived_updated
  ON channels(category_id, is_archived, updated_at DESC);

CREATE TABLE IF NOT EXISTS channel_followers (
  channel_id BIGINT NOT NULL REFERENCES channels(id) ON DELETE CASCADE,
  user_id BIGINT NOT NULL,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  PRIMARY KEY (channel_id, user_id)
);

CREATE INDEX IF NOT EXISTS idx_channel_followers_user_created
  ON channel_followers(user_id, created_at DESC);

CREATE TABLE IF NOT EXISTS channel_favorites (
  channel_id BIGINT NOT NULL REFERENCES channels(id) ON DELETE CASCADE,
  user_id BIGINT NOT NULL,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  PRIMARY KEY (channel_id, user_id)
);

CREATE INDEX IF NOT EXISTS idx_channel_favorites_user_created
  ON channel_favorites(user_id, created_at DESC);

ALTER TABLE topics
  ADD COLUMN IF NOT EXISTS channel_id BIGINT;

ALTER TABLE topics
  DROP CONSTRAINT IF EXISTS fk_topics_channel;

ALTER TABLE topics
  ADD CONSTRAINT fk_topics_channel
  FOREIGN KEY (channel_id) REFERENCES channels(id) ON DELETE SET NULL;

CREATE INDEX IF NOT EXISTS idx_topics_channel_status_published
  ON topics(channel_id, status, published_at DESC);
