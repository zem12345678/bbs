-- Credential version is a durable per-user security state. Existing users
-- keep the legacy initial value so JWTs issued before their first password
-- rotation continue to carry and validate against "0".
ALTER TABLE users
  ADD COLUMN IF NOT EXISTS credential_version TEXT;

UPDATE users
  SET credential_version = '0'
  WHERE credential_version IS NULL OR length(btrim(credential_version)) = 0;

ALTER TABLE users
  ALTER COLUMN credential_version SET DEFAULT '0';

ALTER TABLE users
  ALTER COLUMN credential_version SET NOT NULL;

ALTER TABLE users
  DROP CONSTRAINT IF EXISTS chk_users_credential_version_not_blank;

ALTER TABLE users
  ADD CONSTRAINT chk_users_credential_version_not_blank
  CHECK (length(btrim(credential_version)) > 0);
