# G-⑤ Rider Domain Residue Report v1.0

日期：2026-06-22

实现提交：`745b633c981ccd092a52b507258d79296dbc5492`

结论：**PASS，测试残留为 0**

## 1. 数据库残留

| 项目 | 结果 |
| --- | --- |
| `takeout_riders` 验收数据 | 0 |
| `takeout_fulfillments` 验收数据 | 0 |
| G-⑤ 测试订单 | 0 |
| G-⑤ 测试订单明细 | 0 |
| G-⑤ 测试应结记录 | 0 |
| G-⑤ Catalog 测试菜品 | 0 |
| `takeout_rider_config/global` | 恢复 `1000,row_version=1` |
| `takeout_fee_config/global` | 恢复 `100/50,row_version=1` |
| 020 新表外键 | 0 |
| 最终迁移状态 | 020 已应用，三表和种子保留 |

## 2. 文件与构建残留

- 临时 `node_modules` junction：0；
- MiniApp/Admin 临时 `dist`：0；
- 临时数据库文件、二进制、缓存、日志：0；
- `.env`、DSN、token、云凭证进入 Git：0。

## 3. 范围残留

- HTTP/Gin Route/Controller/Admin/MiniApp 改动：0；
- 支付/F/E/Media/A～F 接线：0；
- 派单/自动分配算法：0；
- Outbox/Consumer：0；
- 迁移 001～019 修改：0；
- `main.go` 修改：0；
- 其它 G 模块或 P0-5 实现：0。
