# P0-4 Module ④ Acceptance Report v1.0

日期：2026-06-21  
验收对象：Upload Completion、COS HEAD固定版本、Staging→Final Promote  
最终结论：**NO-GO / FAIL（真实COS版本控制门禁未满足）**  
Module ④状态：**未通过专项验收**  
Module ⑤～⑩：**禁止进入**

## 1. 验收矩阵

| 验收项 | 结果 | 证据/说明 |
| --- | --- | --- |
| Upload Completion | PASS | ticket/asset前置状态、lease、CAS和专用事务已实现 |
| HEAD声明校验 | PASS（自动化）/ BLOCKED（真实固定版本） | size/type/ETag/MD5校验通过；真实Bucket无versionId |
| 真实格式与尺寸 | PASS | JPEG/PNG/静态WebP严格容器、完整解码、尺寸/像素上限 |
| 可信SHA-256 | PASS | 对固定对象完整字节计算；失败不落trusted值 |
| Copy Promote | PASS（自动化）/ BLOCKED（真实COS） | 恢复与幂等测试通过；真实固定version Copy未执行 |
| CAS | PASS | expected status + row_version + lease token；真实MySQL stale CAS拒绝 |
| Idempotency | PASS（自动化）/ BLOCKED（真实版本数） | 重复调用只Copy一次；真实对象版本数待版本控制启用后验证 |
| Recovery | PASS | Copy已成功但响应丢失时，下次HEAD对账且不二次Copy |
| Real MySQL 8.4 | PASS | 成功/失败事务、CAS、迁移与回滚全部通过 |
| Real STS | PASS | 真实GetFederationToken和精确PUT授权 |
| Real COS PUT | PASS | 正确完整Bucket接收单PUT；无Mock |
| Real COS固定版本HEAD | FAIL/BLOCKED | Bucket versioning未启用，PUT无`x-cos-version-id` |
| Real COS CopyObject | NOT RUN | 固定源版本前置不成立，按H-01停止 |
| 测试残留 | PASS | 测试owner数据库记录为0；已创建staging测试对象执行清理，无测试错误 |

## 2. Upload Completion验收

- 只接受ticket=`uploaded`、asset=`uploaded`、reservation=`active`；
- 首次调用以CAS创建completion并推进asset到`validating`；
- 重复完成返回同一final asset；
- owner、ticket、asset和reservation绑定关系由Repository事务锁校验；
- completion没有注册任何HTTP入口。

结论：**PASS**。

## 3. HEAD与内容校验验收

自动化已覆盖：

- Content-Length与ticket声明一致；
- Content-Type与ticket声明一致；
- ETag转换后与声明Content-MD5一致；
- 固定version完整下载并重新计算MD5；
- JPEG/PNG/静态WebP真实格式；
- 尾随内容、动画WebP、截断、超尺寸和完整解码门禁；
- 对完整原始字节生成可信SHA-256。

真实云上已证明正确Bucket可接受STS单PUT，但Bucket未启用版本控制，无法取得versionId，因此真实固定版本HEAD不通过。

结论：**FAIL/BLOCKED**。

## 4. Copy Promote验收

代码路径具备：

- source key + versionId；
- source ETag precondition；
- destination禁止覆盖头；
- deterministic final Key；
- Copy后固定final version HEAD；
- metadata/size/type/ETag一致性核对；
- Copy响应不确定时先对账再恢复；
- final提交与ticket/reservation同事务。

单元错误注入通过。由于真实Bucket没有source version，真实Copy按安全门禁没有继续执行，不能用latest替代。

结论：**NOT ACCEPTED**。

## 5. MySQL、CAS与事务验收

真实MySQL 8.4结果：

- 011 migration：PASS；
- rollback：PASS；
- rollback后重新migration：PASS；
- 正常完成：PASS；
- 重复完成：PASS；
- stale row_version：PASS；
- 声明不一致：PASS；
- asset进入`delete_pending`、ticket取消、reservation释放同事务：PASS；
- final_ready、storage final、ticket consumed、reservation committed同事务：PASS；
- 测试owner残留：0。

结论：**PASS**。

## 6. 工程验收

```text
go test ./internal/module/media -count=1   PASS
go test ./... -count=1                    PASS
npm run verify                            PASS
```

真实COS测试使用显式门禁：

```text
MEDIA_MODULE4_RUN_REAL_COS=1
go test ./internal/module/media -run '^TestUploadCompletionAgainstRealCOS$' -count=1 -v
```

真实结果：

```text
COS GetService             PASS
完整Bucket解析             PASS
Region/服务身份            PASS
GetVersioning              FAIL: status=""
STS + exact single PUT     已在真实Bucket成功
x-cos-version-id           缺失
fixed-version HEAD/Copy    按门禁停止
```

## 7. 剩余风险

### 高风险

1. **Bucket未启用版本控制**：无法关闭H-01固定版本不变量；
2. **Copy目标原子禁止覆盖尚未真实验证**：只有启用版本后才能执行并发/重放版本数验收；
3. **真实final version未固化**：当前不得接入审核、Gateway或业务引用。

### 中风险

1. 当前长期COS Secret对应高权限身份，生产前需独立最小权限服务角色；本阶段不修改CAM；
2. 正确完整Bucket名称由GetService解析，`.env`中的旧单后缀值仍可能误导其他进程；本报告未越权修改环境配置。

### 低风险

1. GORM在预期的not-found幂等查询中输出红色日志，不影响状态和测试结论；后续可在测试Logger中降噪。

## 8. 关闭门禁所需动作

1. 在腾讯云COS控制台为控制面解析出的完整Bucket开启版本控制（Enabled）；
2. 确认`GetVersioning`返回`Enabled`；
3. 重跑`TestUploadCompletionAgainstRealCOS`；
4. 验证固定source version HEAD/GET、Copy成功、重复Complete不新增final version、final固定version可HEAD；
5. 自动清理staging/final全部测试版本并确认残留0；
6. 重跑`go test ./...`与`npm run verify`；
7. 输出本报告v1.1，只有全部PASS后关闭Module ④。

## 9. 最终结论

Module ④实现已完成到真实环境门禁，但验收要求明确规定Real COS必须PASS。当前Bucket versioning未启用，因此：

- P0-4 Module ④：**NO-GO / FAIL**；
- 是否允许进入Module ⑤：**否**；
- 是否允许进入Module ⑥～⑩：**否**；
- 是否允许降级到latest object或仅ETag：**否**；
- 下一步唯一动作：**启用真实Bucket版本控制并重跑专项验收**。
