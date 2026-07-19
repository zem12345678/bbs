ALTER TABLE topics
  ADD COLUMN IF NOT EXISTS qa_acceptance_cycle BIGINT NOT NULL DEFAULT 0;

ALTER TABLE qa_acceptance_outbox
  DROP CONSTRAINT IF EXISTS qa_acceptance_outbox_topic_id_key;

DROP INDEX IF EXISTS idx_qa_acceptance_outbox_topic_id;

CREATE INDEX IF NOT EXISTS idx_qa_acceptance_outbox_topic_created
  ON qa_acceptance_outbox(topic_id, created_at);
