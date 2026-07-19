# 真实微信账号与稳定用户号验收报告

- 验收日期：2026-07-19
- 代码基线：`74b5ee9`
- 结论：代码、真实微信配置加载和本地健康检查通过；真实账号稳定用户号、真实数据库和真机验收仍为 `PENDING`

## 已完成的代码能力

| 检查项 | 结果 | 说明 |
| --- | --- | --- |
| 真实微信登录 | PASS | 后端通过 `jscode2session` 获取真实微信身份；上游错误不回显请求参数、code、Secret 或完整 URL。 |
| 稳定用户号 | PASS | 同一 OpenID 重复登录复用同一用户；并发首次登录通过唯一约束冲突回读最终用户，避免生成多个用户号。 |
| mock 与真实凭据互斥 | PASS | 真实 AppID/Secret 与 `ALLOW_MOCK_WECHAT_LOGIN=true` 同时存在时配置加载失败，不能用 mock 冒充真实登录。 |
| 密钥提交保护 | PASS | 检查 Git index 中的 UTF-8/UTF-16、PowerShell、YAML、JSON、单行和紧邻跨行赋值；占位符允许，真实形态值拒绝，报错不输出值。 |
| mock 用户审计 | PASS（代码） | `audit-mock-users` 为只读命令；其单元测试和静态检查通过。本报告未连接数据库、未删除、合并或改绑用户。 |
| 安全切换手册 | PASS | 已提供不回显 Secret 的本机配置、启动、两次登录和失败回退步骤。 |

## 自动验证证据

| 命令 | 结果 |
| --- | --- |
| `npm.cmd run check:real-wechat-secret-hygiene` | PASS |
| `node --check scripts/check-real-wechat-secret-hygiene.cjs` | PASS |
| `go test ./internal/config ./internal/module/user ./cmd/audit-mock-users` | PASS |
| `go vet ./internal/config ./internal/module/user ./cmd/audit-mock-users` | PASS |
| MiniApp `typecheck` | PASS |
| Admin `typecheck` | PASS |

独立密钥保护复核还验证了临时 Git index、特殊文件名、UTF-16、二进制跳过和主 index 前后不变；自动检查不会读取或打印本机未跟踪的 Secret。

## 当前本机状态

安全检查只读取了非敏感开关，未输出 Secret：

- `backend/.env` 已存在；
- `APP_ENV=dev`；
- `WECHAT_APP_ID=wx0dacef976daa8e55`，与 MiniApp 工程一致；
- `ALLOW_MOCK_WECHAT_LOGIN=false`；
- `WECHAT_APP_SECRET` 已存在，但本报告和日志均未记录其值；
- `MEDIA_PUBLIC_BASE_URL=https://zhanduiwang.top`，公开健康检查返回 HTTP 200；
- 当前代码已使用上述配置启动，`http://127.0.0.1:8080/healthz` 返回正常。
- 使用不可登录的测试 code 直接调用微信 `code2Session` 返回 `40029` 而非 `40125`，证明 AppID/AppSecret 组合已被微信接受；测试请求未产生 OpenID 或用户数据。

因此当前环境已退出 mock 登录模式，但必须完成两次真实登录并比较页面用户号，才能认定真实微信账号验收通过。

## 必须保持 PENDING 的验收

| 场景 | 状态 | 完成条件 |
| --- | --- | --- |
| 真实微信凭据切换 | PASS | 本机未跟踪的 `backend/.env` 已配置匹配的 AppID/Secret 并关闭 mock；Secret 未进入报告、聊天、日志或版本库。 |
| 后端健康检查 | PASS | 当前代码已使用真实配置启动，且 `/healthz` 成功。 |
| 稳定用户号真验 | PENDING | 同一微信账号连续两次仅清除 `domainA.token` 后登录，“我的”页面用户号完全一致。 |
| mock 用户数据库审计 | PENDING | 在明确允许连接目标数据库后运行只读审计；不得据此直接删除、合并或改绑用户。 |
| 真机与发布 | PENDING | 真机验证登录、胶囊避让、真实工具跳转和资料编辑；之后才可考虑上传体验版或发布。 |

## 下一步手册

按 `docs/runbooks/REAL-WECHAT-ACCOUNT-CUTOVER.md` 执行。没有真实凭据和真机证据前，不上传、不发布，也不把代码 PASS 写成发布验收完成。
