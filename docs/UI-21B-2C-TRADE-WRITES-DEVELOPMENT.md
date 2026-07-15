# UI-21b-2c 二手真实写入闭环 Development Report

日期：2026-07-15
分支：`feat/ui21b2c-trade-writes`
当前实现提交：`226da665`
状态：**Development Complete**

## 1. 开发结论

UI-21b-2c 已完成前端真实写入闭环与 Module 10 交易媒体后端适配。二手商品图片现在覆盖创建、审核发布、固定版本读取、更新替换、审核拒绝、删除撤销、后台下架和已售保留访问；交易前端 API、旧媒体访问与审核安全保持兼容，未新增或修改数据库迁移。

前端闭环保持原有路由与页面结构：

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

## 3. Module 10 交易媒体实现

- 服务启动时装配 Module 10 公共内容解析器、旧媒体公共读取适配、引用绑定服务和交易混合媒体网关。
- Trade 商品服务与交易写 outbox 使用混合网关；社区、用户和其他旧业务继续使用原旧媒体服务，避免扩大兼容面。
- 新资产沿用 Module 10 审核投影与引用约束；旧媒体仍按原字段和旧引用坐标解析，不改变既有前端 URL 契约。
- 稳定内容入口只在引用有效且审核允许时返回 302，并固定到 COS `versionId`，避免同 key 后续覆盖改变已审核内容。
- 图片替换先建立新引用，审核通过后旧引用失效；拒绝、删除和后台下架均撤销可见引用；商品标记已售不会误删已审核图片。
- 修正 `PublishBinding`：发布时忽略历史已撤销引用，仅发布当前引用集合；全部引用均撤销时返回 not found，防止旧图片被重新激活。

## 4. 变更边界

- 保持现有小程序页面路由、请求 URL、响应模型和旧媒体读取兼容。
- 不绕过媒体审核，不直接把 pending/rejected 资产暴露给商品内容入口。
- 后端改动限定在交易媒体适配、启动装配和相应测试；支付、配送和旧外卖逻辑未改。
- 不新增迁移，不修改现有迁移文件。
- 不直接删除本轮产生的新鲜 COS/媒体对象；无可见引用的对象交由标准保留期和 GC 处理。

## 5. 自动化结果

| 检查 | 结果 |
| --- | --- |
| ULID 长度、字符集和差异性实码检查 | PASS |
| `npm run verify` | PASS |
| `npm run check:boundaries` | PASS |
| Admin + MiniApp `typecheck` | PASS |
| 全量后端 `go test ./...` | PASS |
| Module 10/Trade MySQL 生命周期与并发测试 | PASS |
| `npm --workspace apps/miniapp run build:weapp` | PASS |
| `npm run check:miniapp-static-routes` | PASS，25 条路由 |
| `git diff --check` | PASS |
| `backend/migrations` 变更 | 无 |

本机 C 盘空间不足时，全量 Go 测试使用 `D:\codex-go-tmp\ui21b2c` 和 `D:\codex-go-cache\ui21b2c`，测试本身全部通过。小程序构建仅报告既有 Sass `@import` 弃用提示，以及腾讯 IM SDK 进入二手私信页后该页面包体为 724 KiB 的性能提示；均未导致构建失败。

## 6. 真实环境联调结论

2026-07-15 已启动本机 MySQL、Meilisearch 与当前分支后端，并完成真实 HTTP、COS、Meilisearch、旧媒体兼容和腾讯云 IM 联调。真实图片链路已经覆盖媒体票据、COS PUT、complete、审核、带图商品创建、固定版本读取、替换、拒绝、删除、后台下架及已售访问，结果全部通过。

创建首次返回 201，同键同体重放返回 200 且带 replay header，同键异体返回 409；审核通过后 Meilisearch 命中；收藏、联系卖家、viewer state 和腾讯云双向文字消息也均通过。详细证据见 Acceptance 与 Residue 报告。

## 7. 开发状态

实现、回归和真实核心链路验收均已完成。微信开发者工具双账号 GUI 操作未在本轮自动化中执行，属于人工 UI 复核项，不是后端或数据契约阻断。
