# UI-21b-2c 二手真实写入闭环 Acceptance Report

日期：2026-07-15
状态：**CODE PASS / REAL ENVIRONMENT PENDING**

## 1. 验收结论

UI-21b-2c 已通过代码、类型、构建、路由、领域边界和全量后端回归检查。实现没有使用 mock 数据或本地假消息替代真实写入。

由于验收时 `127.0.0.1:8080` 拒绝连接，真实登录、媒体上传、商品创建、双账号收藏和腾讯 IM 双端文字收发未能执行。因此本报告不把真实环境项标记为 PASS。

## 2. 验收矩阵

| 项目 | 证据 | 结果 |
| --- | --- | --- |
| 商品图片使用真实媒体上传 | 页面调用 `chooseAndUploadImages('product_image', ...)` 并保存 `assetId` | CODE PASS |
| 商品创建金额为整数分 | 十进制字符串显式转换并校验 `Number.isSafeInteger` | CODE PASS |
| 创建请求可安全重试 | 26 位 ULID 同时写入 `clientRequestId` 与 `Idempotency-Key`；网络/服务端失败保留键 | CODE PASS |
| 收藏状态由服务端驱动 | viewer state + 确定态 `PUT/DELETE` + 失败回读 | CODE PASS |
| 联系卖家不信任 URL 会话参数 | 详情页只传 `productId`，聊天页重新建立授权上下文 | CODE PASS |
| 腾讯 IM 历史文字消息 | 调用 `loadTencentMessages`，只显示 `TIMTextElem` | CODE PASS |
| 腾讯 IM 实时文字消息 | 调用 `subscribeTencentMessages`，卸载与失败时清理 | CODE PASS |
| 腾讯 IM 文字发送 | 调用 `sendTencentText`，失败保留草稿 | CODE PASS |
| 二手图片私信 fail-closed | 图片按钮不选图、不上传、不发送，只显示尚未开放提示 | PASS |
| 后端与迁移零改动 | diff 边界检查无 `backend/**` 或迁移文件 | PASS |
| 全量自动化 | `npm run verify`、生产构建、静态路由检查 | PASS |
| 真实媒体上传与商品落库 | 本机 API 未启动 | PENDING |
| 真实双账号收藏与联系 | 本机 API 未启动 | PENDING |
| 真实腾讯 IM 双端收发 | 无可用后端、登录态与 IM 会话环境 | PENDING |

## 3. 自动化证据摘要

- `npm run verify`：领域边界 PASS、Admin/MiniApp 类型检查 PASS、后端全部 Go 包测试 PASS。
- `npm --workspace apps/miniapp run build:weapp`：Webpack 编译成功。
- `npm run check:miniapp-static-routes`：25 条路由 PASS。
- ULID 实码检查：长度 26、字符集符合 Crockford Base32、固定输入可重复验证。
- `git diff --check`：无空白错误。

## 4. 真实环境阻断证据

- `npm run check:stack`：`connect ECONNREFUSED 127.0.0.1:8080`。
- `GET http://127.0.0.1:8080/health`：无法连接。
- `GET /api/v1/trade/products?...`：无法连接。
- 本机未发现 3306、7700、8080 监听进程。

## 5. 最终判定

本模块达到 **代码级验收通过**，但真实写入与真实腾讯 IM 验收仍待 owner 在完整服务栈、真实登录账号和腾讯 IM 配置可用时执行。完成真实环境矩阵前，不应把本模块标为完全上线 PASS。
