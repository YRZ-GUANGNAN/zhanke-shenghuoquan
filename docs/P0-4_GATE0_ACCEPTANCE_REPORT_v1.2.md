# P0-4 Gate-0 Acceptance Report v1.2

版本：v1.2  
日期：2026-06-20  
环境：真实腾讯云 STS + COS  
Mock：未使用  
结论：**PASS — Gate-0 全部验收项通过**

## 1. 前置问题关闭

| 问题 | 处理 | 结果 |
| --- | --- | --- |
| STS Client Region 为空 | 从配置传入 COS Region | PASS |
| 配置 Bucket 不存在 | 根据账号 GetService 返回修正完整 Bucket 名 | PASS |
| Region 归属不确定 | GetService、HeadBucket、ListBucket 交叉验证 | PASS |

详细基础设施证据见 [P0-4 COS Infrastructure Diagnostic Report v1.0](./P0-4_COS_INFRASTRUCTURE_DIAGNOSTIC_REPORT_v1.0.md)。

## 2. 真实测试命令

```text
go test ./internal/pkg/media -run '^TestGate0TencentCOSUploadBoundaryV11$' -v -count=1
```

测试使用独立随机 `staging/gate0-v11/<nonce>/` 前缀。凭证、SessionToken、预签名 URL 和完整账号标识未输出。

## 3. Gate-0 验收矩阵

| 验收项 | 真实响应 | 结果 |
| --- | ---: | --- |
| GetFederationToken | 成功 | PASS |
| 精确 staging Key 单 PUT | 200 | PASS |
| Content-Length 篡改 | 403 | PASS |
| Content-MD5 篡改 | 403 | PASS |
| Content-Type 篡改 | 403 | PASS |
| 原授权重复 PUT 覆盖 | 409 Conflict | PASS |
| 其他 staging Key PutObject | 403 | PASS |
| final Key PutObject | 403 | PASS |
| GetObject | 403 | PASS |
| DeleteObject | 403 | PASS |
| ListBucket | 403 | PASS |
| multipart initiate | 403 | PASS |
| multipart upload part | 403 | PASS |
| multipart list parts | 403 | PASS |
| multipart complete（合法 XML、伪 uploadId） | 403 | PASS |
| multipart abort | 403 | PASS |
| multipart list uploads | 403 | PASS |
| 对象版本清理 | 0 残留 | PASS |
| delete marker 清理 | 0 残留 | PASS |
| multipart 清理 | 0 残留 | PASS |

## 4. 关键证据摘要

```text
PASS Content-Length tamper rejected: 403
PASS Content-MD5 tamper rejected: 403
PASS Content-Type tamper rejected: 403
PASS exact staging Key single PUT: 200
PASS signed PUT overwrite rejected: 409
PASS other staging Key/final Key denied: 403
PASS GetObject/DeleteObject/ListBucket denied: 403
PASS multipart initiate/upload/list/complete/abort denied: 403
PASS cleanup: no object version, delete marker, or multipart residue
```

完整测试退出状态：

```text
--- PASS: TestGate0TencentCOSUploadBoundaryV11
PASS
ok campus-circle/backend/internal/pkg/media
```

## 5. 策略边界结论

真实环境已证明：

1. 联合身份只能写一个精确 staging Key；
2. Content-Length、Content-MD5、Content-Type 已进入签名边界；
3. `x-cos-forbid-overwrite` 阻止原授权覆盖；
4. 联合身份不能写其他 staging 或 final Key；
5. 联合身份没有 GET、DELETE、LIST 权限；
6. multipart 全链路未获得权限；
7. 服务身份能够完成测试资源全版本清理。

未采用原始 STS 越权、事后 HEAD、事后删除、Mock 或编译通过等降级结论。

## 6. 修改范围

本阶段只涉及：

- COS/STS adapter Region 参数；
- Gate-0 真实环境测试代码；
- `backend/.env` 的完整 Bucket 名称；
- 基础设施诊断与验收报告。

未修改业务域、数据库、媒体模型、审核、小程序、管理端或 P0-1～P0-3。

## 7. 最终判定

- Gate-0：**PASS**；
- NoSuchBucket 阻断：**已关闭**；
- 是否满足进入 Module ② 的前置条件：**是**；
- 本次是否已启动 Module ②：**否**；
- Module ③～⑩：**未启动**；
- P0-5：**未启动**。

本报告只解除 Gate-0 前置阻断，不代表 P0-4 开发或专项验收整体完成。
