# G-⑥a-2 外卖商家/菜品 F 审核接入 Development Report v1.0

日期：2026-06-22

分支：`feat/g6a2-takeout-review`

基线：`5eb3f7a6`

实现提交：`761c63e267a655f39e803be883357605dc53c967`

## 1. 前置门禁

- master 已包含 G-⑥a-1，tag `g6a1-takeout-admin-baseline` 与 `g5-rider-baseline` 存在。
- `backend/cmd/server` 中 `db.AutoMigrate(` 为 0。
- 主工作区干净。
- 首轮真实代码审查发现边界和菜品拒绝态冲突后按授权停止；owner 裁定后恢复。

## 2. 真实代码审查结论

- F `Submitter`：`Submit(context.Context, review.Submission) (review.Task, error)`。
- F `Target`：`ObjectType() string` 与 `SetReviewStatus(context.Context, uint, review.Status) error`。
- F 状态：`submitted / reviewing / approved / rejected`。
- F `Start/Decide` 真实顺序为先调用 Target，再保存 `review_tasks`；两步不在同一事务且 F 无 CAS。
- community/trade 的既有范式均由组合根注册 Target；本次适配器放在 `admin/takeoutconsole/reviewtarget`，没有让 G 核心依赖 F。
- Merchant 原有 CAS 能力为 `Approve/Reject`；Dish 原有 CAS 能力为 `PutOnShelf`。
- 原代码只有按 public ID 查询；本次追加最小内部数字 ID 只读查询。

## 3. 裁定后的实现

### 3.1 Target 适配器

- `takeout_merchant`：
  - `reviewing`：校验仍为 `pending_review`，不修改实体；
  - `approved`：调用既有 `MerchantService.Approve`；
  - `rejected`：调用既有 `MerchantService.Reject`。
- `takeout_dish`：
  - `reviewing`：校验仍为 `draft`；
  - `approved`：调用既有 `CatalogService.PutOnShelf`；
  - `rejected`：不转移状态，要求并保持 `draft`，拒绝原因由 F Task 保存。
- Target 先按内部 ID 读取实体与 `row_version`，写入只调用既有 CAS Service；错误原样包装并返回，不静默吞掉。

### 3.2 最小只读能力

- `MerchantService.FindMerchantByInternalID`。
- `CatalogService.FindDishByInternalID`。
- 对应 GORM Repository 方法仅按主键读取并使用既有 `normalizeNotFound`。
- 未改变 G-①/G-② 既有状态机、方法签名、迁移或表约束。

### 3.3 组合根

- `main.go` 仅追加 Target 构造与 `reviewService.RegisterTarget`。
- community、trade、admin 的既有注册相对顺序不变。
- 本模块没有新增提交入口；测试通过 F `Submitter.Submit` 创建合法夹具。

## 4. 文件清单

- `backend/cmd/server/main.go`
- `backend/internal/module/takeout/review_lookup.go`
- `backend/internal/module/takeout/review_lookup_repository.go`
- `backend/internal/module/admin/takeoutconsole/reviewtarget/target.go`
- `backend/internal/module/admin/takeoutconsole/reviewtarget/target_test.go`
- `backend/internal/module/admin/takeoutconsole/reviewtarget/boundary_test.go`
- `backend/internal/module/admin/takeoutconsole/reviewtarget/http_mysql_test.go`

## 5. 明确未实施

- 未修改 F review 包、G 状态机、迁移、CHECK 约束或 G-⑥a-1 Handler。
- 未新增商家/菜品创建入口或审核提交入口。
- 未新增 Admin 前端、G-⑥b/⑥c/G-⑦、P0-5 内容。
