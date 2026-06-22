# G-⑥a-2 Residue Report v1.0

日期：2026-06-22

## 数据库残留

真实 MySQL 8.4 验收完成并执行清理后：

- `review_tasks` 中 `risk LIKE 'g6a2-%'`：0；
- 测试商家：0；
- 测试菜品：0；
- 测试用户：0。

## 文件残留

- 临时 `node_modules` junction：0；
- MiniApp/Admin `dist`：0；
- 临时数据库文件、日志或二进制：0；
- `.env`、DSN、JWT、云凭证进入 Git：0。

## 范围残留

- 迁移变更：0；
- F review 包变更：0；
- G 状态机/CHECK 变更：0；
- G-⑥a-1 Handler 行为变更：0；
- Admin 前端或提交入口：0。

## 已登记但未整改的系统风险

F 的 Target 回写与 Task 保存不在同一事务。CAS 竞争时本次实测 Task 保持 `reviewing`、Merchant 保持 `pending_review`，调用方收到 HTTP 400。该风险不属于本次授权整改范围，并未被掩盖。
