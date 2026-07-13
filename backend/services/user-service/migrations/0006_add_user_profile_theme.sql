ALTER TABLE users
  ADD COLUMN IF NOT EXISTS profile_theme TEXT NOT NULL DEFAULT 'default';

UPDATE users
SET profile_theme = 'default'
WHERE profile_theme IS NULL OR BTRIM(profile_theme) = '';
