CREATE TABLE IF NOT EXISTS notification_preferences (
  user_id BIGINT NOT NULL,
  notification_type VARCHAR(64) NOT NULL,
  enabled BOOLEAN NOT NULL DEFAULT TRUE,
  updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  PRIMARY KEY (user_id, notification_type),
  CONSTRAINT chk_notification_preferences_user_id CHECK (user_id > 0),
  CONSTRAINT chk_notification_preferences_type CHECK (notification_type <> '')
);

CREATE INDEX IF NOT EXISTS idx_notification_preferences_user
  ON notification_preferences(user_id);
