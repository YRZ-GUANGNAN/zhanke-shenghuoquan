# G-⑥a-1 外卖后台传输层骨架 Acceptance Report v1.0

日期：2026-06-22

实现提交：`4a76f887f48db568af12c22c47e6a75693621e30`

## 1. 验收矩阵

| 项目 | 结果 | 证据摘要 |
|---|---|---|
| 021 apply→rollback→apply | PASS | MySQL 8.4.0，最终表存在 |
| 021 表结构/索引/CHECK | PASS | 10 列、整数分、三状态 |
| 跨域外键 | PASS | 0 |
| 提现状态机 | PASS | pending→paid/rejected；终态禁止反向 |
| CAS | PASS | stale row_version 拒绝 |
| 支付原子性 | PASS | withdraw + earning 同事务 |
| 故障注入回滚 | PASS | 第二条 earning 更新失败后全部回滚 |
| 重复支付 | PASS | 幂等返回 paid，不重复推进版本 |
| 真实 HTTP | PASS | httptest.Server + Gin + MySQL 8.4 |
| 真实鉴权中间件 | PASS | 无 token 401、普通用户 403、admin 200 |
| 只读列表/详情 | PASS | 商家、分类、菜品、订单、骑手数据一致 |
| 分页信封 | PASS | items/page/pageSize/total |
| 配置 CAS | PASS | fee/rider 更新成功；stale 返回 409 |
| 提现管理 HTTP | PASS | pay/reject 均通过真实路由 |
| Boundary | PASS | G 核心零 HTTP/跨域 import；F 未接入 |
| `go test ./... -count=1` | PASS | 全包通过 |
| `go vet ./...` | PASS | 通过 |
| `go build ./...` | PASS | 通过 |
| `npm run verify` | PASS | boundary/typecheck/backend tests |
| `npm run build` | PASS | MiniApp/Admin 均构建成功 |
| 测试残留 | PASS | 0；全局配置恢复基线 |

## 2. 真实环境结论

- MySQL：`8.4.0`；
- HTTP 不是直接调用 Handler：使用 `httptest.NewServer` 包裹真实 Gin Router；
- 路由真实挂载 `AuthRequired()` 与 `AdminRequired()`；
- Repository 使用真实 GORM/MySQL，不使用 Mock DB；
- 故障注入发生在第二条 earning UPDATE，验证第一条 UPDATE 与 withdraw UPDATE 一并回滚。

## 3. 行为说明

- stale CAS：409 Conflict；
- 非法状态转移：409 Conflict；
- 非法输入：400；
- 不存在：404；
- 重复 `MarkWithdrawPaid`：返回同一 paid 记录，`row_version` 不再次增长；
- 拒绝请求进入 rejected 终态；
- paid/rejected 不允许互转。

## 4. 最终结论

**G-⑥a-1：PASS。**

本阶段在传输管道、后台查询、配置 CAS 和提现事务范围内满足授权验收标准。开发停止在 G-⑥a-1；未进入 G-⑥a-2 F 审核接入、Admin 前端或其它模块。
