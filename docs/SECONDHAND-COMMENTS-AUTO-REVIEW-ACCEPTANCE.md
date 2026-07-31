# 二手留言与自动审核验收记录

日期：2026-07-30

## 已实现

- 商品详情页公开留言与一级回复，回复对象由服务端校验并推导，卖家身份由服务端生成。
- 商品留言创建、回复均使用 `clientRequestId` 与 `Idempotency-Key`，写入留言和 Outbox 保持同一事务。
- 留言通过审核后，新留言通知卖家；回复通知被回复用户；通知携带商品和留言坐标。
- 商品留言、帖子和商品接入混合审核：微信明确通过自动批准，明确风险自动拒绝，可疑、未配置、超时或异常保留为人工审核。
- 自动决定记录 `decision_source=automatic`；管理员可填写原因进行人工改判，记录 `manual_override`。
- 商品卖家可在详情页标记预订、标记售出、重新上架；状态转换仍由服务端校验所有权和当前状态。
- 收藏按钮使用空心/金色实心星；已售商品禁止新增收藏、新建私信和新留言，但允许取消已有收藏。
- 已售、下架或软删除商品的已批准留言可由历史留言接口读取；不允许继续新增留言。
- 通知进入商品留言时会继续分页查找目标留言并定位；后端数值 `0` 不会被前端误当成有效回复 ID。

## 自动化验证

| 检查 | 结果 |
| --- | --- |
| `go test ./internal/module/admin/review ./internal/module/trade ./internal/module/community ./cmd/server ./migrations -count=1` | PASS |
| `npm.cmd --prefix apps/admin run typecheck` | PASS |
| `npm.cmd --prefix apps/miniapp run typecheck` | PASS |
| `node scripts/check-secondhand-comments-contract.cjs` | PASS |
| `npm.cmd run check:miniapp-capsule-layout` | PASS |
| `npm.cmd run check:miniapp-static-routes` | PASS |
| `npm.cmd run check:miniapp-business-closure` | PASS |
| `npm.cmd run check:miniapp-regression-fixes` | PASS |
| `npm.cmd run check:miniapp-publish-contract` | PASS |
| `npm.cmd --prefix apps/admin run build` | PASS |
| `npm.cmd --prefix apps/miniapp run build:weapp` | PASS，保留项目已有 CSS 顺序和 `vendors.js` 体积警告 |
| `git diff --check` | PASS |

## 共享环境说明

`go test ./... -count=1` 仍会受到共享 MySQL 中既有测试数据、既有 CHECK 约束和媒体消费记录影响，失败集中在旧的 Admin/Media/MiniappFlow 集成测试，不属于本次留言与自动审核聚焦回归。不能据此把本次聚焦 PASS 扩大解释为全仓库共享环境 PASS。

## 生产部署记录

- 部署编号：`20260730_073151-ef146eb28e`。
- 已在生产库执行 migration 036，商品留言表、通知商品坐标、通知留言坐标和审核决定来源共 4 项结构检查通过。
- 后端发布目录：`/opt/campus-circle/releases/20260730_073151-ef146eb28e-comments-auto-review`。
- 管理后台发布目录：`/var/www/campus-admin/releases/20260730_073151-ef146eb28e`。
- 回滚备份：`/opt/cutover-backups/comments-auto-review-20260730_073151-ef146eb28e`，包含数据库备份、旧后端、旧管理后台和校验文件。
- `campus-api`、`nginx`、`mysql`、`campus-meilisearch` 服务正常；公开 `/healthz` 返回正常。
- 公开商品列表可读；商品留言路由存在，未登录访问返回 `401`，未出现 `404` 或 `500`。
- 微信小程序生产包已上传为开发版本 `0.2.0`，描述为“体验版：二手留言回复、自动审核与星标收藏”。

## 仍需完成的验收

1. 在微信公众平台的版本管理页面，将开发版本 `0.2.0` 设为体验版。
2. 使用两个真实微信账号、iOS 与 Android 各一台真机验证：
   - 买家留言，卖家收到通知并回复。
   - 买家收到回复通知，点击后定位到目标留言。
   - 自动通过、自动拒绝、转人工三条审核路径。
   - 管理后台人工改判并核对原因和来源。
   - 商品售出后历史留言可读，新留言、新收藏和新联系被禁止。
3. 关闭并重新进入小程序，确认留言、回复、通知定位、商品状态和星标收藏均持久化。

在以上三项完成前，本功能状态为“代码、生产构建与生产部署通过，体验版设置和双真机业务验收待完成”，不能标记为正式上线。
