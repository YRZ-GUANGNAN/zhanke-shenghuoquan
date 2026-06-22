# P0-4 Module ④ Design Review Report

版本：v1.0  
日期：2026-06-21  
阶段：Module ④ 开发前专项设计预审  
审查对象：Upload Completion、固定版本校验、恶意文件验证、Staging→Final Promote 与失败恢复  
最终结论：**NO-GO（暂不允许进入 Module ④ 开发）**

## 1. 审查边界

本次仅进行代码和文档静态审查，没有实现、修改或测试 Module ④ 功能。

本次纳入：

- Upload Completion；
- COS HEAD/GET 固定 staging version 校验；
- Content-MD5、真实格式、尺寸、像素和完整解码验证；
- 可信 SHA-256 生成；
- `CopyObject` 固定源版本到不可变 final Key；
- final version 固化；
- `uploaded -> validating -> promoting -> final_ready`；
- 失败分类、重试、回滚和幂等恢复；
- 与上传票据、配额 reservation 的原子收敛边界。

本次明确不纳入：审核投影、MediaGateway、Outbox/Inbox Consumer、GC 实现、HTTP API、Gin Route、小程序、管理端及 Module ⑤～⑩。P0-1、P0-2、P0-3、P0-5 均未修改。

审查依据：

- `P0-4_MEDIA_PIPELINE_DESIGN_v1.1_FINAL.md`；
- `P0-4_ARCHITECTURE_REVIEW_REPORT.md`；
- `P0-4_ARCHITECTURE_REVIEW_REPORT_v1.1.md`；
- `P0-4_DESIGN_REMEDIATION_REPORT_v1.0.md`；
- Gate-0、Module ②、Module ③及真实 STS 验收报告；
- `backend/internal/module/media`、迁移 009/010；
- 当前仍在运行入口中注册的 `backend/internal/pkg/media` 旧实现。

## 2. 结论摘要

Final Design 给出的方向正确：精确 staging Key、固定源版本、服务端 Copy、内容寻址 final Key、final version 固化和 fail-closed 可见性可以关闭原三项高风险。但 Module ④ 的可执行设计仍缺少四类关键契约：

1. 缺少上传完成时固定 `staging_version_id` 和实物证据的原子事务；
2. 尚未证明 COS `CopyObject` 能对目标执行原子禁止覆盖，普通 `HEAD-not-exists -> Copy` 会产生 TOCTOU；
3. 缺少 processing lease、attempt、deadline 和 Copy 证据字段，进程崩溃后无法安全判定“重试、接管还是补偿”；
4. 新旧媒体实现同时写 `media_assets`，且旧入口仍处于运行组合根中，存在双状态列和双写者漂移。

此外，现有票据过期扫描包含 `uploaded` 状态，可能在资产处于 `validating/promoting` 时把 ticket 置为 `expired` 并释放 reservation；现有 `Consume` 又可在 `final_ready` 前提交配额。这两个路径会破坏最终一致性。

因此，当前不能以 Final Design 的原则性描述直接开始编码。必须先形成 Module ④ Design Addendum，关闭第 5 节全部阻断项并重新复审。

## 3. H-01/H-02/H-03 关闭条件复核

本报告中的 H-01/H-02/H-03 对应原架构报告 P0-01/P0-02/P0-03。

| 风险 | 原关闭目标 | 当前证据 | 本次判定 |
| --- | --- | --- | --- |
| H-01 审核对象可覆盖/版本漂移 | 固定 staging version；只由后端复制；final 不可覆盖；审核、展示、删除固定同一 final version | Gate-0 已证明客户端不能覆盖 staging、不能写 final；模型已有 `staging_version_id`/`final_version_id`；但 Module ④ 尚无固定版本 complete 契约、目标 Copy 原子不覆盖实证和原子 final 固化方法 | **未满足 Module ④ 开发门禁** |
| H-02 超大文件与恶意格式成本攻击 | 入口硬大小/MD5；禁止 multipart；服务端有界完整解码和真实格式门禁 | Gate-0、Module ③已关闭安全入桶窗口；Final Design 定义 5/10 MiB、尺寸和格式规则；但未定义精确读取、内存、CPU、超时和容器尾随数据算法 | **入口已关闭，校验侧有条件保留** |
| H-03 业务对象与媒体引用不可靠 | Domain Outbox + Media Inbox + 引用事实源 + fence + GC 门禁 | 本模块不允许绑定引用、公开展示或实现 GC，因此可以不扩大 H-03；但当前旧实现仍直接绑定同表字段并运行，双写会重新打开该风险 | **仅在新链路完全隔离时保持关闭；当前运行态不满足** |

H-03 在 Module ④ 的安全边界必须是：`final_ready` 仍为未审核、未绑定、不可见资产；Module ④ 不创建/激活引用，不返回 COS URL，不触发物理回收。

## 4. 当前代码与设计事实

### 4.1 已具备的基础

- `Asset` 已包含 `staging_key`、`staging_version_id`、`final_key`、`final_version_id`、`trusted_sha256`、真实 MIME、尺寸、字节数和 `row_version`；
- 技术状态机已声明 `pending_upload -> uploaded -> validating -> promoting -> final_ready`；
- ticket 与 asset 一对一，reservation 与 ticket 一对一；
- ticket、reservation、asset 均已有 CAS 版本字段；
- Module ③已真实完成 STS 签发并把单 PUT 的 Key、长度、MD5、类型、Host 和禁止覆盖头纳入签名边界；
- 本地 COS SDK支持把 source `versionId` 写入 `CopyObject` 请求，也允许自定义 Copy 请求头；这仅证明 SDK 能构造请求，不证明真实 COS 的目标不覆盖语义已经验收。

### 4.2 已确认的设计/实现空洞

- `MarkUploadTicketUploaded` 只把 ticket 从 `authorization_issued` 改为 `uploaded`，没有同时更新 asset，也没有保存 staging version、ETag 或校验证据；
- `CASAssetTechnicalStatus` 和 `CASAssetStorageStage` 是两个独立更新，无法原子提交 `promoting -> final_ready`、`staging -> final` 以及 final key/version/hash/尺寸；
- `media_assets` 没有 processing lease、lease expiry、validation/promote attempts、processing deadline、staging ETag/CRC64、final ETag/CRC64、最近处理错误等字段；
- `ExpireUploadTickets` 会扫描 `uploaded`，不检查 asset 是否已经进入 `validating/promoting`；
- `ConsumeUploadTicket` 只要求 ticket 为 `uploaded`，可在 final 对象尚未固定时提交 reservation；
- 当前 server 仍注册 `internal/pkg/media` 旧路由、旧 worker 和旧 GORM 模型；旧、新实现共用 `media_assets`，旧代码写 `status/object_key/reference_type`，新代码写 `technical_status/staging_key/final_key/media_asset_references`；
- 当前运行入口的旧上传 DTO仍包含临时 Secret 字段，旧对象存储读取“当前 Key”，不满足新链路的固定版本不变量。Module ④不能复用该适配器。

## 5. 高风险阻断项（必须修改）

### M4-H01：Upload Completion 未原子固定 staging 实物

风险：高 / P0 阻断。

当前 ticket `authorization_issued -> uploaded` 与 asset `pending_upload -> uploaded` 分离；没有强制保存上传响应中的 versionId。若先 HEAD 最新版本再保存，服务端角色、旧实现或异常重放可以使被验证对象发生漂移。

必须修改：

1. Complete 命令必须包含 `owner_id + ticket_id + expected_ticket_row_version + staging_version_id + etag`；versionId/ETag 来自 COS 单 PUT 响应，CORS只暴露必要头；
2. Repository 在一个 MySQL 事务内锁定 asset/ticket/reservation，并验证 owner、绑定关系、ticket 状态、未过期、asset 状态和精确 staging Key；
3. 同一事务执行 ticket `authorization_issued -> uploaded`、asset `pending_upload -> uploaded`，保存 `staging_version_id` 和上传 ETag/完成时间；
4. versionId 为空、与该 Key 的固定版本 HEAD 不一致或重复请求携带不同证据时 fail-closed；
5. 相同 ticket、相同 versionId、相同 ETag 的重放返回同一结果，不重复启动任务；不同证据返回冲突。

关闭证据：真实 COS 固定版本 HEAD；错误/空 versionId拒绝；服务端创建新版本后仍只校验原 version；20 路 complete 只有一次状态推进。

### M4-H02：目标 final Key 的原子禁止覆盖尚未证明

风险：高 / P0 阻断。

`HEAD destination not found -> CopyObject` 不是原子操作。两个 Promote 并发时都可能通过 HEAD；若 Copy 默认覆盖，启用版本控制只会生成多个 final version，不能保证唯一版本。数据库最终固定哪个版本取决于竞争时序，H-01会重新打开。

必须修改：

1. 在正式编码前新增 Module ④ Gate-4A 真实 COS capability test；
2. CopySource必须包含已经持久化的 `staging_version_id`，并附 source ETag precondition；
3. destination必须使用腾讯云真实支持并实测有效的原子禁止覆盖条件；禁止以“先 HEAD 再 Copy”替代；
4. 两路并发 Copy 到同一 final Key，必须仅一个创建对象；另一请求只能收到可识别的 already-exists/precondition 结果；
5. 若目标已存在，只能对固定目标 version做 HEAD并逐项核对 asset ID、可信 SHA-256、长度、真实 MIME和来源版本后判定幂等；
6. 若腾讯云 Copy 接口不能原子禁止覆盖，立即停线并重新架构评审，不得以版本控制、后写覆盖或事后删除降级。

关闭证据：真实 Bucket 的并发 Copy、重放 Copy、错误 source version、预存恶意同名对象和目标版本数验收。

### M4-H03：缺少可恢复的 processing lease 和证据模型

风险：高 / P0 阻断。

仅用 `row_version + status` 可以抢到一次状态，但不能处理 worker 在 GET、解码、Copy 或 final HEAD 后崩溃。`validating/promoting` 卡住后，接管者无法区分“尚未执行”和“外部动作已成功但数据库未提交”，可能重复 Copy、误删或错误释放配额。

必须修改：

- 为资产增加或以等价独立表承载：`processing_lease_token`、`processing_lease_expires_at`、`validation_attempts`、`promote_attempts`、`processing_deadline_at`、`last_processing_error_code`、`staging_etag`、`final_etag`、`final_crc64`、`validated_at`、`promoted_at`；
- lease领取、续租、接管必须使用 `row_version + expected_status + expired lease` CAS；
- 外部动作前后持久化足以重建事实的证据；进程重启先 reconcile，不盲目重复 Copy；
- `validating/promoting` 的 1 小时 TTL、最多 5 次重试必须有可查询字段和索引；
- permanent validation failure 与 transient COS/network failure必须分流，只有达到明确终止条件才进入 `delete_pending`。

关闭证据：在 HEAD前、下载中、解码后、Copy响应丢失、final HEAD后、DB提交前逐点注入崩溃，均能恢复到唯一正确状态。

### M4-H04：final_ready 提交不是单事务，存在半完成状态

风险：高 / P0 阻断。

现有通用 CAS会把 technical status、storage stage、final identity和票据/配额拆成多个写操作，可能出现 `final_ready` 但无 version、`storage_stage=final` 但仍 promoting、reservation已释放但对象成功等组合。

必须修改：

Repository提供专用 finalize transaction，在一个事务中同时：

- 条件：asset=`promoting`、lease仍归当前 worker、row_version匹配、ticket=`uploaded`、reservation=`active`；
- 写入并冻结 `trusted_sha256`、verified MIME、byte size、width、height、final key、final version及 final 校验证据；
- asset `storage_stage=final` 且 `technical_status=final_ready`；
- ticket `uploaded -> consumed`；
- reservation `active -> committed`；
- 写入审计记录；
- 所有 CAS任一失败整体回滚。

同样需要专用 terminal-failure transaction：asset进入 `delete_pending`、ticket取消/过期、reservation只释放一次。禁止业务层串联四个独立 Repository 调用。

### M4-H05：票据过期与配额结算存在竞争

风险：高 / P0 阻断。

现有过期任务会处理 `uploaded` ticket；worker可能正在验证或 Copy 时 reservation被释放。现有 `Consume` 又可能在 final_ready前提交配额。

必须修改：

- ticket授权有效期只约束“上传和提交 completion claim”，completion被原子接受后改用 processing deadline；
- 过期扫描不得释放正处于有效 `validating/promoting` lease的 reservation；
- `Consume` 不再作为可独立调用的提前结算路径，只能由 finalize transaction触发；
- 永久校验失败、取消、上传超时释放一次；临时失败和可恢复 promoting 不释放；
- 增加 expire/complete/promote 100 路并发验收。

### M4-H06：旧媒体实现与新 Media Domain 双写同表

风险：高 / P0 阻断。

当前组合根仍注册旧 `internal/pkg/media`，对同一 `media_assets` 表执行 AutoMigrate、HTTP、审核、引用和维护任务。旧实现写 `status/object_key/reference_type`，新实现写 `technical_status/staging_key/final_key/media_asset_references`。任何一方都不能将另一方字段视为事实源，且旧 DTO/对象访问不满足新版最小权限和固定版本条件。

必须修改设计，不在本轮实施：

1. 定义唯一 writer ownership 和 cutover gate；
2. Module ④开发/测试只能引用 `internal/module/media`，禁止 import或复用 `internal/pkg/media`；
3. 真实环境 Module ④测试使用隔离前缀和可识别测试资产，旧 maintenance worker不得扫描这些行；
4. 在任何新链路接入运行入口前，必须另行评审旧路由/worker停用与历史数据迁移；不得借 Module ④修改 P0-1/P0-2/P0-3业务逻辑；
5. boundary check必须阻止同一表出现两个运行期写者。

关闭证据：启动 server 后只有一个 Media Domain writer；旧路由不能创建或读取新资产；AutoMigrate不再以旧模型改变新表约束。

## 6. 中风险问题（进入开发前应固化）

| 编号 | 问题 | 风险 | 必须固化的设计 |
| --- | --- | --- | --- |
| M4-M01 | HEAD固定版本，但完整 GET未明确固定同一版本 | HEAD与下载间仍有漂移 | HEAD、GET、CopySource都使用同一持久化 versionId；禁止读取 latest |
| M4-M02 | MD5/ETag比较规则不明确 | 引号、base64/hex或服务端加密语义造成误判 | 单 PUT下把期望 base64 MD5转换为32位hex；同时对下载字节重新计算MD5，不能只信Content-Type/ETag |
| M4-M03 | 有界解码缺少量化预算 | 解码炸弹造成CPU/内存耗尽 | 读取上限=`expected_size+1`；per-job超时、worker并发、最大内存、最大像素和最长边固定为配置上限且有默认硬上限 |
| M4-M04 | polyglot/尾随数据判定未定义 | 解码成功但携带附加载荷 | JPEG检查EOI后无非允许尾随；PNG完整chunk/IEND；WebP RIFF长度精确、禁止ANIM/ANMF；拒绝未知尾随字节 |
| M4-M05 | SHA-256生成时机/对象不明确 | hash与被Copy对象不一致 | 对固定version完整原始字节计算SHA-256；仅在全部解码和结构校验通过后标记trusted并持久化 |
| M4-M06 | final Key日期来源未定义 | 重试跨月生成不同Key | 年/月取asset创建时间，扩展名只由verified MIME映射，保证重试完全确定性 |
| M4-M07 | Copy metadata契约不足 | 已存在对象无法安全幂等比对 | Copy时使用REPLACE metadata，写asset public ID、trusted SHA、source version、verified MIME/size；禁止写owner或敏感信息 |
| M4-M08 | Copy成功但DB失败的孤儿策略不完整 | final孤儿和重复版本 | 保持asset=`promoting`并reconcile；不得立即删除目标；只有后续GC在固定version及完整门禁下回收 |
| M4-M09 | staging删除时机可能误伤恢复 | Copy后过早删源导致DB失败无法核对 | Module ④只记录cleanup intent/证据，不同步删除源；生命周期24小时兜底，物理删除留给获授权的GC模块 |
| M4-M10 | 状态审计缺口 | 无法解释错误收敛 | 每次状态/lease/最终证据提交写不可变审计；不记录URL、凭证或文件正文 |
| M4-M11 | final HEAD字段可信度未分层 | 只比ETag不足以证明同一对象 | 固定version核对size、ETag、CRC64、metadata、versionId；必要时固定version GET重算SHA |
| M4-M12 | 错误分类未定义 | 临时错误被删除或永久错误无限重试 | 定义permanent、transient、ambiguous三类错误及退避、最大次数、dead-letter/人工处理边界 |

## 7. 低风险/建议修改项

- 把 validator、COS promoter、clock、hash/decoder 作为小接口，单元测试可故障注入；真实 COS验收仍不得用Mock替代；
- 对错误日志只记录 request ID、asset public ID、固定version指纹和错误码，不记录预签名URL、Secret或原始图片；
- final Key、staging Key和versionId不进入对外业务 DTO；
- 为 `validating/promoting` 增加 `(technical_status, processing_lease_expires_at, id)` 索引；
- 保留原始 `expected_content_type` 与 `verified_mime` 的差异审计，显示只使用verified值；
- 对 trusted SHA、versionId、ETag 使用严格长度/字符集校验，拒绝控制字符和路径注入；
- 将真实 COS capability test产生的对象、版本和请求ID全部脱敏记录，并自动清除测试版本。

## 8. 建议的 Module ④ 原子流程

```text
1. AcceptCompletion(tx)
   lock asset/ticket/reservation
   verify owner/binding/status/expiry/version evidence
   ticket authorization_issued -> uploaded
   asset pending_upload -> uploaded
   persist staging_version_id + staging_etag

2. ClaimValidation(tx)
   asset uploaded -> validating
   acquire lease + attempt + processing deadline

3. ValidateOutsideTx
   HEAD exact key?versionId
   GET exact key?versionId with strict byte/time/resource limits
   recompute MD5 + fully validate container/decode/dimensions
   compute SHA-256 over exact bytes; derive deterministic final key

4. ClaimPromote(tx)
   asset validating -> promoting
   persist trusted validation evidence; renew lease

5. PromoteOutsideTx
   CopyObject(source exact version, source ETag precondition,
              destination atomic forbid-overwrite, deterministic metadata)
   HEAD exact returned final version and verify all evidence

6. Finalize(tx)
   CAS asset promoting -> final_ready and staging -> final
   persist final key/version/evidence
   ticket uploaded -> consumed
   reservation active -> committed
   append audit

7. Reconcile
   any ambiguous external result remains promoting
   next lease holder HEADs deterministic destination/version evidence
   never assumes failure solely from timeout; never deletes synchronously
```

状态不允许回退。`validating/promoting` 的retry是同状态租约接管，不是把状态退回 `uploaded`。

## 9. 失败与幂等判定表

| 场景 | 正确动作 |
| --- | --- |
| complete相同证据重放 | 返回当前状态，不重复验证 |
| complete不同version/ETag重放 | 409/fail-closed |
| 固定version不存在 | 永久失败；asset进入delete_pending，释放reservation一次 |
| HEAD/GET网络超时 | 保持validating，lease到期后重试 |
| size/MD5/type/结构/尺寸失败 | 永久失败；不进入Copy |
| Copy明确precondition/already-exists | 固定目标version校验；完全一致才按幂等成功，否则安全告警并停线 |
| Copy超时、结果未知 | 保持promoting；先HEAD/reconcile，禁止立即重Copy或删除 |
| Copy成功、final HEAD失败 | 保持promoting重试HEAD |
| final HEAD成功、DB事务失败 | 保持promoting；按确定性Key和证据恢复提交 |
| finalize CAS被其他worker抢先 | 重新读取；若同一final证据则幂等成功，否则冲突告警 |
| staging生命周期先删除 | 若final已验证则可finalize；否则进入可解释失败，不得伪造成功 |

## 10. Module ④ 验收矩阵

### 10.1 Upload Completion 与固定版本

| 用例 | 验收标准 | 环境 |
| --- | --- | --- |
| 合法单 PUT完成 | ticket与asset在同事务进入uploaded，version/ETag固定 | 真实MySQL 8.4 + 真实COS |
| 空/错误versionId | 拒绝，不启动validator | 真实COS |
| 错误owner/ticket/asset绑定 | 拒绝且无状态变化 | MySQL集成 |
| ticket过期边界 | 未接受complete则过期；已接受后不被上传过期任务释放 | MySQL并发 |
| 20路相同complete | 一次状态变化、一次lease、一次验证 | MySQL并发 |
| 20路不同证据complete | 唯一证据胜出，其余冲突 | MySQL并发 |

### 10.2 HEAD/GET与文件验证

| 用例 | 验收标准 |
| --- | --- |
| HEAD后生成staging新版本 | GET/Copy仍读取已固定旧version |
| Content-Length ±1 | 拒绝 |
| 错误MD5或ETag | 拒绝；下载重算MD5也必须一致 |
| MIME伪造 | 以magic/container为准并拒绝不匹配 |
| JPEG/PNG/静态WebP | 正常读取、完整解码和生成verified字段 |
| SVG/GIF/动画WebP/BMP/TIFF/HEIC | 拒绝 |
| 截断、尾随polyglot、畸形chunk | 拒绝 |
| 超最长边、超像素、解码炸弹 | 在资源上限内拒绝，worker保持可用 |
| 可信SHA | 与固定version完整字节离线SHA-256一致；失败时不落trusted值 |

### 10.3 Promote 与 final 固化

| 用例 | 验收标准 | 环境 |
| --- | --- | --- |
| Copy固定source version | source URL含持久化version；错误version拒绝 | 真实COS |
| 目标禁止覆盖 | 预存同Key后Copy失败且不产生新version | 真实COS |
| 20路并发Promote | 只产生一个final对象版本 | 真实COS |
| Copy重放 | 不新增version；完全相同证据幂等成功 | 真实COS |
| Copy响应丢失 | reconcile后唯一final_ready | 故障注入 + 真实COS |
| final HEAD | 固定返回version，size/ETag/CRC64/metadata全一致 | 真实COS |
| final内容篡改/新增latest version | 固定version仍不变；系统告警且不改DB指针 | 真实COS |

### 10.4 状态、CAS、配额与恢复

| 用例 | 验收标准 |
| --- | --- |
| 非法状态跳转 | Repository拒绝，0行变化 |
| 双worker抢lease | 仅一个成功 |
| lease持有者崩溃 | 超时后一个worker安全接管 |
| 每个外部动作边界崩溃 | 最终收敛为唯一final_ready或可解释delete_pending |
| 成功路径 | asset final_ready、storage final、ticket consumed、reservation committed同事务 |
| 永久失败 | asset delete_pending、ticket terminal、reservation released同事务且仅一次 |
| 临时失败 | reservation保持active，不提前释放 |
| expiry/complete/promote 100路竞争 | 不出现释放后成功、双commit、负数或漂移 |
| 5次重试/1小时TTL | 精确进入终止路径，审计完整 |

### 10.5 可见性、孤儿与边界

| 用例 | 验收标准 |
| --- | --- |
| pending/uploaded/validating/promoting/final_ready | 均不能通过任何业务或旧媒体接口获得URL/内容 |
| Copy成功DB失败 | final对象保留供reconcile，不被并发清理误删 |
| staging删除失败 | 不影响已验证final；等待后续GC/lifecycle |
| GC误删防护契约 | processing lease、固定version和未决状态均构成禁止删除条件 |
| 旧媒体实现隔离 | 不能读写新资产，不能扫描新状态，不能AutoMigrate破坏新约束 |
| 边界检查 | Module ④不import A/B/C、旧media、审核、Gateway、Consumer或GC |

### 10.6 工程门禁

- migration与rollback在真实MySQL 8.4执行成功；
- Module ④单元测试、并发测试和race测试通过；
- `go test ./internal/module/media`、`go test ./...`、`npm run verify`通过；
- 真实COS测试不得用Mock代替，测试对象和全部历史版本残留为0；
- 不新增HTTP路由，不接审核、Gateway、业务域、GC或Module ⑤～⑩；
- 报告必须给出脱敏request ID、key指纹、version指纹、对象版本数和数据库抽查证据。

## 11. 开发准入前必须交付的修改项

1. 《P0-4 Module ④ Design Addendum v1.0》，明确complete固定版本契约、状态/票据/配额事务和错误分类；
2. processing lease、attempt、deadline及校验证据的数据模型与迁移设计；
3. 专用 `AcceptCompletion`、`ClaimValidation`、`ClaimPromote`、`FinalizePromote`、`FailProcessing` Repository事务契约；
4. Module ④ Gate-4A真实COS测试设计，证明source version固定和destination原子禁止覆盖；
5. 票据过期扫描与processing状态的互斥规则，取消提前Consume路径；
6. 新旧媒体实现的writer ownership/隔离与后续cutover方案；
7. 完整的验证器资源预算、容器严格解析和SHA生成规则；
8. Copy结果未知、DB失败、孤儿final和staging残留的reconcile契约；
9. 本报告全部高风险项复审关闭。

## 12. 最终判定

- 高风险/P0阻断：**6项未关闭**；
- 中风险：**12项需固化**；
- 低风险/建议项：**7项**；
- H-01：Gate-0层已部分关闭，但Module ④固定源版本和目标不可覆盖尚未闭环；
- H-02：安全入桶已关闭，服务端严格验证仍需量化设计；
- H-03：仅在Module ④保持未绑定、不可见且新旧写者隔离时不被重新打开；
- 是否允许进入Module ④开发：**否**；
- 是否允许进入Module ⑤～⑩：**否**；
- 是否允许启动P0-5：**否**。

下一步只允许补充Module ④设计与Gate-4A验收方案。所有阻断项关闭并复审PASS后，才可申请Module ④开发授权。
