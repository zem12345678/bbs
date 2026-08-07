ALTER TABLE chat_room_members
  ADD COLUMN IF NOT EXISTS muted_until TIMESTAMPTZ;

ALTER TABLE chat_room_members
  DROP CONSTRAINT IF EXISTS chk_chat_room_members_role;

ALTER TABLE chat_room_members
  ADD CONSTRAINT chk_chat_room_members_role CHECK (role IN (1, 2, 3));

CREATE INDEX IF NOT EXISTS idx_chat_room_members_room_role_joined
  ON chat_room_members(room_id, role, joined_at, user_id)
  WHERE status = 1;
