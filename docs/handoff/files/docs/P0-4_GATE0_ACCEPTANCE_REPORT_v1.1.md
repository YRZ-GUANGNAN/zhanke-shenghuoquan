# P0-4 Gate-0 Acceptance Report v1.1

版本：v1.1  
日期：2026-06-20  
阶段：Gate-0 STS Region Fix 与真实 COS 复验  
依据：[P0-4 Media Pipeline Design v1.1（Final）](./P0-4_MEDIA_PIPELINE_DESIGN_v1.1_FINAL.md)  
结论：**FAIL — P0-4 继续冻结，禁止进入 Module ②**

## 1. 授权范围

本次仅允许：

- 修复 COS/STS 适配器的 STS Region 初始化；
- 修改 Gate-0 真实环境测试代码；
- 在真实腾讯云 STS/COS 环境执行完整入口策略测试；
- 清理 Gate-0 测试对象和 multipart 残留。

未修改业务域、数据库、媒体模型、审核、小程序、管理端或 P0-1～P0-3；未启动 Module ②～⑩ 或 P0-5。

## 2. 修改文件清单

| 文件 | 修改内容 |
| --- | --- |
| `backend/internal/pkg/media/cos.go` | STS Client 初始化从空 Region 改为使用构造参数 `region` |
| `backend/internal/pkg/media/gate0_cos_acceptance_v11_test.go` | 新增真实 COS 完整 Gate-0 验收矩阵、独立 Key、权限负向测试及版本/multipart 清理 |
| `backend/internal/pkg/media/gate0_cos_acceptance_strict_test.go` | 负向上传仅接受 403 权限/签名拒绝，禁止把 404 基础设施错误误判为 PASS |
| `docs/P0-4_GATE0_ACCEPTANCE_REPORT_v1.1.md` | 本脱敏验收报告 |

核心修复：

```go
sts.NewClient(credential, region, clientProfile)
```

除此之外未修改产品代码。

## 3. 真实环境

| 项目 | 状态 |
| --- | --- |
| 腾讯云 SecretId/SecretKey | 已配置，报告不输出值 |
| COS Bucket | 已配置，名称脱敏 |
| COS Region | 已配置，值脱敏 |
| COS AppID | 从 Bucket 后缀推导，值脱敏 |
| STS | 真实腾讯云 `GetFederationToken` |
| COS | 真实腾讯云 COS Endpoint |
| Mock | 未使用 |

## 4. STS Region Fix 验证

修复前：

```text
GetFederationToken -> MissingParameter: Region
```

修复后，真实 `GetFederationToken` 成功返回临时凭证，测试继续生成带 session token 的精确 Key 预签名 PUT，并实际请求 COS Endpoint。

判定：**PASS**。

报告和测试输出均未打印临时 Secret、SessionToken 或预签名 URL。

## 5. 测试实现

每项签名篡改测试使用独立随机 Key，避免“对象已存在”掩盖签名验证：

- `staging/gate0-v11/<nonce>/length.png`；
- `staging/gate0-v11/<nonce>/md5.png`；
- `staging/gate0-v11/<nonce>/type.png`；
- `staging/gate0-v11/<nonce>/exact.png`。

测试计划覆盖：

- 精确 Key 单 PUT；
- Content-Length、Content-MD5、Content-Type 篡改；
- 禁止覆盖；
- 其他 staging Key、final Key；
- GetObject、DeleteObject、ListBucket；
- multipart initiate/upload-part/list-parts/complete/abort/list-uploads；
- 对象全部版本、delete marker 和 multipart 残留清理。

负向用例只有收到 HTTP 403 才能判定权限/签名门禁 PASS。400、404、网络失败或其他基础设施错误均判 FAIL。

## 6. 真实环境测试结果

执行命令：

```text
go test ./internal/pkg/media -run '^TestGate0TencentCOSUploadBoundaryV11$' -v -count=1
```

严格复验的脱敏结果：

```text
=== RUN   TestGate0TencentCOSUploadBoundaryV11
Content-Length tamper was not rejected by COS authentication/policy:
status=404 NoSuchBucket
Gate-0 cleanup failed:
status=404 NoSuchBucket
--- FAIL: TestGate0TencentCOSUploadBoundaryV11
FAIL
```

真实腾讯云返回 `NoSuchBucket`，说明当前配置的 Bucket/Region 组合不存在或不可定位。因为 STS 已成功且请求已到达 COS，这不是 STS Region Fix 失败，也不是 Mock、编译或本地网络替代结果。

第一次执行中，长度/MD5/MIME 三项都收到 404。测试最初仅按“非 2xx”判断，可能产生错误 PASS 日志；随后已收紧为“必须 403”，重新执行后正确判定 FAIL。第一次的三项结果全部作废，不作为验收证据。

## 7. Gate-0 验收矩阵

| 验收项 | 结果 | 证据/原因 |
| --- | --- | --- |
| STS Client 使用配置 Region | PASS | GetFederationToken 已成功并进入 COS 请求 |
| 精确 staging Key 单 PUT | FAIL / BLOCKED | COS 返回 NoSuchBucket |
| Content-Length 篡改拒绝 | FAIL / INVALID | 返回 404，不是预期 403 |
| Content-MD5 篡改拒绝 | NOT VALIDATED | Bucket 基础门禁失败 |
| Content-Type 篡改拒绝 | NOT VALIDATED | Bucket 基础门禁失败 |
| 原授权重复 PUT 覆盖拒绝 | NOT VALIDATED | 正向 PUT 未成功 |
| 其他 staging Key 越权拒绝 | NOT VALIDATED | 未到权限测试阶段 |
| final Key 写入拒绝 | NOT VALIDATED | 未到权限测试阶段 |
| GetObject 拒绝 | NOT VALIDATED | 未到权限测试阶段 |
| DeleteObject 拒绝 | NOT VALIDATED | 未到权限测试阶段 |
| ListBucket 拒绝 | NOT VALIDATED | 未到权限测试阶段 |
| multipart initiate 拒绝 | NOT VALIDATED | 未到权限测试阶段 |
| multipart upload part 拒绝 | NOT VALIDATED | 未到权限测试阶段 |
| multipart list parts 拒绝 | NOT VALIDATED | 未到权限测试阶段 |
| multipart complete 拒绝 | NOT VALIDATED | 未到权限测试阶段 |
| multipart abort 拒绝 | NOT VALIDATED | 未到权限测试阶段 |
| multipart list uploads 拒绝 | NOT VALIDATED | 未到权限测试阶段 |
| 对象与 multipart 清理 | BLOCKED | Bucket 返回 NoSuchBucket，无法列举验证 |

## 8. 清理结果

测试在第一个可用对象写入前被 `NoSuchBucket` 拒绝，因此没有成功创建测试对象或 multipart uploadId。

测试仍使用服务身份执行版本列表和 multipart 列表清理，但同样返回 `NoSuchBucket`。因此：

- 没有已知成功创建的测试对象；
- 没有已知成功创建的 multipart；
- 无法通过 Bucket 列表形成“清理后为空”的正向证据；
- 清理验收不能判 PASS。

## 9. 当前阻断原因

Gate-0 当前唯一先决阻断是：

```text
configured COS Bucket/Region -> 404 NoSuchBucket
```

可能原因仅记录为待外部确认，不在本次授权范围内擅自修改：

1. Bucket 尚未创建；
2. 环境变量中的 Bucket 名称错误；
3. Bucket 实际位于其他 Region；
4. 使用了不属于当前腾讯云账号的 Bucket 标识。

不得通过改用 Mock Bucket、本地对象存储、跳过正向 PUT 或只保留 STS 签发成功来判定 Gate-0 PASS。

## 10. 已知问题与未完成项

### 已知问题

- 真实 COS Bucket/Region 配置不可用；
- Gate-0 的对象入口与权限矩阵尚未形成有效证据；
- 清理后的空列表证据尚未形成。

### 未完成项

- Gate-0 除 STS Region Fix 外的全部 COS 验收项；
- Module ②～⑩ 全部未启动；
- P0-5 未启动。

## 11. 继续 Gate-0 的前置条件

需要由项目方在腾讯云控制台确认并提供同一账号下真实存在的：

- 完整 Bucket 名称（包含 AppID 后缀）；
- Bucket 实际 Region；
- 当前 SecretId 对该 Bucket 的服务端清理权限。

配置修正后必须从头执行完整 Gate-0，不能只从失败点续算，也不能复用本报告中的无效 404 结果。

## 12. 最终判定

- STS Region Fix：**PASS**；
- Gate-0 总体：**FAIL**；
- 是否允许进入 Module ②：**否**；
- 是否允许降级：**否**；
- 是否修改冻结业务域：**否**；
- 是否启动 P0-5：**否**；
- 当前状态：**P0-4 Development 继续冻结，等待真实 COS Bucket/Region 配置修正后完整重测**。
