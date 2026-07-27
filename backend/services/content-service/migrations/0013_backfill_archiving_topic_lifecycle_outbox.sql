WITH candidates AS (
  SELECT id, updated_at, slug
  FROM topics
  WHERE status = 5
  FOR UPDATE
)
INSERT INTO content_lifecycle_outbox (
  event_id,
  message_key,
  event_type,
  payload,
  status,
  created_at,
  updated_at
)
SELECT
  CONCAT(
    'content.topic.archiving:', topics.id::TEXT, ':',
    (EXTRACT(EPOCH FROM topics.updated_at) * 1000000000)::BIGINT::TEXT
  ),
  topics.id::TEXT,
  'topic.archiving.v1',
  jsonb_build_object(
    'event_id', CONCAT(
      'content.topic.archiving:', topics.id::TEXT, ':',
      (EXTRACT(EPOCH FROM topics.updated_at) * 1000000000)::BIGINT::TEXT
    ),
    'event_type', 'topic.archiving.v1',
    'event_version', 1,
    'occurred_at', topics.updated_at,
    'producer', 'content-service',
    'tenant_id', '',
    'aggregate_id', topics.id::TEXT,
    'request_id', '',
    'trace_id', '',
    'payload', jsonb_build_object(
      'event_id', CONCAT(
        'content.topic.archiving:', topics.id::TEXT, ':',
        (EXTRACT(EPOCH FROM topics.updated_at) * 1000000000)::BIGINT::TEXT
      ),
      'topic_id', topics.id,
      'slug', topics.slug
    )
  ),
  'PENDING',
  NOW(),
  NOW()
FROM candidates AS topics
ON CONFLICT (event_id) DO NOTHING;
