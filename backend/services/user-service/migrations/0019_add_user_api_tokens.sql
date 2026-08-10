ALTER TABLE user_sessions
  ADD COLUMN IF NOT EXISTS api_token_name VARCHAR(128) NOT NULL DEFAULT '',
  ADD COLUMN IF NOT EXISTS api_token_scopes VARCHAR(32) NOT NULL DEFAULT '',
  ADD COLUMN IF NOT EXISTS api_token_credential_version VARCHAR(128) NOT NULL DEFAULT '';

ALTER TABLE user_sessions
  DROP CONSTRAINT IF EXISTS chk_user_sessions_api_token_metadata;

ALTER TABLE user_sessions
  ADD CONSTRAINT chk_user_sessions_api_token_metadata CHECK (
    (
      login_method = 'api_token'
      AND char_length(btrim(api_token_name)) BETWEEN 1 AND 128
      AND api_token_scopes IN ('read', 'write', 'read,write')
      AND char_length(btrim(api_token_credential_version)) BETWEEN 1 AND 128
    )
    OR (
      login_method <> 'api_token'
      AND api_token_name = ''
      AND api_token_scopes = ''
      AND api_token_credential_version = ''
    )
  );

CREATE INDEX IF NOT EXISTS idx_user_sessions_api_tokens
  ON user_sessions (user_id, created_at DESC, session_id)
  WHERE login_method = 'api_token';
