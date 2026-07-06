# Data Ownership

## Storage Rules

- PostgreSQL is the source of truth for relational business records.
- MongoDB is the source of truth for comment bodies and reply structure.
- Redis is a cache and high-frequency counter/ranking store, not the final source of truth.
- Elasticsearch is a query/index projection, not a source of truth.
- Kafka carries domain events for projections, side effects, and eventual consistency.

## PostgreSQL Ownership Matrix

| Data / Table Concept | Owner Service | Notes |
| --- | --- | --- |
| users | `user-service` | Profile, counts, status, mute fields. |
| user_tokens | `auth-service` | Or replaced by Redis/JWT depending token strategy. |
| third_users | `auth-service` | OAuth identity binding. |
| email_codes | `auth-service` | Verification and password reset. |
| sms_codes | `auth-service` | SMS login verification. |
| roles | `admin-service` | RBAC. |
| permissions | `admin-service` | Permission registry. |
| role_permissions | `admin-service` | RBAC binding. |
| user_roles | `user-service` + `admin-service` | User owns projection; admin mutates through user service. |
| topics | `content-service` | Topic/tweet/QA unified content. |
| articles | `content-service` | Knowledge base content. |
| categories | `content-service` | Nodes/circles/categories. |
| tags | `content-service` | Public tags. |
| topic_tags | `content-service` | Topic tag relation. |
| article_tags | `content-service` | Article tag relation. |
| votes | `content-service` | Vote definition. |
| vote_options | `content-service` | Vote options. |
| vote_records | `reaction-service` or `content-service` | Prefer `reaction-service` if vote is treated as interaction. |
| favorites | `reaction-service` | Entity favorites. |
| user_likes | `reaction-service` | Likes across entity types. |
| user_reports | `admin-service` + `reaction-service` | Submission through reaction, audit through admin. |
| user_follows | `user-service` | Social graph. |
| user_feeds | `content-service` | Optional read model for followed feed. |
| messages | `notification-service` | Site messages. |
| email_logs | `notification-service` | Delivery log. |
| task_configs | `credit-service` | Admin config through admin service. |
| user_task_events | `credit-service` | Progress counters. |
| user_task_logs | `credit-service` | Reward records. |
| check_ins | `credit-service` | Daily check-in. |
| user_score_logs | `credit-service` | Points ledger. |
| user_exp_logs | `credit-service` | Experience ledger. |
| level_configs | `credit-service` | Level thresholds. |
| badges | `credit-service` | Badge definitions. |
| user_badges | `credit-service` | Awarded badges. |
| sys_configs | `config-service` | Product/site config. |
| dict_types | `admin-service` | Generic admin dictionaries. |
| dicts | `admin-service` | Generic admin dictionary items. |
| links | `content-service` or `config-service` | Public links. Prefer `content-service` if treated as content. |
| forbidden_words | `admin-service` | Moderation rules. Content/comment services consume snapshot. |
| attachments | `file-service` | Metadata and ownership. |
| attachment_download_logs | `file-service` + `credit-service` | Download purchase/deduction history. |
| operate_logs | `audit-service` | Admin/security audit. |
| migrations | per service | Each service owns its schema migration. |

## MongoDB Collections

### comments

Owner: `comment-service`

Recommended document shape:

```json
{
  "_id": "comment_id",
  "entityType": "topic|article",
  "entityId": "content_id",
  "parentId": "root_or_parent_comment_id",
  "quoteId": "quoted_comment_id",
  "rootId": "root_comment_id",
  "userId": "author_id",
  "content": "markdown/html/plain content",
  "contentType": "markdown",
  "images": [{ "url": "..." }],
  "status": "pending|approved|rejected|deleted|published",
  "likeCount": 0,
  "replyCount": 0,
  "ip": "...",
  "ipLocation": "...",
  "userAgent": "...",
  "createdAt": 0,
  "updatedAt": 0,
  "deletedAt": 0
}
```

Indexes:

- `{ entityType: 1, entityId: 1, status: 1, createdAt: -1 }`
- `{ rootId: 1, status: 1, createdAt: 1 }`
- `{ userId: 1, createdAt: -1 }`
- `{ quoteId: 1 }`

### comment_audit_logs

Optional collection for moderation transitions if not stored in PostgreSQL audit logs.

## Redis Usage

| Key Pattern | Owner | Purpose |
| --- | --- | --- |
| `session:{token}` | `auth-service` | Session/token lookup. |
| `captcha:{id}` | `auth-service` | Captcha verification. |
| `rate:{scope}:{key}` | `api-gateway` | Rate limiting. |
| `like:count:{entityType}:{entityId}` | `reaction-service` | Hot like counters. |
| `comment:count:{entityType}:{entityId}` | `comment-service` | Hot comment counters. |
| `view:count:{entityType}:{entityId}` | `content-service` | View counters. |
| `rank:score` | `credit-service` | Score leaderboard. |
| `rank:checkin` | `credit-service` | Check-in leaderboard. |
| `topic:hot` | `content-service` | Hot topics. |
| `user:liked:{userId}:{entityType}` | `reaction-service` | Batch liked lookup. |
| `idempotency:{biz}:{key}` | varies | Prevent duplicate writes/events. |

## Elasticsearch Indices

| Index | Owner | Source Events |
| --- | --- | --- |
| `bbs_topics` | `search-service` | `content.topic.*`, `reaction.*`, `comment.*` |
| `bbs_articles` | `search-service` | `content.article.*`, `reaction.*`, `comment.*` |
| `bbs_users` | `search-service` | `user.created`, `user.updated`, `credit.level.changed` |
| `bbs_tags` | `search-service` | tag changes from `content-service` |

Topic/article indexed fields:

- id
- title
- summary/content excerpt
- category
- tags
- author
- status
- recommend/sticky flags
- counters
- created/update times

## Cross-Service Reference Strategy

Use numeric or string IDs as references across services. Do not enforce cross-service foreign keys at the database level.

Examples:

- A comment references `entityType + entityId` but does not foreign-key to content tables.
- A like references `entityType + entityId` but ownership of entity existence is validated through gRPC or async reconciliation.
- A notification references source data by `sourceType + sourceId`.

## Counters

Recommended counter model:

1. Write interaction/comment event.
2. Update Redis counter immediately.
3. Publish Kafka event.
4. Consumer flushes or reconciles PostgreSQL/MongoDB denormalized counters.
5. Periodic reconciliation job corrects drift.

Counters:

- `topic.view_count`
- `topic.comment_count`
- `topic.like_count`
- `article.view_count`
- `article.comment_count`
- `article.like_count`
- `comment.like_count`
- `comment.reply_count`
- `user.topic_count`
- `user.comment_count`
- `user.follow_count`
- `user.fans_count`
- `attachment.download_count`

## Data Consistency Policy

Strong consistency required:

- Login/session validation.
- User mute/ban checks before posting.
- QA bounty reservation and accepted answer settlement.
- Attachment paid download authorization.
- Admin permission checks.

Eventual consistency acceptable:

- Search index.
- Notifications.
- Like/favorite counters.
- View counters.
- Task progress/rewards, except when rewards affect immediate paid actions.
- Ranking.

