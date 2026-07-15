# UI-21b-2c 二手真实写入闭环 Acceptance Report

日期：2026-07-15
状态：**CORE ACCEPTANCE PASS**

## 1. 验收结论

UI-21b-2c 已通过代码、类型、构建、路由、领域边界和全量后端回归检查。2026-07-15 使用隔离 mock 身份、真实 MySQL、真实 COS、真实 Meilisearch 和真实腾讯云 IM 配置完成核心链路验收。

Module 10 图片商品创建、审核发布、固定版本读取、更新替换、拒绝、删除撤销、后台下架、已售保留访问、旧媒体兼容、幂等创建、搜索、收藏、稳定联系会话和腾讯云双向文字消息全部通过。无数据库迁移变更，无审核绕过。微信开发者工具双账号 GUI 操作仍需人工复核，但不构成后端或数据契约阻断。

## 2. 验收矩阵

| 项目 | 证据 | 结果 |
| --- | --- | --- |
| Meilisearch 与后端运行态 | `GET :7700/health` available；`GET :8080/healthz` 200 | PASS |
| 真实媒体上传 | ticket → COS PUT → complete → 本地审核投影 | PASS |
| 带图商品创建与审核发布 | 图片商品创建 201；审核前内容 404，审核后稳定入口 302 | PASS |
| 固定版本内容 | 302 URL 包含 COS `versionId`；签名 GET 200 | PASS |
| 更新替换 | 旧图片失效；新图片审核前 404、通过后 302/GET 200 | PASS |
| 拒绝、删除和后台下架 | 可见引用撤销，稳定内容入口均返回 404 | PASS |
| 已售图片规则 | 已售商品图片保持可读；禁止新增收藏 | PASS |
| 创建请求可安全重试 | 首次 201；同键同体 200 + replay header；同键异体 409 | PASS |
| 后台审核与搜索 | 审核通过后 Meilisearch 命中商品 | PASS |
| 收藏状态由服务端驱动 | `PUT` 重放计数仍为 1；`DELETE` 可重放；已售新增返回 409 | PASS |
| 联系卖家不信任 URL 会话参数 | 重复联系返回稳定会话，viewer state 与会话列表一致 | PASS |
| 旧媒体兼容 | 临时旧字段/旧引用夹具的稳定入口 302，签名 GET 200；夹具已清理 | PASS |
| 腾讯 IM 历史/双向文字消息 | 腾讯云 REST A→B、B→A 发送成功，双方漫游历史均可见两条消息 | PASS |
| 小程序腾讯 IM SDK 代码集成 | 历史、订阅、发送、卸载清理均通过类型和生产构建 | CODE PASS |
| 微信开发者工具双账号实时 UI | 本轮未完成真实两端 GUI 操作 | PENDING |
| 二手图片私信 fail-closed | 图片按钮不选图、不上传、不发送，只显示尚未开放提示 | PASS |
| 迁移边界 | `backend/migrations` 无 diff | PASS |
| 全量自动化 | `npm run verify`、生产构建、静态路由检查 | PASS |

## 3. 自动化证据摘要

- `npm run verify`：领域边界 PASS、Admin/MiniApp 类型检查 PASS、后端全部 Go 包测试 PASS。
- Module 10/Trade 真实 MySQL 生命周期测试：审核可见性、固定版本、替换、拒绝/删除/下架、已售、并发和清理全部 PASS。
- `npm --workspace apps/miniapp run build:weapp`：Webpack 编译成功。
- `npm run check:miniapp-static-routes`：25 条路由 PASS。
- ULID 实码检查：长度 26、字符集符合 Crockford Base32、固定输入可重复验证。
- `git diff --check`：无空白错误。

## 4. 真实环境证据

本轮验收标记：`ui21b2c-final-20260715164835`。

- 隔离身份：卖家 `900000092`、管理员 `900000093`、买家 `900000094`。
- 资产：`01KXKAW98HXJF75N029BB5WDAH`、`01KXKAWCZR2JKX4QF38P490WED`、`01KXKAWEN2A4BG73J8Y7K34F0T`；商品：`143`、`144`。
- 创建幂等结果：`201 → replay 200 → conflict 409`。
- 固定版本读取、更新替换、拒绝/删除/下架、已售可见性、收藏/联系/viewer state 和 Meilisearch 均为 PASS。
- 旧媒体兼容夹具使用现有 Core 对象验证稳定入口 302 与签名 GET 200，随后已删除夹具，未创建或删除额外 COS 对象。
- 腾讯云真实 C2C 输出：`accountImport=PASS`、`sellerToBuyer=PASS`、`buyerToSeller=PASS`、`roamAsSeller=PASS`、`roamAsBuyer=PASS`。漫游查询使用官方 `Operator_Account`/`Peer_Account` 契约和 `SyncOtherMachine=1`。
- 腾讯云生产 `SDKAppID` 对 `account_delete` 返回 `71000`；本地 IM 映射已清理，云端账号删除能力限制记录在 Residue Report。

验收数据清理前后计数：

| 项目 | 清理前 | 清理后 |
| --- | ---: | ---: |
| users | 6 | 0 |
| products | 2 | 0 |
| productImages | 2 | 0 |
| favorites | 0 | 0 |
| conversations | 1 | 0 |
| adminRoles | 0 | 0 |
| legacyFixtures | 0 | 0 |
| assets | 4 | 4 |
| visibleRefs | 0 | 0 |

4 个无可见引用的新鲜媒体资产按既定策略交由标准 GC，不做直接 COS/数据库删除。

## 5. 运行态

- MySQL `3306`：PID `8024`。
- Meilisearch `7700`：PID `5408`，health 为 `available`。
- 当前分支后端 `8080`：PID `15064`，`/healthz` 返回 200。
- 后端二进制与运行日志位于 `D:\codex-ui21b2c-runtime`；以上服务按要求保持运行，供继续人工复验。

## 6. 最终判定

本模块达到 **核心业务与真实基础设施验收 PASS**。带图发布主路径及完整媒体生命周期已消除原集成阻断；剩余仅为微信开发者工具双账号 GUI 人工复核和标准 GC/云端租户保留期事项，不影响本次代码收口。
