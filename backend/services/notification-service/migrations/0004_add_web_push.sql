CREATE TABLE IF NOT EXISTS web_push_subscriptions (
  id BIGSERIAL PRIMARY KEY,
  user_id BIGINT NOT NULL,
  endpoint TEXT NOT NULL UNIQUE,
  auth TEXT NOT NULL,
  public_key TEXT NOT NULL,
  state VARCHAR(32) NOT NULL DEFAULT 'active',
  send_read_message BOOLEAN NOT NULL DEFAULT FALSE,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  CONSTRAINT chk_web_push_subscriptions_user_id CHECK (user_id > 0),
  CONSTRAINT chk_web_push_subscriptions_endpoint CHECK (endpoint <> '' AND octet_length(endpoint) <= 2048),
  CONSTRAINT chk_web_push_subscriptions_auth CHECK (auth <> '' AND octet_length(auth) <= 512),
  CONSTRAINT chk_web_push_subscriptions_public_key CHECK (public_key <> '' AND octet_length(public_key) <= 512),
  CONSTRAINT chk_web_push_subscriptions_state CHECK (state IN ('active'))
);

CREATE TABLE IF NOT EXISTS web_push_outbox (
  id BIGSERIAL PRIMARY KEY,
  notification_id BIGINT NOT NULL REFERENCES notifications(id) ON DELETE CASCADE,
  subscription_id BIGINT NOT NULL REFERENCES web_push_subscriptions(id) ON DELETE CASCADE,
  attempt_count INTEGER NOT NULL DEFAULT 0,
  available_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  locked_at TIMESTAMPTZ,
  completed_at TIMESTAMPTZ,
  result VARCHAR(32) NOT NULL DEFAULT 'pending',
  last_error TEXT NOT NULL DEFAULT '',
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  UNIQUE(notification_id, subscription_id),
  CONSTRAINT chk_web_push_outbox_attempt_count CHECK (attempt_count >= 0),
  CONSTRAINT chk_web_push_outbox_result CHECK (result IN ('pending', 'delivered', 'gone', 'failed'))
);

CREATE INDEX IF NOT EXISTS idx_web_push_subscriptions_user
  ON web_push_subscriptions(user_id, id);

CREATE INDEX IF NOT EXISTS idx_web_push_outbox_pending
  ON web_push_outbox(available_at, id)
  WHERE completed_at IS NULL;

CREATE INDEX IF NOT EXISTS idx_web_push_outbox_completed
  ON web_push_outbox(completed_at, id)
  WHERE completed_at IS NOT NULL;

CREATE OR REPLACE FUNCTION enqueue_web_push_notification()
RETURNS TRIGGER AS $$
BEGIN
  INSERT INTO web_push_outbox(notification_id, subscription_id)
  SELECT NEW.id, subscriptions.id
  FROM web_push_subscriptions subscriptions
  WHERE subscriptions.user_id = NEW.user_id
    AND subscriptions.state = 'active'
  ON CONFLICT(notification_id, subscription_id) DO NOTHING;
  RETURN NEW;
END;
$$ LANGUAGE plpgsql;

DROP TRIGGER IF EXISTS notifications_enqueue_web_push ON notifications;

CREATE TRIGGER notifications_enqueue_web_push
AFTER INSERT ON notifications
FOR EACH ROW EXECUTE FUNCTION enqueue_web_push_notification();
