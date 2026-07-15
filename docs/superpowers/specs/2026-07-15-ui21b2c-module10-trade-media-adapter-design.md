# UI-21b-2c Module 10 交易媒体适配设计

日期：2026-07-15

分支：`feat/ui21b2c-trade-writes`

状态：设计已确认，等待书面复核

## 1. 背景

UI-21b-2c 前端已使用 Module 10 完成媒体票据申请、COS 上传、complete、固定版本落盘和审核投影。真实验收确认资产达到 `technical_status=final_ready`，且对应 `media_review_projections.status=approved`；但交易服务仍通过 legacy `internal/pkg/media.Gateway` 按 `public_id`、`owner_user_id`、`purpose=product_image` 和旧 `status=approved` 校验资产，导致带图 `POST /trade/products` 返回 500 `invalid media asset`。

现有数据库同时保留 legacy 列与 Module 10 列，并已有 `media_asset_references`、审核投影、固定版本显示服务和交易 write outbox。本设计在不增加迁移、不改变前端 API、不放宽审核的前提下，为交易域增加一层明确的混合媒体适配。

## 2. 目标

- 让 Module 10 `purpose=product` 资产能够用于二手商品创建。
- 覆盖创建、审核发布、更新替换、用户删除、审核驳回、管理员下架和已售展示的完整生命周期。
- 保持现有商品请求与响应结构，包括 `imageAssetIds` 和 `images[].url`。
- 使用固定 `final_version_id` 生成短期 COS 签名 URL。
- 保持 legacy 已有媒体读取和网关回退能力。
- 保持预留、提交、outbox 重放、发布和撤销的幂等及并发安全。
- 不修改 `backend/migrations/**`，不增加表、列、触发器或双向字段同步。

## 3. 非目标

- 不改造社区、外卖、配送或通用 IM 媒体链路。
- 不恢复或扩展 legacy 上传协议。
- 不让前端选择 Module 10 与 legacy 媒体实现。
- 不把 `media_assets.review_status` 当作 Module 10 审核事实；审核事实只取 `media_review_projections`。
- 不直接删除本轮真实验收留下的 COS 对象，也不绕过标准 GC 保留期。
- 不改变商品页面结构、视觉或路由。

## 4. 方案选择

采用混合适配器，而不是修改 legacy 仓库进行双写，也不使用数据库触发器同步新旧字段。

双写会继续制造两套可变事实源；触发器会把审核语义隐藏在数据库中，并扩大迁移和回滚风险。混合适配器可以让 Module 10 字段、审核投影和引用表保持权威，同时将 legacy 行为隔离为明确回退。

## 5. 架构与组件边界

### 5.1 Core Media 引用绑定服务

在 `backend/internal/module/media` 增加通用 `ReferenceBindingService`。它只依赖 Media Domain repository，不导入或查询交易模型。

职责：

- 按稳定顺序锁定资产，避免多资产并发预留产生死锁；
- 验证 `owner_id`、`purpose`、`storage_stage`、`technical_status`、固定 key/version、删除状态和审核投影；
- 复用现有 `media_assets.reservation_token` 与 `reservation_expires_at` 作为跨交易预留围栏；
- 创建、精确重放、发布和撤销 `media_asset_references`；
- 在引用状态变化后重建 `active_ref_count`；
- 提供按 `asset_public_id` 解析可见固定版本对象所需的只读能力。

该服务使用 generic `Purpose`、reference type、reference ID 和 slot，不包含 product 特有模型。

### 5.2 Trade 混合适配器

在 `backend/internal/module/trade` 增加 `HybridMediaGateway`，实现现有 legacy `media.Gateway`，并额外实现交易商品引用发布能力。

- 所有输入资产都具有完整 Module 10 事实和审核投影时，走 Core Media 绑定服务；
- 所有输入资产都属于 legacy 回退集合时，委托现有 legacy gateway；
- Module 10 与 legacy 资产混合提交时 fail-closed；
- Module 10 reservation token 使用 `m10_` 加 26 位 ULID，总长度 30，能够在进程重启和 outbox 重放后稳定选择实现；
- legacy token 与行为保持不变；
- adapter 将 legacy `product_image` 语义映射为 Core Media `product`，但不回写或同步 purpose 列。

更新路径使用带商品坐标的扩展预留能力：同一商品继续保留的资产按 keep 处理，未绑定新资产进入 reservation，已经绑定到其他父对象的资产拒绝复用。

### 5.3 固定版本内容解析

Core Media `MediaDisplayService` 增加按 `asset_public_id` 解析公开内容的能力。解析必须同时满足：

- `storage_stage=final`；
- `technical_status=final_ready`；
- final key 与 `final_version_id` 都存在；
- 审核投影为 approved；
- 至少一个引用为 active、父对象为 visible；
- 引用没有 revocation fence；
- 资产未请求删除且未被删除。

现有公开地址 `/api/v1/media/assets/:id/content` 保持不变。legacy service 先按原逻辑查找；只有 legacy 返回 not found 时，才调用 Core Media resolver。Core Media resolver 使用固定 version 生成短期签名 URL，其他错误不触发宽松回退。

## 6. 数据流

### 6.1 创建

1. 小程序完成 Module 10 上传并等待状态接口返回可使用。
2. `HybridMediaGateway.Reserve` 校验数量、重复 ID 和资产集合类型。
3. Core Media repository 按 asset public ID 排序，以 `SELECT ... FOR UPDATE` 锁定行。
4. 每个 Module 10 资产必须满足：所有者一致、purpose 为 product、final/final_ready、固定版本完整、审核投影 approved、未删除、没有有效的其他 reservation，且没有绑定到其他父对象的非终止引用。
5. 所有校验在同一事务中通过后，写入同一个带期限的 `m10_` token；任一资产失败则整个预留回滚。
6. adapter 返回 legacy `Reservation` DTO，保留有序内部 asset ID，并生成原有稳定内容 URL。
7. `CreateKeyed` 在交易事务中写入商品、`product_images` 和 `trade_write_outbox`。
8. outbox 的 `EnsureCommitted` 原子创建 `pending + hidden` 的 Module 10 引用并清除 reservation，然后提交商品审核。
9. 非 keyed 创建沿用同步 Commit，但创建的引用同样保持 pending/hidden，直到商品审核通过。

同一 client request ID、请求哈希、reservation token、商品 ID 和有序资产集合的重放返回成功；坐标或顺序不一致时拒绝。

### 6.2 审核发布

商品处于 submitted 时，图片引用不可见。商品审核通过后：

1. 交易域先落商品 approved 状态；在此期间商品最多出现短暂的无图状态，不会提前泄露图片；
2. adapter 在事务中验证该商品的精确引用集合；
3. pending 引用转为 active，parent visibility 从 hidden 转为 visible；
4. 已是 exact active/visible 集合的重放直接成功；
5. 重建相关资产的 `active_ref_count`。

发布失败不得把未确认引用标为 visible。失败可重试，商品图片内容地址继续返回 404。

### 6.3 更新替换

1. 读取当前商品和现有图片集合；已属于同一商品且仍在输入中的资产进入 keep 集合。
2. 对新增资产执行 Module 10 或 legacy 预留；属于其他商品的资产拒绝。
3. 在修改商品前先将即将淘汰的 Core Media 引用隐藏；隐藏失败则取消新 reservation，并保留旧商品事实。
4. 更新商品和 `product_images`，商品重新进入 submitted。
5. 提交新增引用为 pending/hidden，撤销不再保留的旧引用。
6. 商品复审通过后发布新的精确引用集合。

若交易更新成功但后续 media commit 暂时失败，商品保持 submitted 且不可公开；同一 token 的 `EnsureCommitted` 可以安全重试。

### 6.4 删除、驳回和下架

用户删除、商品审核驳回和管理员下架都先隐藏/撤销媒体引用，再修改父对象公开状态。这样跨域操作失败时优先保证内容不可见，最坏结果是暂时缺图，而不是已隐藏商品的图片仍可公开访问。

- pending 引用可直接转为 revoked；
- active 引用经 revoking 转为 revoked；
- 撤销时写入 revocation fence 并将 parent visibility 设为 hidden；
- 相同商品和 keep 集合的重复撤销返回成功；
- 已售商品继续保留 active/visible 引用；
- GC 仍以终止引用事实和标准保留期判断删除资格。

### 6.5 Legacy 兼容

- 已有 legacy 商品图片继续使用原 content resolver；
- legacy 资产集合继续委托原 gateway；
- Core Media resolver 只在 legacy not found 时启用；
- 混合资产集合拒绝，避免一个 reservation 跨两套提交语义；
- 不修改 legacy 行，不把 Module 10 状态双写到 legacy `status/reference_*` 列。

## 7. 错误与安全语义

- 非法或重复 asset ID、数量超限：请求错误，不执行预留。
- 资产不存在、非本人、purpose 不符、固定版本不完整、未审核、未 final_ready、已删除或混合实现：统一映射为 409 `invalid media asset`，不泄露归属和审核细节。
- 已被其他请求预留或绑定：409。
- 相同 token 和精确坐标重放：成功。
- 相同 token 但商品、slot、asset 或顺序不一致：永久失败，不修改引用。
- 数据库暂时错误、outbox 租约冲突：保持可重试状态；商品不公开。
- outbox 达到既有重试上限：deadletter，保留定位错误，商品仍不可见。
- public content 不可见、未找到、已撤销或父对象隐藏：统一 404。
- Cancel 只清除 owner 和 token 均匹配且尚未提交的 reservation。
- 所有错误响应不得包含 bucket、object key、version ID、COS URL、凭证、审核任务 ID 或他人 owner ID。

## 8. 并发与幂等

- 多资产锁顺序固定为 `asset_public_id ASC`。
- reservation 校验和写入位于同一数据库事务。
- 过期 reservation 可以被新请求接管；未过期 token 只能由原 owner 使用。
- commit 以 reservation token、owner、有序 asset 集合和父对象坐标为精确键。
- reference 创建依赖现有唯一索引，并在冲突后回读 exact set 判断重放。
- publish 和 revoke 都以引用行锁及 row version 保证并发安全。
- `active_ref_count` 只是重建缓存；GC 和可见性仍以引用行事实为准。

## 9. 测试设计

### 9.1 单元测试

- Module 10/legacy/mixed 分类；
- owner、purpose、storage stage、technical status、final identity、审核投影和删除状态矩阵；
- 重复 ID、顺序、数量和 token 前缀；
- exact commit/replay 与 mismatched replay；
- cancel 不越权；
- publish/revoke 状态机；
- legacy not-found 才回退 Core Media resolver；
- Core Media DTO 和错误不泄露存储信息。

### 9.2 MySQL 集成与并发测试

- 两个 owner/request 并发预留同一资产，只有一个成功；
- 多资产反向输入不死锁；
- reservation 过期接管；
- commit 创建有序 pending/hidden 引用；
- commit、publish、revoke 重放不增加行或 row version；
- 更新 keep/new/remove 集合正确；
- `active_ref_count` 与 active 引用数一致；
- 测试结束资产、引用、商品、outbox 和临时用户残留为 0。

### 9.3 HTTP 生命周期测试

- 带图创建 201；同键同体 200 replay；同键异体 409；
- 未审核商品及图片不可公开；
- 商品审核通过后列表、详情、Meilisearch 和图片内容均可用；
- 更新替换期间旧图片隐藏，新图片复审后可用；
- 用户删除、审核驳回和管理员下架后图片 content 返回 404；
- 已售商品图片继续可用；
- legacy fixture 的内容地址继续工作；
- 非 owner、未审核和其他商品已绑定资产均 fail-closed。

### 9.4 真实验收

- Module 10 ticket → COS PUT → complete → 媒体审核 → 带图商品创建；
- 商品审核 → Meilisearch 命中；
- `/api/v1/media/assets/:id/content` 返回 302，Location 为固定 version 的短签 URL，实际 GET 内容成功；
- 更新替换、删除/下架和 legacy 回退通过；
- 微信开发者工具完成详情图片展示与预览；
- 保留此前收藏、联系卖家和腾讯 IM 文字链路回归。

## 10. 交付门禁

- `go test ./...`；
- `npm run verify`；
- `npm --workspace apps/miniapp run build:weapp`；
- `npm run check:miniapp-static-routes`，25 条路由；
- `git diff --check`；
- `npm run check:boundaries`；
- `backend/migrations/**` 零 diff；
- 真实 HTTP/COS/MySQL/Meilisearch 验收；
- 微信开发者工具真实图片展示与预览；
- 测试权限、商品公开状态和临时云端账号清理；
- Acceptance、Development、Residue 报告更新；
- 最终工作区干净。

## 11. 回滚与发布

实现保持在当前 UI-21b-2c 功能分支，在完全真实验收前不合并、不打上线标签。回滚时移除 composition root 的混合 adapter 与 content fallback，即恢复原 legacy gateway；没有数据库迁移需要回滚。由新 adapter 创建但尚未发布的引用保持 hidden，已经发布的测试引用必须在回滚前通过正常撤销流程收口。
