# 骑手提现资金收口 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking. 本环境未提供该子技能，按本计划在当前线程逐项执行并保留测试证据，不启动新子代理。

**Goal:** 按已确认的人工提现方案，消除余额无来源提现、重复结款和人工处理竞争风险；不调用真实资金接口。

**Architecture:** 056 新增收益分配与不可变处理审计；所有提现资金写入统一使用骑手钱包锁和 READ COMMITTED 事务。可用额度由已结算、已入账且来源可核验收益计算，旧逐单付款和内部队列共用分配互斥；能力默认关闭，启动预检通过后才可受控启用。

**Tech Stack:** Go、GORM、MySQL 8.4、React/Taro、TypeScript。

---

## Task 1: 迁移与统一资金入口

Files: `backend/migrations/056_delivery_rider_withdrawal_allocations.sql`、对应 rollback、`backend/internal/module/delivery/rider_withdrawal*.go`。

- [ ] 为 056 增加分配（in_flight/paid/released）、处理权、异常标记、审计和数据库约束。回滚在有任一事实时拒绝。
- [ ] 删除按钱包总余额扣款的旧提现分支；公开仓库入口始终走来源分配，缺迁移拒绝。
- [ ] 金额使用整数分，上限 `9007199254740991`；加减必须有溢出检查。
- [ ] 创建时以钱包 → 按 id 排序收益 → 提现/分配/队列锁；分配选择另按 settle_at/id 排序。校验原收入流水与钱包归属，无来源的旧余额不能提款。
- [ ] 测试入口：

```powershell
go test ./internal/module/delivery -run 'TestWithdrawal|TestRiderWithdrawal' -count=1
```

预期：纯逻辑通过；MySQL 用 Task 4 的专用 runner，禁止共享数据库。

## Task 2: 处理权、来源锁与互斥

Files: `rider_withdrawal_processing_repository.go`、`rider_withdrawal_funding.go`、`fee_wallet_repository.go`、`rider_payout_queue_repository.go`。

- [ ] 领取者/版本/操作 ID 持久化；无自动租约过期；同键异参冲突。未领取可拒绝，领取后不可拒绝。只有领取者明确尚未付款才可解除。
- [ ] 付款成功登记与异常凭证保存分开：来源变化或结果未知保留占用，独立审计记录，不回充、不允许二次领取。
- [ ] 当前源核验不锁订单（完成配送持有订单后等钱包），不锁退款行（回调持有退款再锁结算）；源锁为结算 SHARE → 按公开ID排序支付 SHARE → 分配 SHARE → 按ID排序充值批次 SHARE。事务 READ COMMITTED，所有需改变来源的业务写入与这些共享锁互斥；不在持有充值批次后反向锁支付。
- [ ] 旧逐单付款只要收益有 in_flight/paid 分配即拒绝；队列筛选与重核验同样拒绝/阻断。取消队列只释放其自身占用。
- [ ] 旧 SQL 假连接测试只证明关门条件；完整事务回滚和重放转入真实隔离 MySQL，不保留不安全分支来满足旧测试。

## Task 3: 能力、页面和部署预检

Files: `backend/internal/config/config.go`、`backend/cmd/server/main.go`、`backend/internal/module/admin/deliveryrider/`、`apps/admin/src/DeliveryConsole.tsx`、`apps/admin/src/services/delivery-admin.ts`、骑手钱包页面/服务类型。

- [ ] 新配置 `ENABLE_DELIVERY_RIDER_WITHDRAWAL=false`；禁止与自动队列同时开启。启动校验 056、历史提现分配守恒、原借/贷流水、队列冲突；异常保持关门。
- [ ] 后台能力接口显式返回开关；处理操作会话来源操作者、CAS、操作 ID、不信客户端用户 ID。旧逐单打款入口在人工提现模式下关闭。
- [ ] 骑手页面只读取服务端 `withdrawableBalanceCents`；字段缺失、错误、切换账号迟到回执均禁用提交；不改布局。
- [ ] 商家收益单独声明 `external_order_source_unavailable`，不能将充值余额伪装成收益或提供假提现。
- [ ] 修正设置页“开发预览/体验版”和一次授权额度文案；生产通知发送不自动开启。

## Task 4: 并发验收与交付

- [ ] MySQL 场景：部分/多收益、两申请竞争、同键异参、领取与拒绝竞争、跨管理员、释放重放、异常证据、通知失败全回滚、旧付款和队列互斥、来源变更、午夜结算、共享充值来源并发退款、历史不一致拒绝启动。
- [ ] 所有测试名匹配 `TestRider.*AgainstIsolatedMySQL`，由无 SKIP runner 必须实际运行。

```powershell
npm run verify
npm run test:backend:fresh
npm run check:backend:vet
npm run test:rider-money:mysql
npm run test:notification:mysql
npm run build
git diff --check
```

- [ ] 更新验收记录，只写本轮留存的 exit code。生产迁移/备份/默认关闭部署、微信平台配置、真实支付/通知/次日结算和手机点击跳转各自记录，未观察到的不能填 PASS。
- [ ] 不发真实付款、退款、打款或虚假业务通知。需要外部账号、收件设备或实付金额时停在具体动作前，不能用测试数据替代真实验收。
