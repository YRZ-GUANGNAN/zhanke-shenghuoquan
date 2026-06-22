# P0-4 Design Remediation Report v1.0

版本：v1.0  
日期：2026-06-20  
整改依据：[P0-4 Architecture Review Report](./P0-4_ARCHITECTURE_REVIEW_REPORT.md)  
整改输出：[P0-4 Media Pipeline Design v1.1（Final）](./P0-4_MEDIA_PIPELINE_DESIGN_v1.1_FINAL.md)  
阶段：设计整改，不包含代码、迁移、云配置或真实环境验收  
总体结论：**全部 High Risk 与 Medium Risk 已在设计层关闭；等待 Final 版正式评审**

## 1. 整改边界

本次只处理 P0-4 图片链路设计风险，范围限于社区帖子图片、二手商品图片和用户主动修改的头像。首次微信登录头像仍只允许客户端缓存显示，不创建正式媒体资产。

未修改、未授权修改：

- P0-1 权限体系；
- P0-2 社区闭环；
- P0-3 交易域、搜索架构、商品状态机及腾讯云 IM；
- 既有统一审核语义；
- 任何业务代码、数据库迁移、小程序上传代码或云配置；
- P0-5 运维专项。

风险关闭仅表示设计具有明确不变量、数据约束和可验证验收条件，不代表实现或真实云验收已经通过。

## 2. 风险处理总览

| 编号 | 风险 | 原等级 | 设计整改状态 |
| --- | --- | --- | --- |
| H-01 | STS 凭证可在审核后覆盖同一对象 | High / P0 | 已关闭 |
| H-02 | 超大文件只做事后校验，存在成本攻击窗口 | High / P0 | 已关闭，受 COS Gate-0 实证约束 |
| H-03 | 父业务对象与媒体引用缺少可靠事务 | High / P0 | 已关闭 |
| M-01 | 媒体与统一审核形成双重事实源 | Medium / P1 | 已关闭 |
| M-02 | 审核失败、卡死与孤儿对象无确定 TTL | Medium / P1 | 已关闭 |
| M-03 | 引用模型和引用计数语义冲突 | Medium / P1 | 已关闭 |
| M-04 | MIME、格式、尺寸和解码真实性校验不足 | Medium / P1 | 已关闭 |
| M-05 | 请求幂等与内容去重混用 | Medium / P1 | 已关闭 |
| M-06 | CORS、签名 URL 和删除后访问窗口不明确 | Medium / P1 | 已关闭 |
| M-07 | 用户次数及字节配额无法原子扣减 | Medium / P1 | 已关闭 |
| M-08 | 上传凭证刷新缺少状态边界 | Medium / P1 | 已关闭 |
| M-09 | 2 万用户成本模型遗漏图片下行流量 | Medium / P1 | 已关闭 |

## 3. High Risk 整改

### H-01：审核后对象可被临时凭证覆盖

**问题描述**

旧设计让客户端在一段时间内持有同一对象 Key 的 PutObject 权限。客户端可先上传合法图片 A，通过 complete 和审核后，在凭证过期前覆盖为图片 B，形成“审核 A、展示 B”的 TOCTOU 漏洞。

**风险等级**

High / P0，阻断开发。

**影响范围**

COS STS 权限边界、对象命名、内容安全审核、MediaGateway 展示、审计证据及删除回收。

**整改方案**

1. 采用 staging Key 与 final immutable Key 双阶段对象模型；
2. 客户端只能对一个精确 staging Key 执行一次单 PUT；
3. 后端固定 staging version，完成 HEAD、MD5、格式、尺寸和安全解码校验；
4. 后端服务角色通过 COS CopyObject 把固定源版本复制到内容寻址 final Key；
5. final 禁止客户端写入、禁止覆盖并开启版本控制；
6. 审核、展示和删除全部固定到 `final_key + final_version_id`；
7. Copy 成功后 staging 立即进入删除队列，24 小时生命周期兜底。

**修改后的设计内容**

- staging：`staging/<env>/<ownerShard>/<ownerPublicId>/<assetPublicId>/<nonce>.<ext>`；
- final：`final/<env>/<purpose>/<yyyy>/<mm>/<assetPublicId>/<trustedSha256>.<verifiedExt>`；
- `ownerShard` 固化为用户公开 ID 的 SHA-256 前 4 个 hex，仅用于目录均衡；
- `assetPublicId` 固化为 26 位大写 ULID；
- final Key 由服务端可信 SHA-256 生成，客户端不能提交 Bucket、Key、目录或版本；
- final 读取必须携带数据库固定 versionId，不读取“当前最新版本”。

**是否关闭风险**

是。设计验收必须证明旧上传授权重放不能改变审核对象或展示对象，客户端写 final 必须被拒绝。

### H-02：超大文件成本攻击窗口

**问题描述**

旧设计仅在 complete 后通过 HEAD 检查大小。攻击者可在凭证有效期内反复上传超大对象，即使事后删除，上传流量、请求和短期存储成本已经产生。

**风险等级**

High / P0，阻断开发。

**影响范围**

STS 签发、COS 上传入口、用户配额、staging 存储、GC 压力、成本预算和可用性。

**整改方案**

1. 客户端不获得可自行签名任意请求的临时 Secret；
2. 后端使用受限 STS 在可信环境生成 120 秒精确预签名单 PUT；
3. 签名绑定精确 Key、Content-Length、Content-MD5、Content-Type 和禁止覆盖头；
4. 明确禁止 multipart 的创建、分片、列举、完成和取消权限；
5. 创建一次性上传票据，票据最多签发一次授权；
6. 创建票据前原子预占用户次数、用户字节、pending 和全局 staging 字节配额；
7. staging 使用独立私有 Bucket、独立预算和 24 小时生命周期；
8. 真实 COS/小程序入口负向测试作为开发第一项 Gate-0，失败不得降级为“上传后再删”。

**修改后的设计内容**

- 帖子/商品单图 10 MiB，头像 5 MiB；
- 每分钟 12 个/100 MiB、每小时 60 个/500 MiB、每日 200 个/1 GiB；
- 同时 pending/reviewing 20 个/200 MiB；
- 全局 staging 签发 20 GiB/日、100 GiB/月；
- 配额以 MySQL 条件更新在同一事务预占，任一窗口超限整体失败；
- 票据状态：`created -> authorization_issued -> uploaded -> consumed`，或 `expired/cancelled`；
- 授权过期不刷新旧票据，必须创建新票据和新 staging Key。

**是否关闭风险**

是，但实现准入受 Gate-0 约束。若真实 COS 无法在入口强制长度、MD5 和禁止覆盖，必须停止并重新评审，不能带风险开发。

### H-03：业务生命周期与媒体引用缺少可靠事务

**问题描述**

帖子、商品或头像更新成功后，如果媒体 Commit/DeleteReference 调用失败，会产生未绑定图片、残留公开引用、引用计数漂移或永久孤儿文件。跨域数据库又禁止直接事务，因此简单双写不能可靠收敛。

**风险等级**

High / P0，阻断开发。

**影响范围**

帖子删除、商品删除、头像替换、审核拒绝、软删除、物理删除、引用计数、孤儿文件和 MediaGateway 可见性。

**整改方案**

1. A/B/C 在父对象同一数据库事务内写 Domain Outbox；
2. Media Consumer 使用 Inbox，以 eventId、operationId、aggregateVersion 和 payloadHash 幂等消费；
3. 使用标准化 `media_asset_references`，引用行是事实源；
4. 删除/替换前建立两分钟 revocation fence，立即阻断旧引用访问；
5. 业务事务提交后由 Outbox 最终确认 bind/replace/revoke；
6. 业务事务回滚时 fence 可取消；补偿失败则两分钟自动解除；
7. GC 删除前必须证明无 active/pending/revoking 引用、无有效 bind intent、无未决 Outbox，且保留期已到。

**修改后的设计内容**

- 事件限定为 ReferenceSet、ReferenceReplace、ReferenceRevoke、ParentVisibilityChanged；
- 相同 eventId 重放直接幂等成功；
- 低版本事件忽略，相同版本不同 payload 进入 dead-letter，高版本应用目标引用集合；
- `active_ref_count` 仅为可重建缓存，GC 不得只依据计数删除；
- 软删除先立即禁止访问，物理删除由异步 GC 完成；
- `NoSuchKey/NoSuchVersion` 视为物理删除幂等成功。

**是否关闭风险**

是。故障注入必须覆盖发布器宕机、消费者停机、事务回滚、事件重复/乱序、GC 与绑定并发。

## 4. Medium Risk 整改

### M-01：审核事实源不唯一

**问题描述**：`review_tasks` 与 `media_assets.status` 同时表达审核结论，迟到回调或人工复核可能导致状态分叉。  
**风险等级**：Medium / P1。  
**影响范围**：微信内容安全、统一审核流、展示状态、人工复核。  
**整改方案**：既有统一审核任务作为唯一事实源；媒体状态仅保存可重建投影；每次决定使用单调递增 `decision_version`。  
**修改后的设计内容**：投影包含 `review_task_id`、`applied_decision_version`、`projected_decision`；只有更高版本可 CAS 更新。重复、旧版本、乱序和未知任务只写审计。父对象拒绝只撤销引用，不篡改媒体审核结论。  
**是否关闭风险**：是。

### M-02：审核失败与孤儿文件缺少确定清理期限

**问题描述**：pending、uploaded、reviewing 或 rejected 可能长期占用 COS，孤儿对象没有确定截止时间和失败兜底。  
**风险等级**：Medium / P1。  
**影响范围**：孤儿文件、审核失败文件、staging/final 存储、GC 和成本。  
**整改方案**：为全部技术状态固化 TTL、最大重试、dead-letter 和回收优先级。  
**修改后的设计内容**：pending 30 分钟；uploaded/promoting/final_ready/submitted 1 小时；reviewing 24 小时；rejected/review_failed 立即不可见并在 1 小时内请求删除；approved 未绑定 24 小时；delete_pending 最多 12 次/24 小时后进入 dead-letter。staging 24 小时生命周期兜底，final 禁止自动生命周期误删。  
**是否关闭风险**：是。

### M-03：引用计数与引用模型冲突

**问题描述**：单资产字段无法表达多业务槽位，引用计数又缺少权威来源，容易漂移并造成误删。  
**风险等级**：Medium / P1。  
**影响范围**：帖子/商品多图、头像替换、内容去重、GC。  
**整改方案**：采用标准化多引用模型，引用行作为事实源。  
**修改后的设计内容**：唯一约束 `(reference_type, reference_id, slot)` 和 `(asset_id, reference_type, reference_id, slot)`；引用状态为 pending/active/revoking/revoked；`active_ref_count` 只作缓存并由每日对账重建。  
**是否关闭风险**：是。

### M-04：恶意文件与伪造格式防护不足

**问题描述**：扩展名、客户端 MIME 或前 512 字节魔数不能证明图片完整、安全、可解码，无法拦截 polyglot、截断文件、动画 WebP 和解码炸弹。  
**风险等级**：Medium / P1。  
**影响范围**：上传服务、审核接口、客户端展示、CPU/内存和安全。  
**整改方案**：强制 Content-MD5、固定版本 HEAD、完整受限解码、真实格式/尺寸/像素/帧数门禁及可信 SHA-256。  
**修改后的设计内容**：只允许 JPEG、PNG、静态 WebP；拒绝 SVG、GIF、动画 WebP、可执行内容；帖子/商品最长边 8192、40 MP，头像 4096、16 MP；可信响应使用 verified MIME 和 `nosniff`。  
**是否关闭风险**：是。

### M-05：请求幂等与内容去重混用

**问题描述**：客户端哈希同时承担幂等和内容查重，哈希不可信且相同幂等键不同内容的行为不明确。  
**风险等级**：Medium / P1。  
**影响范围**：重复上传、重复计费、资产复用和用户隐私。  
**整改方案**：操作幂等键与内容哈希完全分离；服务端安全解码后计算可信 SHA-256。  
**修改后的设计内容**：相同 Idempotency-Key 且参数不同返回 409；只在同用户、同 purpose 内复用 approved canonical asset；禁止跨用户去重和存在性泄露；重复 staging/final 进入 GC。  
**是否关闭风险**：是。

### M-06：COS 访问最小权限与撤销窗口不清

**问题描述**：CORS 过宽、STS 读取权限或长时间 signed GET 会扩大越权和删除后残留访问窗口。  
**风险等级**：Medium / P1。  
**影响范围**：COS STS、上传来源、公开读取、删除与隐私。  
**整改方案**：生产 CORS 精确白名单、仅 PUT、最小必要头；上传身份无 GET；读取只经 MediaGateway。  
**修改后的设计内容**：final signed GET 60 秒，`private,max-age=45`；最长残留访问窗口 60 秒；签名 URL 不落库、不记日志、不进入业务字段；未审核、未绑定、父对象隐藏或删除中的资源统一返回 404。  
**是否关闭风险**：是。

### M-07：配额无法抵抗并发绕过

**问题描述**：先查询再扣减会被并发请求突破分钟、小时、日和 pending 限制。  
**风险等级**：Medium / P1。  
**影响范围**：凭证接口、防刷、存储成本和审核成本。  
**整改方案**：统一 Quota Service 使用数据库条件更新，在创建票据同一事务预占次数和字节。  
**修改后的设计内容**：一分钟/小时/日/pending 以及全局窗口全部成功才签发；reservation 绑定 ticketId，commit/release 用 CAS，禁止重复扣减或释放；100 路并发验收总量不得突破阈值。  
**是否关闭风险**：是。

### M-08：凭证刷新状态边界缺失

**问题描述**：旧接口可能在 approved、rejected、delete_pending 或 deleted 状态继续刷新上传凭证。  
**风险等级**：Medium / P1。  
**影响范围**：对象覆盖、成本攻击、状态机和审计。  
**整改方案**：取消旧资产凭证刷新；一个票据只签发一次，过期或网络失败创建新资产票据和新 staging Key。  
**修改后的设计内容**：`credential_issue_count <= 1` 数据库约束；任何非 pending 状态不得获得上传授权；旧 Key 由过期 GC 清理。  
**是否关闭风险**：是。

### M-09：2 万用户成本模型遗漏下行

**问题描述**：存储和上传量较小，但图片浏览 GET 和公网下行可能成为主要成本，旧模型无法支持预算决策。  
**风险等级**：Medium / P1。  
**影响范围**：COS 请求、外网流量、签名接口、审核调用和月度预算。  
**整改方案**：补充 1万/2万用户存储、请求、审核和三档图片下行预算；客户端平均图片目标 400 KiB。  
**修改后的设计内容**：2 万用户月上传约 6 万张、月新增约 23.4 GiB、12 月约 281.3 GiB；按 400 KiB 计算，120万/600万/1800万 GET 分别约 458 GiB/2.24 TiB/6.71 TiB。超出平均 400 KiB、p95 1.5 MiB 或下行档位必须重新容量评审，不能在本阶段擅自引入 CDN。  
**是否关闭风险**：是。

## 5. 重点领域整改结果

| 重点领域 | 最终设计决定 | 风险状态 |
| --- | --- | --- |
| COS STS 权限边界 | 后端受限 STS 仅用于生成精确单 PUT；客户端无临时 Secret、无 GET/DELETE/LIST/multipart/final 写权 | 已关闭 |
| 对象命名策略 | 服务端生成 staging nonce Key 与内容寻址 final immutable Key；固定版本读取 | 已关闭 |
| 孤儿文件治理 | bind intent + Outbox/Inbox 门禁 + 24小时未绑定回收 + 对账 | 已关闭 |
| 审核失败文件 | 立即不可见，1小时内进入物理删除；失败进入 dead-letter | 已关闭 |
| 引用计数 | 标准化引用表为事实源，计数可重建，GC 不单独依赖计数 | 已关闭 |
| 软删/物理删 | 软删或 fence 先失效访问；异步 GC 删除固定对象版本 | 已关闭 |
| 内容审核状态机 | 统一审核为事实源，decision_version 单调投影，全部失败 fail-closed | 已关闭 |
| 上传事务一致性 | 一次性票据、原子配额、状态 CAS、Copy 幂等、业务事务 Outbox | 已关闭 |
| 资源回收 | staging/final 分治、TTL、lease/CAS、NoSuchKey 幂等和 dead-letter | 已关闭 |
| 恶意文件防护 | MD5、完整受限解码、格式/尺寸/像素/帧数硬限制 | 已关闭 |
| 成本控制 | 客户端 400 KiB 目标、字节配额、staging 硬预算、三档下行预算 | 已关闭 |

## 6. 关闭证据与后续评审门禁

所有 High Risk 已具备以下共同关闭证据：

- 明确且不可降级的安全不变量；
- 对应数据库字段、唯一约束或状态 CAS；
- 正向、负向和故障注入验收用例；
- 失败时先停止展示，再异步收敛；
- 不依赖跨域数据库访问；
- 不修改冻结业务规则。

Final 评审必须逐项确认：

1. 三项 High Risk 的设计不变量无例外路径；
2. 九项 Medium Risk 全部进入正文、数据约束、API 或验收矩阵；
3. 真实 COS 入口策略测试仍是开发 Gate-0；
4. Gate-0 失败时禁止用原始 STS、事后 HEAD 或事后删除替代；
5. P0-1/P0-2/P0-3 保持冻结；
6. P0-5 未启动。

## 7. 最终结论

- High Risk：3 项，设计层全部关闭；
- Medium Risk：9 项，设计层全部关闭；
- 是否已完成开发：否；
- 是否已完成真实 COS/微信验收：否；
- 是否允许立即开发：否，必须先由《P0-4 Media Pipeline Design v1.1（Final）》正式评审得出 PASS；
- 是否允许启动 P0-5：否。
