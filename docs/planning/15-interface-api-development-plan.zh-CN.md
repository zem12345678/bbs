# 接口设计与开发规划（中文版入口）

本文件是接口开发规划的中文入口，完整内容请参阅：

- [接口设计与开发规划（中文版）](./15-interface-api-development-plan.md)

规划基于以下本地资料：

- `D:\projects\bbs\docs\api-1.json`：610 个 API 操作的 OpenAPI 文档。
- `D:\projects\Universe-Federation`：Misskey/Sharkey 请求、响应、权限和错误语义的参考实现。

核心原则：

1. 现有 API 已实现相同业务时，只增加兼容路径和字段转换，不重复实现领域逻辑。
2. 前端保持现有 UI 和基本布局，只增加必要的功能入口、状态和页面。
3. API Gateway 是唯一对外 HTTP 边界，内部服务继续使用 gRPC。
4. Note、可见性、回复/转发、任意 reaction 和联邦等缺失能力，先补领域模型，再提供接口，不能在 HTTP 层伪造成功响应。
5. 每批接口都必须有契约测试、服务测试和真实网关联调，并在清单中记录实现状态。

当前建议的第一批接口为：

`notes/create`、`notes/delete`、`notes/timeline`、`users/notes`、`notes/reactions/create`、`notes/reactions/delete`、`users/reactions`、`users/report-abuse`。

该入口文件只用于中文导航，不单独维护第二份接口内容，避免规划分叉。
