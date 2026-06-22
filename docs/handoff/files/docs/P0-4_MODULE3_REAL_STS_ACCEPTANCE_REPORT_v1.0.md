# P0-4 Module ③ Real STS Acceptance Report v1.0

日期：2026-06-21  
验收对象：腾讯云真实 STS 签发与 Module ③ 最终门禁  
最终结论：**PASS**  
状态变更：Module ③ `CONDITIONAL PASS -> PASS`  
范围声明：本次未进入 Module ④～⑩，未新增 HTTP、业务接线、Copy、审核、Gateway、Consumer 或 GC。

## 1. 脱敏环境证据

| 项目 | 脱敏值 |
| --- | --- |
| 腾讯云 Region | `ap-guangzhou` |
| COS Bucket | `zks***1937` |
| AppID | `******1937` |
| SecretId 指纹 | `sha256:1eedd26c819d...` |
| 凭证文件 | `backend/.env`，已被 Git ignore |

验收过程未输出 SecretId、SecretKey、临时 SecretKey、SessionToken、完整预签名 URL 或 STS 响应正文。代码、迁移、文档和脚本的敏感值扫描结果为 0。

## 2. 真实 STS 执行证据

执行测试：

```text
go test ./internal/module/media \
  -run '^TestTencentUploadAuthorizerAgainstRealSTS$' \
  -count=1 -v
```

脱敏结果：

```text
=== RUN   TestTencentUploadAuthorizerAgainstRealSTS
--- PASS: TestTencentUploadAuthorizerAgainstRealSTS
PASS
ok   campus-circle/backend/internal/module/media
```

该测试通过生产构造器创建腾讯云 STS Client，真实调用 `GetFederationToken`。测试没有 Mock STS，也没有降级到长期密钥签名；调用与服务端预签名总耗时约 0.18 秒。

## 3. STS Policy 审计

真实调用使用与 `TestExactPutPolicyContainsOneKeyAndNoForbiddenAction` 相同的 `buildExactPutPolicy` 代码路径。

| 检查项 | 结果 |
| --- | --- |
| statement 数量 | 1，PASS |
| effect | `allow`，PASS |
| action | 仅 `name/cos:PutObject`，PASS |
| resource 数量 | 1，PASS |
| resource 范围 | 单个精确 staging Key，PASS |
| GetObject/DeleteObject/ListBucket/CopyObject | 均不存在，PASS |
| ACL | 不存在，PASS |
| InitiateMultipartUpload | 不存在，PASS |
| UploadPart/ListParts/Complete/Abort | 均不存在，PASS |

策略没有使用 owner 前缀通配、bucket 通配或 `final/` 写权限。

## 4. 预签名 URL 边界

真实 STS 返回的临时身份只在后端内存中用于生成签名。签名生成后再次校验目标 HTTPS host 和精确对象 path。

| 签名条件 | 结果 |
| --- | --- |
| Content-Length | 已进入 `q-header-list`，PASS |
| Content-MD5 | 已进入 `q-header-list`，PASS |
| Content-Type | 已进入 `q-header-list`，PASS |
| Host | 已进入 `q-header-list`，PASS |
| x-cos-forbid-overwrite | 已进入 `q-header-list`，值为 `true`，PASS |
| x-cos-security-token / q-signature | 均存在，PASS |
| URL TTL | `>0 && <=120s`，PASS |
| 超出票据剩余有效期 | 服务层取两者最小值，PASS |

本测试只签发授权，不执行 Module ④ 的 COS HEAD、对象校验或 Staging→Final Promote。

## 5. 敏感字段审计

`UploadAuthorization` 对外 DTO 仅包含：

```text
TicketID
AssetPublicID
UploadURL
RequiredHeaders
ExpiresAt
```

反射测试确认不存在名称包含 `secret`、`credential`、`sessiontoken` 或 `tmpsecret` 的字段。预签名测试同时确认 URL 不包含长期或临时 SecretKey。

结论：SecretId/SecretKey/SessionCredential 不进入 DTO、数据库、日志或报告。

## 6. Fail-closed 验收

真实 MySQL 8.4 测试 `TestAuthorizationFailureFailClosedAgainstMySQL84` 使用强制 STS 失败路径，并在失败后重新读取数据库：

```text
ticket.status                  cancelled
ticket.credential_issue_count 1
reservation.status             released
ticket.row_version             CAS +1
reservation.row_version        CAS +1
```

结果：PASS。签发失败不会重试同一票据、不会再次增加授权次数、不会遗留 active quota，也不会降级为宽权限凭证。

## 7. 完整回归结果

| 验收命令/检查 | 结果 |
| --- | --- |
| `TestTencentUploadAuthorizerAgainstRealSTS` | PASS |
| `go test ./internal/module/media -count=1`，含真实 STS 与 MySQL | PASS |
| `go test ./... -count=1` | PASS |
| `npm run verify` | PASS |
| A-F + Media 边界检查 | PASS |
| Admin / Miniapp TypeScript | PASS |
| `git diff --check` | PASS；仅既有换行提示 |
| Media→users/posts/products 外键 | 0 |
| Module ③测试数据残留 | 0 |
| 代码/文档敏感值匹配 | 0 |

完整 Media 测试同时覆盖：事务创建、CAS、幂等冲突、owner 越权、授权仅一次、cancel/expire、过期后新票据与新 staging Key、20 路原子配额及失败释放。

## 8. 剩余风险

| 等级 | 风险 | 控制措施 |
| --- | --- | --- |
| 低 | 真实测试服务身份仍保存在本地 ignored `.env` | 禁止提交；测试结束后可轮换或撤销，生产使用独立最小权限身份 |
| 低 | `uploaded` 仍只是上传声明，不代表对象可信 | 必须等待单独授权的 Module ④ 完成 HEAD、内容校验和 Promote；当前不得展示 |

不存在 Module ③ 未关闭的高风险或中风险问题。

## 9. 最终结论

真实腾讯云 `GetFederationToken`、精确单 Key PutObject 策略、短期预签名 URL、敏感字段隔离、真实 MySQL fail-closed、完整 Go Test 与工程 Verify 均已通过。

**P0-4 Module ③ 正式 PASS，并在此冻结。**

本报告只关闭 Module ③ 门禁；未启动 Module ④。后续如需进入 Upload Completion、COS HEAD 与 Staging→Final Promote，必须另行取得专项授权。
