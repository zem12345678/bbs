ALTER TABLE users
  ADD COLUMN IF NOT EXISTS birthday VARCHAR(10),
  ADD COLUMN IF NOT EXISTS following_visibility VARCHAR(16) NOT NULL DEFAULT 'public',
  ADD COLUMN IF NOT EXISTS followers_visibility VARCHAR(16) NOT NULL DEFAULT 'public';

ALTER TABLE users
  DROP CONSTRAINT IF EXISTS chk_users_birthday,
  DROP CONSTRAINT IF EXISTS chk_users_following_visibility,
  DROP CONSTRAINT IF EXISTS chk_users_followers_visibility;

ALTER TABLE users
  ADD CONSTRAINT chk_users_birthday
    CHECK (birthday IS NULL OR birthday ~ '^[0-9]{4}-[0-9]{2}-[0-9]{2}$'),
  ADD CONSTRAINT chk_users_following_visibility
    CHECK (following_visibility IN ('public', 'followers', 'private')),
  ADD CONSTRAINT chk_users_followers_visibility
    CHECK (followers_visibility IN ('public', 'followers', 'private'));
