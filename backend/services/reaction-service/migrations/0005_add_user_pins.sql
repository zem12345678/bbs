CREATE TABLE IF NOT EXISTS user_pins (
  id BIGSERIAL PRIMARY KEY,
  user_id BIGINT NOT NULL,
  entity_type VARCHAR(32) NOT NULL CHECK (entity_type IN ('article', 'topic')),
  entity_id BIGINT NOT NULL,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  UNIQUE(user_id, entity_type, entity_id)
);

CREATE INDEX IF NOT EXISTS idx_user_pins_user_created
  ON user_pins(user_id, created_at DESC, id DESC);
