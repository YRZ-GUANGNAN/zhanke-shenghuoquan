# 骑手钱包与平台待打款队列设计

日期：2026-10-25。基线：当前分支 `codex/native-theme-motion-20261025`，支付资金核验提交 `5d12db4`。

状态：设计已根据用户选择拆分为“本批可施工”和“微信通道后接”。本设计不启用真实转账、不执行生产迁移、不把线下结款登记包装为微信到账。

## 1. 目标与边界

本批完成两个正式入口：

1. 商家钱包：沿用现有蓝色视觉，展示服务端余额、资金状态、待核实提示、分页账单，并修复支付能力检查、刷新失败和充值原单恢复语义。
2. 骑手收入：注册正式 `rider-wallet` 页面，从骑手“我的”进入；展示本人可用结算余额、今日完成单量/收入、待后台结算、已登记线下结算和分页收入流水。骑手端只读，不提供充值、提现或后台线下结款按钮。

平台内部待打款队列作为后续可施工的内部状态边界：允许后台看到“待打款/处理中/已完成/失败”队列和幂等业务号，但当前不调用微信转账，不声称到账，不改变现有 `RiderPayoutRecord` 的“线下实付登记”含义。微信商家转账适配器、收款授权、回调验签、自动 worker 和真实打款属于下一阶段。

## 2. 数据与接口

### 2.1 骑手只读接口

在 `backend/internal/module/admin/deliveryrider` 的正式骑手路由下新增 `GET /delivery/rider/wallet`，继续使用登录和骑手身份守卫。服务端从会话取得 `riderUserID`，不得接受客户端传入用户 ID。

返回聚合对象：

- `wallet`: `walletPublicId`、`availableBalanceCents`、`rowVersion`、`updatedAt`；
- `today`: 复用已有上海时区“已完成且产生骑手收入记录”的今日统计；
- `earnings`: 分页收益记录，包含订单公开编号、金额、`pending/settled/paid_offline` 状态、创建时间和线下结算时间；
- `ledger`: 分页骑手流水，包含方向、类型、金额、余额快照、引用编号和创建时间。

读取必须是只读查询，不调用会创建钱包的 `RiderWallet()` 命令方法；没有钱包时返回零余额和空列表，并明确 `fundingStatus=unavailable`，不虚构收入。

### 2.2 商家钱包

保留既有 `/delivery/merchant/wallet`、`/ledger` 和充值接口。正式页面统一使用支付核验字段：`providerStatus`、`nextVerifyAt`、`verifyAttemptCount`。充值请求在网络未知、取消支付或页面卸载后保留原 `clientRequestId/paymentPublicId`，刷新时查询原单，不创建第二笔订单。

### 2.3 平台待打款队列（后续阶段接口边界）

内部记录至少包含：`payoutRequestPublicId`、骑手 ID、金额、收益分配快照、幂等键、`queued/processing/succeeded/failed/unknown` 状态、失败分类、策略版本和审计时间。当前只允许内部排队与后台查询；任何“成功”必须有平台内部明确结算凭证，不能由 HTTP 2xx 或人工点击产生。

## 3. UI 设计

- 钱包主卡保留当前蓝色渐变和原返回按钮；不换主题、不增加玻璃材质或新导航。
- 商家顺序：余额主卡 → 资金状态/待核实说明 → 账单流水。刷新为现有触控规范，失败保留已知余额和流水，提供原地重试。
- 骑手顺序：余额主卡 → 今日单量/今日收入双指标 → 待结算/已线下结算指标 → 收入流水。状态 chip 必须区分“待结算”“已登记线下结算”“已完成”，不写“微信到账”。
- 骑手页面不出现充值、提现、自动打款或改变结算状态的操作；说明文案明确“平台后台结算/线下登记”。
- 空态为“说明 + 刷新/返回”，错误为“保留已有数据 + 重试”，禁止把能力检查中/网络故障显示为“未开通”。
- 继续遵守当前页面的身份守卫、账号/token/卸载隔离、手势返回与 44px 触控目标。

## 4. 错误与资金安全

- 读取失败不能清空上一份已确认余额或流水；各分区独立显示错误。
- 骑手收益统计、可用余额、待打款金额分开显示，不能用今日收入替代可提现余额。
- `paid_offline` 只表示后台有线下付款登记；它不是微信支付结果，也不触发骑手端“到账”提示。
- 未知/超时状态只允许查原记录、重试读取或进入内部队列；不创建新单、不释放资金、不改成成功。
- 所有新的内部队列写操作使用服务端幂等键和 CAS；前端防重只作为体验层保护。

## 5. 文件范围

本批预计修改：

- `apps/miniapp/src/pages/merchant-wallet/index.tsx`、`index.scss`；
- `apps/miniapp/src/pages/rider-wallet/index.tsx`、`index.scss`、`index.config.ts`；
- `apps/miniapp/src/pages/delivery-profile/index.tsx`、`apps/miniapp/src/app.config.ts`；
- `apps/miniapp/src/services/delivery-payment.ts` 与新增骑手资金 service；
- `backend/internal/module/admin/deliveryrider/**`、`backend/internal/module/delivery/**` 的只读查询和 DTO；
- `backend/cmd/server/main.go` 的路由组合；
- 对应契约测试和验收脚本。

不修改遗留未注册的 `delivery-wallet`、`delivery-merchant` 页面；不向骑手暴露 Admin API；不改帖子、二手、学生端或无关 UI 动效。

## 6. 验收

必须通过：

- 骑手/商家 service、路由、身份越权和空钱包单测；
- 账号切换、token 变化、页面隐藏/卸载、同帧重复刷新/充值、原支付单恢复、分页去重与错误保留测试；
- `npm run check:miniapp-static-routes`、`npm run check:miniapp-business-closure`、`npm run check:role-workbench-ui`、`npm run check:delivery-stage-workbench`、`npm run typecheck`、`npm run build`、`git diff --check`；
- `go test ./...`、相关 `go vet`。

真实微信商家转账、生产数据库迁移、自动 worker 和正式打款均保持 PENDING，必须在下一阶段完成独立设计、资质核验和隔离 MySQL/微信联调后才能启用。
