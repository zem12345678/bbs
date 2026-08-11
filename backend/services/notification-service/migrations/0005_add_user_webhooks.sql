CREATE TABLE IF NOT EXISTS notification_webhooks (
  id BIGSERIAL PRIMARY KEY,
  user_id BIGINT NOT NULL,
  name VARCHAR(100) NOT NULL,
  url TEXT NOT NULL,
  secret TEXT NOT NULL DEFAULT '',
  events TEXT[] NOT NULL,
  active BOOLEAN NOT NULL DEFAULT TRUE,
  latest_sent_at TIMESTAMPTZ,
  latest_status INTEGER,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  CONSTRAINT chk_notification_webhooks_user_id CHECK (user_id > 0),
  CONSTRAINT chk_notification_webhooks_name CHECK (char_length(btrim(name)) BETWEEN 1 AND 100),
  CONSTRAINT chk_notification_webhooks_url CHECK (url <> '' AND octet_length(url) <= 1024),
  CONSTRAINT chk_notification_webhooks_secret CHECK (octet_length(secret) <= 1024),
  CONSTRAINT chk_notification_webhooks_events CHECK (
    cardinality(events) > 0 AND events <@ ARRAY['mention','unfollow','follow','followed','note','reply','renote','reaction','edited']::TEXT[]
  )
);

CREATE TABLE IF NOT EXISTS notification_webhook_outbox (
  id BIGSERIAL PRIMARY KEY,
  webhook_id BIGINT NOT NULL REFERENCES notification_webhooks(id) ON DELETE CASCADE,
  user_id BIGINT NOT NULL,
  event_id VARCHAR(128) NOT NULL,
  event_type VARCHAR(32) NOT NULL,
  url TEXT NOT NULL,
  secret TEXT NOT NULL DEFAULT '',
  payload JSONB NOT NULL,
  attempt_count INTEGER NOT NULL DEFAULT 0,
  available_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  locked_at TIMESTAMPTZ,
  completed_at TIMESTAMPTZ,
  result VARCHAR(32) NOT NULL DEFAULT 'pending',
  last_status INTEGER,
  last_error TEXT NOT NULL DEFAULT '',
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  CONSTRAINT chk_notification_webhook_outbox_user_id CHECK (user_id > 0),
  CONSTRAINT chk_notification_webhook_outbox_event_id CHECK (event_id <> ''),
  CONSTRAINT chk_notification_webhook_outbox_event_type CHECK (
    event_type IN ('mention','unfollow','follow','followed','note','reply','renote','reaction','edited')
  ),
  CONSTRAINT chk_notification_webhook_outbox_attempt_count CHECK (attempt_count >= 0),
  CONSTRAINT chk_notification_webhook_outbox_result CHECK (result IN ('pending','delivered','failed')),
  UNIQUE(webhook_id, event_id, event_type)
);

CREATE INDEX IF NOT EXISTS idx_notification_webhooks_user
  ON notification_webhooks(user_id, id);

CREATE INDEX IF NOT EXISTS idx_notification_webhook_outbox_pending
  ON notification_webhook_outbox(available_at, id)
  WHERE completed_at IS NULL;

CREATE INDEX IF NOT EXISTS idx_notification_webhook_outbox_completed
  ON notification_webhook_outbox(completed_at, id)
  WHERE completed_at IS NOT NULL;
