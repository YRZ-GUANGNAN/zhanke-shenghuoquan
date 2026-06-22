# P0-4 Module ⑩ Acceptance Evidence — 2026-06-21

## Environment

- Database: real MySQL 8.4.0
- Cloud: real Tencent Cloud STS and COS, Versioning Enabled
- WeChat DevTools: Stable v2.01.2510290
- DevTools SDK: 3.15.2
- MiniApp: Taro + TypeScript
- Date: 2026-06-21

所有标识、账号、Bucket、Key、Token 和签名均已脱敏；报告不保存任何 Secret。

## Real MySQL 8.4

```powershell
cd backend
go test ./internal/module/media/miniappflow -count=1 -v
```

Result: PASS.

验证内容：票据读取、owner 隔离、状态投影、上传历史、清理后 owner `99001001` 关联资产/票据/配额均为 0。

## Real Tencent STS/COS end-to-end

```powershell
cd backend
$env:MEDIA_MODULE10_RUN_REAL_E2E='1'
go test ./internal/module/media/miniappflow `
  -run '^TestMiniAppUploadFlowAgainstRealCOSAndMySQL84$' `
  -count=1 -v
```

Result: PASS.

真实调用覆盖：

```text
MySQL Ticket
  -> Tencent STS GetFederationToken
  -> exact staging-key signed PUT
  -> COS object fixed version
  -> frozen Completion HEAD/GET/Copy
  -> final fixed version
  -> Review Projection
  -> Outbox
  -> Consumer/Inbox
  -> server-owned MiniApp status
```

断言：

- exact single-object PUT PASS；
- fixed source/final version PASS；
- Completion PASS；
- repeated Complete idempotency PASS；
- Outbox count = 1；
- Inbox count = 1；
- cross-owner access rejected；
- COS staging/final versions and delete markers after cleanup = 0；
- database fixture residue after cleanup = 0。

冻结 Module ④ 真实 COS 回归：

```powershell
$env:MEDIA_MODULE4_RUN_REAL_COS='1'
go test ./internal/module/media `
  -run '^TestUploadCompletionAgainstRealCOS$' `
  -count=1 -v
```

Result: PASS.

## HTTP authentication evidence

未经认证访问：

```text
GET /api/v1/miniapp/media/uploads
HTTP 401
{"code":401,"message":"authentication required"}
```

生产注册链：

```text
/api/v1/miniapp/media/*
  -> AuthRequired
  -> UnmutedRequired
  -> owner-scoped Handler/Application Service
```

## WeChat DevTools evidence

官方 CLI：`D:\3314\微信web开发者工具\cli.bat`。官方自动化连接成功并取得：

```text
brand: devtools
model: iPhone12/13 Pro
SDKVersion: 3.15.2
current page: pages/home/index
```

工具内 `wx.login` 成功返回真实 code；未使用 `mockWxMethod`。但 IDE 明确显示当前为测试 AppID `wx961b****f25b`，后端配置为 `wx27a****95ed6`，两者不一致。异步自动化尝试未形成有效数据库记录，测试前后相关 idempotency prefix 记录为 0，也未产生 COS 残留。

结论：真实工具载入与运行 PASS；真实工具内同 AppID 的原生选图上传全链路 BLOCKED，不能以 Go 云端 E2E 替代。

## Full regression

冻结迁移验收测试共享同一数据库，使用串行 package 执行：

```powershell
$env:GOFLAGS='-p=1'
cd backend
go test ./... -count=1
```

Result: PASS.

```powershell
$env:GOFLAGS='-p=1'
npm run verify
```

Result:

```text
check:boundaries PASS
admin typecheck PASS
miniapp typecheck PASS
backend tests PASS
```

```powershell
npm run build:miniapp
```

Result: PASS. Taro production bundle generated; vendors.js size warning only.

## Frozen boundary hashes

迁移 009～014 SHA-256：

```text
009 D4C23ECAB20A3A327E47730652BE165C719414A2C36159E180D87EACC50DE5DE
010 60D1316E5776176A130C0955F306188A003D58A4D8E36286E0BB6ADD7022B870
011 7C1058A57EFE07D23DF76B7E73B0682714344C2BD93A2236C5493E658FC9D34A
012 C98AB592F7C99B01870E13589E9BB528B7411EABF4D90A5D5ED52F855CA94C35
013 A39B8FF80F7FFAF1F137A9EF8D6A2EDFADCCA0E6D1F0A7DD21CFA524531E151B
014 16D59D4B83036B2FAEC711BBDB8BFF477988B06E045B1C22B596DA0C932CBD5E
```

- New migration: 0
- Module ②～⑨ frozen production contract changes: 0
- Direct Community/Trade/User writes: 0
- Direct MiniApp long-term COS credential exposure: 0
- New worker/review vendor/GC behavior: 0

## Gate status

```text
Module ⑩ code and cloud integration: PASS
WeChat DevTools same-AppID native E2E: BLOCKED
Overall: CONDITIONAL PASS
```
