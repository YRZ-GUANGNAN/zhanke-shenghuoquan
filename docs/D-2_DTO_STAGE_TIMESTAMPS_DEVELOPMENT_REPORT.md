# D-2 DTO Stage Timestamps Development Report

## 1. 结论

D-2 配送单阶段时间 DTO 增量已完成。实现提交：`e861dbc2`。

本补丁仅把 `delivery.Order` 和 `delivery_orders` 中已经存在的 nullable 阶段时间映射到 HTTP DTO；没有修改领域模型、状态机、Repository、Service、CAS、权限、迁移或数据库结构。

## 2. 前置门禁

- 基线：`master@9ce2325c`。
- 标签：`d2-delivery-dispatch-backend-baseline` 存在。
- 主仓库工作区干净。
- `backend/cmd/server` 中 `db.AutoMigrate(` 命中为 0。
- `delivery.Order` 已存在以下 `*time.Time` 字段：
  - `ReadyForClaimAt`
  - `ClaimedAt`
  - `ToPickupAt`
  - `PickedUpAt`
  - `DeliveringAt`
  - `DeliveredAt`
  - `PaymentFailedAt`
  - `CancelledAt`
- 商家 `OrderView` 和骑手 `AssignedOrderView` 原先都只映射 `createdAt/updatedAt`。

## 3. 修改内容

### 商家 DTO

`admin/deliverymerchant.OrderView` 新增：

- `readyForClaimAt`
- `claimedAt`
- `toPickupAt`
- `pickedUpAt`
- `deliveringAt`
- `deliveredAt`
- `paymentFailedAt`
- `cancelledAt`

`orderView(delivery.Order)` 逐字段映射模型中的原始指针值。

### 骑手 DTO

`admin/deliveryrider.AssignedOrderView` 增加同样的 8 个字段，并在 `assignedView` 映射。

抢单池 `ClaimableOrderView` 保持不变，继续只返回最低必要信息；只有已经分配给当前骑手的订单详情/列表透出阶段时间。

### Null 语义

字段不使用 `omitempty`：

- 已发生阶段返回 RFC3339 时间；
- 未发生阶段显式返回 `null`；
- 前端无需推导状态时间。

## 4. 测试增量

- 商家 DTO 单元测试：验证字段映射与缺失阶段 JSON `null`。
- 骑手 DTO 单元测试：验证字段映射与缺失阶段 JSON `null`。
- D-2 真实 HTTP/MySQL 测试：
  - 新建订单：8 个阶段字段全部 `null`；
  - mock 确认费用后：`readyForClaimAt` 非空，其余未发生阶段为空；
  - 抢单并完成履约后：商家详情和所属骑手详情均返回 ready/claimed/toPickup/pickedUp/delivering/delivered 六段时间；
  - `paymentFailedAt/cancelledAt` 保持 `null`。

## 5. 范围证明

提交只包含：

- `backend/internal/module/admin/deliverymerchant/types.go`
- `backend/internal/module/admin/deliverymerchant/types_test.go`
- `backend/internal/module/admin/deliveryrider/types.go`
- `backend/internal/module/admin/deliveryrider/types_test.go`
- `backend/cmd/server/delivery_http_mysql_test.go`

Delivery 核心、迁移、权限中间件、旧外卖和其他业务域均为零差异。
