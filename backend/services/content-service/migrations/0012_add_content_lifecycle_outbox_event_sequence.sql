ALTER TABLE content_lifecycle_outbox
  ADD COLUMN IF NOT EXISTS event_sequence BIGINT;

CREATE SEQUENCE IF NOT EXISTS content_lifecycle_outbox_event_sequence_seq;

ALTER SEQUENCE content_lifecycle_outbox_event_sequence_seq
  OWNED BY content_lifecycle_outbox.event_sequence;

ALTER TABLE content_lifecycle_outbox
  ALTER COLUMN event_sequence SET DEFAULT nextval('content_lifecycle_outbox_event_sequence_seq');

WITH ordered AS (
  SELECT event_id, ROW_NUMBER() OVER (ORDER BY created_at ASC, event_id ASC) AS ordinal
  FROM content_lifecycle_outbox
  WHERE event_sequence IS NULL
), base AS (
  SELECT COALESCE(MAX(event_sequence), 0) AS sequence_start
  FROM content_lifecycle_outbox
)
UPDATE content_lifecycle_outbox AS outbox
SET event_sequence = base.sequence_start + ordered.ordinal
FROM ordered
CROSS JOIN base
WHERE outbox.event_id = ordered.event_id;

SELECT setval(
  'content_lifecycle_outbox_event_sequence_seq',
  COALESCE((SELECT MAX(event_sequence) FROM content_lifecycle_outbox), 1),
  EXISTS (SELECT 1 FROM content_lifecycle_outbox)
);

ALTER TABLE content_lifecycle_outbox
  ALTER COLUMN event_sequence SET NOT NULL;

CREATE UNIQUE INDEX IF NOT EXISTS idx_content_lifecycle_outbox_event_sequence
  ON content_lifecycle_outbox(event_sequence);

DROP INDEX IF EXISTS idx_content_lifecycle_outbox_message_sequence;

CREATE INDEX IF NOT EXISTS idx_content_lifecycle_outbox_message_event_sequence
  ON content_lifecycle_outbox(message_key, event_sequence)
  WHERE status <> 'PUBLISHED';

CREATE INDEX IF NOT EXISTS idx_content_lifecycle_outbox_dispatch_event_sequence
  ON content_lifecycle_outbox(status, next_attempt_at, lease_expires_at, event_sequence);
