# UI-21b-2a Write Contract Hardening Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 在不改变既有读接口、旧 toggle/create/contact 兼容行为、前端静态壳子的前提下，为社区 B 与二手 C 补齐可重试安全写契约：显式目标态写接口、viewer-state、创建幂等、写 outbox、Media/Review 幂等保障、真实 MySQL 并发验收。

**Architecture:** 保持 B/C 各自域内所有权：community/trade 只通过窄接口调用 media/review 的幂等能力，不直接跨域查表；旧接口保留，新接口增量；迁移 030 增加幂等字段、viewer 索引、域内 outbox；后台 worker 处理外部副作用，失败可重试，核心实体写入与 outbox 同事务。

**Tech Stack:** Go + Gin + GORM + MySQL 8.4 + existing media/review services + existing npm verification/build pipeline.

---

## 0. 当前基线与硬门禁

- [ ] 在主仓库目录 `C:\Users\33141\Documents\New project` 执行并贴出：
  - [ ] `git log --oneline -1 master`
  - [ ] `git tag --list ui21b1-community-trade-readonly-baseline ui21b2a-write-contract-baseline`
  - [ ] `rg "db\.AutoMigrate\(" backend/cmd/server`
  - [ ] `git status`
- [ ] 确认当前 master 含 `docs(UI-21b-2a): 固化安全写契约设计`，且 `ui21b2a-write-contract-baseline` 尚未存在。
- [ ] 确认 `C:` 可用空间达到实施门槛：至少 8GB。
  - 当前已知风险：`C:` 约 0.7GB 可用，禁止直接进入安装、完整构建、真实验收。
  - 若不足 8GB：停止实现，只做清理建议或等待 owner 清理，不启动完整开发。
- [ ] 不触碰保护 worktree：`C:\Users\33141\Documents\p0-identity-switch`。

## 1. 开分支与范围锁定

- [ ] 磁盘空间达标后，从 master 新建独立 worktree：
  - [ ] `git worktree add ../ui21b2a-write-contract -b feat/ui21b2a-write-contract master`
- [ ] 只允许改动以下范围：
  - [ ] `backend/internal/module/community/**`
  - [ ] `backend/internal/module/trade/**`
  - [ ] `backend/internal/module/media/**` 中幂等 Ensure 能力的最小增量
  - [ ] `backend/internal/module/admin/review/**` 中幂等 Ensure 能力的最小增量
  - [ ] `backend/migrations/030_*.sql`
  - [ ] `backend/migrations/rollback/030_*.sql`
  - [ ] `backend/cmd/server/main.go` 中 outbox worker 接线的最小增量（如需要）
  - [ ] 测试与报告 `docs/**`
- [ ] 明确禁止：
  - [ ] 不改前端 UI 静态壳子。
  - [ ] 不改 Delivery、Takeout、支付、骑手钱包、admin funds 逻辑。
  - [ ] 不删除旧 endpoint，不改变旧 endpoint 响应语义。
  - [ ] 不跨域直接查 media/review 表。

## 2. 文件级复核

- [ ] 复核 Community 当前写接口：
  - [ ] `backend/internal/module/community/handler.go`
  - [ ] `backend/internal/module/community/service.go`
  - [ ] `backend/internal/module/community/repository.go`
  - [ ] `backend/internal/module/community/model.go`
- [ ] 复核 Trade 当前写接口：
  - [ ] `backend/internal/module/trade/handler.go`
  - [ ] `backend/internal/module/trade/service.go`
  - [ ] `backend/internal/module/trade/repository.go`
  - [ ] `backend/internal/module/trade/model.go`
- [ ] 复核 Media 当前 commit/publish/display 能力：
  - [ ] `backend/internal/module/media/service.go`
  - [ ] `backend/internal/module/media/repository.go`
  - [ ] `backend/internal/module/media/model.go`
- [ ] 复核 Review 当前 submit/decision 能力：
  - [ ] `backend/internal/module/admin/review/service.go`
  - [ ] `backend/internal/module/admin/review/repository.go`
  - [ ] `backend/internal/module/admin/review/model.go`
- [ ] 输出开发报告中的“文件级复核结论”。

## 3. 迁移 030：幂等字段、viewer 索引、域内 outbox

- [ ] 新增 `backend/migrations/030_community_trade_write_contract.sql`。
- [ ] 新增 `backend/migrations/rollback/030_community_trade_write_contract.sql`。
- [ ] Community 表增量：
  - [ ] posts 增加 `client_request_id CHAR(26) NULL`。
  - [ ] posts 增加 `create_request_hash BINARY(32) NULL`。
  - [ ] comments 增加 `client_request_id CHAR(26) NULL`。
  - [ ] comments 增加 `create_request_hash BINARY(32) NULL`。
  - [ ] 增加 owner + client_request_id 唯一索引。
  - [ ] 增加 client_request_id/create_request_hash 成对为空或成对非空 CHECK。
  - [ ] 增加 viewer 方向索引：
    - [ ] `idx_post_likes_user_post(user_id, post_id)`
    - [ ] `idx_post_favorites_user_post(user_id, post_id)`
- [ ] Trade 表增量：
  - [ ] products 增加 `client_request_id CHAR(26) NULL`。
  - [ ] products 增加 `create_request_hash BINARY(32) NULL`。
  - [ ] 增加 owner + client_request_id 唯一索引。
  - [ ] 增加 client_request_id/create_request_hash 成对为空或成对非空 CHECK。
  - [ ] 增加 viewer 方向索引：
    - [ ] `idx_product_favorites_user_product(user_id, product_id)`
- [ ] 新增域内 outbox：
  - [ ] `community_write_outbox`
  - [ ] `trade_write_outbox`
  - [ ] 字段包含 public id、event type、payload、status、attempts、lease、next_attempt_at、last_error、row_version、created_at、updated_at。
- [ ] 真实 MySQL 8.4 验证：
  - [ ] apply
  - [ ] rollback
  - [ ] apply
  - [ ] 外键数为 0
  - [ ] CHECK 与唯一索引存在

## 4. 显式目标态互动接口与 viewer-state

- [ ] Community 新增只读 viewer-state：
  - [ ] `GET /api/v1/community/posts/:id/viewer-state`
  - [ ] 返回当前登录用户对帖子是否 liked/favorited、作者是否 followed、必要计数快照。
- [ ] Community 新增目标态接口：
  - [ ] `PUT /api/v1/community/posts/:id/like`
  - [ ] `DELETE /api/v1/community/posts/:id/like`
  - [ ] `PUT /api/v1/community/posts/:id/favorite`
  - [ ] `DELETE /api/v1/community/posts/:id/favorite`
  - [ ] `PUT /api/v1/community/users/:id/follow`
  - [ ] `DELETE /api/v1/community/users/:id/follow`
- [ ] Trade 新增只读 viewer-state：
  - [ ] `GET /api/v1/trade/products/:id/viewer-state`
- [ ] Trade 新增目标态接口：
  - [ ] `PUT /api/v1/trade/products/:id/favorite`
  - [ ] `DELETE /api/v1/trade/products/:id/favorite`
  - [ ] `PUT /api/v1/trade/products/:id/contact`
- [ ] 实现规则：
  - [ ] 目标态写入幂等；重复 PUT/DELETE 不产生额外计数、副作用或 outbox。
  - [ ] 旧 toggle endpoint 不删除、不改行为。
  - [ ] 非登录 401，越权或不可见目标按既有风格 404/403。
  - [ ] `contact` 使用 `DoNothing + SELECT`，重复调用不刷新 `updated_at`。

## 5. 创建幂等协议

- [ ] 现有创建接口兼容增强：
  - [ ] `POST /api/v1/community/posts`
  - [ ] `POST /api/v1/community/posts/:id/comments`
  - [ ] `POST /api/v1/trade/products`
- [ ] 请求 key 来源：
  - [ ] body `clientRequestId`
  - [ ] header `Idempotency-Key`
- [ ] 规则：
  - [ ] body/header 同时存在且不一致：400。
  - [ ] 无 key：完全保持旧行为。
  - [ ] key 非 ULID：400。
  - [ ] key + 同 payload 重放：返回原实体，HTTP 200，并带 `Idempotency-Replayed: true`。
  - [ ] key + 不同 payload：409。
- [ ] hash 规则：
  - [ ] 对规范化后的创建请求做 SHA-256，写入 `create_request_hash BINARY(32)`。
  - [ ] hash 只用于判断同 key 同 payload，不包含易漂移字段。

## 6. 创建事务与 outbox

- [ ] 实现 Reserve/Replay 查询逻辑：
  - [ ] 先按 owner + client_request_id 查询是否已有实体。
  - [ ] 若存在且 hash 相同，返回 replay。
  - [ ] 若存在且 hash 不同，返回冲突。
  - [ ] 若不存在，进入创建事务。
- [ ] 创建事务内：
  - [ ] 创建 entity。
  - [ ] 写入对应 domain outbox。
  - [ ] 不在事务内直接外呼 Media/Review。
- [ ] 事务提交后：
  - [ ] 可 inline 触发一次 outbox drain，但失败不得影响 create 返回。
  - [ ] 后台 worker 负责最终重试。

## 7. Media 幂等 Ensure 能力

- [ ] 在 `backend/internal/module/media/**` 增加最小公开幂等能力，例如：
  - [ ] `EnsureCommitted(...)`
  - [ ] 或与现有命名风格一致的等价方法。
- [ ] 要求：
  - [ ] 已 committed/active 时返回成功。
  - [ ] pending 状态可推进。
  - [ ] 非法状态 fail-closed。
  - [ ] 不改变现有 Commit/Resolve/Review 行为。
  - [ ] 不暴露 COS key、永久 URL 或绕过可见性规则。
- [ ] 增加 media 单元/集成测试覆盖重复调用与非法状态。

## 8. Review 幂等 Ensure 能力

- [ ] 在 `backend/internal/module/admin/review/**` 增加最小公开幂等能力，例如：
  - [ ] `EnsureSubmitted(...)`
  - [ ] 或与现有命名风格一致的等价方法。
- [ ] 要求：
  - [ ] 同 target 重复提交返回既有 task。
  - [ ] 不重复创建 review task。
  - [ ] 不改变现有 Submit/Decide 行为。
  - [ ] 不引入跨域直接表访问。
- [ ] 增加 review 测试覆盖重复调用与冲突。

## 9. Outbox worker 与失败重试

- [ ] 为 `community_write_outbox` 和 `trade_write_outbox` 实现独立 worker/drain 能力。
- [ ] worker 规则：
  - [ ] 使用 `FOR UPDATE SKIP LOCKED` 领取任务。
  - [ ] 使用 lease 字段避免重复处理。
  - [ ] 使用 `row_version` CAS 更新状态。
  - [ ] 指数退避写入 `next_attempt_at`。
  - [ ] 记录 `last_error`，不得吞掉错误。
  - [ ] URL/外部服务不可用时不阻断主创建流程。
- [ ] main.go 接线：
  - [ ] 仅做最小装配。
  - [ ] 不影响旧外卖封存门控、Delivery 装配和现有后台 worker。
- [ ] 测试：
  - [ ] worker 重复执行不重复 Media/Review 副作用。
  - [ ] 模拟 Media 成功 Review 失败后可重试。
  - [ ] 模拟 Review 成功 Media 重试不重复。

## 10. HTTP 错误语义与兼容性

- [ ] 新接口错误码：
  - [ ] 400：非法 key、body/header mismatch、请求格式错误。
  - [ ] 401：未登录。
  - [ ] 403：鉴权拒绝。
  - [ ] 404：目标不存在或不可见。
  - [ ] 409：幂等 key payload 冲突、CAS/并发冲突。
  - [ ] 422：业务规则拒绝。
  - [ ] 503/504：外部依赖不可用或超时（仅必要时暴露）。
- [ ] 旧接口兼容：
  - [ ] 旧 toggle 接口仍可用。
  - [ ] 旧 create 请求无 idempotency key 时仍按旧路径工作。
  - [ ] 旧 contact 行为不破坏。

## 11. 测试矩阵

- [ ] Migration：
  - [ ] 030 apply + rollback + apply。
  - [ ] 索引、CHECK、零外键验证。
- [ ] Repository/Service：
  - [ ] 目标态 like/favorite/follow set/unset 幂等。
  - [ ] viewer-state 正确。
  - [ ] contact 重复不 churn。
  - [ ] create 幂等 replay/conflict/no-key 兼容。
- [ ] HTTP + MySQL：
  - [ ] 20-way 并发同 key 创建帖子，只成功一个实体，其余 replay。
  - [ ] 20-way 并发同 key 创建评论，只成功一个实体，其余 replay。
  - [ ] 20-way 并发同 key 创建商品，只成功一个实体，其余 replay。
  - [ ] 同 key 不同 payload 409。
  - [ ] header/body mismatch 400。
  - [ ] 目标态互动重复点击无计数膨胀。
- [ ] Outbox：
  - [ ] Media/Review failure injection。
  - [ ] worker retry 后最终成功。
  - [ ] 重放不会重复提交 review task 或重复 commit media。
- [ ] Query plan：
  - [ ] viewer-state 查询使用新索引，提供 `EXPLAIN` 证据。
- [ ] Contact：
  - [ ] 如果真实 Tencent IM 可用，验证真实联系链路。
  - [ ] 如果真实 Tencent IM 凭证不可用，标记 `CONDITIONAL PASS`，不得夸大。

## 12. 全量验证

- [ ] 磁盘空间达标后执行：
  - [ ] `go test ./... -count=1`
  - [ ] `npm run check:boundaries`
  - [ ] `npm run verify`
  - [ ] 后端 build
  - [ ] miniapp/admin build 如授权要求涉及
- [ ] 验证 `rg "db\.AutoMigrate\(" backend/cmd/server` 仍为 0。
- [ ] 验证 negative scope：
  - [ ] Delivery 无 diff。
  - [ ] Takeout/旧外卖无业务 diff。
  - [ ] 支付/钱包/admin funds 无 diff。
  - [ ] 前端静态 UI 无 diff。

## 13. 报告与交付

- [ ] 新增验收报告到 `docs/`，内容包含：
  - [ ] Development
  - [ ] Acceptance
  - [ ] Acceptance Evidence
  - [ ] Migration 030 apply/rollback/apply
  - [ ] 20-way concurrency
  - [ ] Media/Review outbox retry
  - [ ] viewer-state EXPLAIN
  - [ ] Contact/Tencent IM 真实或 `CONDITIONAL PASS`
  - [ ] Boundary/Negative scope
  - [ ] Residue
- [ ] 完成后提交到 `feat/ui21b2a-write-contract`。
- [ ] worktree 干净，停止等 owner 验收。
- [ ] owner 在主仓库串行 ff 合并后打 tag：
  - [ ] `ui21b2a-write-contract-baseline`

## 14. 实施顺序建议

- [ ] 先做 migration + model/repository 幂等字段。
- [ ] 再做目标态接口和 viewer-state。
- [ ] 再做 create 幂等协议。
- [ ] 再做 Media/Review Ensure 能力。
- [ ] 再做 outbox worker。
- [ ] 最后做真实 MySQL 并发与 failure injection。

## 15. 暂停条件

- [ ] `C:` 可用空间低于 8GB。
- [ ] 发现 master/tag/status 与授权基线不符。
- [ ] 需要改变旧 endpoint 行为。
- [ ] 需要修改前端静态壳子。
- [ ] 需要跨域直接访问 media/review 表。
- [ ] 真实 Tencent IM 无法验证时，不阻塞整体，但必须在报告标注 `CONDITIONAL PASS`。
