CREATE TABLE IF NOT EXISTS favorite_collections (
  id BIGSERIAL PRIMARY KEY,
  user_id BIGINT NOT NULL,
  name VARCHAR(80) NOT NULL CHECK (char_length(name) BETWEEN 1 AND 80),
  description TEXT NOT NULL DEFAULT '' CHECK (char_length(description) <= 500),
  is_public BOOLEAN NOT NULL DEFAULT FALSE,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE UNIQUE INDEX IF NOT EXISTS ux_favorite_collections_owner_name
  ON favorite_collections(user_id, lower(name));

CREATE INDEX IF NOT EXISTS idx_favorite_collections_owner_created
  ON favorite_collections(user_id, created_at DESC, id DESC);

CREATE TABLE IF NOT EXISTS favorite_collection_items (
  id BIGSERIAL PRIMARY KEY,
  collection_id BIGINT NOT NULL REFERENCES favorite_collections(id) ON DELETE CASCADE,
  entity_type VARCHAR(32) NOT NULL CHECK (entity_type IN ('article', 'topic')),
  entity_id BIGINT NOT NULL,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  UNIQUE(collection_id, entity_type, entity_id)
);

CREATE INDEX IF NOT EXISTS idx_favorite_collection_items_list
  ON favorite_collection_items(collection_id, created_at DESC, id DESC);

CREATE INDEX IF NOT EXISTS idx_favorite_collection_items_entity
  ON favorite_collection_items(entity_type, entity_id);
