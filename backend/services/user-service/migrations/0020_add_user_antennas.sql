CREATE TABLE IF NOT EXISTS antennas (
  id BIGINT PRIMARY KEY,
  owner_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  name VARCHAR(100) NOT NULL,
  source VARCHAR(32) NOT NULL,
  user_list_id BIGINT REFERENCES user_lists(id) ON DELETE CASCADE,
  keywords JSONB NOT NULL DEFAULT '[]'::jsonb,
  exclude_keywords JSONB NOT NULL DEFAULT '[]'::jsonb,
  users JSONB NOT NULL DEFAULT '[]'::jsonb,
  case_sensitive BOOLEAN NOT NULL DEFAULT FALSE,
  local_only BOOLEAN NOT NULL DEFAULT FALSE,
  exclude_bots BOOLEAN NOT NULL DEFAULT FALSE,
  with_replies BOOLEAN NOT NULL DEFAULT FALSE,
  with_file BOOLEAN NOT NULL DEFAULT FALSE,
  exclude_notes_in_sensitive_channel BOOLEAN NOT NULL DEFAULT FALSE,
  is_active BOOLEAN NOT NULL DEFAULT TRUE,
  created_at TIMESTAMPTZ NOT NULL,
  updated_at TIMESTAMPTZ NOT NULL,
  last_used_at TIMESTAMPTZ NOT NULL,
  CONSTRAINT chk_antennas_source CHECK (source IN ('home', 'all', 'users', 'list', 'users_blacklist')),
  CONSTRAINT chk_antennas_list_source CHECK ((source = 'list' AND user_list_id IS NOT NULL) OR (source <> 'list' AND user_list_id IS NULL))
);

CREATE INDEX IF NOT EXISTS idx_antennas_owner_created
  ON antennas (owner_id, created_at DESC, id DESC);
