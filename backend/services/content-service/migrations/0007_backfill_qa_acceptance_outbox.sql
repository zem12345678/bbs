INSERT INTO qa_acceptance_outbox (
  event_id,
  topic_id,
  message_key,
  payload,
  status,
  created_at,
  updated_at
)
SELECT
  CONCAT('content.qa.accepted:', topics.id::TEXT, ':', topics.accepted_comment_id::TEXT),
  topics.id,
  topics.id::TEXT,
  jsonb_build_object(
    'event_id', CONCAT('content.qa.accepted:', topics.id::TEXT, ':', topics.accepted_comment_id::TEXT),
    'event_type', 'content.qa.accepted.v1',
    'event_version', 1,
    'occurred_at', topics.updated_at,
    'producer', 'content-service',
    'tenant_id', '',
    'aggregate_id', topics.id::TEXT,
    'request_id', '',
    'trace_id', '',
    'payload', jsonb_build_object(
      'event_id', CONCAT('content.qa.accepted:', topics.id::TEXT, ':', topics.accepted_comment_id::TEXT),
      'topic_id', topics.id,
      'title', topics.title,
      'question_author_id', topics.author_id,
      'accepted_comment_id', topics.accepted_comment_id,
      'accepted_comment_author_id', topics.accepted_comment_author_id,
      'reward_credits', CASE WHEN topics.bounty_score > 0 THEN topics.bounty_score ELSE 10 END
    )
  ),
  'PENDING',
  NOW(),
  NOW()
FROM topics
WHERE LOWER(BTRIM(topics.type)) = 'qa'
  AND LOWER(BTRIM(topics.qa_status)) = 'resolved'
  AND topics.author_id > 0
  AND topics.accepted_comment_id > 0
  AND topics.accepted_comment_author_id > 0
  AND topics.author_id <> topics.accepted_comment_author_id
  AND NOT EXISTS (
    SELECT 1
    FROM qa_acceptance_outbox
    WHERE qa_acceptance_outbox.topic_id = topics.id
  )
ON CONFLICT DO NOTHING;
