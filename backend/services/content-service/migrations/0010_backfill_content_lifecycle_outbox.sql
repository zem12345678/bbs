WITH candidates AS (
  SELECT id, status, updated_at, slug
  FROM articles
  WHERE status IN (3, 4)
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
    'content.article.',
    CASE WHEN articles.status = 3 THEN 'hidden' ELSE 'archived' END,
    ':', articles.id::TEXT, ':',
    (EXTRACT(EPOCH FROM articles.updated_at) * 1000000000)::BIGINT::TEXT
  ),
  articles.id::TEXT,
  CASE WHEN articles.status = 3 THEN 'article.hidden.v1' ELSE 'article.archived.v1' END,
  jsonb_build_object(
    'event_id', CONCAT(
      'content.article.',
      CASE WHEN articles.status = 3 THEN 'hidden' ELSE 'archived' END,
      ':', articles.id::TEXT, ':',
      (EXTRACT(EPOCH FROM articles.updated_at) * 1000000000)::BIGINT::TEXT
    ),
    'event_type', CASE WHEN articles.status = 3 THEN 'article.hidden.v1' ELSE 'article.archived.v1' END,
    'event_version', 1,
    'occurred_at', articles.updated_at,
    'producer', 'content-service',
    'tenant_id', '',
    'aggregate_id', articles.id::TEXT,
    'request_id', '',
    'trace_id', '',
    'payload', jsonb_build_object(
      'event_id', CONCAT(
        'content.article.',
        CASE WHEN articles.status = 3 THEN 'hidden' ELSE 'archived' END,
        ':', articles.id::TEXT, ':',
        (EXTRACT(EPOCH FROM articles.updated_at) * 1000000000)::BIGINT::TEXT
      ),
      'article_id', articles.id,
      'slug', articles.slug
    )
  ),
  'PENDING',
  NOW(),
  NOW()
FROM candidates AS articles
ON CONFLICT (event_id) DO NOTHING;

WITH candidates AS (
  SELECT id, status, updated_at, slug
  FROM topics
  WHERE status IN (3, 4)
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
    'content.topic.',
    CASE WHEN topics.status = 3 THEN 'hidden' ELSE 'archived' END,
    ':', topics.id::TEXT, ':',
    (EXTRACT(EPOCH FROM topics.updated_at) * 1000000000)::BIGINT::TEXT
  ),
  topics.id::TEXT,
  CASE WHEN topics.status = 3 THEN 'topic.hidden.v1' ELSE 'topic.archived.v1' END,
  jsonb_build_object(
    'event_id', CONCAT(
      'content.topic.',
      CASE WHEN topics.status = 3 THEN 'hidden' ELSE 'archived' END,
      ':', topics.id::TEXT, ':',
      (EXTRACT(EPOCH FROM topics.updated_at) * 1000000000)::BIGINT::TEXT
    ),
    'event_type', CASE WHEN topics.status = 3 THEN 'topic.hidden.v1' ELSE 'topic.archived.v1' END,
    'event_version', 1,
    'occurred_at', topics.updated_at,
    'producer', 'content-service',
    'tenant_id', '',
    'aggregate_id', topics.id::TEXT,
    'request_id', '',
    'trace_id', '',
    'payload', jsonb_build_object(
      'event_id', CONCAT(
        'content.topic.',
        CASE WHEN topics.status = 3 THEN 'hidden' ELSE 'archived' END,
        ':', topics.id::TEXT, ':',
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
