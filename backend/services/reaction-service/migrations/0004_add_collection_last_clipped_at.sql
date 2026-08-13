ALTER TABLE favorite_collections
  ADD COLUMN IF NOT EXISTS last_clipped_at TIMESTAMPTZ;

UPDATE favorite_collections
SET last_clipped_at = (
  SELECT MAX(created_at)
  FROM favorite_collection_items
  WHERE collection_id = favorite_collections.id
)
WHERE last_clipped_at IS NULL
  AND EXISTS (
    SELECT 1
    FROM favorite_collection_items
    WHERE collection_id = favorite_collections.id
  );
