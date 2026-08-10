ALTER TABLE channels
  ADD COLUMN IF NOT EXISTS is_featured BOOLEAN NOT NULL DEFAULT FALSE;

UPDATE channels
SET is_featured = FALSE
WHERE is_archived = TRUE AND is_featured = TRUE;

CREATE INDEX IF NOT EXISTS idx_channels_featured_updated
  ON channels(is_featured, updated_at DESC)
  WHERE is_featured = TRUE AND is_archived = FALSE;
