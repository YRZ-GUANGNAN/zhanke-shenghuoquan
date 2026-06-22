# G-③ Order Domain Residue Report v1.0

日期：2026-06-22

实现提交：`95bdabbe411428e06eae14e107c825083c1ebc80`

结论：**PASS，测试残留为 0**

## 1. 数据库残留

| 项目 | 结果 |
| --- | --- |
| `takeout_orders` 验收数据 | 0 |
| `takeout_order_items` 验收数据 | 0 |
| `takeout_rider_earning_records` 验收数据 | 0 |
| G-③ 测试商户对应 Catalog dishes/categories | 0 |
| `takeout_fee_config/global` | 恢复 `100/50/row_version=1` |
| 019 新表跨域外键 | 0 |
| A～F/Media 数据写入 | 0 |
| 最终迁移状态 | 019 已应用，四表与种子配置保留 |

迁移测试在确认订单、明细、应结数据为空且费率为预期种子后执行 rollback，随后重新 apply 019。

## 2. 文件与构建残留

- 临时 `node_modules` junction：0；
- MiniApp/Admin 临时 `dist`：0；
- 临时数据库文件、二进制、缓存、日志：0；
- `.env`、DSN、token、STS/COS 凭证进入 Git：0。

## 3. 范围残留

- 新 HTTP/Gin Route/Controller：0；
- Admin/MiniApp 业务改动：0；
- F/E/真实支付/G-⑤/Media 接线：0；
- Outbox/Consumer：0；
- 迁移 001～018 修改：0；
- `main.go` 修改：0；
- G-④～G-⑦ 实现：0；
- P0-5 实现：0。

