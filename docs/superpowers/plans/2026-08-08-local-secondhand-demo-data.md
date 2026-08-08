# 本机二手商品演示数据 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 在本机数据库写入 8 个可被现有二手列表、分类、搜索和详情接口读取的演示商品，并提供可重复执行、可核验、可完整清理的命令。

**Architecture:** 新增一个独立 CommonJS 运维脚本读取 `backend/.env` 的本机 `MYSQL_DSN`，在事务内维护专属演示卖家、商品和审核记录。脚本拒绝生产环境和非回环地址，不调用线上 API、不上传图片、不执行迁移；清理操作仅按专属 `open_id` 与 `[本机预览]` 标记删除关联数据。另加静态契约检查，确保安全边界不会被后续修改破坏。

**Tech Stack:** Node.js CommonJS、MySQL CLI、MySQL 8、现有 Go API、Taro 小程序

---

## Task 1：实现本机演示数据命令

**Files:**
- Create: `scripts/local-secondhand-demo.cjs`
- Create: `scripts/check-local-secondhand-demo.cjs`
- Modify: `package.json`

- [ ] 解析 `MYSQL_DSN` 与 `APP_ENV`，只允许 `127.0.0.1`、`localhost`、`::1`，拒绝 `production`。
- [ ] 实现 `seed`：在单一事务中写入专属演示用户、8 个跨分类商品、`approved` 审核任务和搜索 upsert 队列。
- [ ] 实现 `status`：输出本机数据库目标、演示卖家和商品数量，不输出密码。
- [ ] 实现 `clear`：仅删除演示商品关联的留言、通知、收藏、图片、会话、搜索/写队列、审核任务、商品和空演示用户。
- [ ] 保证重复 `seed` 不产生重复商品，重复 `clear` 不报错。
- [ ] 在 `package.json` 增加 `demo:secondhand:seed`、`demo:secondhand:status`、`demo:secondhand:clear` 和静态检查命令。

## Task 2：验证安全边界与数据闭环

**Files:**
- Test: `scripts/check-local-secondhand-demo.cjs`
- Test: local MySQL tables

- [ ] 运行静态契约检查，确认无线上域名、无图片伪造、具备本机/生产双重拦截。
- [ ] 记录非演示商品数量，执行 `seed -> status -> clear -> seed`，确认清理可逆且最终保留 8 个预览商品。
- [ ] 再次核对非演示商品数量不变，确认 8 个商品覆盖数码、书籍资料、生活用品、代步和其他分类。

## Task 3：验证现有接口与小程序产物

**Files:**
- Test: `backend/cmd/server`
- Test: `apps/miniapp`

- [ ] 启动本地 Go API 并检查 `/healthz`。
- [ ] 调用公开二手列表接口，确认 8 个商品可见、状态为在售且无虚构图片。
- [ ] 抽查一个商品详情接口，确认标题、价格、卖家、分类和详情均来自数据库。
- [ ] 构建小程序，确认本次演示数据不引入前端静态假数据或编译回归。

## Task 4：提交与交付

**Files:**
- Commit: implementation and verification artifacts

- [ ] 仅提交本任务脚本、计划和 `package.json`，不纳入工作区已有的三个未跟踪计划文件。
- [ ] 向用户交付本机查看方式、清理命令、验证结果和仍需在微信开发者工具中手动刷新的边界。
