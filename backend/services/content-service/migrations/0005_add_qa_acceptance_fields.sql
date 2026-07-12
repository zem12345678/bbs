ALTER TABLE topics
  ADD COLUMN IF NOT EXISTS bounty_score BIGINT NOT NULL DEFAULT 0;

ALTER TABLE topics
  ADD COLUMN IF NOT EXISTS qa_status VARCHAR(16) NOT NULL DEFAULT '';

ALTER TABLE topics
  ADD COLUMN IF NOT EXISTS accepted_comment_id BIGINT NOT NULL DEFAULT 0;

ALTER TABLE topics
  ADD COLUMN IF NOT EXISTS accepted_comment_author_id BIGINT NOT NULL DEFAULT 0;

CREATE INDEX IF NOT EXISTS idx_topics_qa_status
  ON topics(qa_status);

CREATE INDEX IF NOT EXISTS idx_topics_accepted_comment
  ON topics(accepted_comment_id);

CREATE INDEX IF NOT EXISTS idx_topics_accepted_comment_author
  ON topics(accepted_comment_author_id);
