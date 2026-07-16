# UI-21b-2c 二手真实写入闭环 Acceptance Report

日期：2026-07-16
状态：**FUNCTIONAL RC PASS / PRODUCTION PACKAGE BLOCKED**

## 1. 验收结论

UI-21b-2c 已通过代码、类型、构建、路由、领域边界和全量后端回归检查。2026-07-15 使用隔离 mock 身份、真实 MySQL、真实 COS、真实 Meilisearch 和真实腾讯云 IM 配置完成核心链路验收；2026-07-16 在正确导入的 RC 小程序工程中补完真实买家 GUI 回归，并补验卖家身份 HTTP 行为。

Module 10 图片商品创建、审核发布、固定版本读取、更新替换、拒绝、删除撤销、后台下架、已售保留访问、旧媒体兼容、幂等创建、搜索、收藏、稳定联系会话和腾讯云文字消息全部通过。本轮同时修复了小程序将根相对媒体地址误当包内资源，以及卖家查看自己商品时触发 IM 自联系查询并返回 500 的问题。无数据库迁移变更，无审核绕过。

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
| 小程序相对媒体地址兼容 | API 根相对地址按 `API_BASE` origin 解析；旧绝对/CDN/本地协议地址保持不变 | PASS |
| 微信开发者工具真实买家 UI | 商品 145 图片显示；收藏取消/恢复；联系卖家；SDK 发送 `GUI-BUYER-145-RC` | PASS |
| 卖家查看自己商品 | 用户 900000097 请求 viewer-state 为 200，`contactedSeller=false`，无自联系会话 | PASS |
| 微信开发者工具卖家第二端/漫游补充复核 | 本轮未启动第二个 GUI 身份；`admin_getroammsg` 对新消息和已知旧会话均返回 0，不能作为通过或失败证据 | LIMITED |
| 正式上传包 HTTPS 配置 | 当前未注入 `TARO_APP_API_BASE`，生产构建会回退 `http://127.0.0.1:8080/api/v1`；须提供真实 HTTPS API 并配置微信合法域名 | BLOCKED |
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
- 2026-07-16 复跑：`go test ./...`、`go vet ./...`、领域边界、静态路由、全量类型检查和生产构建全部通过。
- 生产构建仅保留既有 Sass `@import` 弃用、聊天包体积和未拆异步 chunk 告警；`npm ci` 同时报告 36 个既有依赖审计项，本轮未执行可能扩大变更范围的自动升级。

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

## 5. 2026-07-16 微信开发者工具真实回归

- RC 工程：`C:\tmp\ui21b2c-trade-writes\apps\miniapp`，分支 `feat/ui21b2c-trade-writes`。
- 商品：`145`；媒体资产：`01KXM24Q8MGE9PJXG2QAJB6PNZ`；买家：`900000099`；卖家：`900000097`。
- 商品列表和详情均显示真实图片，不再降级为“卖家未上传图片”；点击封面成功打开 `1 / 1` 原图预览。稳定内容入口返回 302，COS 签名地址包含非空 `versionId`，签名 GET 返回 200、`image/jpeg`、127886 bytes。
- 买家在真实 UI 中完成“取消收藏 → 再收藏”，最终状态为“已收藏”；随后通过“联系卖家 → 发起私聊”进入商品 145 会话。
- 腾讯 IM SDK 发送 `GUI-BUYER-145-RC` 后消息气泡显示。页面仅在 `chat.sendMessage` Promise 成功后追加该气泡，因此该结果不是本地乐观占位。
- 卖家真实 HTTP 回归：viewer-state 返回 200，`favorited=false`、`contactedSeller=false`；会话列表中商品 145 的 `buyerId=sellerId=900000097` 自联系关系为 0。
- 腾讯 `admin_getroammsg` 在当前环境对新消息及一组已知旧消息会话均返回 0 条，故本轮不把该接口结果计为 GUI 消息通过证据，也不据此判定 SDK 发送失败；2026-07-15 独立 REST 双向消息验收结论保持不变。

## 6. 运行态

- MySQL `3306`：PID `8024`。
- Meilisearch `7700`：PID `5408`，health 为 `available`。
- 当前分支后端 `8080`：PID `32988`，`/healthz` 返回 200。
- 后端二进制与运行日志位于 `D:\codex-ui21b2c-runtime`；以上服务按要求保持运行，供继续人工复验。

## 7. 最终判定

本模块达到 **功能 RC 验收 PASS**，两个回归修复可合并。带图发布主路径、完整媒体生命周期、真实买家收藏/联系、买家腾讯 IM SDK 发送调用以及卖家自查看安全路径均已收口；卖家第二端可见性与腾讯漫游接口仍是补充复核项。

当前产物尚不能认定为可直接上传的生产包：未提供真实 HTTPS `TARO_APP_API_BASE` 时会回退本地 HTTP 地址，微信开发者工具也明确提示图片 HTTP 链接不受支持。正式上传前必须注入并验证生产 HTTPS API 地址、微信 request/download 合法域名和对应证书；该外部配置不阻断本次代码合并，但阻断“可直接上线”结论。
