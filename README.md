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

# 只调试后台登录/菜单时，可使用轻量模式
.\backend\scripts\start-local-visible.ps1 -Profile minimal -Restart -Build
```

若本机同时运行其他项目，可将服务需要的环境变量写入独立文件，并在启动时加载。变量只会传给本次启动的 BBS 子进程，不会写入用户环境变量：

```powershell
# 例如 D:\projects\bbs\backend\deployments\local\.env.override
BBS_MALL_GRPC_SERVER_PORT=19115
BBS_MALL_SERVICE_GRPC_PORT=19115

.\backend\scripts\start-local-visible.ps1 -Profile commercial -EnvironmentFile .\backend\deployments\local\.env.override -Restart -Build
```

前端启动：

```powershell
cd D:\projects\bbs\frontend
npm run dev

cd D:\projects\bbs\vue-pure-admin
npm run dev
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
