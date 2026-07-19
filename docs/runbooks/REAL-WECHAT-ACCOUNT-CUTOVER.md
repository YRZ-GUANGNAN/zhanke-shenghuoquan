# 真实微信账号切换运行手册

## 适用范围

本手册用于在本机开发环境中，将小程序登录从 mock 回退切换为真实微信账号，并验证同一微信账号连续登录时获得稳定的“用户号”。本手册不执行数据库连接、用户合并、OpenID 改绑或历史数据删除。

## 密钥安全边界

- 只使用受信任的本机编辑器打开 `backend/.env`。
- `WECHAT_APP_SECRET` 只在该未跟踪文件中填写，必须与小程序 AppID 匹配。
- 禁止通过 PowerShell、聊天、文档、截图、日志或工单打印、复制、回显该值。
- 不要对该变量运行 `Get-Content`、`Select-String`、`rg` 或其他会显示内容的命令。
- 不要修改或提交 `.env.example`、`backend/.env.example`、`project.config.json` 或任何示例配置来保存真实凭据。

## 1. 本机配置

在受信任编辑器中编辑 `backend/.env`，确认：

- `APP_ENV` 为 `dev`；
- `WECHAT_APP_ID` 与微信开发者工具当前项目的 AppID 完全一致；
- `ALLOW_MOCK_WECHAT_LOGIN` 为 `false`；
- Secret 项已在编辑器中填写且非空，但不要复制其值进行检查。

只运行以下不会显示 Secret 的 PowerShell 检查：

```powershell
Set-Location 'C:\Users\33141\Documents\New project'
Select-String -LiteralPath 'apps/miniapp/project.config.json' -Pattern '"appid"'
Select-String -LiteralPath 'backend/.env' -Pattern '^(APP_ENV|WECHAT_APP_ID|ALLOW_MOCK_WECHAT_LOGIN)='
```

检查输出应满足：开发者工具 AppID 与后端 AppID 一致，环境为 `dev`，mock 登录为 `false`。如果不一致，先在本机编辑器中修正，不能继续启动验收。

## 2. 启动与健康检查

在终端 A 启动后端：

```powershell
Set-Location 'C:\Users\33141\Documents\New project\backend'
go run ./cmd/server
```

保持终端 A 运行，在终端 B 检查健康状态：

```powershell
Invoke-RestMethod -Uri 'http://127.0.0.1:8080/healthz'
```

只有健康检查成功后才继续。若启动失败，停止操作并按“失败排查”处理，不要临时启用 mock 来伪造通过。

## 3. 连续两次真实登录验收

每次验收都只记录页面展示的“用户号”，不得记录 OpenID、token、手机号或 Secret。

第一次：

1. 在微信开发者工具的调试器中打开 Storage / Local Storage。
2. 仅删除旧的 `domainA.token`，不要复制其值。
3. 重新编译小程序并进入“我的”。
4. 记录页面展示的用户号为“第一次用户号”。

第二次：

1. 再次仅删除 `domainA.token`。
2. 重新编译并进入“我的”。
3. 记录页面展示的用户号为“第二次用户号”。
4. 两次用户号必须完全相同；否则验收失败并停止切换。

完成后在终端 A 按 `Ctrl+C` 停止本地后端。没有明确发布授权时，不执行上传、发布或生产环境变更。

## 4. 失败排查

- 只记录微信返回的 `errcode`、发生时间和操作步骤；不要复制完整请求 URL，因为查询参数可能包含敏感信息。
- 使用第 1 节的安全命令重新确认 AppID 与 mock 开关，不得输出 Secret。
- 只在本机编辑器中目视确认 Secret 项存在且非空，不复制内容。
- 网络错误、微信接口拒绝或无法确认凭据时，将真实微信验收标记为 `PENDING`，不要改用 mock 结果冒充真实账号验收。
- 如需向他人求助，只提供脱敏错误描述和 `errcode`，不提供 OpenID、token、手机号、完整请求或环境文件。

## 5. 旧 mock 数据边界

- mock 用户审计报告仅用于只读评估，不授权删除任何用户或业务数据。
- 即使某个 mock 用户当前没有关联数据，也必须另行确认并取得明确清理授权后才能处理。
- 有帖子、商品、IM 会话、配送单或身份申请等业务关联的数据，必须先设计并评审显式归属迁移方案。
- 本任务不执行用户合并、OpenID 改绑、数据迁移、清理或删除，也不连接数据库进行人工修改。
