CREATE INDEX IF NOT EXISTS idx_content_lifecycle_outbox_message_sequence
  ON content_lifecycle_outbox(message_key, created_at, event_id)
  WHERE status <> 'PUBLISHED';
