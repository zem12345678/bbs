# BBS Community Platform

BBS Community Platform 是一个面向商业化社区场景的论坛系统，目标是在传统论坛能力的基础上补齐内容运营、用户成长、互动治理、搜索、通知、商城和后台权限体系。项目参考 `mlogclub/bbs-go` 的功能方向，前端延续当前社区 UI 风格，后端按 DDD 与微服务架构拆分服务边界。

## 项目组成

- `frontend`：C 端社区前台，包含首页、帖子/话题、搜索、用户中心、商城、订单、评价和聊天室等用户侧页面。
- `vue-pure-admin`：运营管理端，基于 vue-pure-admin，承载系统管理、RBAC、社区管理、商城管理、内容治理等后台页面。
- `backend/services/api-gateway`：统一 HTTP 入口，负责 C 端与管理端 API 聚合、鉴权、上游 gRPC 调用和响应格式统一。
- `backend/services/admin-service`：管理端服务，负责系统用户、角色、菜单、部门、Casbin RBAC、登录日志、操作日志和运营配置。
- `backend/services/user-service`：用户账号、注册登录、OAuth 登录、用户资料等能力。
- `backend/services/content-service`：文章、话题、分类、标签、发布状态机和内容列表。
- `backend/services/comment-service`：评论与回复，评论存储使用 MongoDB。
- `backend/services/reaction-service`：点赞、收藏等互动状态与计数。
- `backend/services/feed-service`：信息流聚合与推荐列表。
- `backend/services/search-service`：Elasticsearch 搜索索引与高亮搜索结果。
- `backend/services/credit-service`：积分账户、积分流水、任务奖励和消费扣减。
- `backend/services/mall-service`：商城商品、分类、优惠券、订单、支付、退款、库存和商品评价。
- `backend/services/notification-service`：站内通知与消息事件消费。
- `backend/services/chat-service`：房间、成员关系、消息历史与实时聊天事件。

## 后端架构

后端服务统一按 DDD 骨架组织：

- `api/proto` 放置 proto 文件，生成代码放在 `api/proto/*pb`。
- `internal/domain` 承载领域模型、领域错误和仓储接口。
- `internal/application` 编排用例、状态机和跨资源事务边界。
- `internal/infrastructure` 实现数据库、消息、搜索、缓存等基础设施访问。
- `internal/interfaces` 提供 gRPC/HTTP 等入口适配。
- `internal/ioc` 使用 Wire 风格 ProviderSet 组装配置、DB、Redis、Kafka、ES、gRPC client/server、服务发现等组件。

服务通信采用 Go gRPC，服务注册发现使用 etcd，配置中心使用 Nacos。核心存储和中间件包括 PostgreSQL、Redis、Kafka、MongoDB、Elasticsearch、Nacos、etcd。

## 本地端口

| 模块 | 地址 |
| --- | --- |
| C 端前台 | `http://127.0.0.1:8850` |
| 管理端前台 | `http://127.0.0.1:8849` |
| API Gateway | `http://127.0.0.1:18080/api/v1` |
| Nacos | `http://127.0.0.1:8848/nacos/` |
| etcd | `127.0.0.1:2379` |
| Elasticsearch | `D:\elasticsearch-8.19.0-windows-x86_64\elasticsearch-8.19.0` |

## 本地启动

共享依赖由本机已有服务或容器维护：PostgreSQL、Redis、etcd、Nacos、Kafka、MongoDB、Elasticsearch 和 MinIO 不会由 BBS Compose 创建、停止或重置。`backend/deployments/local` 的 Compose 仅管理 BBS 专用的 Mailpit；服务进程使用可视化脚本启动，便于观察每个服务控制台日志。

```powershell
cd D:\projects\bbs

# 复用已有共享依赖；此 Compose 命令只启动 BBS 专用 Mailpit。
cd backend\deployments\local
docker compose up -d
# 预检完整商业化链路的共享依赖，并将 BBS 配置发布到既有 Nacos 的 bbs-local 命名空间。
.\scripts\bootstrap.ps1 -Full

# 本机 PostgreSQL 要求密码时，在当前会话提供密码后重新执行 bootstrap。
$env:PGPASSWORD = "<本机 PostgreSQL 密码>"
.\scripts\bootstrap.ps1 -Full

# 在启动后端前执行所有应用迁移（包含评论索引、积分和通知表）。
.\scripts\migrate.ps1

# 共享 Kafka 由平台维护；先按 backend\deployments\local\kafka\topics.txt
# 预建全部 BBS topic。BBS 不会自动创建或修改它们。

# 启动完整商业化联调后端服务（用户、内容、评论、互动、搜索、积分、通知、信息流、后台、商城、聊天、网关）
cd D:\projects\bbs
.\backend\scripts\start-local-visible.ps1 -Profile commercial -Restart -Build

# 检查完整联调服务监听与 etcd 服务发现状态
.\backend\scripts\check-local-backend.ps1 -Profile commercial -RequireDiscovery -Strict

# 运行完整商业化 smoke 验收（复用已启动服务，覆盖登录、内容、互动、搜索、通知、后台、积分商城、订单和售后）
.\backend\scripts\smoke-local.ps1 -SkipBuild -KeepRunning

# 只调试后台登录/菜单时，可使用轻量模式
.\backend\scripts\start-local-visible.ps1 -Profile minimal -Restart -Build
```

若本机需要并行运行第二套 BBS 进程，可将端口覆写写入独立文件，并在启动时加载。以下使用所有 gRPC 服务的规范 `BBS_<SERVICE>_GRPC_SERVER_PORT` 变量；变量只会写入当前 PowerShell 会话以供后续检查和 smoke 复用，不会写入持久化的用户或系统环境变量：

```powershell
# 例如 D:\projects\bbs\backend\deployments\local\.env.stack-b
BBS_USER_GRPC_SERVER_PORT=19102
BBS_CONTENT_GRPC_SERVER_PORT=19103
BBS_COMMENT_GRPC_SERVER_PORT=19104
BBS_REACTION_GRPC_SERVER_PORT=19105
BBS_SEARCH_GRPC_SERVER_PORT=19106
BBS_CREDIT_GRPC_SERVER_PORT=19107
BBS_NOTIFICATION_GRPC_SERVER_PORT=19108
BBS_FILE_GRPC_SERVER_PORT=19111
BBS_FEED_GRPC_SERVER_PORT=19113
BBS_ADMIN_GRPC_SERVER_PORT=19114
BBS_MALL_GRPC_SERVER_PORT=19115
BBS_CHAT_GRPC_SERVER_PORT=19116
BBS_GATEWAY_SERVICE_HTTP_PORT=28080
# Keep OAuth callbacks, sitemap links, and other absolute URLs on this stack.
BBS_GATEWAY_HTTP_PUBLIC_BASE_URL=http://127.0.0.1:28080

# 先在没有运行同路径服务时完成构建；Windows 会锁定正在运行的 .exe。
.\backend\scripts\start-local-visible.ps1 -Profile commercial -EnvironmentFile .\backend\deployments\local\.env.stack-b -Restart

# 在同一 PowerShell 会话中，启动、检查和 smoke 会自动识别每项 BBS_<SERVICE>_GRPC_SERVER_PORT
# 及 BBS_GATEWAY_SERVICE_HTTP_PORT。兼容的 *_SERVICE_GRPC_PORT 别名仍可使用；user/chat 也兼容 *_GRPC_PORT。
.\backend\scripts\check-local-backend.ps1 -Profile commercial -RequireDiscovery -Strict
.\backend\scripts\smoke-local.ps1 -SkipBuild -KeepRunning

# 显式参数目前适用于 User、Mall、Search、Chat 和 Gateway；其他服务请使用环境文件。
.\backend\scripts\check-local-backend.ps1 -Profile commercial -UserPort 19102 -MallPort 19115 -SearchPort 19106 -ChatPort 19116 -GatewayPort 28080 -RequireDiscovery -Strict
.\backend\scripts\smoke-local.ps1 -SkipBuild -KeepRunning -UserPort 19102 -MallPort 19115 -SearchPort 19106 -ChatPort 19116 -GatewayPort 28080
```

启动第二套后端后，前端也需要指向同一 Gateway；以下示例不启动、停止或重用任何其他项目进程。`8851` 仅为示例空闲端口，可按实际情况替换：

```powershell
# C 端：使用第二套 Gateway，并避免占用默认 8850。
cd D:\projects\bbs\frontend
$env:VITE_API_BASE = "http://127.0.0.1:28080/api/v1"
npm run dev -- --port 8851

# 管理端：代理到第二套 Gateway；商城推广链接回到上述 C 端。
cd D:\projects\bbs\vue-pure-admin
$env:VITE_API_PROXY_TARGET = "http://127.0.0.1:28080"
$env:VITE_PORT = "8849"
$env:VITE_MALL_FRONTEND_BASE = "http://127.0.0.1:8851"
pnpm dev
```

`start-local-visible.ps1 -Restart` 只会停止路径精确匹配 `backend\services\<服务>\bin\<服务>.exe` 且监听本次选定端口的 BBS 服务进程；它不会按 `powershell`、`cmd`、`go` 或终端名做全局结束，也不会停止另一套使用不同端口的 BBS 进程。

上述覆写隔离的是本机监听端口。若还需隔离数据、Kafka 消费组或 etcd 服务发现，应为第二套栈提供独立的 PostgreSQL、Redis、Kafka 和 etcd/Nacos 配置；共享服务发现时，同名服务会共同注册并参与负载均衡。

聊天双节点/双 Gateway 联调可单独执行。它在预检时不创建进程或数据；`-Run` 只停止本次启动并保存 PID 的四个子进程。需要已有的 `bbs-user-service` etcd 注册及 PostgreSQL、Redis、Kafka、etcd、Nacos：

```powershell
# 默认端口为 chat 9116/19116、Gateway 18080/18081
.\backend\scripts\chat-cluster-e2e.ps1 -Preflight
.\backend\scripts\chat-cluster-e2e.ps1 -Run -Build

# 与其他项目并行时使用独立端口，避免占用或停止外部服务。
.\backend\scripts\chat-cluster-e2e.ps1 -Preflight -PrimaryChatPort 19117 -SecondaryChatPort 19118 -PrimaryGatewayPort 18082 -SecondaryGatewayPort 18083
.\backend\scripts\chat-cluster-e2e.ps1 -Run -Build -PrimaryChatPort 19117 -SecondaryChatPort 19118 -PrimaryGatewayPort 18082 -SecondaryGatewayPort 18083
```

前端启动：

```powershell
cd D:\projects\bbs\frontend
npm run dev

cd D:\projects\bbs\vue-pure-admin
npm run dev
```

C 端商城浏览器联调验收：

```powershell
# 需要先启动 commercial 后端和 api-gateway；若 C 端 dev server 未启动，脚本会自动拉起并在结束时关闭。
# 脚本会创建本地 E2E 用户/商品/优惠券/订单，并覆盖领取优惠券、保存地址、单品兑换、购物车结算、积分支付、订单通知、运营发货、确认收货、提交评价和售后退款。
cd D:\projects\bbs\frontend
npm run e2e:mall

# 如需强制复用手动启动的前端，可关闭自动拉起。
$env:MALL_E2E_NO_AUTO_FRONTEND = "1"
npm run e2e:mall

# 额外覆盖附件上传、改价、付费下载、会员撤销后拒绝新的付费成交及归档的浏览器流程；需要 file-service 与 MinIO 可用。
$env:MALL_E2E_ATTACHMENTS = "1"
npm run e2e:mall

# 若脚本没有自动找到 Chrome/Chromium，可显式指定
$env:CHROME_EXECUTABLE = "C:\path\to\chrome.exe"
npm run e2e:mall
```

付费附件验收：

```powershell
# 需要 file-service、MinIO 与 api-gateway 使用同一存储桶。
cd D:\projects\bbs
.\backend\scripts\attachment-smoke.ps1

# 默认读取 backend\deployments\local\.env 中指向现有 MinIO 的 MINIO_* 配置。
# 其中 MINIO_CONTAINER 仅用于通过已有容器执行对象校验，不会启动新容器。
.\backend\scripts\attachment-smoke.ps1
```

完整商业化端到端验收会默认包含账号安全邮件、付费附件及其 C 端浏览器流程，Mailpit 与 MinIO 是必需前置条件。脚本会刷新受管后端进程，并确认 etcd 中每个业务服务只有一个注册，避免旧二进制或额外实例参与验收：

```powershell
cd D:\projects\bbs\backend\deployments\local
# 仅启动 BBS 专用 Mailpit；其余依赖（包括 Nacos、MinIO、ES）必须已在本机运行。
docker compose up -d
.\scripts\bootstrap.ps1 -Full
.\scripts\migrate.ps1

cd D:\projects\bbs
.\scripts\commercial-e2e.ps1 -SkipBuild

# 如需强制复用手动启动的 C 端和管理端前端，避免脚本自动拉起/关闭 Vite 子进程。
.\scripts\commercial-e2e.ps1 -SkipBuild -NoAutoFrontend

# 如需复用已手动启动的后端服务，避免脚本刷新或自动启动受管服务进程。
.\scripts\commercial-e2e.ps1 -SkipBuild -ReuseRunningBackend -NoAutoFrontend

# 如需复用第二套隔离端口后端，可加载与 start-local-visible 相同的端口环境文件。
.\scripts\commercial-e2e.ps1 -SkipBuild -ReuseRunningBackend -NoAutoFrontend -EnvironmentFile .\backend\deployments\local\.env.stack-b

# 仅在有意不验收附件时显式跳过；其余商业链路仍会执行。
.\scripts\commercial-e2e.ps1 -SkipBuild -SkipAttachments

# 变更现有 MinIO 的地址、桶或凭据时，更新 backend\deployments\local\.env 的 MINIO_* 配置。
```

管理端商城浏览器联调验收：

```powershell
# 需要先启动 commercial 后端和 api-gateway；若管理端 dev server 未启动，脚本会自动拉起并在结束时关闭。
# 脚本会通过真实登录页进入 vue-pure-admin，并覆盖商城概览、分类、商品、评价、优惠券、订单和售后管理页面的核心可用性。
cd D:\projects\bbs\vue-pure-admin
pnpm e2e:mall

# 如需强制复用手动启动的管理端，可关闭自动拉起。
$env:ADMIN_MALL_E2E_NO_AUTO_FRONTEND = "1"
pnpm e2e:mall

# 若脚本没有自动找到 Chrome/Chromium，可显式指定
$env:CHROME_EXECUTABLE = "C:\path\to\chrome.exe"
pnpm e2e:mall
```

## 配置约定

本地服务配置通过 Nacos 发布。为避免多个项目共用本机 Nacos 时 dataId 冲突，BBS 关键服务使用独立 dataId，例如：

- `bbs-api-gateway.yaml`
- `bbs-admin-service.yaml`
- `bbs-user-service.yaml`
- `bbs-content-service.yaml`
- `bbs-comment-service.yaml`
- `bbs-reaction-service.yaml`
- `bbs-search-service.yaml`
- `bbs-feed-service.yaml`
- `bbs-credit-service.yaml`
- `bbs-notification-service.yaml`
- `bbs-mall-service.yaml`
- `bbs-chat-service.yaml`

admin-service 默认本地管理员账号：

- 账号：`admin`
- 密码：`Admin123!`

生产环境必须替换 JWT secret、默认密码、数据库凭证和 OAuth 应用密钥。`admin-service` 在 `trace.env=prod` 或 `production`（也可设置 `BBS_ADMIN_TRACE_ENV`）时会拒绝启动，除非 `BBS_ADMIN_AUTH_JWT_SECRET` 与 `BBS_ADMIN_AUTH_SECRET_ENCRYPTION_KEY` 是彼此不同、至少 32 字节的非默认随机值，且 `BBS_ADMIN_AUTH_DEFAULT_ADMIN_PASSWORD` 为非默认的强密码；首次管理员初始化完成后应立即轮换该密码。

API Gateway 对注册、登录、找回密码、邮件验证和后台登录使用 Redis 滑动窗口限流；可在 Nacos 的 `auth.rateLimit.*`（或对应的 `BBS_GATEWAY_AUTH_RATE_LIMIT_*` 环境变量）调整阈值。`http.trustedProxies` 默认为空，不信任任意转发 IP 头；部署在反向代理后必须显式配置实际代理的 CIDR/IP，避免 IP 限流被伪造的 `X-Forwarded-For` 绕过。

Gateway 在 `/robots.txt`、`/sitemap.xml` 与 `/sitemaps/*` 提供公开 SEO 文档；前端 Nginx 和生产 Kubernetes Ingress 均将这些路径路由到 Gateway。生产环境应将 `BBS_GATEWAY_HTTP_PUBLIC_BASE_URL` 配置为对搜索引擎可访问的前台站点源站（例如 `https://bbs.example.com`）。Sitemap 仅列出公开静态页面和已发布的话题、文章，不会暴露登录后页面或聊天室。

用户改密或完成密码重置时，user-service 会在与 Gateway 共用的 Redis 中更新 `bbs:auth:credential-version:<userID>`；Gateway 会拒绝版本不匹配的旧 JWT。生产环境需将 `BBS_USER_REDIS_ADDR`、`BBS_USER_REDIS_PASSWORD`、`BBS_USER_REDIS_DB` 配置为与 Gateway 的 Redis 连接一致。

chat-service 的 gRPC 业务接口只接受 Gateway 附带的内部 token。生产环境必须为 `BBS_CHAT_GRPC_SERVER_INTERNAL_AUTH_TOKEN` 和 `BBS_GATEWAY_UPSTREAMS_CHAT_INTERNAL_AUTH_TOKEN` 配置同一个非默认、至少 32 字节的随机值；应通过部署密钥管理注入，不能沿用本地开发 token。

user-service 的 `OAuthLogin` 与 `WebmasterLogin` 也只接受 Gateway 附带的内部 token。生产环境必须为 `BBS_USER_GRPC_SERVER_INTERNAL_AUTH_TOKEN` 和 `BBS_GATEWAY_UPSTREAMS_USER_INTERNAL_AUTH_TOKEN` 配置同一个非默认、至少 32 字节的随机值；应通过部署密钥管理注入，不能沿用本地开发 token。

user-service 的全部业务 gRPC 接口均只接受内部 token（仅标准 `grpc.health.v1.Health/Check` 可匿名探活）。admin-service 也会调用 user-service，因此生产环境还必须将 `BBS_ADMIN_UPSTREAMS_USER_INTERNAL_AUTH_TOKEN` 配置为同一随机值。密码重置令牌只会通过安全邮件投递，不会由 gRPC 或 HTTP 响应返回。

admin-service 的全部管理业务 gRPC 接口也只接受 Gateway 附带的内部 token。生产环境必须为 `BBS_ADMIN_GRPC_SERVER_INTERNAL_AUTH_TOKEN` 与 `BBS_GATEWAY_UPSTREAMS_ADMIN_INTERNAL_AUTH_TOKEN` 配置同一个非默认、至少 32 字节的随机值；未经认证的调用方只能使用标准 gRPC health Check。

file-service 的全部附件业务 gRPC 接口也只接受 Gateway 附带的内部 token。生产环境必须为 `BBS_FILE_GRPC_SERVER_INTERNAL_AUTH_TOKEN` 与 `BBS_GATEWAY_UPSTREAMS_FILE_INTERNAL_AUTH_TOKEN` 配置同一个非默认、至少 32 字节的随机值；仅标准 gRPC health Check 可匿名探活。

## 当前开发重点

- 完善社区 C 端用户体验：登录注册页、OAuth 登录、用户中心、内容发布、互动、搜索和商城闭环。
- 完善管理端：系统管理、Casbin RBAC、社区管理、商城管理、内容治理和运营配置。
- 强化后端一致性：Redis/PG 计数一致性、Kafka 事件补偿、ES 索引同步、订单与积分事务边界。
- 补齐商业化能力：积分商城、优惠券、评价、售后、通知、审计日志、运营配置和可观测性。
