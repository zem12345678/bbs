CREATE TABLE IF NOT EXISTS registry_items (
  id BIGINT PRIMARY KEY,
  user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  domain VARCHAR(512),
  scope JSONB NOT NULL DEFAULT '[]'::jsonb,
  key VARCHAR(1024) NOT NULL,
  value JSONB NOT NULL DEFAULT 'null'::jsonb,
  created_at TIMESTAMPTZ NOT NULL,
  updated_at TIMESTAMPTZ NOT NULL,
  CONSTRAINT chk_registry_items_key CHECK (char_length(key) BETWEEN 1 AND 1024)
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_registry_items_identity
  ON registry_items (user_id, key, scope, domain) NULLS NOT DISTINCT;

CREATE INDEX IF NOT EXISTS idx_registry_items_scope
  ON registry_items (user_id, domain, scope);
