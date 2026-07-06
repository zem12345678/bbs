# P0 Implementation Backlog

This document converts the P0 planning artifacts into executable implementation tasks. It is still planning work; it does not create code.

## P0 Product Goal

Deliver a first usable community release where a user can:

1. Register and sign in.
2. Edit profile and follow users.
3. Browse, create, edit, delete, like, favorite, and search topics.
4. Browse, create, edit, delete, like, favorite, and search articles.
5. Comment on topics/articles.
6. Receive basic site messages.
7. Use the current frontend visual system across public, auth, profile, search, and minimal admin pages.
8. Let an admin mute users, audit/delete content, review reports, and see operation logs.

## P0 Assumptions To Lock Before Coding

Unless changed before implementation, use these recommended decisions:

| Decision | P0 Assumption |
| --- | --- |
| Auth token | Opaque token with Redis cache and PostgreSQL session record. |
| Password hash ownership | Add `bbs_auth.credentials`; keep `bbs_user.users` free of password hash. |
| Article scope | Full article CRUD is in P0 because frontend/API planning already includes it. |
| Topic types | P0 supports `topic` and `tweet`; `qa` waits until bounty/accepted answer exists. |
| Comment nesting | Two-level model: root comments plus replies; quote display is allowed. |
| Delete behavior | Status transition/soft delete, not hard delete. |
| Admin RBAC | Real role/permission tables, simple `admin` and `user` roles in P0. |
| ES timestamp type | `date` with `epoch_millis` format. |
| Local PostgreSQL layout | One local PostgreSQL instance with separate schemas first. |
| Local services | Infrastructure in containers; Go services run on host during development. |
| Frontend API | Frontend calls only `api-gateway`; mock data can remain only before backend slice exists. |

If one of these changes, update `08-p0-api-contracts.md`, `09-p0-schema-draft.md`, and `10-local-dev-topology.md` before generating code.

## Delivery Sequence

Use vertical slices. Each slice must leave the application runnable.

| Slice | Outcome | Depends On |
| --- | --- | --- |
| S0: Decision lock | P0 scope has no unresolved contract/schema/topology blockers. | Current planning docs |
| S1: Foundation | Local infra, repo skeleton, shared packages, service bootstrap. | S0 |
| S2: Auth and profile | Register, sign in, current user, public profile, profile edit. | S1 |
| S3: Topic publishing | Topic feed/detail/create/edit/delete, categories/tags. | S2 |
| S4: Comments and reactions | Comments, likes, favorites, reports, counters. | S3 |
| S5: Articles and search | Article CRUD and ES search/indexing for topics/articles. | S3/S4 |
| S6: Messages and admin P0 | Messages, user/content/report governance, audit logs. | S4/S5 |
| S7: Frontend completion pass | Public/auth/profile/search/admin pages wired to gateway. | S2-S6 |
| S8: P0 stabilization | Tests, seed data, docs, local runbook, regression fixes. | S7 |

## Backlog Format

Each item has:

- Output: artifact or behavior that must exist.
- Verify: concrete check before marking done.
- Notes: constraints or cross-document references.

## S0: Decision Lock

| ID | Task | Output | Verify |
| --- | --- | --- | --- |
| P0-S0-01 | Confirm contract decisions | `08-p0-api-contracts.md` reflects final P0 decisions | No conflicting P0/P1 statements for articles, QA, comments, delete behavior, token style |
| P0-S0-02 | Confirm schema decisions | `09-p0-schema-draft.md` reflects auth credentials, ES timestamp, role tables | Schema draft matches P0 assumptions table |
| P0-S0-03 | Confirm local topology decisions | `10-local-dev-topology.md` locks local Postgres/Nacos/service process choices | Local infra plan can be converted directly to Compose |
| P0-S0-04 | Freeze P0 frontend route list | `05-frontend-map.md` marks P0 pages and deferred pages clearly | Every P0 page maps to an API/gateway need |

## S1: Foundation

### Repository And Tooling

| ID | Task | Output | Verify |
| --- | --- | --- | --- |
| P0-S1-01 | Create backend repository skeleton | `backend/` with service folders, `proto/`, `pkg/`, `deployments/`, `migrations/` | Tree matches `02-service-architecture.md` |
| P0-S1-02 | Add Go workspace/module strategy | Root `go.work` or service modules | `go test ./...` can discover packages once stubs exist |
| P0-S1-03 | Add formatting/lint conventions | `gofmt`, `go vet`, optional lint config | Local command documented and repeatable |
| P0-S1-04 | Add frontend/backend env convention | `.env.example` style documentation, no real secrets | New developer can identify required local env vars |

### Local Infrastructure

| ID | Task | Output | Verify |
| --- | --- | --- | --- |
| P0-S1-05 | Create local infrastructure Compose | PostgreSQL, Redis, MongoDB, Kafka, etcd, Nacos, Elasticsearch | Containers start and health checks pass |
| P0-S1-06 | Add infrastructure bootstrap scripts | PG schemas/users, Kafka topics, Mongo indexes, ES mappings | Running bootstrap twice is safe |
| P0-S1-07 | Add seed data loader | Admin/user roles, public config, default category | Fresh local environment has required P0 seed records |
| P0-S1-08 | Add local runbook | Developer instructions for starting infra/services/frontend | Steps work from a clean machine with dependencies installed |

### Shared Backend Packages

| ID | Task | Output | Verify |
| --- | --- | --- | --- |
| P0-S1-09 | Shared config client | Nacos loading plus env bootstrap | Service can read common and service config |
| P0-S1-10 | Shared discovery client | etcd registration and lookup | Service registers with TTL; gateway resolves it |
| P0-S1-11 | Shared logging/tracing context | Structured logs with trace id | Gateway trace id appears in service logs |
| P0-S1-12 | Shared error model | Domain errors map to HTTP/gRPC status | API error matches `08-p0-api-contracts.md` |
| P0-S1-13 | Shared pagination types | Page/pageSize response helpers | List APIs return consistent pagination shape |
| P0-S1-14 | Shared auth context propagation | Gateway attaches actor context to gRPC metadata | Service receives actor id, roles, ip, user agent |
| P0-S1-15 | Shared Kafka wrapper and outbox worker base | Producer/consumer helpers and idempotent consumer pattern | Test event can be produced/consumed locally |

### Service Bootstrap

| ID | Task | Output | Verify |
| --- | --- | --- | --- |
| P0-S1-16 | Add gRPC health checks | Every service exposes health | Gateway/start scripts can check service readiness |
| P0-S1-17 | Add service startup template | Config load, DB connect, etcd register, graceful shutdown | One stub service starts and exits cleanly |
| P0-S1-18 | Add API gateway skeleton | HTTP server, routing, error wrapper, auth middleware placeholder | `GET /healthz` works |

## S2: Auth And Profile

### Backend

| ID | Task | Output | Verify |
| --- | --- | --- | --- |
| P0-S2-01 | Define auth/user proto v1 | `auth.proto`, `user.proto` | Proto lint/generation passes |
| P0-S2-02 | Add auth migrations | `bbs_auth.credentials`, `auth_sessions`, `email_codes` | Migration up/down or reset works locally |
| P0-S2-03 | Add user migrations | `bbs_user.users`, `user_follows`, user role snapshot fields | Migration works and constraints enforce uniqueness |
| P0-S2-04 | Implement `UserService.CreateUser` | Creates profile and user role projection | Duplicate username/email returns conflict |
| P0-S2-05 | Implement password signup/signin | Opaque token, session record, Redis cache | User can sign up and sign in through gateway |
| P0-S2-06 | Implement session validation/signout | Token validation and revocation | Revoked token is rejected |
| P0-S2-07 | Implement reset password email flow | Email code/token record and local Mailpit send when enabled | Reset token updates password and expires/marks used |
| P0-S2-08 | Implement current/public profile APIs | Current user, public user detail | Anonymous can read public profile; private fields hidden |
| P0-S2-09 | Implement profile update | Nickname, avatar URL, background, description, homepage | Update publishes `user.updated` |
| P0-S2-10 | Implement follow/fans/followed | Follow/unfollow and relation lists | Follow is idempotent; counters update |

### Gateway

| ID | Task | Output | Verify |
| --- | --- | --- | --- |
| P0-S2-11 | Add auth HTTP routes | `/api/auth/*` | API contract examples work |
| P0-S2-12 | Add user HTTP routes | `/api/users/*` | Current/profile/follow endpoints work |
| P0-S2-13 | Add auth middleware | Token to gRPC actor context | Protected endpoint rejects anonymous user |

### Frontend

| ID | Task | Output | Verify |
| --- | --- | --- | --- |
| P0-S2-14 | Wire signin/signup pages | `/user/signin`, `/user/signup` call gateway | Successful auth updates UI state |
| P0-S2-15 | Wire forgot/reset password pages | Password reset flow UI | Invalid/expired token shows error state |
| P0-S2-16 | Wire current user shell state | Topbar/profile menu/message entry | Refresh preserves signed-in state |
| P0-S2-17 | Wire profile/settings pages | `/user/:id`, `/user/profile` | Edit profile updates public profile |
| P0-S2-18 | Wire fans/followed pages | `/user/:id/fans`, `/user/:id/followed` | Follow/unfollow state is reflected |

## S3: Topic Publishing

### Backend

| ID | Task | Output | Verify |
| --- | --- | --- | --- |
| P0-S3-01 | Define content proto v1 | Topic/category/tag methods | Proto generation passes |
| P0-S3-02 | Add content migrations | Categories, tags, topics, topic_tags, content_outbox | Seed category exists |
| P0-S3-03 | Implement category/tag reads | List/get/autocomplete | Disabled/deleted records hidden publicly |
| P0-S3-04 | Implement topic create/update/delete | Topic/tweet only, status transition delete | Muted user cannot publish |
| P0-S3-05 | Implement topic feed/detail | Filters: latest, category, tag, author, recommend | Feed and detail match API contract |
| P0-S3-06 | Implement author/category snapshots | Snapshot written when content is created/updated | Feed does not require user-service join per item |
| P0-S3-07 | Implement content outbox publication | Topic created/updated/deleted events | Events visible in Kafka UI |
| P0-S3-08 | Implement view count path | Redis counter and planned flush/reconcile hook | Repeated detail views increment visible count eventually |

### Gateway

| ID | Task | Output | Verify |
| --- | --- | --- | --- |
| P0-S3-09 | Add topic/category/tag HTTP routes | `/api/topics`, `/api/categories`, `/api/tags` | Contract endpoints pass smoke calls |
| P0-S3-10 | Add home page aggregation | `/api/pages/home` | Home payload includes config, topics, articles placeholder, categories |
| P0-S3-11 | Add topic detail aggregation | `/api/pages/topics/:id` | Detail payload includes topic, current user reaction placeholders, comments placeholder |

### Frontend

| ID | Task | Output | Verify |
| --- | --- | --- | --- |
| P0-S3-12 | Replace topic mock feed with API data | Home and `/topics` render live topic list | Empty/loading/error states fit current UI |
| P0-S3-13 | Wire topic detail page | `/topic/:id` | Detail renders content, tags, author, counters |
| P0-S3-14 | Wire topic create/edit pages | `/topic/create`, `/topic/edit/:id` | Publish/edit/delete flows work |
| P0-S3-15 | Wire category/tag pages | Category and tag filtered topic lists | Filtered route matches API results |

## S4: Comments And Reactions

### Backend

| ID | Task | Output | Verify |
| --- | --- | --- | --- |
| P0-S4-01 | Define comment/reaction proto v1 | Comment and reaction/report methods | Proto generation passes |
| P0-S4-02 | Add MongoDB comment indexes | `bbs_comment.comments` indexes | Index bootstrap is idempotent |
| P0-S4-03 | Add reaction migrations | `user_likes`, `favorites`, `user_reports`, `reaction_outbox` | Unique constraints enforce idempotency |
| P0-S4-04 | Implement comment create/list/delete | Two-level comments and replies | Entity visibility is checked through content service |
| P0-S4-05 | Implement comment events/counters | `comment.created`, `comment.deleted` | Topic/article comment_count updates eventually |
| P0-S4-06 | Implement like/unlike | Topic/article/comment likes | Duplicate like does not double count |
| P0-S4-07 | Implement favorite/unfavorite/list | Topic/article favorites | User favorites page data is returned |
| P0-S4-08 | Implement report submit/admin list | Report submission and admin query | Duplicate/report state rules documented and enforced |
| P0-S4-09 | Implement reaction events/counters | Like/favorite/report events | Counters update via Redis/event path |

### Gateway

| ID | Task | Output | Verify |
| --- | --- | --- | --- |
| P0-S4-10 | Add comment HTTP routes | `/api/comments*` | Comment list/create/delete works |
| P0-S4-11 | Add reaction HTTP routes | `/api/reactions/*`, `/api/reports` | Like/favorite/report endpoints work |
| P0-S4-12 | Enrich detail aggregation | Include first comments and current liked/favorited state | Topic detail opens with one payload plus lazy lists |

### Frontend

| ID | Task | Output | Verify |
| --- | --- | --- | --- |
| P0-S4-13 | Wire comment list/editor | Topic/article details support comments | Reply/delete states work |
| P0-S4-14 | Wire like/favorite buttons | Cards and detail pages use live reaction state | Optimistic update reconciles after API response |
| P0-S4-15 | Wire report dialog | User can report content/comment | Success/error states are clear |
| P0-S4-16 | Wire my favorites page | `/user/favorites` | Favorite list opens target content |

## S5: Articles And Search

### Backend

| ID | Task | Output | Verify |
| --- | --- | --- | --- |
| P0-S5-01 | Extend content proto for articles | Article list/detail/create/update/delete methods | Proto generation passes |
| P0-S5-02 | Add article migrations | `articles`, `article_tags` | Article tag constraints work |
| P0-S5-03 | Implement article CRUD | List/detail/create/edit/delete | Author/admin permission rules work |
| P0-S5-04 | Publish article events | Created/updated/deleted/audited events | Events visible in Kafka |
| P0-S5-05 | Define search proto v1 | Search topics/articles/all | Proto generation passes |
| P0-S5-06 | Add ES mappings | `bbs_topics`, `bbs_articles`, `bbs_users`, `bbs_tags` | Mapping bootstrap idempotent |
| P0-S5-07 | Implement search consumers | Consume content/user/reaction/comment events | Topic/article index updates eventually |
| P0-S5-08 | Implement search APIs | Search topics/articles/all | Keyword search returns expected records |
| P0-S5-09 | Add reindex dev command/job | Rebuild index from service APIs or owned data | Fresh ES can be rebuilt locally |

### Gateway

| ID | Task | Output | Verify |
| --- | --- | --- | --- |
| P0-S5-10 | Add article HTTP routes | `/api/articles*` | Contract endpoints work |
| P0-S5-11 | Add article detail aggregation | `/api/pages/articles/:id` | Detail includes comments and reaction state |
| P0-S5-12 | Add search HTTP routes | `/api/search*` | Search tabs receive paged results |

### Frontend

| ID | Task | Output | Verify |
| --- | --- | --- | --- |
| P0-S5-13 | Wire article list/detail | `/articles`, `/article/:id` | Article pages match current visual style |
| P0-S5-14 | Wire article create/edit | `/article/create`, `/article/edit/:id` | Editor works for article body/tags |
| P0-S5-15 | Wire article tag page | `/articles/tag/:id` | Filtered article list works |
| P0-S5-16 | Wire search page | `/search` with topics/articles/users tab shell | Topic/article search works; users tab can show P1 placeholder if not shipped |

## S6: Messages And Admin P0

### Backend

| ID | Task | Output | Verify |
| --- | --- | --- | --- |
| P0-S6-01 | Define notification/admin proto v1 | Messages and P0 admin governance methods | Proto generation passes |
| P0-S6-02 | Add notification migration | `messages` | User message listing works |
| P0-S6-03 | Add admin/audit migrations | Roles, permissions, role_permissions, forbidden_words, operate_logs | Seed admin/user roles and P0 permissions |
| P0-S6-04 | Implement message APIs | List recent/list all/read/read all/create system | Unread count updates |
| P0-S6-05 | Implement notification consumers | Comment/like/favorite/follow/audit events create messages | Message appears after triggering event |
| P0-S6-06 | Implement admin user governance | List users, mute user | Muted user cannot publish/comment |
| P0-S6-07 | Implement admin content governance | List/audit/delete topics/articles | Status transitions hide rejected/deleted content |
| P0-S6-08 | Implement admin report audit | List and audit submitted reports | Audit result updates report status |
| P0-S6-09 | Implement audit logging | Operation logs for admin actions | Log includes actor, target, trace id |

### Gateway

| ID | Task | Output | Verify |
| --- | --- | --- | --- |
| P0-S6-10 | Add message HTTP routes | `/api/messages*` | User can read/mark messages |
| P0-S6-11 | Add admin HTTP routes | `/api/admin/*` | Admin endpoints require admin role |
| P0-S6-12 | Add admin overview aggregation | Counts/pending reports/latest audit data | Dashboard payload is available |

### Frontend

| ID | Task | Output | Verify |
| --- | --- | --- | --- |
| P0-S6-13 | Wire messages page | `/user/messages` | Read/read-all work |
| P0-S6-14 | Build admin shell | `/admin` layout consistent with current UI but denser | Auth/admin guard works |
| P0-S6-15 | Wire admin users page | User list, filters, mute action | Muted status visible after action |
| P0-S6-16 | Wire admin topics/articles pages | Audit/delete actions | Status updates reflect immediately |
| P0-S6-17 | Wire admin reports page | Report list and audit dialog | Report audit updates queue |
| P0-S6-18 | Wire admin overview | Dashboard counts and pending work | Overview loads without direct DB access |

## S7: Frontend Completion Pass

| ID | Task | Output | Verify |
| --- | --- | --- | --- |
| P0-S7-01 | Normalize API client layer | One gateway client with auth/error handling | No page calls service-specific URLs |
| P0-S7-02 | Replace remaining P0 mock data | Public/auth/profile/topic/article/comment/search/admin pages use APIs | `rg` shows no P0 mock feeds used in live routes |
| P0-S7-03 | Complete loading/error/empty states | Consistent states across all P0 pages | Manual viewport check on desktop/mobile |
| P0-S7-04 | Responsive pass | Current three-column desktop and single-column mobile retained | Playwright screenshots for key routes |
| P0-S7-05 | Editor UX pass | Topic/article/comment editors validate and preserve drafts where feasible | Validation messages are clear |
| P0-S7-06 | Permission state pass | Anonymous/authenticated/admin UI states | Unauthorized actions redirect or show sign-in |
| P0-S7-07 | Navigation pass | Top nav and user menu route to implemented pages | No P0 nav item leads to broken page |

## S8: P0 Stabilization

### Automated Checks

| ID | Task | Output | Verify |
| --- | --- | --- | --- |
| P0-S8-01 | Backend unit tests | Service validation, repository, auth/session, idempotency tests | `go test ./...` passes |
| P0-S8-02 | Backend integration tests | Local infra-backed service tests for critical flows | Tests can run against local Compose |
| P0-S8-03 | API gateway contract tests | HTTP routes return expected shapes/errors | Contract examples pass |
| P0-S8-04 | Frontend tests | Component/API state tests for key pages | Frontend test command passes |
| P0-S8-05 | E2E smoke tests | Register, sign in, publish topic, comment, like, favorite, search, admin audit | Playwright smoke passes |

### Reliability And Operations

| ID | Task | Output | Verify |
| --- | --- | --- | --- |
| P0-S8-06 | Outbox retry behavior | Failed event publication retries safely | Simulated Kafka outage does not lose event |
| P0-S8-07 | Consumer idempotency | Duplicate events do not double count or duplicate messages | Duplicate event test passes |
| P0-S8-08 | Counter reconciliation job | Reconcile likes/comments/view counters | Forced drift is corrected |
| P0-S8-09 | Reindex procedure | Search index can be rebuilt | Delete ES index and rebuild successfully |
| P0-S8-10 | Local backup/reset scripts | Developer can reset local infra safely | Reset returns to seeded baseline |
| P0-S8-11 | Logging and trace audit | Gateway trace id appears across service logs/events | One request trace can be followed end to end |
| P0-S8-12 | Security smoke | Password hashes, token revocation, admin guard, muted-user guard | Security checklist passes |

### Documentation

| ID | Task | Output | Verify |
| --- | --- | --- | --- |
| P0-S8-13 | Update local runbook | Start/stop/reset/test instructions | New developer can run P0 locally |
| P0-S8-14 | Update API docs | Public gateway endpoint examples | Examples match actual responses |
| P0-S8-15 | Update service ownership docs | Implemented service boundaries reflect reality | No service writes another service DB |
| P0-S8-16 | P0 release checklist | Manual and automated release checks | Checklist covers all P0 success criteria |

## Cross-Cutting Acceptance Criteria

P0 is not complete until all of the following are true:

1. Frontend calls only `api-gateway`.
2. Internal backend calls use gRPC.
3. Services register through etcd.
4. Runtime config is loaded through Nacos.
5. PostgreSQL writes are scoped to the owning service schema.
6. Comments are stored in MongoDB.
7. Search results come from Elasticsearch projection.
8. Mutating content/comment/reaction/user/admin flows publish or intentionally skip documented Kafka events.
9. Redis is used only for cache/session/counter/ranking behavior, not as the only source of truth.
10. Admin moderation does not directly mutate another service database.
11. Public pages match the current frontend visual direction.
12. The main P0 E2E flow passes from a fresh local environment.

## Deferred Explicitly Out Of P0

These should not be implemented during P0 unless the plan changes:

- OAuth login and third-party binding.
- SMS login.
- QA bounty and accepted answer.
- Votes/polls.
- Hidden content.
- Full task/credit/badge engine.
- Paid attachments and download scoring.
- Full RBAC management UI.
- SEO sitemap jobs.
- Install wizard.
- I18n.
- Marketplace/commerce features beyond placeholder navigation.

## Suggested First Coding Order

When the user approves implementation, start here:

1. Lock S0 decisions in the planning docs.
2. Create `backend/` skeleton and local infrastructure Compose.
3. Implement S1 service bootstrap with `api-gateway`, `auth-service`, and `user-service`.
4. Ship S2 auth/profile end to end through frontend.
5. Add S3 topic publishing.

Reason: this sequence proves the architecture with the smallest useful product loop before adding Kafka-heavy and MongoDB/ES-heavy slices.
