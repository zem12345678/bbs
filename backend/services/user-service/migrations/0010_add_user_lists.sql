CREATE TABLE IF NOT EXISTS user_lists (
  id BIGINT PRIMARY KEY,
  owner_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  name VARCHAR(100) NOT NULL,
  is_public BOOLEAN NOT NULL DEFAULT FALSE,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  CONSTRAINT chk_user_lists_name CHECK (char_length(btrim(name)) BETWEEN 1 AND 100)
);

CREATE UNIQUE INDEX IF NOT EXISTS uk_user_lists_owner_name_ci
  ON user_lists (owner_id, lower(name));
CREATE INDEX IF NOT EXISTS idx_user_lists_owner_created
  ON user_lists (owner_id, created_at DESC, id DESC);
CREATE INDEX IF NOT EXISTS idx_user_lists_public_created
  ON user_lists (created_at DESC, id DESC)
  WHERE is_public = TRUE;

CREATE TABLE IF NOT EXISTS user_list_memberships (
  list_id BIGINT NOT NULL REFERENCES user_lists(id) ON DELETE CASCADE,
  user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  PRIMARY KEY (list_id, user_id)
);

CREATE INDEX IF NOT EXISTS idx_user_list_memberships_user_list
  ON user_list_memberships (user_id, list_id);
CREATE INDEX IF NOT EXISTS idx_user_list_memberships_list_created
  ON user_list_memberships (list_id, created_at DESC, user_id DESC);

CREATE TABLE IF NOT EXISTS user_list_favorites (
  list_id BIGINT NOT NULL REFERENCES user_lists(id) ON DELETE CASCADE,
  user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  PRIMARY KEY (list_id, user_id)
);

CREATE INDEX IF NOT EXISTS idx_user_list_favorites_user_created
  ON user_list_favorites (user_id, created_at DESC, list_id DESC);
