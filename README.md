# BBS Community Platform

BBS Community Platform 是一个面向商业化社区场景的论坛系统，目标是在传统论坛能力的基础上补齐内容运营、用户成长、互动治理、搜索、通知、商城和后台权限体系。项目参考 `mlogclub/bbs-go` 的功能方向，前端延续当前社区 UI 风格，后端按 DDD 与微服务架构拆分服务边界。

## 项目组成

- `frontend`：C 端社区前台，包含首页、帖子/话题、搜索、用户中心、商城、订单、评价等用户侧页面。
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

本地依赖由 `backend/deployments/local` 维护，服务进程使用可视化脚本启动，便于观察每个服务控制台日志。

```powershell
cd D:\projects\bbs

# 启动或检查基础设施，按本机实际环境执行
cd backend\deployments\local
.\scripts\bootstrap.ps1

# 使用已安装在本机 5432 端口的 PostgreSQL（不启动 Docker PostgreSQL）
$env:PGPASSWORD = "<本机 PostgreSQL 密码>"
.\scripts\bootstrap.ps1 -UseLocalPostgres

# 启动完整商业化联调后端服务（用户、内容、评论、互动、搜索、积分、通知、信息流、后台、商城、网关）
cd D:\projects\bbs
.\backend\scripts\start-local-visible.ps1 -Profile commercial -Restart -Build

# 检查完整联调服务监听状态
.\backend\scripts\check-local-backend.ps1 -Profile commercial -Strict

# 运行完整商业化 smoke 验收（复用已启动服务，覆盖登录、内容、互动、搜索、通知、后台、积分商城、订单和售后）
.\backend\scripts\smoke-local.ps1 -SkipBuild -KeepRunning

# 只调试后台登录/菜单时，可使用轻量模式
.\backend\scripts\start-local-visible.ps1 -Profile minimal -Restart -Build
```

若本机同时运行其他项目，可将服务需要的环境变量写入独立文件，并在启动时加载。变量只会传给本次启动的 BBS 子进程，不会写入用户环境变量：

```powershell
# 例如 D:\projects\bbs\backend\deployments\local\.env.override
BBS_MALL_GRPC_SERVER_PORT=19115
BBS_MALL_SERVICE_GRPC_PORT=19115

.\backend\scripts\start-local-visible.ps1 -Profile commercial -EnvironmentFile .\backend\deployments\local\.env.override -Restart -Build

# 如果当前 PowerShell 会话里已设置 BBS_MALL_* 端口变量，检查和 smoke 会自动识别；
# 如果只通过 EnvironmentFile 传给 start-local-visible，则仍可显式传入 -MallPort。
.\backend\scripts\check-local-backend.ps1 -Profile commercial -Strict
.\backend\scripts\smoke-local.ps1 -SkipBuild -KeepRunning

# 等价的显式写法：
.\backend\scripts\check-local-backend.ps1 -Profile commercial -MallPort 19115 -Strict
.\backend\scripts\smoke-local.ps1 -SkipBuild -KeepRunning -MallPort 19115
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

# 额外覆盖附件上传、改价、付费下载与归档的浏览器流程；需要 file-service 与 MinIO 可用。
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

# 自定义 MinIO 容器或桶时显式传入对应参数。
.\backend\scripts\attachment-smoke.ps1 -MinIOContainer bbs-local-minio -MinIOBucket bbs-local -MinIOAccessKey minioadmin -MinIOSecretKey minioadmin
```

完整商业化端到端验收会默认包含付费附件及其 C 端浏览器流程，MinIO 是必需前置条件。脚本会刷新受管后端进程，并确认 etcd 中每个业务服务只有一个注册，避免旧二进制或额外实例参与验收：

```powershell
cd D:\projects\bbs\backend\deployments\local
docker compose --profile comments --profile events --profile search --profile files up -d
.\scripts\bootstrap.ps1 -Full

cd D:\projects\bbs
.\scripts\commercial-e2e.ps1 -SkipBuild

# 仅在有意不验收附件时显式跳过；其余商业链路仍会执行。
.\scripts\commercial-e2e.ps1 -SkipBuild -SkipAttachments

# 自定义 MinIO 端口时，健康检查地址会同时用于配置本次验收拉起的 api-gateway。
.\scripts\commercial-e2e.ps1 -MinIOEndpoint http://127.0.0.1:19002/minio/health/live
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

admin-service 默认本地管理员账号：

- 账号：`admin`
- 密码：`Admin123!`

生产环境必须替换 JWT secret、默认密码、数据库凭证和 OAuth 应用密钥。

## 当前开发重点

- 完善社区 C 端用户体验：登录注册页、OAuth 登录、用户中心、内容发布、互动、搜索和商城闭环。
- 完善管理端：系统管理、Casbin RBAC、社区管理、商城管理、内容治理和运营配置。
- 强化后端一致性：Redis/PG 计数一致性、Kafka 事件补偿、ES 索引同步、订单与积分事务边界。
- 补齐商业化能力：积分商城、优惠券、评价、售后、通知、审计日志、运营配置和可观测性。
