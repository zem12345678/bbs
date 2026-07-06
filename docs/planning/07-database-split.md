# Database Split Plan

This document turns the `bbs-go` table model into a microservice-oriented database plan.

The guiding idea is: use `bbs-go` tables as a feature/entity reference, but split by service ownership rather than copying a single shared schema.

## Recommendation

Use independent logical PostgreSQL databases or schemas per service from day one.

For local development and early deployment, they can share one PostgreSQL instance:

```text
postgres instance
  bbs_auth
  bbs_user
  bbs_content
  bbs_reaction
  bbs_credit
  bbs_notification
  bbs_admin
  bbs_config
  bbs_file
  bbs_audit
```

Production can later move high-load services to separate physical PostgreSQL instances without changing ownership rules.

MongoDB, Redis, and Elasticsearch remain separate infrastructure stores:

```text
MongoDB
  bbs_comment.comments
  bbs_comment.comment_audit_logs

Redis
  cache, counters, sessions, rate limits, rankings

Elasticsearch
  bbs_topics
  bbs_articles
  bbs_users
  bbs_tags
```

## Non-Negotiable Rules

1. A service may only write its own database.
2. No service may directly query another service's database.
3. No cross-database foreign keys.
4. Cross-service reads use gRPC, API gateway aggregation, cache projection, or search/read models.
5. Cross-service side effects use Kafka events.
6. Redis and Elasticsearch are projections/caches, not source-of-truth storage.
7. Schema migrations are owned and run per service.

## ID Strategy

Recommended:

- Use globally unique sortable IDs for externally referenced entities.
- Good options: Snowflake-style int64 IDs or UUIDv7.
- Keep IDs stable across services and events.

Suggested entity ID style:

| Entity | ID Type | Reason |
| --- | --- | --- |
| users | int64 or UUIDv7 | Referenced everywhere. |
| topics/articles | int64 or UUIDv7 | Needs URL-friendly ID. |
| comments | UUIDv7 string | MongoDB-friendly and globally unique. |
| attachments | UUIDv7 string | Already aligns with bbs-go attachment UUID idea. |
| events | UUIDv7 string | Idempotency. |

Avoid auto-increment IDs if services may later be horizontally sharded or physically split.

## PostgreSQL Split

### bbs_auth

Owner: `auth-service`

Tables:

| Table | Purpose | Notes |
| --- | --- | --- |
| `auth_sessions` | Login sessions/tokens | Can replace or supplement Redis session keys. |
| `third_identities` | OAuth identity binding | Equivalent to `third_users`. |
| `email_codes` | Email verification/password reset codes | Include biz type and token. |
| `sms_codes` | SMS login codes | Include provider SMS id and status. |
| `auth_login_logs` | Login/security audit | Optional; can also publish to `audit-service`. |

Core fields:

```text
auth_sessions(id, user_id, token_hash, device_id, user_agent, ip, expires_at, revoked_at, created_at)
third_identities(id, user_id, provider, open_id, nickname, avatar, raw_payload, created_at, updated_at)
email_codes(id, user_id, biz_type, email, code_hash, token, used_at, expires_at, created_at)
sms_codes(id, phone, code_hash, provider, provider_msg_id, status, expires_at, created_at)
```

Cross-service references:

- `user_id` references `user-service` logically only.
- No database-level foreign key to `bbs_user.users`.

### bbs_user

Owner: `user-service`

Tables:

| Table | Purpose | Notes |
| --- | --- | --- |
| `users` | User profile and account state | Main user source of truth. |
| `user_follows` | Follow/fans relationship | Unique by follower/followee. |
| `user_roles` | User role projection | Mutated through admin/user service flow. |
| `user_profile_stats` | Optional denormalized stats | Can also live in `users` as counters. |

Core fields:

```text
users(
  id, username, email, email_verified, phone, nickname, avatar,
  gender, birthday, background_image, homepage, description,
  score_snapshot, exp_snapshot, level_snapshot,
  topic_count, article_count, comment_count, follow_count, fans_count,
  status, forbidden_until, created_at, updated_at
)

user_follows(id, user_id, target_user_id, status, created_at, updated_at)
user_roles(id, user_id, role_id, role_code, created_at)
```

Source-of-truth fields:

- Profile fields.
- Mute/status.
- Follow relationships.

Event-projected fields:

- `score_snapshot`, `exp_snapshot`, `level_snapshot` from `credit-service`.
- Content counters from content/comment events.

### bbs_content

Owner: `content-service`

Tables:

| Table | Purpose | Notes |
| --- | --- | --- |
| `topics` | Topic/tweet/Q&A | Unified bbs-go `Topic` model. |
| `articles` | Knowledge base articles | Article source of truth. |
| `categories` | Nodes/circles/categories | Supports parent tree and QA type. |
| `tags` | Tags | Shared by topics and articles. |
| `topic_tags` | Topic-tag relation | Many-to-many. |
| `article_tags` | Article-tag relation | Many-to-many. |
| `votes` | Vote definition | Attached to topic. |
| `vote_options` | Vote options | Vote result counters can be projected. |
| `links` | Friendly links | If treated as content rather than config. |
| `content_outbox` | Reliable event outbox | Recommended for Kafka publishing. |

Core fields:

```text
topics(
  id, type, category_id, qa_status, accepted_comment_id, solved_at,
  bounty_score, author_id, title, content_type, content, image_list,
  hide_content, vote_id, recommend, recommend_at, sticky, sticky_at,
  status, view_count, comment_count, like_count, favorite_count,
  last_comment_at, ip, ip_location, user_agent, created_at, updated_at, deleted_at
)

articles(
  id, author_id, title, summary, content, content_type, cover,
  source_url, status, view_count, comment_count, like_count, favorite_count,
  created_at, updated_at, deleted_at
)

categories(id, parent_id, name, type, description, logo, sort_no, status, created_at, updated_at)
tags(id, name, description, status, created_at, updated_at)
votes(id, topic_id, type, title, expired_at, max_select, status, created_at)
vote_options(id, vote_id, content, sort_no, vote_count, created_at)
```

Source-of-truth fields:

- Topic/article body and lifecycle.
- Category/tag/vote definitions.
- QA accepted answer marker.

Event-projected fields:

- `comment_count` from `comment-service`.
- `like_count`, `favorite_count` from `reaction-service`.
- `view_count` can be content-owned but flushed from Redis.

Author references:

- Store `author_id`.
- For list performance, optionally store `author_snapshot` JSON with nickname/avatar at publish time.
- Keep snapshot eventual; current profile still comes from `user-service`.

### bbs_comment

Owner: `comment-service`

Primary store: MongoDB.

Optional PostgreSQL schema:

| Table | Purpose | Notes |
| --- | --- | --- |
| `comment_indexes` | Relational moderation/query index | Optional but useful for admin filters. |
| `comment_outbox` | Reliable event outbox | If comment service writes PG and Mongo in one workflow, design carefully. |

MongoDB collections:

```text
comments
comment_audit_logs
```

Recommended comment document fields are already defined in [03-data-ownership.md](./03-data-ownership.md).

Important decision:

- Comment body and reply structure live in MongoDB.
- Content comment counters are not directly updated by comment service; publish `comment.created` / `comment.deleted`.
- Admin moderation may query MongoDB directly through `comment-service`, not through database access.

### bbs_reaction

Owner: `reaction-service`

Tables:

| Table | Purpose | Notes |
| --- | --- | --- |
| `user_likes` | Likes across entity types | Topic/article/comment. |
| `favorites` | Favorites/bookmarks | Topic/article. |
| `vote_records` | Vote cast records | Prefer here if vote is modeled as interaction. |
| `user_reports` | User report submission | Audit state can be managed by admin service through this service. |
| `reaction_outbox` | Reliable event outbox | For counters, notifications, tasks. |

Core fields:

```text
user_likes(id, user_id, entity_type, entity_id, status, created_at, updated_at)
favorites(id, user_id, entity_type, entity_id, created_at)
vote_records(id, vote_id, option_id, user_id, created_at)
user_reports(id, data_type, data_id, user_id, reason, audit_status, audit_user_id, audit_time, created_at, updated_at)
```

Constraints:

- Unique active like by `user_id + entity_type + entity_id`.
- Unique favorite by `user_id + entity_type + entity_id`.
- Vote uniqueness depends on vote type.

### bbs_credit

Owner: `credit-service`

Tables:

| Table | Purpose | Notes |
| --- | --- | --- |
| `check_ins` | Daily check-in state | Consecutive days. |
| `task_configs` | Task definitions | Newbie/daily/achievement. |
| `user_task_events` | Event accumulation | Progress tracking. |
| `user_task_logs` | Completion/reward log | Idempotent reward record. |
| `user_score_logs` | Points ledger | Source of truth for point changes. |
| `user_exp_logs` | Experience ledger | Source of truth for exp changes. |
| `level_configs` | Level thresholds | Admin configurable. |
| `badges` | Badge definitions | Admin configurable. |
| `user_badges` | Awarded badges | Unique user/badge. |
| `credit_accounts` | Current balances | Recommended for fast balance checks. |
| `credit_outbox` | Reliable event outbox | Level/badge/score changes. |

Core fields:

```text
credit_accounts(user_id, score, exp, level, updated_at)
check_ins(id, user_id, latest_day, consecutive_days, created_at, updated_at)
task_configs(id, group_name, event_type, title, description, score, exp, badge_id, period, max_finish_count, event_count, action_url, sort_no, status, start_at, end_at)
user_score_logs(id, user_id, source_type, source_id, description, change_type, score, balance_after, created_at)
user_exp_logs(id, user_id, source_type, source_id, description, change_type, exp, total_after, created_at)
```

Strong consistency required:

- QA bounty reserve/settlement.
- Paid attachment download deduction.
- Manual admin point adjustment.

### bbs_notification

Owner: `notification-service`

Tables:

| Table | Purpose | Notes |
| --- | --- | --- |
| `messages` | Site messages | Equivalent to bbs-go messages. |
| `notification_deliveries` | Delivery attempts/status | Site/email/SMS. |
| `email_logs` | Email send logs | Admin visible. |
| `notification_preferences` | Optional user preferences | Later phase. |
| `notification_outbox` | Reliable event outbox | Optional. |

Core fields:

```text
messages(id, user_id, type, title, content, quote_content, source_type, source_id, status, created_at, read_at)
notification_deliveries(id, user_id, channel, type, target, payload, status, error_message, created_at, sent_at)
email_logs(id, to_email, subject, content, biz_type, status, error_message, created_at)
```

### bbs_admin

Owner: `admin-service`

Tables:

| Table | Purpose | Notes |
| --- | --- | --- |
| `roles` | Role definitions | RBAC. |
| `permissions` | Permission registry | Generated/seeded. |
| `role_permissions` | Role-permission relation | RBAC. |
| `dict_types` | Generic dictionaries | Optional parity feature. |
| `dicts` | Generic dictionary entries | Optional parity feature. |
| `forbidden_words` | Sensitive words/regex | Consumed by content/comment services. |
| `admin_outbox` | Config/permission/rule events | Cache refresh. |

Core fields:

```text
roles(id, type, name, code, sort_no, remark, status, created_at, updated_at)
permissions(id, type, code, name, group_name, description, sort_no, status, created_at, updated_at)
role_permissions(id, role_id, permission_id, created_at)
forbidden_words(id, type, word, remark, status, created_at, updated_at)
```

Admin-service should orchestrate governance actions but should not directly mutate another service's tables.

Examples:

- To mute user: admin-service calls `user-service.MuteUser`.
- To audit topic: admin-service calls `content-service.AuditTopic`.
- To audit report: admin-service calls `reaction-service.AuditReport`.

### bbs_config

Owner: `config-service`

Tables:

| Table | Purpose | Notes |
| --- | --- | --- |
| `sys_configs` | Product-level settings | Site title, nav, modules, upload settings. |
| `config_versions` | Optional version history | Useful for rollback. |
| `config_outbox` | Config changed events | Refresh services/Nacos. |

Core fields:

```text
sys_configs(id, key, value_json, name, description, visibility, created_at, updated_at)
config_versions(id, key, value_json, operator_id, change_reason, created_at)
```

Nacos relation:

- Nacos is runtime config distribution.
- `bbs_config.sys_configs` is product config source of truth.
- On admin save, config-service validates, persists, then publishes to Nacos or emits config refresh event.

Config categories:

- Site base config.
- Module switches.
- Login switches.
- OAuth settings.
- SMTP settings.
- Notification type switches.
- Upload and attachment config.
- QA bounty config.
- Captcha and email verification gates.
- Script injections.

### bbs_file

Owner: `file-service`

Tables:

| Table | Purpose | Notes |
| --- | --- | --- |
| `files` | Generic uploaded file metadata | Avatar, cover, images. |
| `attachments` | Topic attachments | Download score and count. |
| `attachment_download_logs` | Paid/free download history | Prevent duplicate charge. |
| `file_outbox` | File-related events | Audit and content binding. |

Core fields:

```text
files(id, owner_user_id, biz_type, file_name, file_url, file_size, file_type, storage_provider, status, created_at)
attachments(id, topic_id, user_id, file_name, file_url, file_size, file_type, download_score, download_count, status, created_at, updated_at)
attachment_download_logs(id, user_id, attachment_id, charged_score, created_at)
```

Storage:

- Metadata in PostgreSQL.
- Binary files in local storage or object storage.
- Production should prefer S3-compatible object storage.

### bbs_audit

Owner: `audit-service`

Tables:

| Table | Purpose | Notes |
| --- | --- | --- |
| `operate_logs` | Admin/user operation logs | Equivalent to bbs-go operate logs. |
| `security_logs` | Optional security events | Login risk, permission denial. |

Core fields:

```text
operate_logs(id, actor_id, op_type, data_type, data_id, description, ip, user_agent, referer, trace_id, created_at)
security_logs(id, actor_id, event_type, risk_level, description, ip, user_agent, trace_id, created_at)
```

## Outbox Pattern

For services that write PostgreSQL and publish Kafka events, use an outbox table.

Recommended common columns:

```text
id
aggregate_type
aggregate_id
event_type
payload_json
status
retry_count
next_retry_at
created_at
published_at
```

Services that need outbox immediately:

- `content-service`
- `comment-service` if it writes a PG index
- `reaction-service`
- `credit-service`
- `notification-service`
- `config-service`
- `admin-service`
- `file-service`

Reason:

- Avoid "DB write succeeded but Kafka publish failed".
- Keep event delivery retryable.
- Support idempotent downstream projections.

## Cross-Service Read Models

Some pages need mixed data. Do not join across service databases.

Use one of these:

1. API gateway aggregation through gRPC.
2. Snapshot fields stored at write time.
3. Kafka-built read projections.
4. Elasticsearch for search/list discovery.

Recommended snapshots:

| Owner | Snapshot | Why |
| --- | --- | --- |
| `content-service` | `author_snapshot` on topic/article | Fast feed rendering. |
| `comment-service` | `author_snapshot` on comment | Fast comment rendering. |
| `notification-service` | source title/excerpt | Notifications should remain readable if source changes. |
| `search-service` | author/category/tag names | Search result rendering. |

Snapshots are not source of truth and may be stale.

## Migration Strategy From bbs-go Concepts

Do not import bbs-go schema directly. Instead:

1. Translate table purpose into service ownership.
2. Rename columns to consistent style if needed.
3. Replace cross-table assumptions with service calls/events.
4. Add outbox tables where events are needed.
5. Add snapshots where frontend pages need fast rendering.
6. Add indexes based on target query patterns, not only original bbs-go indexes.

## P0 Database Cut

Minimum schemas for the first usable release:

```text
bbs_auth
  auth_sessions
  email_codes

bbs_user
  users
  user_follows

bbs_content
  topics
  articles
  categories
  tags
  topic_tags
  article_tags
  content_outbox

bbs_comment
  MongoDB comments

bbs_reaction
  user_likes
  favorites
  user_reports
  reaction_outbox

bbs_notification
  messages

bbs_admin
  roles
  permissions
  role_permissions
  forbidden_words

bbs_config
  sys_configs

bbs_audit
  operate_logs
```

Defer to P1/P2:

- OAuth identities.
- SMS codes.
- Votes.
- Full task engine.
- Badges/levels.
- Paid attachments.
- Dict/dict type.
- SEO sitemap jobs.
- Full email delivery logs.

## Risk Notes

- Over-splitting too early can slow development. Use separate logical schemas but one local Postgres instance initially.
- Cross-service transactions should be avoided. Use reservation/settlement APIs for money-like or score-like flows.
- Counters will drift unless there is a reconciliation job.
- Search and notification must tolerate duplicate/out-of-order events.
- Admin pages will need gateway aggregation because admin workflows span multiple services.
