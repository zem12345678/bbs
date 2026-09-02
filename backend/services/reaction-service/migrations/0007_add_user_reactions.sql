CREATE TABLE IF NOT EXISTS user_reactions (
  id BIGSERIAL PRIMARY KEY,
  user_id BIGINT NOT NULL,
  entity_type VARCHAR(32) NOT NULL,
  entity_id BIGINT NOT NULL,
  reaction VARCHAR(128) NOT NULL,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE UNIQUE INDEX IF NOT EXISTS ux_user_reactions_identity
  ON user_reactions(user_id, entity_type, entity_id);

CREATE INDEX IF NOT EXISTS idx_user_reactions_user_created
  ON user_reactions(user_id, created_at DESC, id DESC);

CREATE INDEX IF NOT EXISTS idx_user_reactions_entity
  ON user_reactions(entity_type, entity_id, created_at DESC);

ALTER TABLE reaction_erased_users
  ADD COLUMN IF NOT EXISTS deleted_reactions BIGINT NOT NULL DEFAULT 0;
