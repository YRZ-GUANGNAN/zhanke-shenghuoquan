# P0-4 Module ④ Acceptance Report v1.1

日期：2026-06-21  
验收对象：Upload Completion、COS固定版本HEAD/GET、Staging→Final CopyObject  
最终结论：**PASS**  
状态变更：Module ④ `NO-GO / FAIL -> PASS`  
冻结声明：Module ④验收通过后冻结；本次未启动Module ⑤～⑩

## 1. 验收结论

真实腾讯云Bucket Versioning门禁已关闭。真实环境完成两组端到端链路：

1. 正常链路：真实STS、单PUT、固定source version HEAD/GET、固定source version Copy、固定final version HEAD、重复Complete；
2. 故障恢复链路：真实CopyObject成功后主动丢弃本地响应，再次Complete通过final HEAD对账恢复，未执行第二次Copy。

两组final Key均只有一个对象版本。测试结束后staging、final、历史版本、delete marker和数据库测试数据残留为0。

## 2. Versioning Enabled证据

验收使用腾讯云官方COS控制面和数据面API，没有使用Mock。

| 检查项 | 脱敏结果 | 结论 |
| --- | --- | --- |
| COS GetService | 找到验收账号唯一目标Bucket | PASS |
| 完整Bucket指纹 | `zksh...1937` | PASS |
| Region | `ap-guangzhou` | PASS |
| 变更前GetVersioning | `""` | 已确认原阻断 |
| PutBucketVersioning | `Status=Enabled`，真实请求成功 | PASS |
| 变更后GetVersioning | `Enabled` | PASS |
| 再次真实测试前GetVersioning | `Enabled` | PASS |

执行证据：

```text
=== RUN   TestEnableModule4COSVersioning
using control-plane resolved full bucket fingerprint zksh...1937
PASS real COS versioning changed to Enabled and was read back
--- PASS: TestEnableModule4COSVersioning
```

桌面浏览器控制连接当时不可用，因此没有伪造控制台截图；Versioning变更通过相同腾讯云账号的官方`PutBucketVersioning`完成，并由官方`GetVersioning`回读`Enabled`，属于真实云控制面证据。

## 3. Fixed Source Version证据

真实STS授权完成精确staging Key单PUT，COS返回非空`x-cos-version-id`。Completion随后：

- HEAD latest只用于发现本次上传version；
- 立即HEAD该精确version并把versionId/ETag持久化；
- GET使用相同固定version；
- CopySource使用相同固定version和source ETag precondition；
- 不读取latest替代固定version；
- 不使用ETag替代versionId。

脱敏证据：

| 链路 | Source Version指纹 | HEAD固定版本 | GET固定版本 | 结果 |
| --- | --- | --- | --- | --- |
| 正常完成 | `MTg0...3NTI` | PASS | PASS | PASS |
| Copy响应丢失恢复 | `MTg0...3ODU` | PASS | PASS | PASS |

声明及实物校验同时通过：Content-Length、Content-Type、ETag/Content-MD5、下载字节MD5、真实格式、尺寸、完整解码和可信SHA-256。

## 4. Fixed Final Version与CopyObject证据

真实Promote执行：

```text
CopyObject(
  sourceKey,
  sourceVersionId,
  sourceETag precondition,
  deterministic finalKey,
  x-cos-forbid-overwrite=true
)
```

Copy后使用返回的精确final version执行HEAD，核对：

- final key；
- final versionId；
- Content-Length；
- verified Content-Type；
- ETag；
- asset/ticket/SHA-256/source-version metadata。

脱敏证据：

| 链路 | Final Version指纹 | 固定final HEAD | Final版本数 |
| --- | --- | --- | ---: |
| 正常完成 | `MTg0...zNzg` | PASS | 1 |
| Copy响应丢失恢复 | `MTg0...0MTM` | PASS | 1 |

结论：**Fixed Version Copy PASS；Final Version固化PASS。**

## 5. 幂等验收

正常链路连续执行两次`CompleteUpload(ownerID, ticketID)`：

- 返回相同asset；
- final key相同；
- final version相同；
- ticket保持`consumed`；
- reservation保持`committed`；
- final对象版本数为1；
- 未执行第二次Copy。

结论：**Idempotency PASS。**

## 6. Recovery验收

故障注入没有Mock或跳过真实Copy：

1. 真实固定version CopyObject在腾讯云成功；
2. 测试层丢弃一次成功响应，模拟网络在服务端成功后断开；
3. 首次Complete返回不确定错误，asset保持`promoting`；
4. 再次Complete领取新lease；
5. 先HEAD确定性final Key；
6. metadata、source version、hash、size、MIME和ETag全部一致；
7. 直接执行finalize，不再次Copy；
8. `realCopyCalls=1`，final版本数=1。

执行证据：

```text
PASS real STS/PUT/fixed HEAD/GET/Copy/final HEAD/idempotency/recovery
sourceVersion=MTg0...3NTI
finalVersion=MTg0...zNzg
recoverySourceVersion=MTg0...3ODU
recoveryFinalVersion=MTg0...0MTM
finalVersions=1
recoveryFinalVersions=1
realCopyCalls=1
```

结论：**Recovery after uncertain Copy PASS。**

## 7. Cleanup证据

真实COS测试为每个staging/final Key注册清理：

- 列举全部object versions；
- 按versionId删除全部版本；
- 删除全部delete marker；
- 再次GetObjectVersions；
- 任一version或delete marker残留都会使测试失败。

测试无cleanup错误并最终PASS。真实MySQL测试owner对应asset、ticket、reservation和completion记录计数均为0。

| 残留项 | 结果 |
| --- | ---: |
| staging当前版本 | 0 |
| staging历史版本 | 0 |
| final当前版本 | 0 |
| final历史版本 | 0 |
| delete marker | 0 |
| 测试数据库记录 | 0 |

结论：**Cleanup PASS。**

## 8. 完整测试矩阵

| 验收项 | 结果 |
| --- | --- |
| Real COS Versioning Enabled | PASS |
| Real STS GetFederationToken | PASS |
| Real exact single PUT | PASS |
| PUT返回versionId | PASS |
| Fixed source version HEAD | PASS |
| Fixed source version GET | PASS |
| Content-Length/Type/MD5 | PASS |
| 真实格式/尺寸/完整解码 | PASS |
| 可信SHA-256 | PASS |
| CopyObject(source version) | PASS |
| Final version HEAD | PASS |
| Repeated CompleteUpload | PASS |
| No duplicate final version | PASS |
| Recovery after uncertain Copy | PASS |
| CAS与原子finalize | PASS |
| MySQL 8.4 migration/rollback | PASS |
| Cleanup residue=0 | PASS |

## 9. 回归结果

```text
go test ./... -count=1     PASS
npm run verify             PASS
  check:boundaries         PASS
  admin typecheck          PASS
  miniapp typecheck        PASS
  backend go test ./...    PASS
```

真实专项命令：

```text
MEDIA_MODULE4_RUN_REAL_COS=1 \
go test ./internal/module/media \
  -run '^TestUploadCompletionAgainstRealCOS$' \
  -count=1 -v

PASS
ok campus-circle/backend/internal/module/media
```

## 10. 安全与边界复核

- 未使用Mock COS、Fake HEAD或Fake Copy；
- 故障注入发生在真实Copy成功之后，仅模拟响应丢失；
- 未输出SecretId、SecretKey、STS临时密钥、SessionToken或预签名URL；
- 未修改Module ②、Module ③或Module ④核心逻辑；
- 只新增显式门禁测试与v1.1报告；
- 未新增HTTP、Gin Router、业务接线、审核、引用、Gateway、Consumer或GC；
- Module ⑤～⑩未启动。

## 11. 剩余风险

### 高风险

无Module ④未关闭高风险。

### 中风险

1. 当前验收长期COS Secret权限较高；生产运行前应使用最小权限Media服务角色。本次不修改CAM。
2. `.env`中的单后缀Bucket历史值可能被其他进程误用；真实验收通过GetService解析完整Bucket。后续配置治理需单独授权，不影响本次真实对象闭环结论。

### 低风险

GORM会为预期的幂等not-found查询输出错误色日志，但不影响事务、残留或验收结果。

## 12. 最终判定

- Real COS Versioning：**PASS**；
- Fixed Version HEAD/GET/Copy：**PASS**；
- Final Version固化：**PASS**；
- Idempotency：**PASS**；
- Recovery：**PASS**；
- Cleanup：**PASS**；
- P0-4 Module ④：**PASS并冻结**；
- 是否已启动Module ⑤：**否**；
- 下一阶段状态：只有获得新的明确授权后，方可进入Module ⑤ Review Projection。
