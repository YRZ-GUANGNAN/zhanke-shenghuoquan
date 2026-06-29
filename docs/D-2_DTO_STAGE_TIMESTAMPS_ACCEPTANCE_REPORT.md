# D-2 DTO Stage Timestamps Acceptance Report

## 1. 最终判定

**PASS**

验收对象：`feat/d2-dto-timestamps@e861dbc2`。

## 2. 验收矩阵

| 项目 | 结果 | 证据 |
|---|---|---|
| 模型字段已存在 | PASS | 8 个字段均为 `*time.Time` |
| 商家 DTO 映射 | PASS | 8 字段全部映射 |
| 骑手已分配订单 DTO 映射 | PASS | 8 字段全部映射 |
| 抢单池最小披露 | PASS | `ClaimableOrderView` 零修改 |
| 未发生阶段 null | PASS | 单元 JSON + 真实 HTTP 均验证 |
| ready 时间 | PASS | mock confirm 后 `readyForClaimAt` 非空 |
| 完整履约时间线 | PASS | 商家和所属骑手均得到 6 个已发生阶段时间 |
| owner fence | PASS | 他人商家详情仍为 404 |
| 骑手隔离 | PASS | 非所属骑手详情仍为 404 |
| CAS/并发 | PASS | 两骑手并发仍为一方成功、一方 409 |
| MySQL | PASS | 真实 MySQL 8.4.0 |
| Go 全量回归 | PASS | `go test ./... -count=1` |
| Boundary | PASS | A-G + Delivery + P0-4 media |
| Verify | PASS | `npm run verify` |
| Build | PASS | MiniApp 与 Admin 均通过 |
| AutoMigrate | PASS | 命中 0 |
| 领域/迁移零改动 | PASS | 差异仅 DTO 与测试 |

## 3. 真实环境证据

使用真实 Gin handler、JWT 中间件、Repository 和 MySQL 8.4：

```text
real_mysql_version=8.4.0
d2_http=PASS admin_required+owner_fence+fees+idempotency+claim_1v1+delivery+outbox+gates
```

新增断言已包含在同一真实 HTTP 用例中：

1. 创建响应含 8 个 `null` 字段；
2. 商家 ready 详情含非空 `readyForClaimAt`；
3. 完整履约后，商家详情与所属骑手详情包含非空：
   - `readyForClaimAt`
   - `claimedAt`
   - `toPickupAt`
   - `pickedUpAt`
   - `deliveringAt`
   - `deliveredAt`
4. 未发生的 `paymentFailedAt/cancelledAt` 为 `null`。

## 4. 回归结果

```text
go test ./... -count=1     PASS
npm run check:boundaries   PASS
npm run verify             PASS
npm run build              PASS
```

小程序只保留既有 `vendors.js` 体积告警，本补丁没有前端代码或依赖变化。

## 5. 残留与安全

- `dispatch_residue orders=0 outbox=0`。
- HTTP 测试数据按 D-2 fence 精确清理。
- 临时 `node_modules` junction 已删除。
- 未启动常驻进程。
- 未修改或提交 `.env`、DSN、JWT、Webhook URL 或密钥。
- AutoMigrate 仍为 0。
- 无迁移、表结构、状态机、Repository、Service 或权限变化。

## 6. 结论

DTO 缺口已关闭，D-2 前端可基于真实 nullable 字段展示配送时间线。本分支未合并 master、未打标签、未进入 D-3。
