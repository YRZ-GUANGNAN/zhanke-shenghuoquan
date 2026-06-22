# P0-4 COS Infrastructure Diagnostic Report v1.0

版本：v1.0  
日期：2026-06-20  
范围：仅腾讯云 COS/STS 基础设施核查  
结论：**PASS — Bucket、Region、账号可见性与服务端 Head/List 已确认**

## 1. 冻结边界

本次未修改 Module ②～⑩、媒体模型、数据库迁移、审核系统、小程序上传链路、管理端或 P0-1～P0-3。未使用 Mock COS，未绕过 Gate-0。

唯一配置变更：修正 `backend/.env` 中 `TENCENT_COS_BUCKET` 的完整名称。Region、SecretId 和 SecretKey 未修改。

## 2. 环境变量核查

| 检查项 | 修正前 | 修正后 | 结论 |
| --- | --- | --- | --- |
| Bucket 格式 | `zk***-[AppID]` | `zk***-[AppID]-[AppID]` | 修正前不是控制面登记的完整名称 |
| AppID 后缀 | 存在一段 | 完整名称包含控制面返回的第二段后缀 | 已修正 |
| Region | `ap-guangzhou` | `ap-guangzhou` | 配置正确，无需修改 |
| SecretId/SecretKey | 已配置 | 未修改 | 可正常调用 STS/COS |

根因：创建 Bucket 时的自定义 BucketName 本身已经包含一段 AppID，腾讯云完整 Bucket 名又自动追加 AppID，导致实际资源名称表现为 `name-[AppID]-[AppID]`。原环境变量少了最后一段，因此请求命中了不存在的 Bucket。

## 3. 账号归属关系（脱敏）

使用真实 STS `GetCallerIdentity`：

| 项目 | 脱敏结果 |
| --- | --- |
| 调用者主账号 UIN | `10***26` |
| 密钥 Principal | `10***26` |
| 调用者类型 | `Root` |
| Bucket AppID | `14***37` |

UIN 与 COS AppID 是不同类型的腾讯云账号标识，不要求数值相等。归属关系通过以下更强证据确认：

1. 使用该 SecretId 调用 COS `GetService` 成功；
2. 该账号可见 Bucket 数量为 1；
3. 修正后的完整 Bucket 名在返回列表中精确命中；
4. 返回 Region 为 `ap-guangzhou`；
5. 同一身份对该 Bucket 执行 HeadBucket 与 ListBucket 均为 200。

因此 SecretId/SecretKey 与目标 Bucket 处于可用的同一账号授权关系。

## 4. Bucket 存在性与 Region 核查

| 验证 | 真实结果 | 判定 |
| --- | ---: | --- |
| COS GetService | 200，精确找到配置 Bucket | PASS |
| GetService 返回 Region | `ap-guangzhou` | PASS |
| 配置 Region | `ap-guangzhou` | PASS |
| HeadBucket | 200 | PASS |
| ListBucket（MaxKeys=1） | 200 | PASS |

腾讯云控制台浏览器自动化在当前执行环境不可用，未生成控制台 UI 截图。作为替代，本报告使用腾讯云官方控制面 `GetService` 与数据面 Head/List 三重真实接口证据；这些请求均由当前服务端 SecretId 签名并直接到达腾讯云，不是本地推断或 Mock。

## 5. 配置修正

修正内容：

```text
TENCENT_COS_BUCKET:
  修正前 = zk***-[AppID]
  修正后 = zk***-[AppID]-[AppID]

TENCENT_COS_REGION:
  保持 ap-guangzhou
```

未输出 SecretId、SecretKey、临时凭证、SessionToken 或预签名 URL。

## 6. 服务端身份验证

修正配置后重复执行：

- STS GetCallerIdentity：PASS；
- STS GetFederationToken：PASS；
- COS GetService：PASS；
- COS HeadBucket：PASS（200）；
- COS ListBucket：PASS（200）；
- 配置 Bucket 精确匹配控制面列表：PASS；
- 配置 Region 精确匹配控制面返回：PASS。

## 7. 安全观察

当前长期 Secret 对应 `Root` 身份。该身份足以执行 Gate-0 的服务端清理与账号诊断，但权限范围大于未来生产运行所需。后续 Upload Signer/GC 实施时必须按 Final 设计改为最小权限服务身份；本次不修改账号、密钥或 CAM 策略，避免越过基础设施核查范围。

该观察不影响本次临时 STS Policy 的 Gate-0 结果：下发的联合身份仍仅允许精确 staging Key 的 PutObject，其他读写、列表和 multipart 均被真实 COS 拒绝。

## 8. 最终结论

- Bucket 完整名称：**已修正并精确命中**；
- Bucket 实际 Region：**ap-guangzhou**；
- Secret 与 Bucket 账号可见关系：**已确认**；
- Bucket 实际存在：**已由 GetService + HeadBucket + ListBucket 确认**；
- NoSuchBucket：**已关闭**；
- 是否修改 Module ②～⑩：**否**；
- 是否启动 P0-5：**否**。
