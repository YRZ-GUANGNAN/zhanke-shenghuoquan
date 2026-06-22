# G-⑥a-ui Takeout Admin Acceptance Report v1.0

日期：2026-06-23
结果：**PASS**

## 1. 验收矩阵

| 项目 | 结果 | 真实证据摘要 |
|---|---|---|
| 商家管理 | PASS | 真实列表/详情；`active → suspended → active`，row version 连续推进 |
| 菜品/分类 | PASS | 未筛选 2 条；`categoryId` 筛选 1 条；菜品和分类详情可读 |
| 订单管理 | PASS | 真实订单明细、金额、`paidAt` 阶段时间均正确展示 |
| 骑手管理 | PASS | `active → resting → active`，真实 CAS 写入 |
| 费率配置 | PASS | 成功更新后重新 GET；外部并发更新后旧版本提交返回 409 |
| 在途上限 | PASS | 成功更新后重新 GET 最新值及 row version |
| 结算报表 | PASS | 真实 pending settlement 汇总可见 |
| 提现处理 | PASS | 二次确认后 `pending → paid`，row version 推进 |
| 鉴权 | PASS | 全程使用既有 A 域 JWT、`AuthRequired + AdminRequired` |
| 浏览器运行时 | PASS | 8 个视图逐项操作，控制台 error/warn 为 0 |

## 2. CAS 五项安全要求

1. 详情/配置中的 `rowVersion` 保存在前端状态并作为 `expectedRowVersion` 提交：PASS。
2. 费率、在途上限、提现打款等写操作均有明确确认：PASS。
3. 写入期间按钮禁用并显示 loading：PASS。
4. stale CAS 返回 409 时显示指定文案并提供“刷新最新数据”：PASS。
5. 配置成功后重新请求服务端数据：PASS。

## 3. 真实环境说明

- 数据库：真实 MySQL `8.4.0`。
- HTTP：真实 Gin handler、G 域 Service/Repository、A 域 JWT 中间件；没有 Mock API 或内存 Repository。
- 完整 `cmd/server` 在当前 shell 会被非本专项的 COS 启动前置配置阻止，因此验收使用临时 composition root 装配同一生产 Handler/Service/Repository/Auth；该临时入口未提交且验收后已删除。
- 浏览器：真实 Vite 管理端，所有请求经 `/api/v1/admin/takeout/*` 到真实 Gin 与 MySQL。

## 4. 回归

- `go test ./... -count=1`：PASS。
- `npm run verify`：PASS（Boundary、Admin/MiniApp typecheck、backend tests）。
- `npm run build`：PASS（MiniApp + Admin）。
- `go build ./...`、`go vet ./...`：PASS。
- 仅有既存 MiniApp bundle size warning，不属于本模块回归失败。

## 5. 最终结论

G-⑥a-ui 达到专项验收标准，结论 **PASS**。本分支仅等待 owner 审核与合并，不自行打标签或进入 G-⑦/P0-5。
