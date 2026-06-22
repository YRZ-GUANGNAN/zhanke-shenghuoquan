# P0-4 Module ④ Development Report v1.0

日期：2026-06-21  
模块：Upload Completion、固定版本校验、Staging→Final Promote  
开发状态：**代码与数据库实现完成；真实 COS 门禁阻塞**  
范围结论：未进入 Module ⑤～⑩

## 1. 实现内容

本模块新增独立 completion 应用层，没有注册 HTTP/Gin 路由，也没有接入业务域、审核、Gateway、Inbox/Outbox Consumer 或 GC。

实现流程：

```text
ticket=uploaded + asset=uploaded
  -> CAS领取 validating lease
  -> HEAD latest并立即固定versionId
  -> HEAD/GET固定同一versionId
  -> size/type/ETag/MD5/真实格式/尺寸/完整解码校验
  -> 生成可信SHA-256和确定性final Key
  -> CAS validating -> promoting
  -> CopyObject(source version + source ETag + destination禁止覆盖)
  -> HEAD固定final version并核对metadata/size/type/ETag
  -> 单事务finalize
     asset=final_ready + storage_stage=final + final key/version
     ticket=consumed
     reservation=committed
```

失败行为：

- 声明、MD5、真实格式、尺寸或完整解码不一致：completion=`validation_failed`；asset按冻结状态机进入`delete_pending`；ticket=`cancelled`；reservation=`released`，同事务完成；
- HEAD/GET/Copy临时错误：保留`validating/promoting`并释放处理lease供重试，不提前释放配额；
- Copy结果不确定：下次先HEAD确定性final Key并核对媒体元数据，不盲目重复Copy；
- final已存在且证据完全一致：视为幂等成功；证据不一致：fail-closed。

## 2. 数据库变更

新增迁移：

- `backend/migrations/011_media_upload_completion.sql`；
- `backend/migrations/rollback/011_media_upload_completion.sql`。

新增表`media_upload_completions`，一对一绑定ticket和asset，保存：

- processing phase；
- staging/final固定version与ETag；
-可信SHA-256、真实MIME、尺寸和字节数；
- lease token、lease expiry、processing deadline；
- validation/promote attempts；
- final CRC64、错误码、完成时间、row_version。

迁移没有引用用户、社区或交易域表。Module ②核心模型和Module ③文件未修改。

## 3. 代码清单

新增：

- `completion_types.go`：completion数据契约、COS服务身份适配器；
- `completion_repository.go`：claim/pin/promote/finalize/fail专用事务；
- `completion_service.go`：固定版本校验、严格图片验证、Copy恢复编排；
- `completion_test.go`：单元与错误注入测试；
- `completion_mysql_test.go`：MySQL 8.4迁移、CAS、事务和残留测试；
- `completion_cos_external_test.go`：真实STS/COS外部验收。

没有修改HTTP API、router、小程序、管理端、审核、MediaGateway、业务引用或GC。

## 4. 接口清单

本模块没有HTTP接口。

新增内部应用服务：

```go
CompleteUpload(ctx, ownerID, ticketID) (Asset, error)
```

新增内部存储边界：

- `HeadObject(key, versionID)`；
- `OpenObject(key, versionID)`；
- `PromoteObject(request)`。

业务层不会获得COS URL、Secret或原始对象访问能力。

## 5. 安全与一致性措施

- HEAD之后持久化固定staging version，GET和Copy始终使用该version；
- CopySource附带source version与source ETag precondition；
- destination发送`x-cos-forbid-overwrite:true`；
- final Key由环境、purpose、资产创建年月、asset ULID和可信SHA-256确定；
- final metadata固化asset、ticket、SHA-256、source version和verified size；
- final_ready、storage stage、final identity、ticket消费和reservation提交为一个MySQL事务；
- 所有领取和状态更新使用expected status、row_version、lease token CAS；
- 不同步删除staging或final对象，不实现GC；
- `final_ready`仍未审核、未绑定且不可公开访问。

## 6. 测试结果

| 测试 | 结果 |
| --- | --- |
| 正常完成 | PASS |
| 重复完成不重复Copy | PASS |
| 声明不一致fail-closed | PASS |
| Copy响应丢失后恢复 | PASS |
| stale row_version CAS | PASS |
| MySQL成功事务 | PASS |
| MySQL失败释放reservation | PASS |
| migration forward | PASS |
| rollback + reapply | PASS |
| `go test ./internal/module/media` | PASS |
| `go test ./...` | PASS |
| `npm run verify` | PASS |
| 真实STS + PUT | PASS（到达真实正确Bucket） |
| 真实固定版本HEAD/Copy | BLOCKED |

## 7. 已知问题与未完成项

真实COS控制面`GetService`已找到正确完整Bucket，Region和服务端身份可用；但`GetVersioning`返回空状态，即Bucket未开启版本控制。真实PUT因此没有返回`x-cos-version-id`。

在该条件下：

- 无法固定staging version；
- 无法证明CopySource固定版本；
- 无法固化final version；
- 无法执行H-01要求的版本漂移和重复Copy验收。

实现没有降级为“读取latest”或“只使用ETag”。真实外部测试在版本控制前置检查处fail-closed。

## 8. 开发结论

Module ④代码、迁移、单元测试和MySQL 8.4测试完成。真实COS门禁未通过，因此本Development Report不构成Module ④专项PASS。

下一步唯一允许动作：为真实测试Bucket启用版本控制，随后重跑`TestUploadCompletionAgainstRealCOS`和完整验收。Module ⑤～⑩继续冻结。
