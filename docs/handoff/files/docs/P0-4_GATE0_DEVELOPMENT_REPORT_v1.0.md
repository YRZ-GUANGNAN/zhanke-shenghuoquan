# P0-4 Gate-0 Development Report v1.0

版本：v1.0  
日期：2026-06-20  
阶段：P0-4 Development Phase / Module ① Gate-0  
依据：[P0-4 Media Pipeline Design v1.1（Final）](./P0-4_MEDIA_PIPELINE_DESIGN_v1.1_FINAL.md)  
结论：**FAIL — 已停止 P0-4 后续开发**

## 1. 本模块目标

在真实腾讯云 COS 环境验证以下不可降级入口约束：

1. STS 只允许精确 staging Key 的 PutObject；
2. final Key、其他 staging Key、GetObject 均拒绝；
3. 预签名单 PUT 绑定 Content-Length、Content-MD5、Content-Type；
4. 原授权重复 PUT 不得覆盖对象；
5. multipart 初始化及后续权限必须拒绝；
6. 测试使用真实 COS/STS，不使用 Mock 或本地伪实现。

Gate-0 未通过时，按冻结设计立即停止，不得创建数据迁移、实现 STS 服务或推进后续模块。

## 2. 实现内容

新增真实环境 Gate-0 隔离测试：

- 文件：`backend/internal/pkg/media/gate0_cos_external_test.go`；
- 测试入口：`TestGate0TencentCOSUploadBoundary`；
- 使用独立 `staging/gate0/<nonce>/exact.png` Key；
- 使用真实腾讯云 STS 与 COS Endpoint；
- 凭证仅从本地环境注入，测试输出不打印 Secret、Token 或预签名 URL；
- 计划在成功/失败后使用服务凭证清理测试 Key；
- 正向与负向用例覆盖精确 PUT、长度、MD5、MIME、覆盖、越权 Key、final Key、GET 和 multipart。

未修改任何产品实现。

## 3. 数据库变更

无。

- 未创建或修改迁移；
- 未创建媒体表；
- 未连接业务数据库；
- 未修改 P0-1、P0-2、P0-3 数据结构。

## 4. 接口清单

本模块未新增或修改 HTTP API。

本次仅调用真实腾讯云接口：

| 接口 | 目的 | 实际结果 |
| --- | --- | --- |
| STS `GetFederationToken` | 获取精确 Key、PutObject-only 临时权限 | **FAIL** |
| COS PutObject | 验证精确单 PUT | 未执行 |
| COS HeadObject | 验证对象实物 | 未执行 |
| COS GetObject | 验证临时身份无读权限 | 未执行 |
| COS InitiateMultipartUpload | 验证 multipart 被拒绝 | 未执行 |
| COS DeleteObject | 清理测试对象 | 无对象产生，无需执行 |

## 5. 测试环境

- 腾讯云 COS SecretId：已配置，值未输出；
- 腾讯云 COS SecretKey：已配置，值未输出；
- COS Bucket：已配置，值未输出；
- COS Region：已配置，值未输出；
- COS AppID：从 Bucket 后缀安全推导，值未输出；
- 测试类型：真实腾讯云外部集成测试；
- Mock：未使用。

## 6. 测试结果

执行命令：

```text
go test ./internal/pkg/media -run '^TestGate0TencentCOSUploadBoundary$' -v -count=1
```

结果：

```text
=== RUN   TestGate0TencentCOSUploadBoundary
issue exact-key STS:
[TencentCloudSDKError] Code=MissingParameter,
Message=请求缺少必传参数 `Region`。
--- FAIL: TestGate0TencentCOSUploadBoundary
FAIL
```

测试在 STS 签发阶段失败，未进入任何 COS 对象写入步骤。因此以下 Gate-0 条件均为 **NOT TESTED**，不能判定 PASS：

| 验收项 | 状态 |
| --- | --- |
| 精确 staging Key 单 PUT | NOT TESTED |
| Content-Length 精确绑定及 ±1 拒绝 | NOT TESTED |
| Content-MD5 正确校验及错误拒绝 | NOT TESTED |
| Content-Type 绑定 | NOT TESTED |
| `x-cos-forbid-overwrite` | NOT TESTED |
| 其他 staging Key 越权拒绝 | NOT TESTED |
| final Key 写入拒绝 | NOT TESTED |
| GetObject 拒绝 | NOT TESTED |
| multipart 初始化拒绝 | NOT TESTED |

## 7. 失败原因

当前腾讯云适配器创建 STS Client 时传入空 Region：

```go
sts.NewClient(credential, "", clientProfile)
```

位置：`backend/internal/pkg/media/cos.go:34`。

真实腾讯云 STS Endpoint 要求 Region，因此 `GetFederationToken` 在签发任何临时凭证前返回 `MissingParameter: Region`。这不是网络、Mock、编译或本地数据库问题，而是当前真实 STS 初始化参数不完整。

## 8. 已知问题

1. `NewTencentCOS` 没有把配置中的 COS Region 传给 STS Client；
2. 当前产品模型仍向客户端暴露原始临时 Secret，与 Final 设计的“后端预签名、客户端无 Secret”不一致；
3. 当前 STS 测试尚无法验证 Content-Length 和 Content-MD5 是否进入真实 COS 签名边界；
4. 当前 COS 代码尚未提供 Final 设计要求的 staging/final 双 Bucket 与 CopyObject；
5. 当前小程序上传代码仍使用原始 STS + COS SDK PutObject；这些属于后续模块，Gate-0 失败后禁止提前整改。

## 9. 未完成项

- Gate-0 全部真实 COS 正向/负向用例；
- Module ② Media Domain 数据模型与迁移；
- Module ③ Upload Ticket 与 STS 签发服务；
- Module ④ Staging → Final Copy；
- Module ⑤ 审核投影；
- Module ⑥ MediaGateway；
- Module ⑦ Outbox/Inbox；
- Module ⑧ GC；
- Module ⑨ 管理端页面；
- Module ⑩ 小程序上传链路。

## 10. 恢复开发的唯一条件

必须先在 Gate-0 范围内修正 STS Client Region 初始化，并重新执行全部真实 COS 验收。只有下列全部 PASS 后才能进入 Module ②：

- 精确 Key 单 PUT 成功；
- Content-Length、Content-MD5、Content-Type 篡改均被拒绝；
- 原签名重复 PUT 无法覆盖；
- 其他 staging Key 与 final Key 写入被拒绝；
- 临时身份无 GET/DELETE/LIST 权限；
- multipart 全路径被拒绝；
- 测试对象和残留 multipart 全部清理；
- 输出脱敏真实环境证据。

不允许使用以下降级方案：

- 原始 STS Secret 直接下发客户端；
- 仅依赖客户端声明大小；
- 上传完成后才用 HEAD 判断大小；
- 超限或覆盖后再删除；
- 用 Mock、编译通过或单元测试代替真实 COS 证据。

## 11. 最终判定

- Gate-0：**FAIL**；
- 是否允许进入 Module ②：**否**；
- 是否修改了冻结业务域：**否**；
- 是否启动 P0-5：**否**；
- 当前动作：**立即停止开发，等待 Gate-0 STS Region 问题获准修正后重测**。
