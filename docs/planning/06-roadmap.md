# Implementation Roadmap

This roadmap prioritizes working product slices over building every infrastructure piece first.

## Phase 0: Planning And Contracts

Deliverables:

- Feature inventory.
- Service ownership.
- Data ownership.
- Event contracts.
- Frontend page map.
- Initial proto/API design.
- P0 gRPC and HTTP API contract draft.
- P0 schema draft for PostgreSQL, MongoDB, and Elasticsearch.
- Local development topology design.
- P0 implementation backlog.
- Local infrastructure Compose draft.

Exit criteria:

- Every P0 feature has an owning service.
- Every P0 entity has an owning datastore.
- Every async side effect has a Kafka event or is explicitly synchronous.

## Phase 1: Foundation

Backend:

- Repository skeleton.
- Shared Go packages: logging, error model, pagination, auth context, config client, discovery client.
- gRPC service bootstrap.
- etcd service registration/discovery.
- Nacos config loading.
- PostgreSQL migration framework.
- Redis client wrapper.
- Kafka producer/consumer wrapper.
- API gateway with auth middleware.

Frontend:

- Keep current UI baseline.
- Add app shell routing and shared layout.
- Build auth pages shell.
- Build admin shell skeleton.

Exit criteria:

- Services start locally.
- API gateway can call at least one gRPC service.
- Config can be read from Nacos.
- Services register in etcd.
- Kafka test event can be produced and consumed.

## Phase 2: P0 Community Core

Services:

- `auth-service`: password signup/signin/signout, reset password.
- `user-service`: profile, current user, public profile.
- `content-service`: topic create/list/detail/edit/delete, category list, tag list/autocomplete.
- `comment-service`: create/list/delete comments.
- `reaction-service`: like/unlike, favorite/unfavorite.
- `search-service`: basic topic/article indexing and search.
- `notification-service`: site message basics.
- `admin-service`: user/content/report minimum governance.

Frontend:

- Home.
- Plaza/topic feed.
- Topic detail.
- Topic create/edit.
- User profile.
- Signin/signup/password reset.
- Search.
- Basic admin users/topics/articles/reports.

Exit criteria:

- A user can register, sign in, publish a topic, comment, like, favorite, search, and view profile.
- Admin can mute a user and delete/audit content.
- Search index updates from Kafka.

## Phase 3: Articles, Q&A, And Moderation

Backend:

- Article create/list/detail/edit/delete.
- QA mode with bounty.
- Accept/unaccept answer rules.
- Hidden content.
- Forbidden words.
- Comment moderation.
- Report audit workflow.
- Email verification gates for publishing/commenting.

Frontend:

- Article list/detail/create/edit.
- Q&A list and detail states.
- Accepted answer UI.
- Report dialog.
- Moderation states.

Exit criteria:

- Users can use topic, QA, and article flows.
- Bounty points are reserved and settled correctly.
- Reports and forbidden words have an admin workflow.

## Phase 4: Growth System

Backend:

- Check-in.
- Task engine.
- Score logs.
- Experience logs.
- Level config.
- Badges and user badges.
- Leaderboards.

Frontend:

- Tasks page.
- Check-in card.
- Member/growth page.
- Badge wall.
- Score/experience logs.
- Leaderboards.
- Admin tasks/badges/levels/logs.

Exit criteria:

- Domain events drive task progress.
- Users receive points/exp/badges from configured tasks.
- Score and check-in rankings work.

## Phase 5: Files, Attachments, And Storage

Backend:

- Image upload.
- Avatar/background/cover upload.
- Topic attachment upload.
- Attachment download.
- Paid attachment download with points.
- S3-compatible object storage.
- Upload config management.

Frontend:

- Editor image upload.
- Attachment uploader.
- Attachment download UI.
- Download score editing.

Exit criteria:

- Topics can include images and attachments.
- Paid attachment downloads deduct points only once per user/attachment.

## Phase 6: Admin, Config, SEO, And Polish

Backend:

- Full RBAC.
- Settings/config management.
- Notification delivery switches.
- Email logs.
- Operation logs.
- SEO sitemap generation.
- Search reindex UI/job.
- Dict/dict type management if still needed.
- OAuth/SMS login if deferred.
- I18n if required.

Frontend:

- Full admin pages.
- Settings page.
- Email logs and operation logs.
- SEO/reindex controls.
- OAuth binding UI.
- Mobile polish and accessibility checks.

Exit criteria:

- Admin can operate the site without direct database access.
- Config changes propagate safely.
- Core pages meet responsive and accessibility standards.

## Suggested P0 Cut

For a first usable release, include:

- Password auth.
- User profile.
- Topic feed, detail, create/edit/delete.
- Category and tag basics.
- Comments.
- Likes and favorites.
- Basic search.
- Basic notifications.
- Admin user/content/report governance.
- Redis counters.
- Kafka indexing events.
- PostgreSQL + MongoDB + Elasticsearch minimal setup.

Defer:

- OAuth/SMS login.
- Full task engine.
- Badges/levels.
- Paid attachments.
- Full RBAC granularity.
- SEO sitemap.
- Install wizard.
- I18n.

## Immediate Next Planning Tasks

1. Smoke-run the local Compose default profile and fix any image-specific healthcheck issues.
2. Turn the P0 contracts into real `.proto` drafts.
3. Turn the P0 schema draft into migration files once implementation starts.
4. Create the backend repository skeleton and service bootstrap.
5. Wire the first auth/profile vertical slice through `api-gateway` and the current frontend.
