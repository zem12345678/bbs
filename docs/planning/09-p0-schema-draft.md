# P0 Schema Draft

This document defines the P0 storage schema draft. It is not an executable migration file.

The goal is to provide enough detail for later PostgreSQL migrations, MongoDB indexes, and Elasticsearch mappings without creating implementation code yet.

## Scope

P0 includes:

- Password auth and reset email.
- Users and follows.
- Topics, articles, categories, tags.
- MongoDB comments.
- Likes, favorites, reports.
- Basic messages.
- Minimal admin roles/permissions and forbidden words.
- Public site config.
- Operation logs.
- Search indices for topics/articles/users/tags.

P0 excludes:

- OAuth identities.
- SMS login.
- Votes.
- Task/credit/badge engine.
- Paid attachments.
- SEO sitemap jobs.
- Full RBAC granularity.

## Conventions

### Types

PostgreSQL:

```text
id                  varchar(36)     // UUIDv7 string preferred
*_id               varchar(36)
status             smallint
created_at         bigint          // Unix milliseconds
updated_at         bigint
deleted_at         bigint          // 0 means not deleted
payload/json data  jsonb
content/body       text
```

MongoDB:

- `_id`: UUIDv7 string.
- Times: Unix milliseconds.
- Status fields use stable strings for readability.

### Common Status Values

User status:

```text
0 disabled
1 normal
2 muted
3 deleted
```

Content status:

```text
0 draft
1 pending
2 published
3 rejected
4 deleted
```

Reaction/report status:

```text
0 inactive/deleted
1 active
```

Audit status:

```text
0 pending
1 approved
2 rejected
3 ignored
```

Message status:

```text
0 unread
1 read
2 deleted
```

### Cross-Service Foreign Keys

Do not create foreign keys across service schemas.

Within the same service schema, foreign keys are optional. Prefer application-level constraints if they make migrations and sharding easier.

## PostgreSQL Schemas

## bbs_auth

Owner: `auth-service`

### credentials

Purpose: password credential storage owned by auth-service.

```text
id              varchar(36) primary key
user_id         varchar(36) not null
identity_type   varchar(32) not null default 'password'
identity_value  varchar(128) not null
password_hash   varchar(255) not null
status          smallint not null default 1
created_at      bigint not null
updated_at      bigint not null
```

Indexes:

```text
unique(identity_type, identity_value)
unique(user_id, identity_type)
index(user_id, status)
```

Notes:

- `identity_value` is the normalized username or email used for password login.
- Store password hashes only in `bbs_auth`, never in `bbs_user`.
- OAuth identities remain deferred and can later use a separate `third_identities` table.

### auth_sessions

Purpose: opaque token sessions with revocation support.

```text
id              varchar(36) primary key
user_id         varchar(36) not null
token_hash      varchar(128) not null
device_id       varchar(128) null
ip              varchar(64) null
user_agent      text null
status          smallint not null default 1
expires_at      bigint not null
revoked_at      bigint not null default 0
created_at      bigint not null
updated_at      bigint not null
```

Indexes:

```text
unique(token_hash)
index(user_id, status)
index(expires_at)
```

Notes:

- Store token hash only, never raw token.
- Redis may cache `token -> principal`; PostgreSQL remains revocation/audit source.

### email_codes

Purpose: password reset and later email verification.

```text
id              varchar(36) primary key
user_id         varchar(36) null
biz_type        varchar(32) not null
email           varchar(128) not null
code_hash       varchar(128) not null
token           varchar(64) not null
title           varchar(256) null
content         text null
used_at         bigint not null default 0
expires_at      bigint not null
created_at      bigint not null
```

Indexes:

```text
unique(token)
index(email, biz_type, created_at)
index(user_id, biz_type)
index(expires_at)
```

## bbs_user

Owner: `user-service`

### users

Purpose: user profile, account state, and denormalized counters.

```text
id                  varchar(36) primary key
username            varchar(32) not null
email               varchar(128) null
email_verified      boolean not null default false
phone               varchar(32) null
nickname            varchar(32) not null
avatar              text null
gender              varchar(16) not null default ''
birthday            date null
background_image    text null
homepage            varchar(1024) null
description         text null

score_snapshot      integer not null default 0
exp_snapshot        integer not null default 0
level_snapshot      integer not null default 1

topic_count         integer not null default 0
article_count       integer not null default 0
comment_count       integer not null default 0
follow_count        integer not null default 0
fans_count          integer not null default 0

roles_snapshot      jsonb not null default '[]'
status              smallint not null default 1
forbidden_until     bigint not null default 0

created_at          bigint not null
updated_at          bigint not null
deleted_at          bigint not null default 0
```

Indexes:

```text
unique(username)
unique(email) where email is not null
unique(phone) where phone is not null
index(status)
index(score_snapshot desc)
index(level_snapshot desc)
index(created_at desc)
```

Notes:

- Password hashes are owned by `bbs_auth.credentials`; `bbs_user.users` stores profile and account state only.
- `roles_snapshot` is P0 simple-role support; full RBAC joins come later.

### user_follows

Purpose: follow/fans relationship.

```text
id              varchar(36) primary key
user_id         varchar(36) not null
target_user_id  varchar(36) not null
status          smallint not null default 1
created_at      bigint not null
updated_at      bigint not null
```

Indexes:

```text
unique(user_id, target_user_id)
index(target_user_id, status, created_at desc)
index(user_id, status, created_at desc)
```

Rules:

- `user_id != target_user_id`.
- Follow/unfollow is idempotent.

## bbs_content

Owner: `content-service`

### categories

Purpose: nodes/circles/categories.

```text
id              varchar(36) primary key
parent_id       varchar(36) not null default ''
name            varchar(64) not null
type            varchar(16) not null default 'normal'
description     varchar(1024) null
logo            text null
sort_no         integer not null default 0
status          smallint not null default 1
created_at      bigint not null
updated_at      bigint not null
deleted_at      bigint not null default 0
```

Indexes:

```text
unique(parent_id, name) where deleted_at = 0
index(parent_id, status, sort_no)
index(type, status)
```

### tags

Purpose: tags used by topics and articles.

```text
id              varchar(36) primary key
name            varchar(64) not null
slug            varchar(96) not null
description     varchar(1024) null
status          smallint not null default 1
created_at      bigint not null
updated_at      bigint not null
deleted_at      bigint not null default 0
```

Indexes:

```text
unique(name) where deleted_at = 0
unique(slug) where deleted_at = 0
index(status, created_at desc)
```

### topics

Purpose: topic/tweet/QA-ready content.

```text
id                  varchar(36) primary key
type                varchar(16) not null default 'topic'
category_id         varchar(36) not null
category_snapshot   jsonb not null default '{}'
author_id           varchar(36) not null
author_snapshot     jsonb not null default '{}'

title               varchar(160) null
content             text not null
content_type        varchar(16) not null default 'markdown'
summary             varchar(512) null
image_list          jsonb not null default '[]'

qa_status           varchar(16) not null default 'none'
accepted_comment_id varchar(36) not null default ''
solved_at           bigint not null default 0

recommend           boolean not null default false
recommend_at        bigint not null default 0
sticky              boolean not null default false
sticky_at           bigint not null default 0

view_count          bigint not null default 0
comment_count       bigint not null default 0
like_count          bigint not null default 0
favorite_count      bigint not null default 0
last_comment_at     bigint not null default 0

status              smallint not null default 2
ip                  varchar(64) null
ip_location         varchar(64) null
user_agent          text null
created_at          bigint not null
updated_at          bigint not null
deleted_at          bigint not null default 0
```

Indexes:

```text
index(status, created_at desc)
index(type, status, created_at desc)
index(category_id, status, created_at desc)
index(author_id, status, created_at desc)
index(recommend, recommend_at desc)
index(sticky, sticky_at desc)
index(last_comment_at desc)
```

Rules:

- `title` required when `type = 'topic'`.
- `title` optional when `type = 'tweet'`.
- `qa_status` remains `none` in P0 unless QA is enabled early.

### topic_tags

```text
id              varchar(36) primary key
topic_id        varchar(36) not null
tag_id          varchar(36) not null
status          smallint not null default 1
created_at      bigint not null
```

Indexes:

```text
unique(topic_id, tag_id)
index(tag_id, status, created_at desc)
```

### articles

Purpose: knowledge base articles.

```text
id                  varchar(36) primary key
author_id           varchar(36) not null
author_snapshot     jsonb not null default '{}'
title               varchar(180) not null
summary             text null
content             text not null
content_type        varchar(16) not null default 'markdown'
cover               jsonb null
source_url          text null

view_count          bigint not null default 0
comment_count       bigint not null default 0
like_count          bigint not null default 0
favorite_count      bigint not null default 0

status              smallint not null default 2
created_at          bigint not null
updated_at          bigint not null
deleted_at          bigint not null default 0
```

Indexes:

```text
index(status, created_at desc)
index(author_id, status, created_at desc)
index(view_count desc)
```

### article_tags

```text
id              varchar(36) primary key
article_id      varchar(36) not null
tag_id          varchar(36) not null
status          smallint not null default 1
created_at      bigint not null
```

Indexes:

```text
unique(article_id, tag_id)
index(tag_id, status, created_at desc)
```

### content_outbox

Purpose: reliable Kafka publication.

```text
id                  varchar(36) primary key
aggregate_type      varchar(32) not null
aggregate_id        varchar(36) not null
event_type          varchar(96) not null
payload_json        jsonb not null
status              smallint not null default 0
retry_count         integer not null default 0
next_retry_at       bigint not null default 0
created_at          bigint not null
published_at        bigint not null default 0
```

Indexes:

```text
index(status, next_retry_at, created_at)
index(aggregate_type, aggregate_id)
```

## bbs_reaction

Owner: `reaction-service`

### user_likes

```text
id              varchar(36) primary key
user_id         varchar(36) not null
entity_type     varchar(32) not null
entity_id       varchar(36) not null
status          smallint not null default 1
created_at      bigint not null
updated_at      bigint not null
```

Indexes:

```text
unique(user_id, entity_type, entity_id)
index(entity_type, entity_id, status)
index(user_id, status, created_at desc)
```

### favorites

```text
id              varchar(36) primary key
user_id         varchar(36) not null
entity_type     varchar(32) not null
entity_id       varchar(36) not null
created_at      bigint not null
deleted_at      bigint not null default 0
```

Indexes:

```text
unique(user_id, entity_type, entity_id)
index(user_id, deleted_at, created_at desc)
index(entity_type, entity_id, deleted_at)
```

### user_reports

```text
id              varchar(36) primary key
data_type       varchar(32) not null
data_id         varchar(36) not null
user_id         varchar(36) not null
reason          varchar(1024) not null
audit_status    smallint not null default 0
audit_user_id   varchar(36) not null default ''
audit_note      varchar(1024) null
audit_at        bigint not null default 0
created_at      bigint not null
updated_at      bigint not null
```

Indexes:

```text
index(audit_status, created_at desc)
index(data_type, data_id)
index(user_id, created_at desc)
```

### reaction_outbox

Same shape as `content_outbox`.

## bbs_notification

Owner: `notification-service`

### messages

```text
id              varchar(36) primary key
user_id         varchar(36) not null
type            varchar(32) not null
title           varchar(160) not null
content         text null
quote_content   text null
source_type     varchar(32) not null default ''
source_id       varchar(36) not null default ''
status          smallint not null default 0
created_at      bigint not null
read_at         bigint not null default 0
deleted_at      bigint not null default 0
```

Indexes:

```text
index(user_id, status, created_at desc)
index(user_id, type, created_at desc)
index(source_type, source_id)
```

P0 message types:

```text
system
comment
like
favorite
follow
report_audit
content_audit
```

## bbs_admin

Owner: `admin-service`

### roles

```text
id              varchar(36) primary key
type            smallint not null default 1
name            varchar(64) not null
code            varchar(64) not null
sort_no         integer not null default 0
remark          varchar(256) null
status          smallint not null default 1
created_at      bigint not null
updated_at      bigint not null
```

Indexes:

```text
unique(code)
index(status, sort_no)
```

### permissions

```text
id              varchar(36) primary key
type            varchar(32) not null
code            varchar(128) not null
name            varchar(64) not null
group_name      varchar(64) not null
description     varchar(256) null
sort_no         integer not null default 0
status          smallint not null default 1
created_at      bigint not null
updated_at      bigint not null
```

Indexes:

```text
unique(code)
index(type, status)
index(group_name, sort_no)
```

### role_permissions

```text
id              varchar(36) primary key
role_id         varchar(36) not null
permission_id   varchar(36) not null
created_at      bigint not null
```

Indexes:

```text
unique(role_id, permission_id)
index(permission_id)
```

### forbidden_words

```text
id              varchar(36) primary key
type            varchar(16) not null default 'word'
word            varchar(128) not null
remark          varchar(1024) null
status          smallint not null default 1
created_at      bigint not null
updated_at      bigint not null
deleted_at      bigint not null default 0
```

Indexes:

```text
unique(type, word) where deleted_at = 0
index(status)
```

## bbs_config

Owner: `config-service`

### sys_configs

```text
id              varchar(36) primary key
key             varchar(128) not null
value_json      jsonb not null
name            varchar(128) not null
description     varchar(1024) null
visibility      varchar(16) not null default 'public'
created_at      bigint not null
updated_at      bigint not null
```

Indexes:

```text
unique(key)
index(visibility)
```

P0 required keys:

```text
site.base
site.navs
site.modules
login.password
content.publish_rules
```

## bbs_audit

Owner: `audit-service`

### operate_logs

```text
id              varchar(36) primary key
actor_id        varchar(36) not null default ''
op_type         varchar(32) not null
data_type       varchar(32) not null
data_id         varchar(36) not null default ''
description     varchar(1024) not null
ip              varchar(64) null
user_agent      text null
referer         text null
trace_id        varchar(64) null
created_at      bigint not null
```

Indexes:

```text
index(actor_id, created_at desc)
index(op_type, created_at desc)
index(data_type, data_id)
index(trace_id)
```

## MongoDB: bbs_comment

Owner: `comment-service`

### comments

Document:

```json
{
  "_id": "comment_id",
  "entityType": "topic",
  "entityId": "topic_id",
  "parentId": "",
  "rootId": "",
  "quoteId": "",
  "authorId": "user_id",
  "authorSnapshot": {
    "nickname": "许恪",
    "avatar": "https://..."
  },
  "content": "comment body",
  "contentType": "markdown",
  "imageList": [],
  "status": "published",
  "likeCount": 0,
  "replyCount": 0,
  "ip": "",
  "ipLocation": "",
  "userAgent": "",
  "createdAt": 1783160000000,
  "updatedAt": 1783160000000,
  "deletedAt": 0
}
```

Indexes:

```text
{ entityType: 1, entityId: 1, status: 1, createdAt: -1 }
{ rootId: 1, status: 1, createdAt: 1 }
{ parentId: 1, status: 1, createdAt: 1 }
{ authorId: 1, createdAt: -1 }
{ quoteId: 1 }
```

P0 reply model:

- Recommended P0: two-level comments.
- Root comments have `parentId = ""` and `rootId = ""`.
- Replies have `parentId = root comment id` and `rootId = root comment id`.
- `quoteId` can point to another reply for quote display, but query remains under root.

## Elasticsearch P0 Indices

Owner: `search-service`

### bbs_topics

Fields:

```text
id                  keyword
type                keyword
title               text + keyword subfield
content_excerpt     text
category_id         keyword
category_name       keyword/text
tag_ids             keyword
tag_names           keyword/text
author_id           keyword
author_nickname     keyword/text
status              integer
recommend           boolean
sticky              boolean
view_count          long
comment_count       long
like_count          long
favorite_count      long
created_at          date(format: epoch_millis)
updated_at          date(format: epoch_millis)
```

### bbs_articles

Fields:

```text
id                  keyword
title               text + keyword subfield
summary             text
content_excerpt     text
tag_ids             keyword
tag_names           keyword/text
author_id           keyword
author_nickname     keyword/text
status              integer
view_count          long
comment_count       long
like_count          long
favorite_count      long
created_at          date(format: epoch_millis)
updated_at          date(format: epoch_millis)
```

### bbs_users

Fields:

```text
id                  keyword
username            keyword/text
nickname            keyword/text
description         text
avatar              keyword
status              integer
level_snapshot      integer
score_snapshot      integer
created_at          date(format: epoch_millis)
```

### bbs_tags

Fields:

```text
id                  keyword
name                keyword/text
slug                keyword
description         text
status              integer
created_at          date(format: epoch_millis)
```

## Seed Data

P0 seed records:

```text
bbs_admin.roles
  admin
  user

bbs_admin.permissions
  dashboard.view
  dashboard.user.view
  dashboard.user.forbidden
  dashboard.topic.view
  dashboard.topic.audit
  dashboard.topic.delete
  dashboard.article.view
  dashboard.article.audit
  dashboard.article.delete
  dashboard.userReport.view
  dashboard.userReport.audit

bbs_config.sys_configs
  site.base
  site.navs
  site.modules
  login.password
  content.publish_rules

bbs_content.categories
  recommendation/default category
```

## Migration Order

Recommended order for P0:

1. `bbs_admin`: roles, permissions, role_permissions, forbidden_words.
2. `bbs_config`: sys_configs.
3. `bbs_user`: users, user_follows.
4. `bbs_auth`: credentials, auth_sessions, email_codes.
5. `bbs_content`: categories, tags, topics, topic_tags, articles, article_tags, content_outbox.
6. `bbs_reaction`: user_likes, favorites, user_reports, reaction_outbox.
7. `bbs_notification`: messages.
8. `bbs_audit`: operate_logs.
9. MongoDB comment indexes.
10. Elasticsearch mappings.

Reasoning:

- Admin/config/user/auth are needed before content publishing.
- Content exists before reactions/comments/search indexing are meaningful.
- MongoDB and ES can be initialized after service bootstrap but before full integration tests.

## Locked P0 Schema Decisions

1. Password hashes live in `bbs_auth.credentials`; `bbs_user.users` stores no credential secrets.
2. P0 articles support full CRUD.
3. P0 topic types are `topic` and `tweet`; `qa` waits for bounty/accepted-answer support.
4. Admin roles/permissions are real P0 tables seeded with simple `admin` and `user` roles.
5. Elasticsearch timestamps use `date` with `epoch_millis` format.
