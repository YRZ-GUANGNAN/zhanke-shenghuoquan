# 真实微信账号与稳定用户号验收报告

- 验收日期：2026-07-19
- 代码基线：`870cd0e`
- 结论：代码、真实微信配置、本地数据库迁移、稳定用户号和 mock 用户只读审计通过；真机体验和发布仍为 `PENDING`

## 已完成的代码能力

| 检查项 | 结果 | 说明 |
| --- | --- | --- |
| 真实微信登录 | PASS | 后端通过 `jscode2session` 获取真实微信身份；上游错误不回显请求参数、code、Secret 或完整 URL。 |
| 稳定用户号 | PASS | 同一 OpenID 重复登录复用同一用户；并发首次登录通过唯一约束冲突回读最终用户，避免生成多个用户号。 |
| mock 与真实凭据互斥 | PASS | 真实 AppID/Secret 与 `ALLOW_MOCK_WECHAT_LOGIN=true` 同时存在时配置加载失败，不能用 mock 冒充真实登录。 |
| 密钥提交保护 | PASS | 检查 Git index 中的 UTF-8/UTF-16、PowerShell、YAML、JSON、单行和紧邻跨行赋值；占位符允许，真实形态值拒绝，报错不输出值。 |
| mock 用户审计 | PASS | `audit-mock-users` 已对当前本地数据库执行只读审计；未删除、合并或改绑任何用户。 |
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
| `npm.cmd run check:miniapp-capsule-layout` | PASS |
| `npm.cmd run check:home-real-tools-contract` | PASS |
| `npm.cmd run check:miniapp-static-routes` | PASS（24 条路由） |
| `npm.cmd run check:miniapp-business-closure` | PASS（24 条已注册路由） |
| 公网 API 生产构建 | PASS（`Compiled successfully`） |
| 生产产物地址与页面配置 | PASS（公网 API 命中 1 个文件、本地 API 命中 0 个文件、`PAGES=24`） |

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
- MySQL 8.4 已应用迁移 034，四张身份表完整，原有 44 个用户内容未被删除或重写；登录、用户资料和身份接口均返回 HTTP 200。
- 同一微信账号两次仅清除 `domainA.token` 后重新登录，“我的”页面显示相同用户号；报告不记录该号码的具体值。
- mock 用户只读审计共识别 36 个历史 mock 用户：其中 23 个仅为后续人工核查候选，13 个存在业务关联并受保护；关联数据合计为 7 个商品和 14 条 IM 关系，帖子、配送单和身份申请均为 0。候选身份不代表可以删除，本次没有执行任何清理、合并或改绑。

因此当前环境已退出 mock 登录模式，真实微信账号与稳定用户号验收通过。

## 验收状态

| 场景 | 状态 | 完成条件 |
| --- | --- | --- |
| 真实微信凭据切换 | PASS | 本机未跟踪的 `backend/.env` 已配置匹配的 AppID/Secret 并关闭 mock；Secret 未进入报告、聊天、日志或版本库。 |
| 后端健康检查 | PASS | 当前代码已使用真实配置启动，且 `/healthz` 成功。 |
| 稳定用户号真验 | PASS | 同一微信账号连续两次仅清除 `domainA.token` 后登录，“我的”页面用户号完全一致。 |
| mock 用户数据库审计 | PASS | 已完成当前本地数据库只读审计：36 个历史 mock 用户中，23 个为后续人工核查候选，13 个因业务关联受保护；本次未修改数据。 |
| 微信胶囊独占行 | PENDING | 胶囊布局静态契约、MiniApp 类型检查及公网 API 生产构建已通过；仍需在真实设备逐页验证视觉位置与取消、保存、发布、搜索、返回、主页和身份切换等按钮可点击，代码与构建结果不能替代真机验收。 |
| 真机与发布 | PENDING | 真机验证登录、胶囊避让、真实工具跳转和资料编辑；之后才可考虑上传体验版或发布。 |

## 下一步手册

按 `docs/runbooks/REAL-WECHAT-ACCOUNT-CUTOVER.md` 继续真机验证登录、胶囊避让、真实工具跳转和资料编辑。真机证据完整前不上传、不发布，也不把本地代码与数据库 PASS 写成发布验收完成。
