# UI-21b-2a 社区/二手安全写契约加固设计

> 状态：DESIGN APPROVED，等待 owner 审阅文档后进入 `writing-plans`。
>
> 基线：`master@7fde39a9`（`docs(UI-21b-1): 记录社区二手只读接线验收`）
>
> 前置标签：`ui21b1-community-trade-readonly-baseline`
>
> 设计日期：2026-07-12

## 1. 背景

UI-21b-1 已把社区帖子与二手商品的真实只读数据接入新版小程序壳，但写操作仍存在以下系统性风险：

1. 点赞、收藏、关注等接口采用“切换”语义。客户端遇到超时后无法安全重试，重放可能把状态切回去。
2. 发帖、评论、发布二手商品没有客户端幂等键。网络重试或并发点击可能重复创建资源。
3. 创建资源同时涉及业务库、Media、Review 等多个模块。现状不是一个本地事务，进程在中间崩溃时可能留下半完成状态。
4. `review.Submit` 使用 `FirstOrCreate + Assign`，重复调用可能把已经 `approved`、`rejected` 或 `reviewing` 的任务重新写回 `submitted`。
5. 旧 Media `Commit` 在 reservation token 被清除后不是重放幂等操作。
6. 二手“联系卖家”关系虽然已有唯一约束，但冲突路径会更新 `updated_at`，重放并非真正无副作用。
7. UI-21b-1 DTO 仍缺当前用户视角的点赞、收藏、关注、联系状态，页面刷新后无法诚实恢复交互状态。

本设计先加固后端写契约与恢复机制，再允许新版前端接入真实写操作。

## 2. 目标

- 为社区和二手新增确定态、可安全重试的写接口。
- 为发帖、评论、发布商品提供客户端幂等创建契约。
- 保留全部旧接口及旧调用方行为，不做破坏性替换。
- 用域内 outbox 闭合“业务实体创建 → Media 绑定 → Review 提交”的崩溃恢复链路。
- 为当前用户提供只读 viewer state，页面刷新后可恢复真实状态。
- 所有金额继续使用整数分；不得引入 `float` 或 `decimal`。
- 保持 Community、Trade、Media、Review 的模块边界，不允许跨域直接查表。
- 用真实 MySQL 8.4、真实 HTTP、并发与失败注入证明幂等和恢复能力。

## 3. 非目标

- 不删除或改变旧 `POST .../like|favorite|follow|contact` 的既有切换语义。
- 不在本阶段重做社区或二手页面视觉。
- 不把腾讯 IM 消息正文存入业务数据库。
- 不在本阶段补腾讯 IM 图片消息；该能力需独立授权依赖、合法域名与真机验收。
- 不修改 Delivery、封存的旧 Takeout、支付、骑手、商家钱包或 Admin 资金逻辑。
- 不宣称跨数据库/外部服务的全局 exactly-once；本设计实现的是可恢复、可判定、可重放的业务完成协议。

## 4. 总体裁定

采用“兼容旧接口 + 新增安全接口”的方案：

- 旧客户端继续使用现有 toggle/create/contact 接口，行为不变。
- 新客户端使用 `PUT/DELETE` 确定态接口和带幂等键的创建接口。
- viewer state 从现有关系事实表实时派生，不创建冗余 viewer-state 表。
- keyed create 走域内 creation outbox；无 key 的旧创建路径保持原行为。
- Media 和 Review 仅新增幂等便捷方法，原有方法零行为修改。

## 5. HTTP 契约

### 5.1 社区 viewer state

```http
GET /api/v1/community/posts/:id/viewer-state
Authorization: Bearer <token>
```

成功响应：

```json
{
  "code": 0,
  "message": "ok",
  "data": {
    "liked": true,
    "favorited": false,
    "followingAuthor": true
  }
}
```

规则：

- 必须登录。
- 帖子不存在、不可见或无权查看时统一返回 `404`，不得泄露审核或下架状态。
- `liked`、`favorited`、`followingAuthor` 分别从 `post_likes`、`post_favorites`、`user_follows` 派生。

### 5.2 社区确定态写接口

```http
PUT    /api/v1/community/posts/:id/like
DELETE /api/v1/community/posts/:id/like

PUT    /api/v1/community/posts/:id/favorite
DELETE /api/v1/community/posts/:id/favorite

PUT    /api/v1/community/users/:id/follow
DELETE /api/v1/community/users/:id/follow
```

语义：

- `PUT` 表示最终状态必须为 `true`。
- `DELETE` 表示最终状态必须为 `false`。
- 重复执行相同请求必须成功且无副作用。
- 返回当前确定态，例如 `{ "liked": true }`、`{ "favorited": false }`、`{ "following": true }`。
- 只有关系真实发生变化时，才允许修改聚合计数、创建通知或写搜索 outbox。
- no-op 不得更新关系时间戳，不得重复通知，不得重复写 outbox。

旧接口继续保留：

```http
POST /api/v1/community/posts/:id/like
POST /api/v1/community/posts/:id/favorite
POST /api/v1/community/users/:id/follow
```

### 5.3 二手 viewer state

```http
GET /api/v1/trade/products/:id/viewer-state
Authorization: Bearer <token>
```

成功响应：

```json
{
  "code": 0,
  "message": "ok",
  "data": {
    "favorited": true,
    "contactedSeller": false
  }
}
```

规则与社区一致：不可见或无权时统一 `404`，状态从事实表派生。

### 5.4 二手确定态写接口

```http
PUT    /api/v1/trade/products/:id/favorite
DELETE /api/v1/trade/products/:id/favorite
PUT    /api/v1/trade/products/:id/contact
```

其中：

- 收藏遵循确定态 `PUT/DELETE` 规则。
- `PUT .../contact` 表示确保当前买家与该商品的联系关系存在。
- 首次创建返回关系与腾讯 IM 会话上下文；重放返回同一业务关系。
- 重放不得更新 relation 的 `updated_at`。
- 不得在响应中泄漏内部 buyer/seller ID；只返回前端建立会话所需的净化字段。
- 联系自己返回 `422`；已售或业务冲突返回 `409`；不可见返回 `404`。
- 腾讯 IM 上游不可用或超时分别映射为稳定的 `503`/`504`，不得再错误映射为 `400`。

旧接口继续保留：

```http
POST /api/v1/trade/products/:id/favorite
POST /api/v1/trade/products/:id/contact
```

## 6. 创建幂等契约

### 6.1 适用接口

以下既有创建接口增加可选幂等能力，不改变无 key 调用方行为：

```http
POST /api/v1/community/posts
POST /api/v1/community/posts/:id/comments
POST /api/v1/trade/products
```

客户端可任选一种方式提交 ULID 格式的 request ID：

```http
Idempotency-Key: 01J...
```

或：

```json
{
  "clientRequestId": "01J..."
}
```

若 header 与 body 同时存在：

- 两者相同：接受。
- 两者不同：返回 `400`。

### 6.2 重放规则

以 `(owner_user_id, client_request_id)` 为幂等坐标：

| 情况 | 行为 |
| --- | --- |
| 首次请求 | 创建资源并推进创建协议；完成后返回 `201` |
| 同用户、同 key、同规范化请求 | 返回原资源，不重复创建；返回 `200`，响应头 `Idempotency-Replayed: true` |
| 同用户、同 key、不同规范化请求 | 返回 `409` |
| 未提供 key | 完全沿用旧路径与旧响应 |
| 协议尚未完成且可重试 | 返回 `503` 并提示客户端稍后使用同一 key 重试 |
| 协议永久失败 | 返回稳定业务错误，不伪装成功 |

### 6.3 请求哈希

`create_request_hash` 使用 canonical SHA-256，输入必须包括：

- 命令版本，例如 `community-post-create:v1`。
- 业务正文、分类、campus、目标 post ID 等实际业务字段。
- 商品价格与原价的整数分值。
- 按请求顺序排列的 asset public ID。
- 其他会改变创建结果的稳定字段。

不得包括：

- 服务端时间戳。
- 自动生成 public ID。
- 审核状态、Media URL、签名 URL。
- 日志 trace ID 等非业务字段。

序列化必须固定字段顺序、字符串编码、空值表达及数组顺序，避免不同进程算出不同摘要。

## 7. 迁移 030

新增：

```text
backend/migrations/030_ui21b2a_write_contract.sql
backend/migrations/rollback/030_ui21b2a_write_contract.sql
```

不修改 `001–029`。

### 7.1 创建幂等字段

在 `posts`、`comments`、`products` 各新增：

```sql
client_request_id   CHAR(26) NULL
create_request_hash BINARY(32) NULL
```

并增加：

- owner/user + `client_request_id` 唯一索引。
- 两字段必须“同时为空或同时非空”的 `CHECK`。
- 历史行保持 `NULL/NULL`，不改变历史数据行为。

索引名称按现有命名规范确定，必须在迁移验收中验证唯一性和回滚完整性。

### 7.2 viewer state 反向覆盖索引

```sql
CREATE INDEX idx_post_likes_user_post
  ON post_likes(user_id, post_id);

CREATE INDEX idx_post_favorites_user_post
  ON post_favorites(user_id, post_id);

CREATE INDEX idx_product_favorites_user_product
  ON product_favorites(user_id, product_id);
```

`user_follows` 已有 `(follower_id, followee_id)` 主键，`im_conversations` 已有业务关系索引，不重复建索引。

### 7.3 域内 creation outbox

新增两张域自有表：

- `community_write_outbox`
- `trade_write_outbox`

禁止跨域外键。每张表至少包含：

- `id BIGINT UNSIGNED PK AUTO_INCREMENT`
- `event_public_id CHAR(26) NOT NULL UNIQUE`
- `event_type VARCHAR(...) NOT NULL`
- `aggregate_public_id CHAR(26) NOT NULL`
- `owner_user_id BIGINT UNSIGNED NOT NULL`
- `reservation_token VARCHAR(...) NULL`
- 有序 asset public ID 快照（JSON 或与现有规范一致的稳定载荷）
- 风险/审核上下文快照
- `attempt_count INT UNSIGNED NOT NULL`
- `available_at DATETIME NOT NULL`
- `lease_token CHAR(26) NULL`
- `lease_expires_at DATETIME NULL`
- `processed_at DATETIME NULL`
- `failed_at DATETIME NULL`
- `last_error VARCHAR(...) NOT NULL DEFAULT ''`
- `row_version BIGINT UNSIGNED NOT NULL`
- `created_at/updated_at DATETIME NOT NULL`

唯一约束必须保证一个 aggregate 的同类创建事件只存在一条。

## 8. keyed create 协议

### 8.1 正常路径

1. 校验请求、幂等 key 与规范化哈希。
2. 查询 `(owner, key)`：
   - 已存在同哈希，进入重放判定。
   - 已存在不同哈希，返回 `409`。
3. 对请求中的 Media 资源执行 Reserve，取得 reservation token。
4. 在本域单个 MySQL 事务中：
   - 创建业务聚合。
   - 创建图片映射等域内附属记录。
   - 写入 creation outbox。
   - Trade 如有既有搜索删除/更新 outbox，仍按原有领域规则写入。
5. 提交本地事务。
6. inline consumer 立即尝试处理 creation outbox：
   - `EnsureMediaCommitted`
   - `EnsureReviewSubmitted`
7. 两步均完成后 CAS 标记 outbox `processed_at`。
8. 首次请求返回 `201`；同 key 重放返回 `200` + replay header。

### 8.2 为什么需要 outbox

MySQL 事务不能覆盖 COS/Media/Review 等跨模块副作用。若只在 handler 顺序调用，进程可能在任一步之间退出，造成：

- 实体存在但图片未绑定。
- 图片已绑定但审核任务未创建。
- 审核任务已创建但客户端收到超时，再次提交生成重复实体。

creation outbox 把“尚待完成的跨模块工作”持久化在业务域内。inline consumer 负责低延迟，后台 worker 负责崩溃恢复，两者执行同一幂等处理器。

### 8.3 worker 领取与重试

worker 必须：

- 使用 `FOR UPDATE SKIP LOCKED` 批量领取任务。
- 写入 lease token 和 lease expiry，避免多个 worker 同时处理同一任务。
- 对完成标记使用 `row_version` CAS。
- 对暂时错误按指数退避更新 `attempt_count/available_at/last_error`。
- lease 过期后允许其他 worker 接管。
- 对确定的永久不一致写 `failed_at`，不无限重试。
- 配置关闭或无待处理任务时安全空转，不阻断主业务。

## 9. Media 幂等补充能力

仅新增便捷方法，例如：

```go
EnsureCommitted(ctx, reservationToken, ownerUserID, referenceType, referenceID, orderedAssetPublicIDs)
```

行为：

1. 若 token 仍处于 reserved，原子绑定到目标对象并完成 commit。
2. 若 token 已被清除，但全部 assets 已按相同顺序、相同 owner、相同 reference type/id 精确绑定，返回成功。
3. 若部分绑定、顺序不一致、owner 不一致或绑定到其他对象，返回永久不一致错误。
4. 不修改旧 `Commit` 的行为和签名。

该方法属于 Media 自己的服务/Repository 实现；Community/Trade 不得直接查 Media 表。

## 10. Review 幂等补充能力

仅新增便捷方法，例如：

```go
EnsureSubmitted(ctx, targetType, targetID, submitterUserID, snapshot)
```

行为：

- 任务不存在：创建 `submitted` 任务。
- 任务已存在且为任何合法状态：原样返回，不改变状态、reason 或版本。
- 不修改旧 `Submit`；旧方法继续服务显式重新提交/重新审核业务。

这避免 keyed replay 把 `approved/rejected/reviewing` 任务重置为 `submitted`。

## 11. 确定态关系事务

### 11.1 Set true

1. 锁定并验证目标资源可见性。
2. 使用冲突忽略插入关系。
3. 根据 `RowsAffected` 判断是否首次插入。
4. 仅首次插入时增加计数并产生通知/搜索 outbox。
5. 同一 MySQL 事务提交。

### 11.2 Set false

1. 锁定并验证目标资源。
2. 精确删除当前用户关系。
3. 根据 `RowsAffected` 判断是否真实删除。
4. 仅真实删除时减少计数并产生必要 outbox。
5. 计数不得减为负数；发现事实表与计数异常时 fail-closed 并记录诊断。
6. 同一 MySQL 事务提交。

### 11.3 并发保证

- 多个 `PUT` 最终只有一条关系，计数只加一。
- 多个 `DELETE` 最终没有关系，计数只减一次。
- `PUT/DELETE` 竞争的最终状态由最后成功提交的确定态命令决定，不允许计数漂移。
- no-op 不更新时间、不发重复通知、不写重复 outbox。

## 12. 二手联系卖家幂等

保留现有业务唯一键，冲突路径改为：

1. `INSERT ... ON CONFLICT/ON DUPLICATE KEY DO NOTHING` 等价语义。
2. 插入成功则创建/确认腾讯 IM 会话上下文。
3. 已存在则 `SELECT` 原关系，不执行 update。
4. 返回净化后的 DTO。

同一买卖双方联系多个商品时，业务 relation 可按商品区分；腾讯 C2C conversation 仍可能按用户对共享。前端必须以 relation/product context 展示商品卡片，不得声称每个商品拥有隔离的腾讯 C2C 消息历史。

## 13. 错误、超时与客户端重试

### 13.1 HTTP 错误语义

| HTTP | 含义 |
| --- | --- |
| 400 | JSON、key 格式或 header/body key 不一致 |
| 401 | 未登录 |
| 403 | 封禁、禁言或明确无权限 |
| 404 | 资源不存在或按防泄漏规则不可见 |
| 409 | 同 key 异参、状态/CAS/业务冲突 |
| 422 | 语义校验失败，例如联系自己 |
| 503 | 可重试的下游不可用或创建协议尚未完成 |
| 504 | 明确的下游超时 |

响应继续使用既有 `{code,message,data}` 信封；新增稳定错误类型由 handler 统一映射，不允许通过字符串猜测。

### 13.2 客户端原则

- 确定态 `PUT/DELETE` 可使用相同请求重试。
- keyed create 只能使用原 key 重试；不得超时后生成新 key。
- 非幂等旧 toggle 接口不得自动重试。
- 前端写按钮必须有 pending 锁，防止用户重复点击。
- 收到 `409` 时刷新 viewer state/实体状态，不在本地猜测。
- 收到 `503/504` 时保留原 key，并给出明确“稍后重试”提示。

## 14. 架构边界

- Community 只拥有 Community 聚合、关系和 `community_write_outbox`。
- Trade 只拥有 Trade 聚合、关系和 `trade_write_outbox`。
- Media 的幂等提交只能由 Media 自己实现和读取其表。
- Review 的幂等提交只能由 Review 自己实现和读取其表。
- 组合由现有应用/传输层注入窄接口完成。
- 核心域不得 import Gin、COS、腾讯 IM SDK 或其他域的 Repository 实现。
- 禁止跨域外键；逻辑引用使用 public ID 或冻结的业务坐标。
- `db.AutoMigrate(` 必须继续为 0。
- 如需新增 boundary 登记，只做最小文件登记，不放宽既有规则。

## 15. 真实验收矩阵

### 15.1 前置门禁

- 记录 `master` HEAD 与要求标签。
- 主仓库工作区干净。
- 最大迁移为 029，030 不存在。
- `rg "db\.AutoMigrate\(" backend/cmd/server` 为 0。
- 受保护 worktree `feat/p0-identity-switch` 原样存在且不被触碰。
- C 盘可用空间至少 8 GB；低于该值不得开始实现、安装依赖或跑完整构建。

### 15.2 迁移 030（真实 MySQL 8.4）

- apply → apply（幂等）→ rollback → apply 全部 PASS。
- 历史 posts/comments/products 数量及内容不变。
- 新字段历史行为为 `NULL/NULL`。
- 唯一索引、CHECK、outbox 表结构符合契约。
- 外键数为 0。

### 15.3 keyed create 并发

每类创建至少验证：

- 20 个并发请求使用同一 owner/key/请求体：只创建一个资源、一个域内 outbox，所有成功响应指向同一 public ID。
- 同 key 异参：稳定 `409`。
- 不同 owner 可使用同一 key，互不冲突。
- 无 key 的旧请求行为不变。

### 15.4 失败注入与恢复

逐一在以下位置强制失败/退出：

1. Reserve 后、实体事务前。
2. 实体 + creation outbox 提交后。
3. Media commit 完成后。
4. Review task 创建后。
5. worker 领取后、lease 过期前。

证明：

- 同 key 重试不产生重复实体。
- worker 重启后可恢复。
- 两个 worker 竞争只完成一次。
- Media 精确重放成功，坐标不一致永久失败。
- Review 已进入任何状态时不被重置。
- outbox 最终 `processed_at` 或明确 `failed_at`，不出现静默悬空。

### 15.5 确定态交互并发

对点赞、收藏、关注分别验证：

- 20 个并发 `PUT`：关系一条、计数 +1、通知/outbox 一次。
- 再次 20 个 `PUT`：完全 no-op。
- 20 个并发 `DELETE`：关系归零、计数 -1 一次。
- 再次 20 个 `DELETE`：完全 no-op。
- `PUT/DELETE` 竞争后事实表与计数对账一致。

### 15.6 viewer state

- 未登录为 `401`。
- 登录用户看到准确关系状态。
- 不可见目标统一 `404`，不泄漏审核/下架信息。
- `EXPLAIN FORMAT=JSON` 证明命中新增 viewer 索引。

### 15.7 联系卖家

- 首次 `PUT` 创建一条 relation。
- 20 个并发相同请求仍只有一条 relation。
- 重放不改变 `updated_at`。
- DTO 不泄漏内部 user ID。
- 422/404/409/503/504 映射正确。
- 若无法取得真实腾讯 IM 环境证据，该验收项必须保持 `CONDITIONAL PASS`，不得写成全通过。

### 15.8 兼容与负范围

- 所有旧 toggle/create/contact 测试继续 PASS。
- Community/Trade 公开列表与详情只读行为不变。
- Delivery、封存旧 Takeout、支付、Admin 资金模块零行为修改。
- Media 旧 `Commit`、Review 旧 `Submit` 零行为修改。

### 15.9 全仓验证

```powershell
go test ./... -count=1
npm run check:boundaries
npm run verify
npm run build
```

并验证：

- AutoMigrate = 0。
- 测试数据 scoped 清理。
- worker/后端/前端验收进程全部关闭。
- worktree 干净。

## 16. 实施拆分建议

设计获 owner 审阅后，再使用 `writing-plans` 生成逐步实施计划。建议计划按以下顺序拆分：

1. 迁移 030 与 Repository 基础。
2. viewer state 与确定态关系命令。
3. 创建幂等坐标、规范化哈希与域内 creation outbox。
4. Media `EnsureCommitted`。
5. Review `EnsureSubmitted`。
6. inline consumer、后台 worker、lease/CAS/退避。
7. Community/Trade HTTP 安全接口与错误映射。
8. 真实 MySQL/HTTP/并发/失败注入验收。
9. 最后才接新版前端写操作。

实现必须新建隔离 worktree，不得直接在 `master` 开发，也不得触碰 `C:\Users\33141\Documents\p0-identity-switch`。

## 17. 交付与冻结

- 实现分支与 worktree 名称由实施计划确定。
- 完成后提交 Development / Acceptance / Residue 报告并入库 `docs/`。
- owner 验收后在主仓库串行 `git merge --ff-only`。
- 合并后打 annotated tag：`ui21b2a-write-contract-baseline`。
- 在真实腾讯 IM 证据缺失时，相关项继续标记 `CONDITIONAL PASS`。

## 18. 当前阻断

设计文档编写时 C 盘可用空间约 1 GB，低于本项目实现门禁 8 GB。该空间足以提交本设计文档，但不足以安全创建实现 worktree、安装依赖或运行完整构建。

因此：

- 本文档提交后停止。
- owner 审阅通过后，先恢复至少 8 GB 可用空间。
- 再进入 `writing-plans` 和实现阶段。
- 不得为了绕过空间门禁删除 Claude、Codex、微信数据、项目源码或个人文件。
