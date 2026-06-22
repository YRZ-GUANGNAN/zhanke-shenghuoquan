# G-⑥a-2 外卖商家/菜品 F 审核接入 Acceptance Report v1.0

日期：2026-06-22

实现提交：`761c63e267a655f39e803be883357605dc53c967`

## 1. 验收矩阵

| 验收项 | 结果 | 证据摘要 |
|---|---|---|
| 前置门禁 | PASS | G-⑥a-1 基线/tag、AutoMigrate=0、工作区干净 |
| 真实 MySQL | PASS | `8.4.0` |
| 真实 HTTP/Gin | PASS | `httptest.NewServer` + 真实 Admin Handler + 真实鉴权中间件 |
| 无 Token / 普通用户 | PASS | 401 / 403 |
| Merchant approved | PASS | F `approved`，G 实体为 `active` |
| Merchant rejected | PASS | F `rejected`，G 实体为 `rejected` |
| Dish approved | PASS | F `approved`，G 实体为 `on_shelf` |
| Dish rejected | PASS | F `rejected` 且 reason 保存，G 实体保持 `draft` |
| CAS 纪律 | PASS | 仅调用既有 `Approve/Reject/PutOnShelf`，无直接 UPDATE |
| F 非原子性暴露 | PASS | CAS 竞争返回 HTTP 400，错误对调用方可见 |
| Boundary | PASS | takeout 核心 F/HTTP import=0；无边界脚本修改 |
| 迁移影响 | PASS | 无新增或修改迁移 |
| 全量 Go | PASS | `go test ./... -count=1`、vet、build |
| Workspace verify | PASS | boundary、Admin/MiniApp typecheck、backend tests |
| Build | PASS | MiniApp 与 Admin build；仅保留既有 bundle size warning |
| 残留 | PASS | review_tasks/merchants/dishes/users=0；临时 junction/dist=0 |

## 2. F 非原子性真实结果

验收在 Merchant Target 第二次读取后模拟并发更新 `row_version`，随后由真实 HTTP 调用 F `Decide(approved)`：

```text
HTTP status = 400
error = apply takeout merchant review decision: merchant changed concurrently
review_tasks.status = reviewing
takeout_merchants.status = pending_review
```

结论：Target 的 CAS 错误没有被吞掉，不存在“HTTP 成功但实体未转移”的静默不一致。与此同时，F 任务停留在 `reviewing`，准确暴露了 F 回写与任务保存非事务的遗留弱点。本任务未越权修复 F。

## 3. 边界结论

- `internal/module/takeout` 未导入 `admin/review`、Gin 或 `net/http`。
- F 类型只存在于 `admin/takeoutconsole/reviewtarget`。
- G 核心数据库访问仍只通过 Repository。
- 未修改 `check-boundaries.cjs`、F review 包、冻结状态机和迁移。

## 4. 最终结论

**G-⑥a-2：PASS（授权范围内）**。

已知遗留风险仍为 F `SetReviewStatus → Save(task)` 非原子；该风险已被测试诚实暴露并保留，需未来独立授权治理。不得据此进入未授权的 Admin 前端或其它模块。
