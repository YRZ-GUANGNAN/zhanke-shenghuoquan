# 外部订单推送底层 Implementation Plan

> 执行方式：沿用用户全自动实施授权，在当前隔离 worktree 内逐项实现和自审，不部署生产、不配置第三方密钥。基线 b419240，分支 codex/order-intake-market-20261017。

**Goal:** 建立经过平台适配器验证的订单通知 → 持久化收件箱 → 现有配送建单的可运行底层，未来接入官方 API 时复用。

**Architecture:** 可信商家绑定解析器与平台适配器注入；回调原文、签名和凭据不落库，只保存经过校验的标准事件。收件箱具备事件幂等、内容摘要冲突检查、租约、退避重试、终止失败及人工重试入口。建单和处理成功在同一事务提交，使用原费率/待支付状态，不触发扣款或骑手通知。

**Tech Stack:** Go、GORM、MySQL 8、现有配送状态机。

## 明确边界与自审

- `Receive(ctx, bindingKey, platform, envelope)` 先解析服务器商家绑定，再经注册适配器验证签名/时间戳/重放，再标准化；缺少适配器或有效绑定拒绝。
- 绑定解析器必须检查店铺授权和商家资格，处理队列时再次检查，撤销或改绑的事件进入人工处理，不自动创建跨商家订单。
- 同平台/店铺/事件 ID 只落一条；同 ID 内容改变返回冲突；不同事件指向同来源订单复用完全相同的订单快照，差异进入人工处理。
- 新建事件只创建 `pending_fee_payment`。更改/取消事件保留并标为 `needs_review`，不擅自改已付款/履约单或退款。若更改/取消先到，后到的创建事件也进入人工处理，防止旧事件重建订单。
- 网络/数据库故障保留收件箱，可退避重试；进程中断用租约回收；订单与收件箱成功原子提交。错误只存固定分类码，不把手机号、地址或签名写到日志。
- 不添加无鉴权 HTTP 回调，不把连接状态改为已连接；本轮完成的是未来 API 可调用的服务和消费循环。生产开关、数据库和部署不变。

## 文件与执行

- [ ] `backend/internal/module/delivery/external_intake*.go`：收件箱模型、契约、Receive、ProcessNext、Run、Retry、事务仓库。
- [ ] `backend/internal/module/delivery/dispatch_service.go`：抽出已有建单核心以便事务复用，原对外 API 不变。
- [ ] `backend/migrations/046_delivery_external_inbox.sql`：只新增收件箱表，二进制比较来源标识和事件标识，重复执行安全。
- [ ] `backend/internal/module/delivery/external_intake_test.go` 与 `external_intake_mysql_test.go`：验证拒绝签名/绑定不匹配、事件冲突、并发去重、来源重复、改取消乱序、租约过期、重试、无支付和 outbox 副作用。
- [ ] `scripts/test-delivery-source-mysql.cjs --external-intake`：复用独立临时 MySQL，应用到 046，实际完成上述数据库验证，结束只清理临时实例。

接口约定：

```go
type ExternalBindingResolver interface {
    ResolveExternalBinding(context.Context, string) (ExternalShopBinding, error)
}
// 返回的 Receipt 只有接收状态；只有 status=created 才表示本地待支付订单已创建。
// needs_review/deadletter 都不能映射成“已自动发单”。
```

## 验证与交付

先跑配送模块单元测试，再跑独立 MySQL 集成测试；并发通知必须只产生一单，配送订单状态、支付/账本、骑手 outbox 不变。不使用生产测试订单。完成后记录实际通过情况和 API 接入剩余边界，单独保存本范围提交。
