# Feature Inventory

This document tracks the target feature set for functional parity with `bbs-go`, enriched by repository route/model inspection.

Legend:

- `P0`: minimum viable community launch.
- `P1`: important for parity and retention.
- `P2`: advanced operation, scale, or polish.

## 1. User And Identity

| Feature | Priority | Owner Service | Notes |
| --- | --- | --- | --- |
| Email/username password signup | P0 | `auth-service`, `user-service` | Include captcha and configurable email verification. |
| Password sign-in/sign-out | P0 | `auth-service` | Token/session lifecycle. |
| Password reset by email | P0 | `auth-service`, `notification-service` | Email code/token flow. |
| SMS login | P1 | `auth-service`, `notification-service` | Keep behind config switch. |
| WeChat login and bind/unbind | P1 | `auth-service` | OAuth config from Nacos/config service. |
| GitHub login and bind/unbind | P1 | `auth-service` | Feature parity with bbs-go. |
| Google login, one-tap, bind/unbind | P1 | `auth-service` | Feature parity with bbs-go. |
| Current user endpoint | P0 | `api-gateway`, `user-service` | Used by global layout. |
| User profile detail | P0 | `user-service` | Nickname, avatar, background, homepage, description, gender. |
| Edit user profile | P0 | `user-service` | User self-service. |
| Set username | P1 | `user-service` | One-time or restricted update policy. |
| Set/verify email | P1 | `auth-service`, `user-service` | Configurable requirement for posting/commenting. |
| Update avatar/background | P0 | `file-service`, `user-service` | File upload and profile mutation. |
| Update password | P0 | `auth-service` | Requires old password. |
| Admin create/update/reset user | P1 | `admin-service`, `user-service` | Back-office governance. |
| User forbidden/mute | P0 | `admin-service`, `user-service` | Includes temporary and permanent mute. |
| User roles | P1 | `admin-service`, `user-service` | Role assignment and permission projection. |
| User public pages | P0 | `user-service`, `content-service` | Profile, posts, articles, fans, followed, badges. |

## 2. Social Graph

| Feature | Priority | Owner Service | Notes |
| --- | --- | --- | --- |
| Follow user | P0 | `user-service` | Write follow relation and counters. |
| Unfollow user | P0 | `user-service` | Idempotent. |
| Check followed status | P0 | `user-service` | Used by profile/cards. |
| Fans list | P0 | `user-service` | Paginated. |
| Followed list | P0 | `user-service` | Paginated. |
| Recent fans | P1 | `user-service` | Sidebar/profile widget. |
| Recent follows | P1 | `user-service` | Sidebar/profile widget. |
| User feed | P1 | `content-service`, `user-service` | Activity stream from followed authors. |

## 3. Topics, Tweets, And Q&A

`bbs-go` models topic-like content with `TopicType` and module switches for tweet/topic/QA.

| Feature | Priority | Owner Service | Notes |
| --- | --- | --- | --- |
| Topic/tweet/QA create | P0 | `content-service` | Title optional for tweet, required for topic/QA. |
| Topic edit form/data | P0 | `content-service` | User and admin variants. |
| Topic update | P0 | `content-service` | Permission and author checks. |
| Topic delete/remove | P0 | `content-service` | Soft delete preferred. |
| Topic detail | P0 | `content-service`, `comment-service` | Include author, category, tags, attachments, vote, comments. |
| Topic list | P0 | `content-service` | Latest, category, recommend, follow filters. |
| Recent topics | P0 | `content-service` | Home/feed/sidebar. |
| User topics | P0 | `content-service` | Profile page. |
| Category topics | P0 | `content-service` | Node/circle page. |
| Tag topics | P0 | `content-service` | Topic discovery. |
| Recommend/unrecommend topic | P1 | `admin-service`, `content-service` | Admin or moderator action. |
| Sticky/unsticky topic | P1 | `admin-service`, `content-service` | List ranking. |
| Topic audit | P1 | `admin-service`, `content-service` | Pending/approved/rejected states. |
| Topic undelete | P2 | `admin-service`, `content-service` | Moderation recovery. |
| Hide content / reply-visible content | P1 | `content-service` | Config switch. |
| Topic images | P0 | `file-service`, `content-service` | Image list metadata. |
| Topic attachments | P1 | `file-service`, `content-service`, `credit-service` | Download score and purchase/download log. |
| QA bounty score | P1 | `content-service`, `credit-service` | Configurable min/max/required. |
| Accept answer | P1 | `content-service`, `comment-service`, `credit-service` | Award bounty and mark solved. |
| Unaccept answer | P2 | `content-service`, `credit-service` | Requires compensation rules. |
| Mark solved/unsolved | P1 | `admin-service`, `content-service` | Admin override. |

## 4. Articles And Knowledge Base

| Feature | Priority | Owner Service | Notes |
| --- | --- | --- | --- |
| Article create | P0 | `content-service` | Markdown/HTML content type. |
| Article edit | P0 | `content-service` | Author/admin permissions. |
| Article delete | P0 | `content-service` | Soft delete. |
| Article detail | P0 | `content-service`, `comment-service` | Cover, source URL, tags, comments. |
| Article list | P0 | `content-service` | Latest and filters. |
| User articles | P0 | `content-service` | Profile page. |
| Tag articles | P0 | `content-service` | Tag discovery. |
| Article favorite | P0 | `reaction-service` | Same favorite model as topic. |
| Article tags admin maintenance | P1 | `admin-service`, `content-service` | Admin edit tag relations. |
| Article audit | P1 | `admin-service`, `content-service` | Optional pending mode. |
| Article redirect/source link | P2 | `content-service` | External source URL behavior. |

## 5. Comments And Replies

| Feature | Priority | Owner Service | Notes |
| --- | --- | --- | --- |
| Comment list | P0 | `comment-service` | Entity type + entity id. |
| Reply list | P0 | `comment-service` | Quote/reply target. |
| Create comment | P0 | `comment-service` | Supports images and content type. |
| Delete comment | P0 | `comment-service` | User/admin behavior differs. |
| Comment like count | P0 | `reaction-service`, `comment-service` | Count cache and event sync. |
| Comment moderation status | P1 | `admin-service`, `comment-service` | Pending/approved/rejected/deleted. |
| Comment IP/user-agent metadata | P1 | `comment-service`, `audit-service` | Required for moderation and audit. |
| Comments in MongoDB | P0 | `comment-service` | Store body/reply structure in MongoDB. |
| Comment counters in Redis/PG | P0 | `comment-service`, `content-service` | Denormalized counts for lists. |

## 6. Reactions: Like, Favorite, Report

| Feature | Priority | Owner Service | Notes |
| --- | --- | --- | --- |
| Like entity | P0 | `reaction-service` | Topic, article, comment. |
| Unlike entity | P0 | `reaction-service` | Idempotent. |
| Query liked ids | P0 | `reaction-service` | Batch for list rendering. |
| Query liked entities | P1 | `reaction-service` | User profile/activity. |
| Favorite add | P0 | `reaction-service` | Topic/article. |
| Favorite remove | P0 | `reaction-service` | Idempotent. |
| User favorites | P0 | `reaction-service`, `content-service` | Profile page. |
| Report submit | P0 | `reaction-service`, `admin-service` | Data type + data id + reason. |
| Report list/audit | P0 | `admin-service` | Governance workflow. |

## 7. Tags, Categories, Links

| Feature | Priority | Owner Service | Notes |
| --- | --- | --- | --- |
| Tag list | P0 | `content-service` | Public. |
| Tag detail | P0 | `content-service` | Public. |
| Tag autocomplete | P0 | `content-service` | Editor support. |
| Admin tag CRUD | P1 | `admin-service`, `content-service` | Governance. |
| Category navs | P0 | `content-service` | Top/side navigation. |
| Category tree/list | P0 | `content-service` | Supports parent/child. |
| Category detail | P0 | `content-service` | Topic listing. |
| Category CRUD/sort | P1 | `admin-service`, `content-service` | Node management. |
| Normal/QA category type | P1 | `content-service` | For Q&A separation. |
| Friendly links | P1 | `content-service` or `config-service` | Public list/top links and admin CRUD. |

## 8. Voting

| Feature | Priority | Owner Service | Notes |
| --- | --- | --- | --- |
| Create vote with topic | P1 | `content-service` | Single/multiple choice. |
| Vote options | P1 | `content-service` | Ordered options. |
| Cast vote | P1 | `reaction-service` or `content-service` | Prevent duplicate votes. |
| Vote detail/results | P1 | `content-service` | For topic detail. |
| Admin vote CRUD | P2 | `admin-service`, `content-service` | Parity with bbs-go admin routes. |
| Vote records admin list | P2 | `admin-service` | Audit/troubleshooting. |

## 9. Search And SEO

| Feature | Priority | Owner Service | Notes |
| --- | --- | --- | --- |
| Topic search | P0 | `search-service` | Elasticsearch. |
| Article search | P0 | `search-service` | Elasticsearch. |
| User search | P1 | `search-service` | Elasticsearch or PG fallback. |
| Rebuild search index | P1 | `admin-service`, `search-service` | Async job. |
| Reindex status | P1 | `search-service` | Admin page. |
| Sitemap generation | P2 | `search-service` or `content-service` | SEO job and status. |
| Redirect `/sitemap.xml` | P2 | `api-gateway` | Return generated sitemap URL/content. |

## 10. Growth System

| Feature | Priority | Owner Service | Notes |
| --- | --- | --- | --- |
| Daily check-in | P1 | `credit-service` | Consecutive days. |
| Check-in status | P1 | `credit-service` | User widget. |
| Check-in rank | P1 | `credit-service` | Leaderboard. |
| Task list | P1 | `credit-service` | Newbie/daily/achievement groups. |
| Task groups | P1 | `credit-service` | UI tabs. |
| Task event engine | P1 | `credit-service` | Consumes Kafka domain events. |
| Task config CRUD | P1 | `admin-service`, `credit-service` | Score/exp/badge rewards. |
| User task logs | P1 | `credit-service` | Admin and user history. |
| Points/score logs | P1 | `credit-service` | User and admin list. |
| Score leaderboard | P1 | `credit-service` | Redis sorted set + PG fallback. |
| Experience logs | P1 | `credit-service` | Level calculation. |
| Level config | P1 | `credit-service` | Admin save all levels. |
| Badges config | P1 | `credit-service` | Badge CRUD and icons. |
| User badges | P1 | `credit-service` | Profile display and admin list. |

## 11. Messages And Notifications

| Feature | Priority | Owner Service | Notes |
| --- | --- | --- | --- |
| Recent messages | P0 | `notification-service` | Header dropdown. |
| Message list | P0 | `notification-service` | User messages page. |
| Interaction reminders | P0 | `notification-service` | Like/comment/follow/answer. |
| System messages | P1 | `notification-service` | Admin/system announcements. |
| Email notifications | P1 | `notification-service` | Configurable per type. |
| Email send logs | P1 | `notification-service`, `admin-service` | Admin audit. |
| Notification type config | P1 | `config-service`, `notification-service` | Site/email switches. |

## 12. Files, Upload, Attachments

| Feature | Priority | Owner Service | Notes |
| --- | --- | --- | --- |
| Generic image upload | P0 | `file-service` | Avatar, covers, post images. |
| Local storage | P0 | `file-service` | Development/default. |
| S3-compatible storage | P1 | `file-service` | Include AWS S3, Tencent COS, Aliyun OSS equivalents. |
| Attachment upload | P1 | `file-service`, `content-service` | Topic attachments. |
| Attachment download | P1 | `file-service`, `credit-service` | Deduct score if configured. |
| Attachment score update | P1 | `content-service`, `file-service` | Author/admin. |
| Attachment download log | P1 | `file-service`, `credit-service` | Avoid duplicate charge. |
| Upload config | P1 | `config-service` | Allowed types, size, count, backend. |

## 13. Site Config And Install

| Feature | Priority | Owner Service | Notes |
| --- | --- | --- | --- |
| Install status | P2 | `install-service` or `admin-service` | Optional if deployment is scripted. |
| DB connection test | P2 | `install-service` | Optional. |
| Initial install/admin creation | P2 | `install-service` | Optional for self-hosting. |
| Public configs | P0 | `config-service` | Site title, logo, navs, modules, login switches. |
| Admin configs | P1 | `config-service`, `admin-service` | Full site config. |
| About page config | P1 | `config-service` | Rich content. |
| Footer links | P1 | `config-service` | Public UI. |
| Script injections | P2 | `config-service` | Head scripts, analytics. |
| Module switches | P1 | `config-service` | Tweet/topic/QA/article. |
| Captcha settings | P1 | `config-service`, `auth-service` | Login/post captcha. |
| Email whitelist | P1 | `config-service`, `auth-service` | Registration restrictions. |

## 14. Admin And Governance

| Feature | Priority | Owner Service | Notes |
| --- | --- | --- | --- |
| Dashboard overview | P1 | `admin-service` | Counts and trends. |
| Role CRUD | P1 | `admin-service` | Role code, status, sorting. |
| Permission registry | P1 | `admin-service` | Dashboard permissions. |
| Role permission update | P1 | `admin-service` | RBAC. |
| Dict type CRUD | P2 | `admin-service` | Generic dictionary. |
| Dict CRUD/sort | P2 | `admin-service` | Generic dictionary items. |
| User governance | P0 | `admin-service`, `user-service` | List/create/update/forbid/password reset. |
| Topic governance | P0 | `admin-service`, `content-service` | List/recommend/sticky/audit/delete/solve. |
| Article governance | P0 | `admin-service`, `content-service` | List/update/audit/delete/tags. |
| Comment governance | P0 | `admin-service`, `comment-service` | Delete/audit if implemented. |
| Report governance | P0 | `admin-service` | List/create/update/audit. |
| Forbidden words | P1 | `admin-service`, `content-service` | Word/regex. |
| Search reindex | P1 | `admin-service`, `search-service` | Job control. |
| Sitemap generation | P2 | `admin-service`, `search-service` | SEO. |
| Email logs | P1 | `admin-service`, `notification-service` | List/detail. |
| Operation logs | P1 | `admin-service`, `audit-service` | List/detail. |

## 15. Internationalization

| Feature | Priority | Owner Service | Notes |
| --- | --- | --- | --- |
| zh-CN and en-US text support | P2 | Frontend, `config-service` | bbs-go includes both. |
| Localized site content | P2 | `config-service` | About/footer/nav labels. |

