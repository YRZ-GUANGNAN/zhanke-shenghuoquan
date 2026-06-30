# D-3 配送单资金只读查询 Residue Report

日期：2026-06-30

实现提交：`0d0ae9f41e8ca0ee76374b5c266d5a56ab47ab06`

## 数据库残留

真实 MySQL 8.4 专项测试结束后，由测试清理并再次查询确认：

| 专项数据 | 剩余行数 |
|---|---:|
| 配送单 | 0 |
| 商家钱包 | 0 |
| 钱包流水 | 0 |
| 支付尝试 | 0 |
| 支付通知 | 0 |
| 配送费结算 | 0 |
| 退款记录 | 0 |
| 专项测试用户 | 0 |

清理范围仅使用 D-3 专项 mock-openid、merchant user ID 和专项微信 transaction
ID，不删除既有业务数据。

## 进程与文件残留

- 验收只使用进程内 `httptest.Server`，测试结束自动关闭。
- 未启动常驻后端、Vite、Taro dev server 或 worker。
- 为复用主仓库依赖临时建立的三个 `node_modules` junction 已验证后删除。
- miniapp/admin 构建产物已删除。
- 未复制、打印、提交 `.env`、DSN、token、微信凭证或其他秘密。

## Git 残留

- 代码提交位于 `feat/d3-order-funds`。
- 未合并 `master`，未创建基线标签。
- 未新增或修改迁移。
- 未修改前端、D-4、旧外卖或其他业务域。
- AutoMigrate 仍为 0。

最终残留结论：**PASS，专项数据库、进程、临时依赖联接和构建产物残留均为 0。**
