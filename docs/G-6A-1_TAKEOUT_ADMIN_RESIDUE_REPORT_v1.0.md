# G-⑥a-1 Residue Report v1.0

日期：2026-06-22

## 数据库残留

真实 MySQL 8.4 验收结束后：

- 测试 users：0；
- 测试 merchants：0；
- 测试 categories：0；
- 测试 dishes：0；
- 测试 orders/order_items：0；
- 测试 riders：0；
- 测试 withdrawals：0；
- 测试 earning records：0；
- fault-injection 部分 settled：0；
- `takeout_fee_config`：恢复 `global / 100 / 50 / row_version=1`；
- `takeout_rider_config`：恢复 `global / 1000 / row_version=1`。

## 文件残留

- 临时 `node_modules` junction：0；
- MiniApp/Admin `dist`：0；
- 临时数据库文件、日志、二进制：0；
- `.env`、DSN、token、云凭证进入 Git：0。

## 范围残留

- F 审核接入：0；
- Admin 前端页面：0；
- Apps 改动：0；
- 001～020 迁移改动：0；
- boundary 放宽：0；
- takeout 核心 HTTP/A～F import：0；
- P0-5 或其它 G 模块实现：0。
