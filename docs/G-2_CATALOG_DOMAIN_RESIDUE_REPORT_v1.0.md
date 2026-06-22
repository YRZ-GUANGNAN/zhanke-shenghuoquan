# G-② Catalog Domain Residue Report v1.0

日期：2026-06-22

实现提交：`9a021c88`

结论：**PASS，测试残留为 0**

## 1. 数据库残留

| 项目 | 结果 |
| --- | --- |
| `takeout_dish_categories` 验收数据 | 0 |
| `takeout_dishes` 验收数据 | 0 |
| G-② 固定 merchant ULID 测试区间数据 | 0 |
| 跨域外键 | 0 |
| A～F/Media 数据写入 | 0 |
| 最终迁移状态 | 017 已应用，两张空表保留 |

真实库迁移执行顺序为 apply → rollback → apply。回滚测试仅在 Catalog 两表为空时执行，防止误删已有数据。

## 2. 文件与构建残留

- 临时 `node_modules` junction：0；
- MiniApp/Admin 临时 `dist`：0；
- 临时数据库文件、二进制、缓存、日志：0；
- `.env`、DSN、token、STS/COS 凭证进入 Git：0。

## 3. 范围残留

- 新 HTTP/Gin Route/Controller：0；
- Admin/MiniApp 业务改动：0；
- F/E/支付/Media 业务接线：0；
- 迁移 001～016、018 修改：0；
- `main.go` 修改：0；
- G-③～G-⑦ 实现：0；
- P0-5 实现：0。

