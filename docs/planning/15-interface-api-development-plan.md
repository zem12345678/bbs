# 接口设计与开发规划（中文版）

> 本文为 `api-1.json` 对应的中文版接口设计与开发规划。接口路径、字段名和代码标识保留原文形式，便于直接对照网关实现和自动化测试。

## 1. 文档状态

- 状态：开发规划，尚未作为“全部接口已完成”的声明。
- 更新时间：2026-08-23。
- 目标仓库：`D:\projects\bbs`。
- 参考实现：`D:\projects\Universe-Federation`，仅用于核对 Misskey/Sharkey 的请求、响应、权限和错误语义，不复制其实现代码。
- 主接口文档：`D:\projects\bbs\docs\api-1.json`。

本规划覆盖接口契约、网关适配、服务归属、数据模型、前端调用和验证方式。已有业务能力优先通过兼容路径暴露，不因路径不同重复建立领域服务。

## 2. 目标与边界

### 2.1 目标

1. 以 `api-1.json` 为外部契约基线，逐项实现 610 个操作的可用行为。
2. 保留当前 BBS API、前端 UI 和页面布局；Misskey 兼容接口作为网关适配层增加。
3. 对已存在的文章、主题、评论、点赞、收藏、用户、通知、文件、商城和后台能力做语义映射，而不是复制一套平行业务逻辑。
4. 对 Note、可见性、回复/转发、文件、投票、任意 reaction、联邦等当前领域没有表达能力的部分，补充最小领域模型和事件，而不是在 HTTP 层伪造成功响应。
5. 每一批接口都有契约测试、服务测试和至少一条真实网关联调路径。

### 2.2 不在本规划中隐含承诺的内容

- 不把当前已通过的 smoke 测试等同于 610 个接口全部完成。
- 不把远程联邦数据伪装成本地数据；联邦相关能力必须有明确的远端读写、失败和重试语义。
- 不改变现有前端主导航、三栏桌面布局和单栏移动布局；新增页面只沿用现有组件和样式。
- 不把 Elasticsearch 当作业务事实源。搜索和时间线可读 ES 投影，但写入仍由拥有领域的服务完成。

## 3. 现状基线

### 3.1 文档规模

对 `api-1.json` 的 `paths` 和 HTTP 方法做了机器统计：

| 指标 | 数量 |
| --- | ---: |
| 操作总数 | 610 |
| 需要 bearer 凭证 | 475 |
| 公开操作 | 135 |
| 主要标签 | admin、notes、users、chat、drive、meta、charts、federation 等 |

文档中的所有操作当前都是 `POST`；网关内部已有 REST 风格 `GET/PUT/DELETE` 接口时，优先保留该接口并增加兼容 POST 路径。

### 3.2 已有能力与复用策略

以下能力已经存在或已有等价业务逻辑，应由兼容适配层调用，不另建领域服务：

| 领域 | 当前能力/入口 | 兼容适配原则 |
| --- | --- | --- |
| 账号与当前用户 | 登录、token、`/i`、资料更新、密码、注销 | 统一使用现有认证中间件和用户服务；只转换请求字段和错误码 |
| 用户目录 | 用户详情、批量查询、关注关系、用户列表 | `/users/show` 等 Misskey 形状由 gateway 转换 `userpb`，不泄露内部字段 |
| 主题与文章 | 创建、发布、隐藏、归档、详情、列表、评论 | 现有 content-service 继续拥有写模型；兼容接口调用同一 gRPC 命令 |
| 评论与互动 | 评论会话、like/favorite、举报 | 复用 comment/reaction 服务；任意 reaction 另走扩展任务 |
| 搜索 | Elasticsearch 搜索和 `/notes/search` 等兼容查询 | 统一分页、超时和 ES 不可用时的错误策略 |
| Clips/摘录 | clips CRUD、note 关联、收藏、导出 | 后端兼容路由已存在；前端补个人中心入口，不重复实现存储 |
| 天线、列表、注册表、导入导出 | gateway 已有对应路由和服务调用 | 先补路径、字段和错误契约，再补 UI 缺口 |
| 通知、图表、后台 RBAC、商城 | 已有服务和 smoke 覆盖 | 以现有权限和审计链为准，不绕过后台鉴权 |

最近已验证的兼容接口包括 `/i`、`/users/show`、用户目录、`/notes/show` 和 `/notes/search`。它们仍需纳入统一清单回归，不代表其余文档操作已经完成。

### 3.3 当前高价值缺口

| 优先级 | 操作 | 缺口 | 计划处理 |
| --- | --- | --- | --- |
| P0 | `/notes/create` | 没有 Misskey Note 写入契约 | 第一批；复用 content-service 的 tweet/topic 能力并补 Note 元数据 |
| P0 | `/notes/delete` | 没有按 Note ID 解析、所有权校验和 204 响应 | 与 create 同批 |
| P0 | `/notes/timeline` | 现有 feed 返回 BBS 响应，不是 Note 数组 | create/delete 后实现 |
| P1 | `/notes/reactions/create`、`/notes/reactions/delete` | reaction 服务当前只有 like/favorite 语义 | 扩展通用 reaction 类型和唯一约束 |
| P1 | `/users/notes` | 没有公开用户 Note 时间线 | 复用 Note 查询和用户可见性策略 |
| P1 | `/users/reactions` | 没有公开用户 reaction 列表 | 与任意 reaction 查询同批 |
| P1 | `/users/report-abuse` | 只有内容/评论举报，没有用户举报契约 | 复用 report 持久化和 admin 审核队列 |
| P1 | Clips 前端入口 | API 已有，个人中心没有 UI 调用 | 增加 `/user/clips`，不改现有布局 |

## 4. 兼容契约规则

### 4.1 路径与方法

1. Misskey 操作的规范路径是 `/api/v1/<operation>`。
2. 已有兼容路由按现有约定提供 `/<operation>` 和 `/api/<operation>` 别名；新增别名必须在路由契约测试中同时验证。
3. 现有 BBS 路径继续保留，例如 `/api/v1/topics`、`/api/v1/articles`；不能用兼容路径替换或破坏它们。
4. 文档操作均以 JSON `POST` 为外部契约。网关已有的资源式 HTTP 方法继续服务当前前端。

### 4.2 字段与响应

1. 外部 ID 统一序列化为字符串；兼容请求同时接受数字和字符串，使用结构化 JSON 解码，不用字符串替换。
2. Unix 时间使用文档要求的毫秒格式；内部 `time.Time` 只在 gateway 处转换。
3. Misskey 响应使用文档字段名，例如 `createdNote`、`userId`、`noteId`；现有 BBS 响应不改名。
4. 204 操作必须返回空 body；不能返回 `{success:true}` 代替 204。
5. 响应中的用户、Note、文件和 reaction 使用 allowlist 转换，禁止把内部 token、邮箱、审计字段带出。
6. 请求未知字段是否拒绝，以对应文档契约为准；敏感账户接口继续使用严格解码。

### 4.3 认证、权限与所有权

| 文档权限 | 网关实现 |
| --- | --- |
| `read:account` | `requireAuthScope("read")` |
| `write:notes` | `requireAuthScope("write")` 加发布、禁言和邮箱验证检查 |
| `write:reaction`/互动写权限 | `requireAuthScope("write")` 加目标可见性检查 |
| `write:report-abuse` | `requireAuthScope("write")` 加自报、管理员和重复举报规则 |
| admin 操作 | `requireAdminAuth` + 最小 RBAC permission |
| 无凭证 | `optionalAuth`；私有内容由目标服务决定是否隐藏 |

所有修改操作都必须在服务或 gateway 完成目标作者/当前用户校验，不能只依赖前端传入的 `userId`。

### 4.4 错误映射

1. 新接口沿用当前 API Gateway 的错误响应格式，不引入新的 `code/message/details/traceId` 包装格式。
2. 错误响应为现有 `exception.ApiException` JSON：

   ```json
   {
     "service": "api-gateway",
     "trace_id": "",
     "request_id": "",
     "http_code": 400,
     "code": 400,
     "reason": "请求不合法",
     "message": "Invalid param.",
     "meta": {
       "legacy_code": "INVALID_PARAM",
       "error_id": "optional-compat-error-id"
     },
     "data": null
   }
   ```

3. `code` 和 `http_code` 使用现有数字错误码；兼容接口的字符串错误码放在 `meta.legacy_code`，文档中的错误 ID 放在可选的 `meta.error_id`。`trace_id` 和 `request_id` 继续由现有 response middleware 注入。
4. 新 handler 必须复用 `writeError`、`writeRPCError` 或已有兼容错误 helper，禁止自行定义新的错误 envelope。
5. 参数错误为 400，缺少凭证为 401，权限或所有权错误为 403，不存在或不可见内容按契约返回 400/404，冲突为 409，依赖不可用为 503，限流为 429。
6. 每个新操作至少覆盖：缺字段、非法 ID、匿名访问、越权访问、上游 NotFound 和上游不可用。
7. 不为了通过 happy path 而吞掉上游错误；错误必须能在 gateway 日志中关联 trace id。

## 5. 领域与服务归属

| 接口域 | 主要拥有服务 | gateway 责任 | 备注 |
| --- | --- | --- | --- |
| `i/*`、认证、token | user/auth | token 解析、scope、字段转换 | 账号敏感操作要求 interactive auth |
| `users/*`、following、blocking、muting | user-service | 公开/私有字段裁剪、关系状态 | 用户举报写入 reaction/admin 队列 |
| `notes/*`、`users/notes` | content-service + comment-service | Note 形状、作者打包、分页 | Note 是 content 的 tweet 能力，不复制 article 服务 |
| `notes/reactions/*`、`users/reactions` | reaction-service | entity 引用、204、通知触发 | 任意 reaction 与 like/favorite 分开建模 |
| `clips/*` | reaction/content 现有 clip 模块 | 兼容字段、用户可见性 | 前端仅增加入口和操作面板 |
| `drive/*`、文件上传 | file-service | 上传限制、URL、权限 | 文件 ID 必须归属当前用户或公开实体 |
| `channels/*`、`antennas/*`、`lists/*` | content/user/feed | 过滤器和 Note 列表转换 | 与时间线查询共用可见性过滤 |
| `notifications/*` | notification-service | Misskey notification shape | 不能绕过通知偏好和去重 |
| `admin/*`、`roles/*` | admin-service + owning services | RBAC、审计、分页 | 后台操作必须记录 operation log |
| `charts/*`、`stats`、`meta` | owning query services/config | 只读聚合和缓存 | ES/Redis 不可用要有明确降级 |
| `federation/*`、`ap/*` | federation adapter（规划新增） | 签名、远端地址和错误翻译 | 本地能力完成后单独做联邦垂直切片 |
| `chat/*`、`reversi/*`、`gallery/*`、`pages/*`、`flash/*` | 现有对应模块或新增专属模块 | 仅做 API 兼容 | 不能把复杂实时域塞进 content-service |

## 6. Note 兼容垂直切片设计

### 6.1 选择

Note 与文章/普通主题的生命周期和字段并不相同。实现时不强行给 Note 填充用户不可见的标题和 slug，也不再建立第二套内容服务；采用以下折中：

- 在 content-service 中复用 `TopicTypeTweet` 作为本地短内容载体。
- 为 tweet 增加 Note 元数据/边关系：可见性、回复目标、转发目标、文件关联、reaction acceptance、局部发布标记等。
- 现有主题/文章继续使用原有命令和查询；`/notes/show` 先解析 tweet，再兼容已发布主题/文章的历史 Note 映射。
- 所有 Note ID 使用 content-service 的全局 ID 生成器；解析时禁止同一 ID 同时命中多个实体。
- 软删除沿用归档/删除状态和事件；对外按 `/notes/delete` 返回 204，查询侧隐藏已删除内容。

### 6.2 第一批接口

| 操作 | 请求重点 | 内部动作 | 外部结果 |
| --- | --- | --- | --- |
| `/notes/create` | `text`、`visibility`、`replyId`、`renoteId`、`fileIds`、`poll` | 校验用户状态和目标可见性；创建 tweet + 元数据；发布领域事件 | 200 `{createdNote}` |
| `/notes/delete` | `noteId` | 解析 tweet/历史内容；校验作者；软删除并发事件 | 204 |
| `/notes/timeline` | since/until、limit、文件/转发过滤 | feed 查询、用户可见性过滤、Note 打包 | Note 数组 |
| `/users/notes` | `userId`、分页/日期 | user-service 校验公开设置；content 查询 | Note 数组 |
| `/notes/reactions/create` | `noteId`、`reaction` | reaction 唯一约束、目标可见性、通知事件 | 204 |
| `/notes/reactions/delete` | `noteId` | 删除当前用户对应 reaction | 204 |
| `/users/reactions` | `userId`、分页/日期 | reaction 查询并批量打包 Note | NoteReaction 数组 |
| `/users/report-abuse` | `userId`、`comment` | 校验不可自报/不可举报管理员；写 report | 204 |

### 6.3 依赖顺序

1. content proto/数据库：tweet 元数据、边关系、可见性和软删除。
2. content 查询：按 ID、作者、时间、游标读取并过滤。
3. gateway Note packer：用户、文件、reaction、时间格式和兼容错误。
4. reaction 扩展：任意 reaction 的类型、唯一键、删除和通知。
5. 契约测试与真实联调：匿名、双用户、越权、删除后查询、ES 重建。

## 7. 分阶段交付路线

### 阶段 A：清单与契约冻结

交付：

- 从 `api-1.json` 生成 610 操作清单，字段为 path、method、auth、request schema、response schema、当前状态、owner、测试位置。
- 将每个操作标记为 `reuse`、`adapter`、`extend-domain`、`new-domain` 或 `federation`。
- 为同一功能的 BBS 路径和 Misskey 路径建立映射表，避免重复注册和响应漂移。

验收：清单可重复生成；每个操作有唯一 owner 和下一步；不再使用“全量完成”这种无法核验的描述。

### 阶段 B：Note 核心写读

范围：`notes/create`、`notes/delete`、`notes/show` 补充 tweet、`notes/timeline`、`users/notes`。

验收：

- 两个用户可分别创建、查看、回复、转发和删除自己的 Note。
- 被隐藏、被删除、followers/specified 可见性符合契约。
- 时间线分页不会重复或跳过；since/until 和 limit 有边界测试。
- 文章/主题原有详情、列表和 `/notes/show` 回归通过。

### 阶段 C：Reaction 与举报

范围：`notes/reactions/create`、`notes/reactions/delete`、`users/reactions`、`users/report-abuse`。

验收：

- 任意 reaction 幂等，删除只影响当前用户。
- reaction acceptance、被屏蔽用户和私有 Note 的访问规则正确。
- 用户举报不能自报或举报管理员；后台可看到并审核该 report。
- 通知、计数、Kafka/outbox 重试和重复消费测试通过。

### 阶段 D：已有后端能力的全量兼容覆盖

按域推进：`account/i`、`users`、`following`、`clips`、`antennas`、`lists`、`notifications`、`drive`、`channels`、`hashtags`、`charts/meta`、`admin/roles`、`chat`、`gallery/pages/flash`。

每个域先做路径和响应契约，再做缺失字段和权限，最后补前端入口。能调用现有服务的操作不得新建表或新建同义 RPC。

### 阶段 E：文件、投票和复杂内容

补齐 Note 文件、poll、频道、媒体、导入导出及内容关联；复用 file-service 和现有 topic poll 能力，补充 Note-specific 校验和返回形状。

验收包含大文件限制、归属校验、投票过期、多选、重复投票、附件不可见和错误回滚。

### 阶段 F：联邦和远程能力

范围：`federation/*`、`ap/*`、远程用户/Note、签名、入站/出站队列。

先实现本地对象与事件模型，再引入远程 resolver、签名验证、重试、幂等和隔离。远程服务不可用时返回稳定错误，不返回本地伪造对象。

### 阶段 G：前端可用性与发布

- API client 只从 `frontend/public/config.js`/生产 runtime config 读取 base URL，禁止页面写死服务地址。
- 在不改变现有布局的前提下增加 Clips 管理、Note 发布/时间线、reaction 和举报入口。
- 每个新增页面提供 loading、empty、error、未登录和权限状态。
- 使用前端现有测试、Playwright smoke 和桌面/移动截图回归。

## 8. 测试与验证矩阵

### 8.1 单元与契约

- Go handler：路径别名、JSON 字段、ID 转换、状态码、错误 code、响应 allowlist。
- Go domain/service：所有权、可见性、幂等、状态机和 outbox。
- Frontend API：请求路径、method、base URL、认证头和错误传播。
- 契约 fixture：从 `api-1.json` 保留最小请求/响应样例，避免测试依赖外网。

### 8.2 集成与真实联调

- API gateway -> gRPC -> PostgreSQL/Redis/MongoDB/Elasticsearch。
- Elasticsearch 启动后验证索引、查询超时、无索引和重建流程；ES 副本未分配的单节点 `yellow` 状态不应被误判为服务失败。
- 双用户流程：登录、创建 Note、回复/转发、timeline、reaction、删除、举报、通知。
- 管理员流程：RBAC、report 审核、操作日志。
- 完整 commercial smoke、契约测试和前端测试作为每批提交门槛。

### 8.3 发布门槛

每一批提交必须同时满足：

1. `git diff` 只包含本批接口和测试/文档。
2. 相关 Go 包 `go test`、gateway 契约测试和 frontend test 通过。
3. 至少一条真实 HTTP 联调记录请求、响应状态和关键字段。
4. 清单状态仅在测试通过后从 `planned` 更新为 `implemented`；失败或未覆盖保留为 `partial`。
5. 提交说明包含本批覆盖的 operation 列表和未覆盖项。

## 9. 风险与决策

| 风险 | 处理决策 |
| --- | --- |
| 将所有 Note 塞进文章模型导致标题/slug/可见性错误 | 使用现有 tweet 类型 + Note 元数据，不强行复用 article 字段 |
| ID 同时命中文章和主题 | 统一 ID 解析器；冲突直接返回稳定 conflict，不猜测 |
| 兼容接口与现有 API 响应互相污染 | gateway 只做边界转换；内部 proto 保持领域命名 |
| ES 查询结果滞后 | 明确最终一致性；写入不依赖 ES 成功，提供重建和状态检查 |
| 跨服务通知/计数丢失 | outbox、幂等消费、重试和 reconciliation 测试 |
| 610 个接口范围过大 | 以机器清单分批交付，每批可独立验证和提交 |
| Universe-Federation 行为持续变化 | 以本地 `api-1.json` 版本为锁定契约，外部代码只作语义参考 |

## 10. 第一批开发任务清单

当前建议在本规划获确认后按以下顺序进入编码：

1. 生成并提交接口清单基线，补充每个操作的当前状态。
2. 定义 tweet Note 元数据和 content-service proto 变更。
3. 实现 `/notes/create`、`/notes/delete` 和 Note packer。
4. 实现 `/notes/timeline`、`/users/notes`，补游标/日期/可见性测试。
5. 扩展任意 reaction 和用户举报。
6. 补 gateway 契约、前端 API 方法和真实联调。
7. 将 Clips 接入现有个人中心，保留原有布局。
8. 运行全套测试和 smoke，提交一版并更新本清单状态。

完成以上第一批后，再进入 drive/poll/channel 和 federation 等后续域；不要在第一批中同时改动无关 UI 或重构已有服务。

## 11. 中文接口设计速览

### 11.1 对外接口分层

| 层级 | 约定 | 说明 |
| --- | --- | --- |
| 现有业务接口 | `/api/v1/...` | 保持当前 BBS 前端正在使用的路径、响应和权限，不做破坏性改名 |
| Misskey 兼容接口 | `/api/v1/<operation>` | 按 `api-1.json` 的请求字段、响应字段、状态码和错误码实现 |
| 兼容别名 | `/<operation>`、`/api/<operation>` | 仅对已注册的兼容操作提供，别名必须共用同一个 handler |
| 内部调用 | gRPC | 由 API Gateway 转换为服务请求，前端不直接访问内部服务 |

### 11.2 首批接口设计

| 接口 | 作用 | 认证 | 返回 |
| --- | --- | --- | --- |
| `notes/create` | 发布短内容、回复或转发 | `write:notes` | `{ "createdNote": Note }` |
| `notes/delete` | 删除当前用户自己的 Note | `write:notes` | HTTP 204，无响应体 |
| `notes/show` | 查看 Note；兼容已有主题和文章映射 | 可选 | `Note` |
| `notes/timeline` | 按时间读取当前用户可见的 Note | `read:account` | `Note[]` |
| `users/notes` | 读取公开用户的 Note | 可选 | `Note[]` |
| `notes/reactions/create` | 对 Note 添加任意 reaction | 写权限 | HTTP 204 |
| `notes/reactions/delete` | 删除当前用户对 Note 的 reaction | 写权限 | HTTP 204 |
| `users/reactions` | 读取用户公开的 reaction | 可选 | `NoteReaction[]` |
| `users/report-abuse` | 举报用户 | `write:report-abuse` | HTTP 204 |

### 11.3 与现有功能的关系

- 文章、主题、评论、点赞、收藏、用户关系、通知、搜索、Clips 和后台 RBAC 继续使用现有服务。
- Note 使用 content-service 已有的 tweet 能力，并增加可见性、回复/转发、文件和 reaction 元数据；不强行填充文章的标题和 slug。
- 兼容接口只负责字段转换、权限入口、错误翻译和 Note 打包；业务状态和数据一致性仍由拥有领域的服务负责。
- 前端 API 地址继续从 `frontend/public/config.js` 或生产 runtime config 读取，页面不写死服务地址。

### 11.4 中文验收标准

每批接口交付前必须确认：

1. 请求字段、响应字段和错误码可在 `api-1.json` 中逐项对照；错误响应继续使用现有 `ApiException` 格式，兼容码位于 `meta.legacy_code`。
2. 匿名、正常用户、越权用户、管理员和上游不可用场景均有测试。
3. 204 接口没有多余响应体，ID 和时间格式符合兼容约定。
4. 现有 BBS 接口和前端页面回归通过，新增功能不改变现有布局。
5. Elasticsearch 已启动时完成搜索/时间线联调；单节点副本未分配造成的 `yellow` 状态按正常本地部署处理。
6. 只有测试通过的操作才从 `planned` 更新为 `implemented`，未覆盖的操作保留 `partial`。
