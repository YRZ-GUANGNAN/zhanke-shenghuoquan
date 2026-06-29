# D-2 Delivery Dispatch Backend Residue Report

## 1. 判定

**PASS — 验收残留为 0。**

## 2. 数据残留

真实 MySQL 验收使用 D-2 专属 fence。清理顺序为 Outbox → 配送单 → 骑手身份 → 测试用户，未触碰其他既有数据。

最终证据：

```text
dispatch_residue orders=0 outbox=0
```

真实 HTTP 用例通过 `Unscoped` 精确清理 `mock_openid_d2-delivery-dispatch-%` 测试用户，并仅按这些用户 ID 清理对应骑手、配送单和事件。

## 3. 进程和文件残留

- 未启动常驻手工后端进程；测试服务器均由 `httptest.Server` 自动关闭。
- 没有真实企业微信群外呼进程。
- 为复用主仓库依赖临时创建的三个 `node_modules` junction 已全部验证为 ReparsePoint 后删除。
- worktree 中不存在 `node_modules` junction。
- 未创建临时数据库。
- 未修改 `backend/.env`，未提交任何 Secret、DSN、JWT、Webhook URL 或 key。
- 构建产物均由既有 ignore 规则排除，未进入提交。

## 4. Git 范围残留

- 社区、二手、media、IM、apps：相对 `930f7647` 零差异。
- 旧外卖业务实现和迁移：零修改；takeout 仅新增骑手身份窄接口文件。
- AutoMigrate：0。
- D-2 代码提交：`bf386a46`。
- 本分支未合并 master、未打 baseline tag、未进入 D-3。
