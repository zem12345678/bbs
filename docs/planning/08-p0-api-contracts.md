# P0 API And Proto Contract Draft

This document defines the first usable API contract for P0. It is a planning artifact, not generated `.proto` code.

P0 scope is intentionally smaller than full `bbs-go` parity:

- Password auth.
- User profile.
- Topic feed/detail/create/edit/delete.
- Category and tag basics.
- Article list/detail/create/edit/delete.
- Comments.
- Likes and favorites.
- Basic search.
- Basic notifications.
- Minimal admin governance for users, topics, articles, reports.

Deferred to P1/P2:

- OAuth/SMS login.
- QA bounty and accepted answer.
- Votes.
- Hidden content.
- Full task/credit/badge system.
- Paid attachments.
- Full RBAC granularity.
- SEO sitemap.
- Install wizard.
- I18n.

## API Layers

```text
frontend
  -> api-gateway HTTP JSON API
    -> internal gRPC services
      -> service-owned databases
      -> Kafka events for async side effects
```

Rules:

1. Frontend only calls `api-gateway`.
2. `api-gateway` translates HTTP JSON into gRPC calls.
3. Internal services do not expose public HTTP.
4. Service-to-service calls are gRPC.
5. Async side effects use Kafka events.

## Common Types

### IDs

Use string IDs at external API boundaries even if internal storage uses int64.

Reason:

- Keeps API stable if ID strategy changes from int64 to UUIDv7.
- Avoids JavaScript large integer issues.
- Works for MongoDB comment IDs.

### Timestamps

Use Unix milliseconds as JSON numbers in P0.

Example:

```json
{
  "createdAt": 1783160000000
}
```

### Pagination

Request:

```json
{
  "page": 1,
  "pageSize": 20
}
```

Response:

```json
{
  "items": [],
  "page": 1,
  "pageSize": 20,
  "total": 128,
  "hasMore": true
}
```

For feed-style APIs, cursor pagination can be added later:

```json
{
  "cursor": "opaque_cursor",
  "limit": 20
}
```

### Error Model

HTTP response:

```json
{
  "code": "AUTH_REQUIRED",
  "message": "Login required",
  "details": {},
  "traceId": "..."
}
```

Common codes:

| Code | HTTP | Meaning |
| --- | --- | --- |
| `BAD_REQUEST` | 400 | Invalid input. |
| `AUTH_REQUIRED` | 401 | Not logged in. |
| `PERMISSION_DENIED` | 403 | Authenticated but not allowed. |
| `NOT_FOUND` | 404 | Entity does not exist or is invisible. |
| `CONFLICT` | 409 | Duplicate or state conflict. |
| `RATE_LIMITED` | 429 | Too many requests. |
| `VALIDATION_FAILED` | 422 | Field-level validation failed. |
| `INTERNAL` | 500 | Unexpected server error. |

gRPC should map domain errors to canonical gRPC status codes plus structured error details.

### Auth Context

`api-gateway` attaches auth context to all internal gRPC calls:

```text
actor_id
roles
permission_codes
ip
user_agent
trace_id
locale
```

Services should not parse frontend tokens directly.

## Internal gRPC Services

The following proto package names are recommended for later implementation:

```text
proto/bbs/auth/v1/auth.proto
proto/bbs/user/v1/user.proto
proto/bbs/content/v1/content.proto
proto/bbs/comment/v1/comment.proto
proto/bbs/reaction/v1/reaction.proto
proto/bbs/search/v1/search.proto
proto/bbs/notification/v1/notification.proto
proto/bbs/admin/v1/admin.proto
proto/bbs/config/v1/config.proto
```

### AuthService

Owner: `auth-service`

P0 methods:

```text
SignUp(SignUpRequest) returns (AuthSession)
SignIn(SignInRequest) returns (AuthSession)
SignOut(SignOutRequest) returns (Empty)
ValidateSession(ValidateSessionRequest) returns (AuthPrincipal)
SendResetPasswordEmail(SendResetPasswordEmailRequest) returns (Empty)
ResetPassword(ResetPasswordRequest) returns (Empty)
UpdatePassword(UpdatePasswordRequest) returns (Empty)
```

Key messages:

```text
SignUpRequest:
  username
  email
  password
  nickname
  captcha_id
  captcha_code

SignInRequest:
  username_or_email
  password
  captcha_id
  captcha_code

AuthSession:
  token
  expires_at
  user

AuthPrincipal:
  user_id
  roles
  permission_codes
  status
  forbidden_until
```

Synchronous dependencies:

- Calls `UserService.CreateUser` during signup.
- Calls `NotificationService.SendEmail` for reset email.

Events:

- `user.created` is produced by `user-service`, not `auth-service`.
- `user.email.verified` deferred to P1.

### UserService

Owner: `user-service`

P0 methods:

```text
CreateUser(CreateUserRequest) returns (User)
GetCurrentUser(GetCurrentUserRequest) returns (UserDetail)
GetUser(GetUserRequest) returns (UserDetail)
UpdateProfile(UpdateProfileRequest) returns (UserDetail)
SetAvatar(SetAvatarRequest) returns (UserDetail)
SetBackgroundImage(SetBackgroundImageRequest) returns (UserDetail)
FollowUser(FollowUserRequest) returns (FollowState)
UnfollowUser(UnfollowUserRequest) returns (FollowState)
IsFollowing(IsFollowingRequest) returns (FollowState)
ListFans(ListUserRelationRequest) returns (UserList)
ListFollowed(ListUserRelationRequest) returns (UserList)
MuteUser(MuteUserRequest) returns (UserDetail)
ListUsersForAdmin(AdminListUsersRequest) returns (AdminUserList)
UpdateUserForAdmin(AdminUpdateUserRequest) returns (UserDetail)
```

Key messages:

```text
User:
  id
  username
  nickname
  avatar
  description
  level_snapshot
  score_snapshot
  status

UserDetail:
  user
  email
  email_verified
  background_image
  homepage
  gender
  birthday
  topic_count
  article_count
  comment_count
  follow_count
  fans_count
  is_followed
```

Events:

- `user.created`
- `user.updated`
- `user.followed`
- `user.unfollowed`
- `user.muted`

### ContentService

Owner: `content-service`

P0 methods:

```text
CreateTopic(CreateTopicRequest) returns (Topic)
UpdateTopic(UpdateTopicRequest) returns (Topic)
DeleteTopic(DeleteTopicRequest) returns (Empty)
GetTopic(GetTopicRequest) returns (TopicDetail)
ListTopics(ListTopicsRequest) returns (TopicList)
ListUserTopics(ListUserTopicsRequest) returns (TopicList)

CreateArticle(CreateArticleRequest) returns (Article)
UpdateArticle(UpdateArticleRequest) returns (Article)
DeleteArticle(DeleteArticleRequest) returns (Empty)
GetArticle(GetArticleRequest) returns (ArticleDetail)
ListArticles(ListArticlesRequest) returns (ArticleList)
ListUserArticles(ListUserArticlesRequest) returns (ArticleList)

ListCategories(ListCategoriesRequest) returns (CategoryList)
GetCategory(GetCategoryRequest) returns (Category)
ListTags(ListTagsRequest) returns (TagList)
GetTag(GetTagRequest) returns (Tag)
AutocompleteTags(AutocompleteTagsRequest) returns (TagList)

AuditTopic(AuditTopicRequest) returns (Topic)
AuditArticle(AuditArticleRequest) returns (Article)
RecommendTopic(RecommendTopicRequest) returns (Topic)
```

Topic fields:

```text
Topic:
  id
  type              // topic | tweet
  category_id
  category_snapshot
  author_id
  author_snapshot
  title
  content
  content_type
  image_list
  tags
  status
  recommend
  sticky
  view_count
  comment_count
  like_count
  favorite_count
  created_at
  updated_at
```

Article fields:

```text
Article:
  id
  author_id
  author_snapshot
  title
  summary
  content
  content_type
  cover
  source_url
  tags
  status
  view_count
  comment_count
  like_count
  favorite_count
  created_at
  updated_at
```

List filters:

```text
ListTopicsRequest:
  page
  page_size
  type
  category_id
  tag_id
  author_id
  recommend
  order_by        // latest | hot | recommend
  status

ListArticlesRequest:
  page
  page_size
  tag_id
  author_id
  order_by
  status
```

Events:

- `content.topic.created`
- `content.topic.updated`
- `content.topic.deleted`
- `content.topic.audited`
- `content.article.created`
- `content.article.updated`
- `content.article.deleted`
- `content.article.audited`
- `content.tag.changed`
- `content.category.changed`

### CommentService

Owner: `comment-service`

P0 methods:

```text
CreateComment(CreateCommentRequest) returns (Comment)
DeleteComment(DeleteCommentRequest) returns (Empty)
ListComments(ListCommentsRequest) returns (CommentList)
ListReplies(ListRepliesRequest) returns (CommentList)
GetComment(GetCommentRequest) returns (Comment)
ListCommentsForAdmin(AdminListCommentsRequest) returns (AdminCommentList)
```

Comment fields:

```text
Comment:
  id
  entity_type      // topic | article
  entity_id
  parent_id
  root_id
  quote_id
  author_id
  author_snapshot
  content
  content_type
  image_list
  status
  like_count
  reply_count
  created_at
```

Validation:

- `api-gateway` or `comment-service` must verify entity visibility through `ContentService`.
- User mute check comes from auth context or `UserService`.

Events:

- `comment.created`
- `comment.deleted`

### ReactionService

Owner: `reaction-service`

P0 methods:

```text
Like(LikeRequest) returns (ReactionState)
Unlike(UnlikeRequest) returns (ReactionState)
ListLikedIds(ListLikedIdsRequest) returns (LikedIds)
Favorite(FavoriteRequest) returns (FavoriteState)
Unfavorite(UnfavoriteRequest) returns (FavoriteState)
ListFavorites(ListFavoritesRequest) returns (FavoriteList)
SubmitReport(SubmitReportRequest) returns (Report)
ListReportsForAdmin(AdminListReportsRequest) returns (AdminReportList)
AuditReport(AuditReportRequest) returns (Report)
```

Entity reference:

```text
EntityRef:
  entity_type      // topic | article | comment
  entity_id
```

Events:

- `reaction.liked`
- `reaction.unliked`
- `reaction.favorited`
- `reaction.unfavorited`
- `reaction.reported`

### SearchService

Owner: `search-service`

P0 methods:

```text
SearchTopics(SearchTopicsRequest) returns (SearchTopicList)
SearchArticles(SearchArticlesRequest) returns (SearchArticleList)
SearchAll(SearchAllRequest) returns (SearchResult)
```

P1 methods:

```text
SearchUsers(SearchUsersRequest) returns (SearchUserList)
StartReindex(StartReindexRequest) returns (ReindexJob)
GetReindexStatus(GetReindexStatusRequest) returns (ReindexJob)
```

P0 search filters:

```text
keyword
page
page_size
category_id
tag_id
order_by
```

### NotificationService

Owner: `notification-service`

P0 methods:

```text
ListMessages(ListMessagesRequest) returns (MessageList)
ListRecentMessages(ListRecentMessagesRequest) returns (MessageList)
MarkMessageRead(MarkMessageReadRequest) returns (Empty)
MarkAllMessagesRead(MarkAllMessagesReadRequest) returns (Empty)
CreateSystemMessage(CreateSystemMessageRequest) returns (Message)
SendEmail(SendEmailRequest) returns (EmailDelivery)
```

Message fields:

```text
Message:
  id
  user_id
  type
  title
  content
  source_type
  source_id
  status       // unread | read
  created_at
  read_at
```

### ConfigService

Owner: `config-service`

P0 methods:

```text
GetPublicConfig(GetPublicConfigRequest) returns (PublicConfig)
GetAdminConfig(GetAdminConfigRequest) returns (AdminConfig)
SaveAdminConfig(SaveAdminConfigRequest) returns (AdminConfig)
```

P0 public config:

```text
site_title
site_description
site_logo
site_navs
site_notification
recommend_tags
modules
login_config
topic_list_style
```

### AdminService

Owner: `admin-service`

P0 methods:

Admin service should mostly orchestrate calls to owning services, not own all data.

```text
GetDashboardOverview(GetDashboardOverviewRequest) returns (DashboardOverview)

ListAdminUsers(AdminListUsersRequest) returns (AdminUserList)
MuteUser(AdminMuteUserRequest) returns (AdminUser)

ListAdminTopics(AdminListTopicsRequest) returns (AdminTopicList)
AuditTopic(AdminAuditTopicRequest) returns (AdminTopic)
DeleteTopic(AdminDeleteTopicRequest) returns (Empty)

ListAdminArticles(AdminListArticlesRequest) returns (AdminArticleList)
AuditArticle(AdminAuditArticleRequest) returns (AdminArticle)
DeleteArticle(AdminDeleteArticleRequest) returns (Empty)

ListAdminReports(AdminListReportsRequest) returns (AdminReportList)
AuditReport(AdminAuditReportRequest) returns (AdminReport)
```

P1:

- Roles.
- Permissions.
- Forbidden words CRUD.
- Settings.
- Operation logs.

## Public HTTP API Shape

All endpoints are served by `api-gateway`.

### Auth

| Method | Path | gRPC |
| --- | --- | --- |
| `POST` | `/api/auth/signup` | `AuthService.SignUp` |
| `POST` | `/api/auth/signin` | `AuthService.SignIn` |
| `POST` | `/api/auth/signout` | `AuthService.SignOut` |
| `POST` | `/api/auth/password/reset-email` | `AuthService.SendResetPasswordEmail` |
| `POST` | `/api/auth/password/reset` | `AuthService.ResetPassword` |
| `POST` | `/api/auth/password/update` | `AuthService.UpdatePassword` |

### Users

| Method | Path | gRPC |
| --- | --- | --- |
| `GET` | `/api/users/current` | `UserService.GetCurrentUser` |
| `GET` | `/api/users/:id` | `UserService.GetUser` |
| `PATCH` | `/api/users/current/profile` | `UserService.UpdateProfile` |
| `POST` | `/api/users/current/avatar` | `UserService.SetAvatar` |
| `POST` | `/api/users/current/background` | `UserService.SetBackgroundImage` |
| `POST` | `/api/users/:id/follow` | `UserService.FollowUser` |
| `DELETE` | `/api/users/:id/follow` | `UserService.UnfollowUser` |
| `GET` | `/api/users/:id/fans` | `UserService.ListFans` |
| `GET` | `/api/users/:id/followed` | `UserService.ListFollowed` |

### Topics

| Method | Path | gRPC |
| --- | --- | --- |
| `GET` | `/api/topics` | `ContentService.ListTopics` |
| `POST` | `/api/topics` | `ContentService.CreateTopic` |
| `GET` | `/api/topics/:id` | `ContentService.GetTopic` |
| `PATCH` | `/api/topics/:id` | `ContentService.UpdateTopic` |
| `DELETE` | `/api/topics/:id` | `ContentService.DeleteTopic` |
| `GET` | `/api/users/:id/topics` | `ContentService.ListUserTopics` |

### Articles

| Method | Path | gRPC |
| --- | --- | --- |
| `GET` | `/api/articles` | `ContentService.ListArticles` |
| `POST` | `/api/articles` | `ContentService.CreateArticle` |
| `GET` | `/api/articles/:id` | `ContentService.GetArticle` |
| `PATCH` | `/api/articles/:id` | `ContentService.UpdateArticle` |
| `DELETE` | `/api/articles/:id` | `ContentService.DeleteArticle` |
| `GET` | `/api/users/:id/articles` | `ContentService.ListUserArticles` |

### Categories And Tags

| Method | Path | gRPC |
| --- | --- | --- |
| `GET` | `/api/categories` | `ContentService.ListCategories` |
| `GET` | `/api/categories/:id` | `ContentService.GetCategory` |
| `GET` | `/api/tags` | `ContentService.ListTags` |
| `GET` | `/api/tags/:id` | `ContentService.GetTag` |
| `POST` | `/api/tags/autocomplete` | `ContentService.AutocompleteTags` |

### Comments

| Method | Path | gRPC |
| --- | --- | --- |
| `GET` | `/api/comments` | `CommentService.ListComments` |
| `GET` | `/api/comments/:id/replies` | `CommentService.ListReplies` |
| `POST` | `/api/comments` | `CommentService.CreateComment` |
| `DELETE` | `/api/comments/:id` | `CommentService.DeleteComment` |

Query for list:

```text
entityType=topic|article
entityId=...
page=1
pageSize=20
```

### Reactions

| Method | Path | gRPC |
| --- | --- | --- |
| `POST` | `/api/reactions/like` | `ReactionService.Like` |
| `DELETE` | `/api/reactions/like` | `ReactionService.Unlike` |
| `POST` | `/api/reactions/liked-ids` | `ReactionService.ListLikedIds` |
| `POST` | `/api/reactions/favorite` | `ReactionService.Favorite` |
| `DELETE` | `/api/reactions/favorite` | `ReactionService.Unfavorite` |
| `GET` | `/api/users/current/favorites` | `ReactionService.ListFavorites` |
| `POST` | `/api/reports` | `ReactionService.SubmitReport` |

### Search

| Method | Path | gRPC |
| --- | --- | --- |
| `GET` | `/api/search` | `SearchService.SearchAll` |
| `GET` | `/api/search/topics` | `SearchService.SearchTopics` |
| `GET` | `/api/search/articles` | `SearchService.SearchArticles` |

### Messages

| Method | Path | gRPC |
| --- | --- | --- |
| `GET` | `/api/messages/recent` | `NotificationService.ListRecentMessages` |
| `GET` | `/api/messages` | `NotificationService.ListMessages` |
| `POST` | `/api/messages/:id/read` | `NotificationService.MarkMessageRead` |
| `POST` | `/api/messages/read-all` | `NotificationService.MarkAllMessagesRead` |

### Config

| Method | Path | gRPC |
| --- | --- | --- |
| `GET` | `/api/configs/public` | `ConfigService.GetPublicConfig` |

### Admin P0

| Method | Path | gRPC / Orchestration |
| --- | --- | --- |
| `GET` | `/api/admin/overview` | `AdminService.GetDashboardOverview` |
| `GET` | `/api/admin/users` | `UserService.ListUsersForAdmin` |
| `POST` | `/api/admin/users/:id/mute` | `UserService.MuteUser` |
| `GET` | `/api/admin/topics` | `ContentService.ListTopics` with admin filters |
| `POST` | `/api/admin/topics/:id/audit` | `ContentService.AuditTopic` |
| `DELETE` | `/api/admin/topics/:id` | `ContentService.DeleteTopic` |
| `GET` | `/api/admin/articles` | `ContentService.ListArticles` with admin filters |
| `POST` | `/api/admin/articles/:id/audit` | `ContentService.AuditArticle` |
| `DELETE` | `/api/admin/articles/:id` | `ContentService.DeleteArticle` |
| `GET` | `/api/admin/reports` | `ReactionService.ListReportsForAdmin` |
| `POST` | `/api/admin/reports/:id/audit` | `ReactionService.AuditReport` |

## Page Aggregation Contracts

The following endpoints are optional but recommended because they reduce frontend request waterfalls.

### Home Page

```text
GET /api/pages/home
```

Aggregates:

- public config
- current user summary if logged in
- recommended topics
- latest articles
- hot categories
- recent messages count

### Topic Detail Page

```text
GET /api/pages/topics/:id
```

Aggregates:

- topic detail
- author current profile
- category
- tags
- current user's liked/favorited state
- first page comments
- related topics

### Article Detail Page

```text
GET /api/pages/articles/:id
```

Aggregates:

- article detail
- author current profile
- tags
- current user's liked/favorited state
- first page comments
- related articles

### User Profile Page

```text
GET /api/pages/users/:id
```

Aggregates:

- user detail
- follow state
- recent topics
- recent articles
- profile counters

## Permission Rules For P0

Public:

- Read published topics/articles/categories/tags.
- Read public user profiles.
- Search published content.

Authenticated user:

- Create/edit/delete own topics and articles.
- Create/delete own comments.
- Like/favorite/report.
- Follow/unfollow users.
- Read own messages/favorites.
- Edit own profile.

Admin:

- List users/content/reports.
- Mute users.
- Audit/delete topics and articles.
- Audit reports.

P0 can start with simple roles:

```text
user
admin
```

Full permission-code RBAC is P1.

## Validation Rules

P0 minimum:

- Username unique.
- Email unique if provided.
- Password minimum length.
- Topic title required for `topic`, optional for `tweet`.
- Topic/article content non-empty.
- Category must exist and be enabled.
- Tags are normalized and length-limited.
- Comment entity must exist and be visible.
- Muted users cannot create topics/articles/comments.
- Deleted or rejected content is invisible to normal users.

## Event Side Effects

Every mutating P0 method should either publish or intentionally skip an event.

| Mutation | Event |
| --- | --- |
| Sign up | `user.created` |
| Update profile | `user.updated` |
| Follow user | `user.followed` |
| Create topic | `content.topic.created` |
| Update topic | `content.topic.updated` |
| Delete topic | `content.topic.deleted` |
| Create article | `content.article.created` |
| Update article | `content.article.updated` |
| Delete article | `content.article.deleted` |
| Create comment | `comment.created` |
| Delete comment | `comment.deleted` |
| Like | `reaction.liked` |
| Unlike | `reaction.unliked` |
| Favorite | `reaction.favorited` |
| Unfavorite | `reaction.unfavorited` |
| Report | `reaction.reported` |
| Admin audit topic | `content.topic.audited` |
| Admin audit article | `content.article.audited` |

## Proto Generation Later

When implementation starts, generate code from `.proto` definitions into each Go service.

Recommended conventions:

- `buf` for linting and breaking-change checks.
- `connect-go` or standard `grpc-go`; choose one and stay consistent.
- Package versioning starts at `v1`.
- No service should import another service's database model.
- Shared proto messages should be minimal; prefer service-local messages over a huge shared model package.

## Locked P0 Contract Decisions

1. P0 exposes full article CRUD.
2. P0 topic types are `topic` and `tweet`; `qa` is deferred until bounty/accepted-answer support.
3. P0 comments use root comments plus replies, not unlimited nesting.
4. P0 admin delete is a status transition/soft delete, not hard delete.
5. P0 auth uses opaque Redis-backed tokens with PostgreSQL session records for revocation.
