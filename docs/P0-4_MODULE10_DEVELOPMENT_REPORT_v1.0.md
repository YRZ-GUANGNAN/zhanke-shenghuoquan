# P0-4 Module ⑩ Development Report v1.0

日期：2026-06-21  
模块：MiniApp Upload Flow  
开发状态：Development Complete

## 1. 实施边界

本阶段仅将已冻结的媒体领域能力接入微信小程序上传流程。未修改 Module ②～⑨ 的数据库契约、迁移 009～014、状态机、Repository、Service、Consumer、GC、Admin Console、HTTP Contract 或既有测试；未新增媒体模型、迁移、审核供应商、常驻 Worker 或后续基础设施模块。

业务域接线保持只读：帖子和头像页面可以发起媒体上传、读取媒体状态，但不会把媒体标识写入冻结的社区、交易或用户领域。正式引用绑定留待 P0-4 整体联调阶段。

## 2. 后端实现

新增独立应用编排包 `backend/internal/module/media/miniappflow`：

- `types.go`：小程序只读 DTO、服务端媒体状态、恢复动作；
- `status_repository.go`：仅通过 Repository 按 owner 读取 Asset、Ticket、Projection 与上传历史；
- `service.go`：编排 Create Ticket → STS Authorization → Mark Uploaded → Completion → Review Projection → Outbox → Consumer；
- `handler.go`：请求校验、身份绑定、错误码映射和只读状态响应；
- 自动化测试：服务链、幂等、身份隔离、边界检查、真实 MySQL、真实 STS/COS 全链路。

应用服务没有直接访问 GORM、COS SDK 或冻结表；所有领域状态均来自既有 Service/Repository 返回结果。重复 Complete 不会重复创建 Final Object、Projection、Outbox 或 Inbox。

## 3. MiniApp Media API

路由统一注册在 `/api/v1/miniapp/media`，并经过现有 `AuthRequired + UnmutedRequired` 中间件：

| Method | Path | 用途 |
|---|---|---|
| POST | `/uploads` | 创建一次性上传票据并取得精确单 PUT 授权 |
| POST | `/uploads/:ticketId/complete` | 完成上传并推进既有媒体流程 |
| GET | `/assets/:assetId` | 查询本人媒体的服务端事实状态 |
| GET | `/uploads` | 查询本人上传历史 |

未注册匿名路由、管理端写接口或业务域写接口。无 Token 返回 401；owner 不匹配返回 404，避免枚举他人资产。

## 4. 小程序实现

- 上传前本地压缩：帖子/商品最长边 1920 px，头像最长边 1024 px，优先 WebP；
- 计算压缩后文件 MD5，并把 Content-Length、Content-MD5、Content-Type 与禁止覆盖头原样带入 COS 单 PUT；
- 最多 3 张并发上传，展示选择、压缩、申请、上传、完成和审核状态进度；
- 票据过期或上传失败后重新创建新票据和新 staging key，不复用授权；
- 支持上传历史、刷新状态、继续完成和重新上传；
- 所有展示状态和恢复动作由后端返回，前端不推导审核或技术状态；
- `approved` 才允许作为可用媒体展示；`rejected/review_failed/expired/upload_failed` 均有明确提示和恢复路径；
- 移除小程序端 `cos-wx-sdk-v5`，客户端不接触长期 Secret 或自由 COS 凭证。

## 5. 状态与错误映射

客户端状态由服务端投影为：

`waiting_upload`、`processing`、`review_pending`、`approved`、`rejected`、`review_failed`、`expired`、`upload_failed`、`deleted`。

可恢复动作由服务端显式给出：`poll`、`new_upload`、`complete_upload` 或 `none`。HTTP 错误统一映射为 400、401、403、404、409、410、422、429、503，不回传 SQL、COS Key、VersionId、Secret 或内部堆栈。

## 6. 测试与验证

- 单元测试：敏感字段排除、授权不可重复签发、服务链调用、状态映射、幂等；
- Boundary Check：无 A/B/C 域依赖、无直接 COS SDK、无新迁移、生产路由必须鉴权；
- 真实 MySQL 8.4.0：owner 隔离、票据/状态读取和清理；
- 真实腾讯云：STS 精确授权、COS PUT、固定版本 Completion、HEAD/GET/Copy、Projection、Outbox/Inbox Consumer、重复完成与清理；
- 微信开发者工具：项目构建、载入、模拟器启动和页面运行通过；完整原生选图上传尚需使用与后端一致的正式/测试 AppID 完成最终证据采集；
- 全量 Go Test、Admin/MiniApp Type Check、Boundary Check、MiniApp Build 均通过。

## 7. 已知问题与未完成项

唯一未关闭门禁是微信开发者工具真实 UI 端到端证据：当前 IDE 会话实际使用测试 AppID `wx961b****f25b`，后端配置使用 `wx27a****95ed6`，因此该会话的 `wx.login` code 不能完成对应后端登录。真实腾讯云上传链路已经由外部集成测试通过，但不能替代微信开发者工具中的用户操作验收。

完成一次“同 AppID 登录 → 原生选图 → 压缩 → PUT → Complete → 状态刷新”的无 Mock 运行并保存日志后，方可申请把 Module ⑩ 从 Conditional PASS 升级为 PASS。

## 8. 停止点

Module ⑩ 开发已停止。未进入新的媒体基础设施模块，未启动 P0-5，也未执行正式业务引用绑定、压测、监控或上线准备。
