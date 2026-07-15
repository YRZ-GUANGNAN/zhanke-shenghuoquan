# UI-21b-2c 二手真实写入闭环 Acceptance Report

日期：2026-07-15
状态：**PARTIAL PASS / BLOCKED BY FROZEN MEDIA CONTRACT**

## 1. 验收结论

UI-21b-2c 已通过代码、类型、构建、路由、领域边界和全量后端回归检查。2026-07-15 已补齐本机 Meilisearch 与后端，并以隔离 mock 身份、真实数据库、真实 COS、真实 Meilisearch 和真实腾讯云 IM 配置执行联调。

商品真实写入、幂等重放/冲突、后台审核、搜索入索引、确定态收藏、稳定联系会话及腾讯云双向文字消息均通过。唯一上线阻断是“新版媒体资产绑定到交易商品”返回 500 `invalid media asset`；根因位于冻结后端的两套媒体契约不兼容，不可由前端安全修复。因此本报告不能给出完全 PASS。

## 2. 验收矩阵

| 项目 | 证据 | 结果 |
| --- | --- | --- |
| Meilisearch 与后端运行态 | `GET :7700/health` available；`GET :8080/healthz` 200 | PASS |
| 真实媒体上传 | ticket → COS PUT → complete → 本地审核投影，asset 可用 | PASS |
| 图片资产绑定商品 | `POST /trade/products` 返回 500 `invalid media asset` | **BLOCKED** |
| 商品创建金额为整数分 | 实际落库价格 `129900` 分 | PASS |
| 创建请求可安全重试 | 首次 201；同键同体 200 + replay header；同键异体 409 | PASS |
| 后台审核与搜索 | 审核通过后 Meilisearch 命中商品 | PASS |
| 收藏状态由服务端驱动 | `PUT` 重放计数仍为 1；`DELETE` 可重放；已售新增返回 409 | PASS |
| 联系卖家不信任 URL 会话参数 | 重复联系返回稳定会话 `C2Ccampus_900000081`，viewer state 与会话列表一致 | PASS |
| 腾讯 IM 历史/双向文字消息 | 腾讯云 REST A→B、B→A 发送成功，双方漫游历史均可见两条消息 | PASS |
| 小程序腾讯 IM SDK 代码集成 | 历史、订阅、发送、卸载清理均通过类型和生产构建 | CODE PASS |
| 微信开发者工具双账号实时 UI | 本轮未完成真实两端 GUI 操作 | PENDING |
| 二手图片私信 fail-closed | 图片按钮不选图、不上传、不发送，只显示尚未开放提示 | PASS |
| 后端与迁移零改动 | diff 边界检查无 `backend/**` 或迁移文件 | PASS |
| 全量自动化 | `npm run verify`、生产构建、静态路由检查 | PASS |

## 3. 自动化证据摘要

- `npm run verify`：领域边界 PASS、Admin/MiniApp 类型检查 PASS、后端全部 Go 包测试 PASS。
- `npm --workspace apps/miniapp run build:weapp`：Webpack 编译成功。
- `npm run check:miniapp-static-routes`：25 条路由 PASS。
- ULID 实码检查：长度 26、字符集符合 Crockford Base32、固定输入可重复验证。
- `git diff --check`：无空白错误。

## 4. 真实环境证据

本轮验收标记：`20260715091015-314834`。

- 媒体 ticket：`01KXJGN180DZ2SSQ4B8FM3DDB5`；asset：`01KXJGN18064XADJYHDD6M0TJ1`；媒体状态为 approved/canUse，但商品绑定返回 500。
- 无图回退商品：ID `126`，价格 `129900` 分；首次创建 201、同键重放 200、同键异体 409；审核任务 `271` 通过后 Meilisearch 命中。验收后商品已下架。
- 卖家与买家分别为隔离测试用户 `900000081`、`900000082`；临时管理员权限已恢复，权限残留为 0。
- 腾讯云真实 C2C 测试输出：`account_import=PASS A_to_B=PASS B_to_A=PASS roam_as_A=PASS roam_as_B=PASS`。使用官方 [单发单聊消息](https://cloud.tencent.com/document/product/269/2282) 和 [查询单聊消息](https://cloud.tencent.com/document/product/269/42794) 接口；临时云端测试账号已删除。
- `npm run check:stack` 的后端健康、trade、navigation、community 检查均通过，之后仅因本轮未启动 Admin Vite `127.0.0.1:5173` 而停止；这不影响小程序本模块 API 验收。

## 5. 阻断定位

真实媒体上传使用 Module 10 的 `asset_public_id`、`purpose=product`、`technical_status=final_ready` 及审核投影。冻结的交易服务仍经旧媒体包 `Reserve` 查询/校验 `public_id`、`purpose=product_image`、旧 `status=approved` 及旧引用字段。两者共享表但没有兼容适配，导致新资产即使显示 canUse，也无法通过交易服务校验。

改用旧上传链路并不是合规的前端修复：它需要另一套 STS/COS 客户端，并会绕开本任务采用的现行媒体票据与审核协议。正确收口需要在后端增加媒体网关适配或统一契约，并保持现有审核与引用安全约束。

## 6. 最终判定

本模块达到 **前端代码 PASS + 除带图发布外的真实链路 PASS**。带图发布是产品主路径，当前不能标记完全上线 PASS，也不应据此合并或打上线标签。下一步需 owner 明确授权后端适配，完成后重新执行带图商品创建及微信开发者工具双账号实时 UI 验收。
