# D-2 Delivery Dispatch Backend Acceptance Report

## 1. 最终判定

**PASS**

验收对象：`feat/d2-delivery-dispatch@bf386a46`。

## 2. 验收矩阵

| 项目 | 结果 | 证据摘要 |
|---|---|---|
| MySQL 版本 | PASS | 真实实例返回 `8.4.0` |
| 024 apply/rollback/apply | PASS | `delivery_outbox=1`、建单唯一键=1、外键=0 |
| 建单幂等 | PASS | 同 merchant/request ID 只生成一单；不同请求快照 fail-closed |
| 服务端费用 | PASS | 恶意客户端费用被忽略，持久化 150/100/50 分 |
| 生产初态 | PASS | 创建后仅为 `pending_fee_payment` |
| 状态 + Outbox 原子性 | PASS | ready 事务提交时事件版本等于新 `row_version` |
| Outbox 幂等 | PASS | 同聚合版本和事件类型只存在一条 |
| Outbox 租约/CAS | PASS | 旧 lease/stale row_version 被拒 |
| 重试/死信 | PASS | retry_wait、指数退避、deadletter 均验证 |
| URL 为空 | PASS | Consumer 正常启动，事件进入 skipped |
| Webhook 失败隔离 | PASS | 外呼失败时订单保持 ready，不回滚 |
| 消息隐私 | PASS | payload/消息无姓名、电话、详细地址、房间号 |
| 管理员开通骑手 | PASS | 无 token=401、普通用户=403、不存在用户=404、管理员成功 |
| 未开通骑手 | PASS | `/me` 诚实提示，抢单池/抢单拒绝 |
| 抢单并发 | PASS | 两骑手并发：1 个成功、1 个 HTTP 409 |
| 履约归属 | PASS | 非所属骑手详情 404，不能推进订单 |
| stale CAS | PASS | 过期 `expectedRowVersion` 返回 409 |
| 履约闭环 | PASS | claimed → to_pickup → picked_up → delivering → delivered |
| 商家 owner fence | PASS | 他人订单详情返回 404 |
| Mock 隔离 | PASS | 关闭时端点 404；prod + mock 配置启动失败 |
| ENABLE_DELIVERY | PASS | false 时 runtime=nil、路由 404 |
| 封存外卖隔离 | PASS | 未构造旧订单/RiderOrderOps/旧 RiderService 或旧路由 |
| Boundary Check | PASS | A-G + independent Delivery + P0-4 media |
| AutoMigrate | PASS | `db.AutoMigrate(` 命中 0 |
| 全量 Go Test | PASS | `go test ./... -count=1` |
| Verify | PASS | `npm run verify` |
| Build | PASS | miniapp + admin 均通过；仅既有 vendors 体积告警 |

## 3. 真实 MySQL 8.4 证据

执行时从本机既有 `backend/.env` 只读取 DSN，不打印或写入凭证：

```text
real_mysql_version=8.4.0
migration_023_apply_rollback_apply=PASS tables=1 foreign_keys=0 checks=status+fee_snapshot
migration_024_apply_rollback_apply=PASS outbox=1 merchant_request_unique=1 foreign_keys=0
delivery_cas_competition=PASS winners=1 conflicts=1
dispatch_claim_competition=PASS ... conflict=1 lifecycle=delivered
dispatch_outbox=PASS lease_recovery+stale_CAS+retry+deadletter+skipped
dispatch_residue orders=0 outbox=0
```

MySQL 输出中的 CHECK 约束错误和唯一键错误来自负向验收用例，测试按预期 PASS。

## 4. 真实 HTTP + JWT + MySQL 证据

使用真实 Gin handler、真实 JWT/AuthRequired/AdminRequired、真实 Repository 和真实 MySQL；未 mock 路由或数据库：

```text
d2_http=PASS admin_required+owner_fence+fees+idempotency+claim_1v1+delivery+outbox+gates
```

覆盖：

- 管理员开通骑手 401/403/404/201；
- 商家 JWT 身份与请求体伪造隔离；
- 幂等建单；
- owner fence；
- mock fee → ready + Outbox；
- 抢单池不泄露 PII；
- 两骑手真实并发争抢；
- loser 409、非 owner 404、stale 409；
- 完整履约；
- Webhook 失败不回滚；
- mock 端点关闭 404；
- Delivery 总开关关闭后路由 404。

## 5. 自动化回归

```text
go test ./... -count=1              PASS
npm run check:boundaries            PASS
npm run verify                      PASS
npm run build                       PASS
```

小程序构建保留既有 `vendors.js` 大小告警，不是 D-2 新增回归。

## 6. 安全结论

- 商家身份只取 JWT，不信任请求体 merchant ID。
- 骑手只有管理员开通且 active 后才能访问敏感配送信息。
- 抢单前仅展示最小字段，抢单成功后才返回完整配送信息。
- 所有状态写入使用 D-1 CAS；传输层将并发冲突映射为 409。
- Mock 配送费确认有独立开关并被生产环境硬拒绝。
- Webhook 配置为半敏感信息，仅从环境读取，未入库、未日志化、未写入报告。

## 7. 范围确认

本次未实现前端、真实支付、商家钱包、照片、IM、动态费率或 D-3；未修改旧外卖业务实现、社区、二手、media、IM。
