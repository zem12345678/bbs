ALTER TABLE favorite_collections
  ALTER COLUMN name TYPE VARCHAR(100);

ALTER TABLE favorite_collections
  DROP CONSTRAINT IF EXISTS favorite_collections_name_check;

ALTER TABLE favorite_collections
  ADD CONSTRAINT favorite_collections_name_check
  CHECK (char_length(name) BETWEEN 1 AND 100);

ALTER TABLE favorite_collections
  DROP CONSTRAINT IF EXISTS favorite_collections_description_check;

ALTER TABLE favorite_collections
  ADD CONSTRAINT favorite_collections_description_check
  CHECK (char_length(description) <= 2048);
