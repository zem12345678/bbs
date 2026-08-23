# BBS Platform Planning Overview

## Goal

Build a community platform with functional parity to `mlogclub/bbs-go`, while using the current `frontend` visual direction as the UI baseline and designing the backend as a Go microservice system.

This planning phase does not implement code. It establishes feature scope, service boundaries, data ownership, event contracts, and delivery phases.

## Source Scope

Primary references inspected:

- `mlogclub/bbs-go` README: https://github.com/mlogclub/bbs-go/blob/master/README.md
- `bbs-go` repository structure, route definitions, model definitions, config DTOs, admin permissions, and frontend route files from a temporary local clone.

Important licensing note: `bbs-go` is GPL-3.0 licensed. The target here is feature parity and architectural planning, not copying source code.

## Product Positioning

The target product is not only a forum. It should cover:

- Forum discussion
- Q&A / help community
- Article and knowledge base publishing
- User profiles and social relations
- Notifications and private site messages
- Likes, favorites, follows, reports
- Search
- Growth system: check-in, tasks, points, experience, levels, badges
- Operation backend: content moderation, user governance, permission governance, settings, logs

## Target Architecture

Required backend stack:

- Go services
- gRPC internal communication
- API gateway / BFF for frontend HTTP APIs
- etcd service discovery
- Nacos configuration center
- Kafka message queue
- Redis cache
- PostgreSQL primary relational store
- Elasticsearch search engine
- MongoDB comment storage

Recommended service set:

- `api-gateway`
- `user-service`
- `auth-service`
- `content-service`
- `comment-service`
- `reaction-service`
- `search-service`
- `credit-service`
- `notification-service`
- `admin-service`
- `file-service`
- `config-service`
- `audit-service`

## UI Direction

The existing frontend should remain the visual foundation:

- Light gray page background
- White cards and panels
- Blue primary accent
- Fixed top navigation
- Three-column desktop layout
- Single-column mobile layout
- Community feed as the first-class experience

Future pages should extend this style rather than switching to a generic admin template or traditional bulletin-board skin.

## Planning Documents

- [01-feature-inventory.md](./01-feature-inventory.md): feature parity checklist.
- [02-service-architecture.md](./02-service-architecture.md): service boundaries and dependencies.
- [03-data-ownership.md](./03-data-ownership.md): datastore ownership and major entities.
- [04-event-contracts.md](./04-event-contracts.md): Kafka event topics and responsibilities.
- [05-frontend-map.md](./05-frontend-map.md): frontend page map based on current UI.
- [06-roadmap.md](./06-roadmap.md): recommended implementation phases.
- [07-database-split.md](./07-database-split.md): microservice database/schema split plan.
- [08-p0-api-contracts.md](./08-p0-api-contracts.md): P0 gRPC and HTTP API contract draft.
- [09-p0-schema-draft.md](./09-p0-schema-draft.md): P0 PostgreSQL, MongoDB, and Elasticsearch schema draft.
- [10-local-dev-topology.md](./10-local-dev-topology.md): local infrastructure, service ports, config, discovery, and startup topology.
- [11-p0-implementation-backlog.md](./11-p0-implementation-backlog.md): executable P0 implementation backlog and acceptance criteria.
- [12-local-infra-compose-draft.md](./12-local-infra-compose-draft.md): Docker Compose design draft for local infrastructure.
- [15-interface-api-development-plan.md](./15-interface-api-development-plan.md): Misskey/Sharkey 610 操作的接口设计、兼容、服务归属、分阶段交付和验收规划（中文版）。
- [15-interface-api-development-plan.zh-CN.md](./15-interface-api-development-plan.zh-CN.md): 接口规划中文版入口和中文摘要。

## Key Decisions

1. Use PostgreSQL for primary business records, not MySQL.
2. Store comments in MongoDB, but maintain necessary counters and moderation metadata in PostgreSQL/Redis for query and governance.
3. Treat search as eventually consistent through Kafka and Elasticsearch.
4. Treat likes, favorites, follows, counters, and rankings as high-frequency interactions backed by Redis and eventually persisted.
5. Keep user-facing HTTP contracts at `api-gateway`; internal services expose gRPC.
6. Use Nacos for runtime config and feature switches; use etcd only for service discovery.

## Later Product Questions

- Should the frontend be a pure SPA, or should SEO pages later use SSR/prerendering?
- Should admin backend be integrated into the same frontend app or separated under `/admin`?
- Should comments evolve beyond the P0 two-level reply model?
- Should production file storage start local-only, or immediately support S3-compatible object storage?
- Should OAuth include WeChat, GitHub, Google from day one, or phase them after password/email login?
