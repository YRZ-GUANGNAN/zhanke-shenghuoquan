# UI-21b-2c 二手真实写入闭环 Development Report

日期：2026-07-15
分支：`feat/ui21b2c-trade-writes`
实施基线：`fccf86ca`
范围：仅小程序前端与本模块交付文档；后端、迁移和领域逻辑保持冻结。

## 1. 开发结论

UI-21b-2c 的代码实施已完成。新版二手发布、详情和私信页面保留原有路由与视觉结构，原静态写入提示已替换为现有真实前端服务调用：

- 发布二手使用 `product_image` 媒体上传链路、审核状态确认和 `POST /trade/products`；
- 收藏先读取 viewer state，再使用确定态 `PUT/DELETE /trade/products/:id/favorite`，失败后回读服务端事实；
- 联系卖家使用稳定 `PUT /trade/products/:id/contact`，只把 `productId` 传给新版私信页；
- 新版私信页重新读取商品和会话授权，接入腾讯 IM 历史文字消息、实时订阅与文字发送；
- 二手私信图片入口保持 fail-closed，只提示“二手私信图片发送尚未开放”。

## 2. 主要实现

### 2.1 发布二手

- 新增无业务域依赖的 26 位 Crockford Base32 请求 ID 生成器。
- `createProduct` 同时发送 body `clientRequestId` 与同值 `Idempotency-Key`。
- 图片状态保存真实 `assetId` 和本地预览路径；提交前等待媒体进入可使用状态。
- 元金额通过十进制字符串精确转换为整数分，拒绝超过两位小数或非正金额。
- 网络错误及 5xx/503/504 保留本次幂等键；表单变化、成功或确定的 4xx 业务结果清除旧键。
- 提交失败保留表单内容，不追加本地假商品。

### 2.2 收藏与联系卖家

- 详情加载时读取真实 viewer state；收藏操作以服务端确定态结果驱动 UI。
- 请求结果不明确时再次读取 viewer state，不在前端猜测成功。
- 已售商品仍允许取消已有收藏，但禁止新增收藏和新建联系关系。
- 联系卖家成功后进入 `/pages/secondhand-chat/index?productId=<id>`；会话 ID、对端用户 ID 不取自 URL。

### 2.3 腾讯 IM 文字私信

- 页面按 `productId` 重新调用商品读取和联系卖家服务。
- 先建立消息订阅，再拉取历史消息；按腾讯消息 ID 去重并按时间排序。
- 只渲染 `TIMTextElem`，未知或图片消息不生成替代内容。
- 发送成功后才清空草稿；失败保留草稿并提示可重试错误。
- 页面卸载、初始化失败或重试时清理消息订阅。

## 3. 必要的基线兼容修正

全量类型检查发现基线中的 `pages/home/index.tsx` 仍以旧的三个位置参数调用已经对象化的 `getCommunityPostsPage`。本次将其等价改为 `{ page, sort, pageSize }`，不改变社区读取行为、页面结构或视觉，仅关闭既有 TypeScript 阻断。

## 4. 变更边界

生产代码改动限定在：

- `apps/miniapp/src/services/client-request-id.ts`
- `apps/miniapp/src/services/c-trade.ts`
- `apps/miniapp/src/pages/publish-secondhand/**`
- `apps/miniapp/src/pages/secondhand-detail/**`
- `apps/miniapp/src/pages/secondhand-chat/**`
- `apps/miniapp/src/pages/home/index.tsx` 的一行兼容修正

`backend/**`、数据库迁移、支付、配送、媒体审核协议和旧外卖代码均无 diff。

## 5. 自动化结果

| 检查 | 结果 |
| --- | --- |
| ULID 长度、字符集和差异性实码检查 | PASS |
| `npm run verify` | PASS |
| `npm run check:boundaries` | PASS |
| Admin + MiniApp `typecheck` | PASS |
| 全量后端 `go test ./...` | PASS |
| `npm --workspace apps/miniapp run build:weapp` | PASS |
| `npm run check:miniapp-static-routes` | PASS，25 条路由 |

构建仅报告既有 Sass `@import` 弃用提示，以及腾讯 IM SDK 进入二手私信页后该页面包体为 724 KiB 的性能提示；均未导致构建失败。

## 6. 真实环境联调结论

2026-07-15 已启动本机 MySQL、Meilisearch v1.46.0 与当前分支后端，并完成真实 HTTP、COS、Meilisearch 和腾讯云 IM 联调。除带图商品创建外，以下真实链路已通过：

- 无图商品创建返回 201；同键同体重放返回 200 且带 `Idempotency-Replayed: true`；同键异体返回 409；
- 审核通过后商品可被 Meilisearch 检索；
- 收藏 `PUT` 重放不重复计数，取消收藏可重放，已售商品新增收藏返回 409；
- 联系卖家重复调用返回稳定会话，viewer state、IM 凭证和会话列表一致；
- 通过腾讯云官方 `openim/sendmsg` 与 `openim/admin_getroammsg` 接口完成 A→B、B→A 文字消息及双方漫游历史校验。

真实图片链路已完成媒体票据申请、COS PUT、complete 和本地审核投影，但将该 `assetId` 绑定到 `POST /trade/products` 时返回 500 `invalid media asset`。定位结果是现有 Module 10 媒体资产与冻结的交易媒体网关使用了两套字段/状态约定：新版链路使用 `asset_public_id`、`purpose=product` 和 `technical_status=final_ready`/审核投影，交易服务仍通过旧媒体包校验 `public_id`、`purpose=product_image` 和旧 `approved` 状态。因此前端无法单独消除该阻断。

本分支没有为通过验收而修改后端、迁移或放宽媒体审核。详细证据见 Acceptance 与 Residue 报告。

## 7. 开发状态

前端代码状态为 **Development Complete**；跨模块真实验收状态为 **Integration Blocked**。在 owner 明确扩大后端范围、补齐媒体适配并复验之前，本模块不应标记为完全上线 PASS。
