# ADR-001：重开范围，引入外卖业务域 G

> 建议存放路径：`docs/adr/ADR-001-reopen-takeout-domain-G.md`

- **状态**：草案（待 owner 接受）
- **日期**：2026-06-22
- **决策者**：owner
- **相关文件**：`ARCHITECTURE.md`、`README.md`、`DEVELOPMENT.md`、`docs/DEVELOPMENT_DISCIPLINE.md`

---

## 背景

项目此前按“半成品系统收敛重构”固定为 A–F 六业务域，并明确删除旧原型的订单与支付能力：

- `ARCHITECTURE.md` 第 43 行：`sold` 为终态；旧的订单模型已删除，当前系统不承担支付或自建订单能力。
- `ARCHITECTURE.md` 第 58 行：已删除订单表 / 接口、JSON 商品图片、工具链接单表、分散审核日志、mock 混合数据服务。
- `ARCHITECTURE.md` 第 60 行：不恢复未归类的关注、粉丝、**支付**、公告、通用上传或系统设置功能。
- `DEVELOPMENT.md`：无法归属 A–F 的需求不进入当前系统。
- 迁移 `003_domain_convergence.sql` 专门清理了旧原型订单表；真实库现有 31 张表，**无任何** order / merchant / dish / payment / rider 表。

owner 决定重新引入“校园外卖”。外卖的本质是**下单 + 支付 + 配送**，无法归入 A–F 任何一域——它恰好需要上述被删除并禁止恢复的两样（订单、支付）。因此这不是一次普通的功能新增，而是对一个**已冻结架构决策的有意反转**，必须以本决策记录显式登记，而非以一个与现行架构矛盾的模块绕过。

## 决策

1. 正式引入第七业务域 **G 外卖**，与 A–F 同级，遵循同一套开发纪律（见 `DEVELOPMENT_DISCIPLINE.md`）。
2. **在且仅在域 G 范围内**，反转以下既有条款，重新引入订单与支付能力：
   - `ARCHITECTURE.md` 第 43、58、60 行关于“订单 / 支付已删除且不恢复”。
   - 反转仅适用于域 G；A–F 与媒体子系统的相应约束**不变**。
3. 域 G 登记：
   - 后端目录：`internal/module/takeout`
   - 入口：`services/g-takeout.ts`；路由根 `/takeout`（`/api/v1/takeout`）
   - 自有数据：仅登记域名义；**具体表由 G 各模块迁移定义，本 ADR 不建表**。
4. 依赖规则（复用而非重建）：
   - 身份与权限复用 **A**；G 不自建 token / 权限。买家、商家、骑手均为 `user` + 角色。
   - 菜品图、商家图走**媒体子系统**：通过 `media_asset_references` 以新 purpose（如 `dish_image`、`merchant_logo`）引用，展示经 **MediaGateway**（仅 `final + approved`）；G 不新建任何媒体逻辑。
   - 商家入驻、菜品上架审核提交 **F** 统一审核流（`review_tasks`）；F 经注册领域服务回写，不直接访问 G 表。
   - 买家↔商家、买家↔骑手会话经 **E** 的 `ConversationGateway`；E 不读 G 表、不存消息。
   - G 自有仓储只访问 G 自有表；禁止跨域外键。
5. 实现范式与 A–F / 媒体一致：状态机用 `expected_status + row_version` 的 CAS；副作用走 Outbox → Consumer + Inbox 去重；全程 fail-closed；真实 MySQL 8.4 + 真实 STS/COS 验收；残留 = 0。
6. 模块拆分（各自专项授权、PASS 后冻结；迁移号自 `016` 起，每条配 `rollback/`）：

   | 模块 | 范围 | 复用 |
   | --- | --- | --- |
   | G-① Merchant | 商家实体、入驻、营业状态机 | A 身份；logo 引用 media |
   | G-② Catalog | dish / category 元数据、上下架状态机 | 菜品图引用 media，展示走 MediaGateway |
   | G-③ Order | orders / order_item、下单、订单状态机（CAS）、金额服务端重算、下单快照 | A 身份 |
   | G-④ Payment | 微信支付下单 + 回调，结构对齐 Ticket + Completion + Outbox；幂等、退款、fail-closed | 媒体上传链路同构 |
   | G-⑤ Delivery / Rider | rider 实体、抢单（CAS，无派单算法）、配送状态推进 | A 身份 |
   | G-⑥ Admin 接线 | 商家 / 菜品 / 订单管理 + 接入 F 统一审核流 | F 审核流 |
   | G-⑦ Miniapp 接线 | 小程序页面 + 业务接线、`Taro.requestPayment`、上传走既有 media 链路 | A / E / media |

   购物车置于前端 Zustand，不单列后端模块。

## 影响与风险

### 范围与纪律
- A–F 收敛被**有意放宽**：系统重新承担订单与支付。后续不得以“既然 G 能扩，别处也能扩”类推；任何其它域扩张仍需新的 ADR。
- 媒体子系统（Module ②~⑩）与 A–F 契约不受本 ADR 影响，保持冻结。

### 支付与资金（owner 需在 G-④ 启动前确认）
- 微信支付商户资质、商户号与收款主体；
- 资金存管、退款、对账口径；
- 平台与商家 / 骑手的结算方式（建议初期：平台单商户号收款 + 后台统计应结 + 线下 / 周期结算，**不做实时分账**，量大后再上分账）；
- 校园场景下支付与交易的资质与合规边界。

> 以上为重新引入支付的**前置条件**；未明确前 G-④ 不应启动。

### 数据与一致性
- G 引用 media 与 F 审核，必须经既有网关 / 服务，不得绕过；
- 订单 / 支付状态推进一律 CAS + Outbox，避免重复扣款 / 超抢。

## 后续

1. 据本 ADR 修订 `ARCHITECTURE.md`、`README.md`、`DEVELOPMENT.md`（登记域 G、对“支付 / 订单”禁止项加例外指针 → 本 ADR）。
2. owner 接受本 ADR 后，按 G-①…G-⑦ 逐条专项授权。
3. 本 ADR 不定义任何表结构；迁移自 `016` 起，每条配 `rollback/`。

---

> **待 owner 填实 / 修订**：背景动机的取舍、支付合规与结算主体的最终结论、模块优先级与排期。本草案为讨论基线。
