# G-① Merchant Domain Residue Report v1.0

日期：2026-06-22

结论：**PASS，测试残留为 0**

## 数据库

| 项目 | 结果 |
| --- | --- |
| `takeout_merchants` 测试数据 | 0 |
| G-① acceptance owner 区间数据 | 0 |
| 迁移最终状态 | 016 已应用，表结构保留 |
| 跨域外键 | 0 |
| A～F/Media 数据写入 | 0 |

迁移 apply/rollback/apply 仅操作新建的 G 域表。回滚前有非空保护，禁止误删已有商户数据。

## 文件与构建

- 临时 worktree 依赖目录联接在验收后移除；
- MiniApp/Admin 生成的 `dist` 构建目录在验收后清理；
- 未新增二进制、缓存、日志或临时数据库文件；
- `.env`、数据库 DSN、token、STS/COS 凭证均未纳入变更。

## 范围残留

- 新 HTTP/Gin Route：0
- 新 Controller：0
- 新 Admin/MiniApp 业务文件：0
- 新支付/IM/F 审核接线：0
- 对迁移 001～015 的修改：0
- 对 P0-1～P0-4、A～F 或 68 项漂移的修改：0
- G-②～G-⑦ 实现：0

G-① 完成后保持停止状态，等待 owner 验收与合并。
