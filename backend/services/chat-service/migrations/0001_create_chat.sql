CREATE TABLE IF NOT EXISTS chat_rooms (
  id BIGINT PRIMARY KEY,
  room_no VARCHAR(12) NOT NULL UNIQUE,
  name VARCHAR(80) NOT NULL,
  creator_id BIGINT NOT NULL,
  announcement TEXT NOT NULL DEFAULT '',
  announcement_version BIGINT NOT NULL DEFAULT 0,
  last_message_seq BIGINT NOT NULL DEFAULT 0,
  status SMALLINT NOT NULL DEFAULT 1,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  CONSTRAINT chk_chat_rooms_status CHECK (status IN (1, 2)),
  CONSTRAINT chk_chat_rooms_announcement_version CHECK (announcement_version >= 0),
  CONSTRAINT chk_chat_rooms_last_message_seq CHECK (last_message_seq >= 0)
);

CREATE TABLE IF NOT EXISTS chat_room_groups (
  id BIGINT PRIMARY KEY,
  user_id BIGINT NOT NULL,
  name VARCHAR(40) NOT NULL,
  sort_order INTEGER NOT NULL DEFAULT 0,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  UNIQUE (id, user_id)
);

CREATE UNIQUE INDEX IF NOT EXISTS uq_chat_room_groups_user_name
  ON chat_room_groups(user_id, LOWER(name));

CREATE TABLE IF NOT EXISTS chat_room_members (
  room_id BIGINT NOT NULL REFERENCES chat_rooms(id),
  user_id BIGINT NOT NULL,
  role SMALLINT NOT NULL DEFAULT 2,
  status SMALLINT NOT NULL DEFAULT 1,
  joined_at_seq BIGINT NOT NULL DEFAULT 0,
  last_read_seq BIGINT NOT NULL DEFAULT 0,
  last_seen_announcement_version BIGINT NOT NULL DEFAULT 0,
  group_id BIGINT,
  sort_order INTEGER NOT NULL DEFAULT 0,
  joined_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  left_at TIMESTAMPTZ,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  PRIMARY KEY (room_id, user_id),
  CONSTRAINT fk_chat_room_members_group
    FOREIGN KEY (group_id, user_id) REFERENCES chat_room_groups(id, user_id),
  CONSTRAINT chk_chat_room_members_role CHECK (role IN (1, 2)),
  CONSTRAINT chk_chat_room_members_status CHECK (status IN (1, 2)),
  CONSTRAINT chk_chat_room_members_joined_seq CHECK (joined_at_seq >= 0),
  CONSTRAINT chk_chat_room_members_read_seq CHECK (last_read_seq >= 0),
  CONSTRAINT chk_chat_room_members_announcement_version CHECK (last_seen_announcement_version >= 0)
);

CREATE TABLE IF NOT EXISTS chat_messages (
  id BIGINT PRIMARY KEY,
  room_id BIGINT NOT NULL REFERENCES chat_rooms(id),
  seq BIGINT NOT NULL,
  sender_id BIGINT NOT NULL,
  client_message_id UUID NOT NULL,
  body TEXT NOT NULL,
  status SMALLINT NOT NULL DEFAULT 1,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  deleted_at TIMESTAMPTZ,
  UNIQUE (room_id, seq),
  UNIQUE (room_id, sender_id, client_message_id),
  CONSTRAINT chk_chat_messages_seq CHECK (seq > 0),
  CONSTRAINT chk_chat_messages_status CHECK (status IN (1, 2))
);

CREATE TABLE IF NOT EXISTS chat_outbox (
  event_id UUID PRIMARY KEY,
  aggregate_type VARCHAR(32) NOT NULL,
  aggregate_id VARCHAR(64) NOT NULL,
  event_type VARCHAR(96) NOT NULL,
  event_version INTEGER NOT NULL DEFAULT 1,
  partition_key VARCHAR(64) NOT NULL,
  payload JSONB NOT NULL,
  status VARCHAR(16) NOT NULL DEFAULT 'pending',
  attempts INTEGER NOT NULL DEFAULT 0,
  next_attempt_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  published_at TIMESTAMPTZ,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  CONSTRAINT chk_chat_outbox_status CHECK (status IN ('pending', 'publishing', 'published', 'failed')),
  CONSTRAINT chk_chat_outbox_attempts CHECK (attempts >= 0)
);

CREATE INDEX IF NOT EXISTS idx_chat_room_members_user_sidebar
  ON chat_room_members(user_id, status, group_id, sort_order, room_id);

CREATE INDEX IF NOT EXISTS idx_chat_room_members_room_active
  ON chat_room_members(room_id, status, user_id);

CREATE INDEX IF NOT EXISTS idx_chat_messages_room_seq
  ON chat_messages(room_id, seq);

CREATE INDEX IF NOT EXISTS idx_chat_outbox_pending
  ON chat_outbox(status, next_attempt_at, created_at)
  WHERE status IN ('pending', 'failed');
