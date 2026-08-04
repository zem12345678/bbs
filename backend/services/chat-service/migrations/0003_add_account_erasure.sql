CREATE TABLE IF NOT EXISTS chat_erased_users (
  user_id BIGINT PRIMARY KEY,
  deletion_job_id BIGINT NOT NULL,
  policy_version INTEGER NOT NULL,
  redacted_messages BIGINT NOT NULL DEFAULT 0,
  deleted_memberships BIGINT NOT NULL DEFAULT 0,
  deleted_groups BIGINT NOT NULL DEFAULT 0,
  transferred_rooms BIGINT NOT NULL DEFAULT 0,
  closed_rooms BIGINT NOT NULL DEFAULT 0,
  suppressed_outbox_events BIGINT NOT NULL DEFAULT 0,
  erased_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_chat_erased_users_job
  ON chat_erased_users(deletion_job_id);

ALTER TABLE chat_messages
  DROP CONSTRAINT IF EXISTS chat_messages_room_id_sender_id_client_message_id_key;

CREATE UNIQUE INDEX IF NOT EXISTS uq_chat_messages_live_sender_client
  ON chat_messages(room_id, sender_id, client_message_id)
  WHERE sender_id > 0;
