# D-3 配送费支付、钱包与退款前端 Residue Report

## 1. 结论

**Residue = 0**

本专项创建的数据库测试数据、临时进程、Meilisearch 临时库、构建产物、临时依赖链接和日志均已清理。

## 2. 数据库残留

专项测试账号范围：

- `mock_openid_d3-frontend-e2e-*`
- `mock_openid_d3-frontend-devtools-ui`
- `mock_openid_d3-frontend-devtools-poor`

清理后真实 MySQL 8.4 查询结果：

| 对象 | 残留数 |
|---|---:|
| users | 0 |
| delivery_orders | 0 |
| delivery_merchant_wallets | 0 |
| delivery_wallet_ledger | 0 |
| delivery_payment_attempts | 0 |
| delivery_fee_settlements | 0 |
| delivery_refunds | 0 |
| delivery_payment_notifications | 0 |

对应 `delivery_outbox` 事件在删除订单前按订单 public id 同步删除。

## 3. 进程与端口

- 专项 `cmd/server` 进程已停止。
- Go 临时编译子进程已停止。
- 专项 Meilisearch 进程已停止。
- `127.0.0.1:8080`：无监听。
- `127.0.0.1:7700`：无监听。
- 本专项打开的微信开发者工具项目窗口已关闭。

## 4. 文件系统残留

已删除：

- `%TEMP%\d3-frontend-*`
- `%TEMP%\d3-meili-*`
- 专项 Meilisearch 临时数据库
- `apps/miniapp/dist`
- `apps/admin/dist`
- `apps/miniapp/.swc`
- 为 worktree 临时创建的三个 `node_modules` Junction

未修改或提交：

- `.env`
- 微信支付、数据库或其他密钥
- 本地运行日志
- 构建产物
- 测试 Token

## 5. Git 状态

实现提交已完成，报告提交后工作树应保持干净。分支只包含 D-3 前端实现和本专项报告，不包含后端、迁移、社区、二手、媒体、IM 或 D-4 内容。
