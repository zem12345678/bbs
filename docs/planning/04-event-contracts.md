# Event Contracts

Kafka is the backbone for asynchronous side effects. This document defines first-pass topic names and payload conventions.

## Conventions

Topic naming:

```text
<domain>.<entity>.<event>
```

Envelope:

```json
{
  "eventId": "uuid",
  "eventType": "content.topic.created",
  "occurredAt": 0,
  "producer": "content-service",
  "traceId": "request-trace-id",
  "actorId": "user-id-or-system",
  "version": 1,
  "payload": {}
}
```

Consumer requirements:

- Handle duplicate events idempotently.
- Persist consumer offsets normally through Kafka consumer group.
- Use `eventId` or business idempotency keys for side-effect dedupe.
- Do not assume event ordering across different aggregate IDs.

## Content Events

| Event | Producer | Consumers | Purpose |
| --- | --- | --- | --- |
| `content.topic.created` | `content-service` | `search-service`, `credit-service`, `notification-service`, `audit-service` | Index, task reward, notify followers. |
| `content.topic.updated` | `content-service` | `search-service`, `audit-service` | Reindex and audit. |
| `content.topic.deleted` | `content-service` | `search-service`, `audit-service` | Remove/deactivate index. |
| `content.topic.audited` | `admin-service`/`content-service` | `search-service`, `notification-service`, `audit-service` | Publish approval/rejection result. |
| `content.topic.recommended` | `content-service` | `search-service`, `notification-service` | Ranking/index metadata. |
| `content.topic.sticky_changed` | `content-service` | `search-service` | List ranking metadata. |
| `content.qa.accepted` | `content-service` | `credit-service`, `notification-service`, `search-service` | Bounty settlement, notify answer author. |
| `content.article.created` | `content-service` | `search-service`, `credit-service`, `notification-service`, `audit-service` | Index/reward/notify. |
| `content.article.updated` | `content-service` | `search-service`, `audit-service` | Reindex and audit. |
| `content.article.deleted` | `content-service` | `search-service`, `audit-service` | Remove/deactivate index. |
| `content.article.audited` | `content-service` | `search-service`, `notification-service` | Approval/rejection. |
| `content.tag.changed` | `content-service` | `search-service` | Reindex tag projections. |
| `content.category.changed` | `content-service` | `search-service`, `config-service` | Refresh nav/category cache. |

## Comment Events

| Event | Producer | Consumers | Purpose |
| --- | --- | --- | --- |
| `comment.created` | `comment-service` | `content-service`, `reaction-service`, `search-service`, `credit-service`, `notification-service` | Counters, task progress, notify entity author. |
| `comment.deleted` | `comment-service` | `content-service`, `search-service`, `notification-service`, `audit-service` | Counter decrement, index update. |
| `comment.audited` | `comment-service` | `notification-service`, `audit-service` | Notify moderation result. |

Payload sketch:

```json
{
  "commentId": "id",
  "entityType": "topic",
  "entityId": "id",
  "rootId": "id",
  "parentId": "id",
  "quoteId": "id",
  "authorId": "id",
  "status": "published"
}
```

## Reaction Events

| Event | Producer | Consumers | Purpose |
| --- | --- | --- | --- |
| `reaction.liked` | `reaction-service` | `content-service`, `comment-service`, `notification-service`, `credit-service`, `search-service` | Counters, notify, task progress. |
| `reaction.unliked` | `reaction-service` | `content-service`, `comment-service`, `search-service` | Counter decrement. |
| `reaction.favorited` | `reaction-service` | `content-service`, `notification-service`, `credit-service` | Counters/tasks. |
| `reaction.unfavorited` | `reaction-service` | `content-service` | Counter decrement. |
| `reaction.reported` | `reaction-service` | `admin-service`, `notification-service` | Create moderation work item. |
| `vote.cast` | `reaction-service` | `content-service`, `credit-service` | Vote counts and task progress. |

## User Events

| Event | Producer | Consumers | Purpose |
| --- | --- | --- | --- |
| `user.created` | `user-service` | `search-service`, `credit-service`, `notification-service` | Index user, newbie task, welcome message. |
| `user.updated` | `user-service` | `search-service`, `notification-service` | Update index/profile projections. |
| `user.followed` | `user-service` | `notification-service`, `credit-service` | Notify followed user, task progress. |
| `user.unfollowed` | `user-service` | `notification-service` | Optional notification/cache update. |
| `user.muted` | `user-service` | `notification-service`, `audit-service` | Governance notice. |
| `user.email.verified` | `auth-service` | `credit-service`, `notification-service` | Task progress. |

## Credit Events

| Event | Producer | Consumers | Purpose |
| --- | --- | --- | --- |
| `credit.checkin.completed` | `credit-service` | `notification-service`, `search-service` | Notify and ranking update. |
| `credit.score.changed` | `credit-service` | `notification-service`, `search-service` | User points update. |
| `credit.exp.changed` | `credit-service` | `search-service` | User experience update. |
| `credit.level.changed` | `credit-service` | `notification-service`, `search-service` | User level display. |
| `credit.badge.granted` | `credit-service` | `notification-service`, `search-service` | Badge display and notice. |
| `credit.task.completed` | `credit-service` | `notification-service` | Task completion notice. |

## File Events

| Event | Producer | Consumers | Purpose |
| --- | --- | --- | --- |
| `file.uploaded` | `file-service` | `audit-service` | Upload audit. |
| `file.attachment.bound` | `content-service` | `file-service`, `audit-service` | Bind attachment to topic. |
| `file.attachment.downloaded` | `file-service` | `credit-service`, `audit-service` | Charge or log download. |
| `file.attachment.score_updated` | `file-service` | `audit-service` | Governance/audit. |

## Admin And Audit Events

| Event | Producer | Consumers | Purpose |
| --- | --- | --- | --- |
| `admin.permission.changed` | `admin-service` | `api-gateway`, `user-service` | Refresh permission cache. |
| `admin.config.changed` | `config-service` | all services | Refresh runtime config. |
| `admin.forbidden_word.changed` | `admin-service` | `content-service`, `comment-service` | Refresh moderation rules. |
| `audit.operation.recorded` | any service | `audit-service` | Central audit log. |

## Dead Letter Topics

Recommended:

- `dlq.search`
- `dlq.notification`
- `dlq.credit`
- `dlq.audit`

Each DLQ message should include original topic, partition, offset, event body, error, and retry count.

## Initial Consumer Groups

| Consumer Group | Service | Topics |
| --- | --- | --- |
| `search-indexer` | `search-service` | content, comment, reaction, user, credit events |
| `notification-dispatcher` | `notification-service` | content, comment, reaction, user, credit events |
| `credit-task-engine` | `credit-service` | user, content, comment, reaction, vote events |
| `counter-projector` | varies | comment/reaction/view events |
| `audit-writer` | `audit-service` | audit/admin/content governance events |
