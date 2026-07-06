# Frontend Page Map

The current frontend visual style is the baseline. Future pages should extend the same system instead of replacing it.

## Current UI Baseline

Observed current design in `frontend`:

- Top navigation: 首页、广场、圈子、求助、资源、商城、会员、更多.
- Desktop: left nav + central content + right sidebar.
- Mobile: single-column layout, no right sidebar.
- Visual language: light gray background, white panels, blue accent, compact cards, Lucide icons.

## Target Public Navigation

| Current Nav | Target Meaning | Backing Features |
| --- | --- | --- |
| 首页 | Community dashboard/home | Recommendations, latest updates, check-in, tasks, hot categories, user recommendations. |
| 广场 | Topic/tweet feed | Topic/tweet list, composer, tags, likes, comments, favorites, voting. |
| 圈子 | Categories/nodes | Category tree, category detail, category topic list. |
| 求助 | Q&A | QA list, bounty, solved/unsolved, accepted answer. |
| 资源 | Articles/knowledge base | Article list, article tags, article detail, favorites. |
| 商城 | Optional monetization/resources | Can later host paid resources, service packages, or attachment/download marketplace. Not from bbs-go core; keep optional. |
| 会员 | Growth profile | Points, exp, levels, badges, tasks, check-in. |
| 更多 | Links, about, search, tasks, settings entry | Friendly links, about, global search, announcements. |

## Required Public Pages

| Page | Route Concept | Priority | Notes |
| --- | --- | --- | --- |
| Home | `/` | P0 | Feed summary and community widgets. |
| Topic list | `/topics` | P0 | Supports filters: latest, recommend, category, tag, follow. |
| Topic detail | `/topic/:id` | P0 | Content, images, attachments, vote, hidden content, comments. |
| Topic create | `/topic/create` | P0 | Supports tweet/topic/QA mode depending module switches. |
| Topic edit | `/topic/edit/:id` | P0 | Author/admin permissions. |
| Category page | `/topics/category/:id` | P0 | Category detail and topic list. |
| Topic tag page | `/topics/tag/:id` | P0 | Tag detail and topic list. |
| Article list | `/articles` | P0 | Knowledge base index. |
| Article detail | `/article/:id` | P0 | Article content, tags, comments. |
| Article create | `/article/create` | P0 | Rich/markdown editor. |
| Article edit | `/article/edit/:id` | P0 | Author/admin permissions. |
| Article tag page | `/articles/tag/:id` | P0 | Tag article list. |
| Search | `/search` | P0 | Tabs: topics, articles, users. |
| Sign in | `/user/signin` | P0 | Password login first, then OAuth/SMS. |
| Sign up | `/user/signup` | P0 | Captcha and email constraints. |
| Forgot password | `/user/password/forgot` | P0 | Email reset. |
| Reset password | `/user/password/reset` | P0 | Token flow. |
| Email verify | `/user/email/verify` | P1 | Verification flow. |
| User profile | `/user/:userId` | P0 | Overview, posts, stats, badges. |
| User articles | `/user/:userId/articles` | P0 | Profile content tab. |
| User fans | `/user/:userId/fans` | P0 | Fans list. |
| User followed | `/user/:userId/followed` | P0 | Followed users. |
| User badges | `/user/:userId/badges` | P1 | Badge wall. |
| My profile settings | `/user/profile` | P0 | Edit nickname, avatar, background, description. |
| Account settings | `/user/profile/account` | P1 | Email, password, OAuth binding. |
| My favorites | `/user/favorites` | P0 | Favorite topics/articles. |
| My messages | `/user/messages` | P0 | Notifications and site messages. |
| My scores | `/user/scores` | P1 | Points logs and rank. |
| Tasks | `/tasks` | P1 | Newbie/daily/achievement tasks. |
| Links | `/links` | P1 | Friendly links. |
| About | `/about` | P1 | Configurable about page. |
| Install | `/install` | P2 | Optional if self-hosting setup flow is required. |

## Required Admin Pages

| Page | Priority | Backing Service |
| --- | --- | --- |
| Dashboard overview | P1 | `admin-service` |
| Users | P0 | `admin-service`, `user-service` |
| Topics | P0 | `admin-service`, `content-service` |
| Articles | P0 | `admin-service`, `content-service` |
| Categories | P1 | `admin-service`, `content-service` |
| Badges | P1 | `admin-service`, `credit-service` |
| Levels | P1 | `admin-service`, `credit-service` |
| Tasks | P1 | `admin-service`, `credit-service` |
| User badges | P1 | `admin-service`, `credit-service` |
| User exp logs | P1 | `admin-service`, `credit-service` |
| User task logs | P1 | `admin-service`, `credit-service` |
| User reports | P0 | `admin-service` |
| Forbidden words | P1 | `admin-service` |
| Links | P1 | `admin-service`, `content-service` |
| Roles | P1 | `admin-service` |
| Settings | P1 | `config-service`, `admin-service` |
| Email logs | P1 | `notification-service`, `admin-service` |
| Operation logs | P1 | `audit-service`, `admin-service` |

## Frontend Component Families

Recommended component families based on current UI:

- `layout`: topbar, left rail, right rail, responsive shell.
- `feed`: composer, topic card, article card, user card, poll card.
- `content`: markdown/rich editor, attachment uploader, tag input, category picker.
- `comments`: comment list, reply editor, accepted answer marker, moderation state.
- `user`: profile header, follow button, badge wall, score panel.
- `growth`: task list, check-in card, level progress, leaderboard.
- `admin`: data table, filter toolbar, form dialog, audit action bar.
- `common`: status badge, empty state, pagination, modal, toast.

## API Gateway Page Aggregation

To keep frontend simple, `api-gateway` should aggregate these pages:

| Page | Aggregated Data |
| --- | --- |
| Home | current user, site config, recommended topics, recent articles, tasks, check-in, hot categories, right rail. |
| Topic detail | topic, author, category, tags, vote, attachments, comments, liked/favorited status, related topics. |
| Article detail | article, author, tags, comments, liked/favorited status, related articles. |
| User profile | user detail, follow status, stats, recent topics/articles, badges. |
| Admin dashboard | counts, pending audits, latest reports, trend metrics. |

## UI Implementation Notes

- Preserve the current top navigation labels unless product direction changes.
- Keep cards at 8px radius or less to match existing style.
- Use Lucide icons consistently.
- Avoid adding marketing hero pages; first screen should be the usable community page.
- Keep mobile as a single-column feed with horizontal top nav where needed.
- Use dense but readable admin tables; do not overuse cards inside cards.

