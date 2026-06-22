# P0-4 Architecture Review Report v1.1

版本：v1.1  
日期：2026-06-20  
复审对象：[P0-4 Media Pipeline Design v1.1](./P0-4_MEDIA_PIPELINE_DESIGN_v1.1.md)  
对照基准：[P0-4 Architecture Review Report](./P0-4_ARCHITECTURE_REVIEW_REPORT.md)  
审查范围：设计复审，不包含代码、迁移、真实 COS 或真实微信验收  
最终结论：**PASS（设计阶段）**

## 1. 复审结论

v1.1 已关闭原报告 3 项 P0 阻断问题，并把 9 项 P1、6 项 P2 整改落实到安全不变量、数据模型、接口契约、状态 TTL、成本预算和验收矩阵。设计允许进入 P0-4 开发阶段的首个 Gate-0，但不代表 Gate-0、开发或专项验收已通过。

允许范围仅为 P0-4 媒体链路。P0-1/P0-2/P0-3 继续冻结，P0-5 禁止启动。

## 2. 三项 P0 关闭审查

### P0-01：审核后对象覆盖

结论：**关闭**。

- 客户端仅获得精确 staging Key 的一次性单 PUT 授权；
- 后端校验固定 staging version 后，以服务角色 CopyObject 到内容寻址 final Key；
- final 禁止覆盖、开启版本控制，数据库固定 `final_version_id`；
- 微信审核、展示和删除使用同一 final key/version；
- 旧上传授权不能写 final，重放不能改变审核或展示对象；
- 设计 §20.2 包含覆盖、错误版本和对象同一性自动化验收。

### P0-02：超大文件成本攻击

结论：**关闭（受 Gate-0 实证门禁约束）**。

- 客户端不获得可自行签名的 STS Secret；
- 预签名单 PUT 绑定精确 Key、Content-Length、Content-MD5、Content-Type，TTL 120 秒；
- multipart 全权限明确禁止；
- 一次性票据和分钟/小时/日/pending 的次数、字节配额使用同事务条件更新；
- staging 使用独立私有 Bucket、独立全局签发硬预算和 24 小时生命周期；
- 真实 COS 负向测试是开发第一步 Gate-0，失败不得降级为原始 STS 或事后校验。

该设计没有用“以后再验证”替代不变量：入口精确长度是强制架构条件；真实产品若不支持，结论是停止开发，而不是接受风险。

### P0-03：父业务对象与媒体引用不可靠

结论：**关闭**。

- A/B/C 在父对象同一事务写 Domain Outbox；
- Media Inbox 以 event ID、aggregate version、payload hash 幂等收敛；
- 标准化引用表是引用事实源，计数仅是可重建缓存；
- 删除/替换先设置两分钟 revocation fence，阻断 Outbox 延迟期的旧图访问；
- 绑定 intent、未决 Outbox、引用状态和保留期共同构成 GC 门禁；
- 重复、乱序、延迟、事务回滚和消费者宕机均有故障注入验收。

## 3. P1 整改复审

| 原问题 | v1.1 设计 | 结论 |
| --- | --- | --- |
| 双重审核事实源 | 统一审核任务为事实源；媒体是 decision_version 投影 | 关闭 |
| 审核/卡死无 TTL | 所有状态固定 TTL、重试上限和 dead-letter | 关闭 |
| 引用/计数冲突 | 标准化多引用表；计数可重建 | 关闭 |
| 文件真实性不足 | MD5、完整受限解码、维度/像素/帧数硬门禁 | 关闭 |
| 幂等与去重混用 | Idempotency-Key 与可信 SHA-256 分离；仅同用户去重 | 关闭 |
| CORS/签名撤销窗口 | 精确 CORS；GET 60 秒；最长残留 60 秒 | 关闭 |
| 配额非原子 | MySQL 条件更新，同事务预占次数和字节 | 关闭 |
| credential 刷新状态不明 | 单票据只签一次；失败创建新票据和新 Key | 关闭 |
| 成本遗漏下行 | 补齐 2 万用户三档 GET/下行预算与 400 KiB 目标 | 关闭 |

## 4. P2 整改复审

| 原问题 | v1.1 设计 | 结论 |
| --- | --- | --- |
| public ID 未定 | 固化 26 位大写 ULID | 关闭 |
| owner shard 未定 | 固化 SHA-256 前 4 hex，且不用于鉴权 | 关闭 |
| 历史 URL 无退出 | 新写强制引用模型；历史清单、批次迁移和停读门禁进入开发计划 | 关闭 |
| 审计保留未定 | 固化 365 天、脱敏且不记录凭证/签名 URL | 关闭 |
| 生命周期范围模糊 | 仅 staging Bucket 24 小时；final 禁止自动生命周期删除 | 关闭 |
| signed GET 缓存不清 | `private,max-age=45`、URL 60 秒、撤销窗口 60 秒 | 关闭 |

## 5. Review Report 第 8 节复核

| 强制整改项 | 结果 |
| --- | --- |
| staging 与 final immutable 分离 | PASS |
| 上传入口硬大小限制 | PASS，且真实实证为 Gate-0 |
| 禁止 multipart + Content-MD5 | PASS |
| A/B/C 事务 Outbox + 幂等消费 | PASS |
| 标准化引用与权威计数 | PASS |
| 审核事实源 + decision_version | PASS |
| TTL、最大重试、dead-letter、GC | PASS |
| 文件尺寸/像素/动画/安全解码 | PASS |
| CORS、签名 URL TTL、撤销窗口 | PASS |
| 原子配额与并发验收 | PASS |
| 2 万用户下行预算和平均字节目标 | PASS |
| 数据流、状态图、表、API、验收矩阵 | PASS |

## 6. Review Report 第 9 节复核

1. P0-01、P0-02、P0-03 已有明确、可实现、可自动化验证的闭环：**满足**。
2. 安全不变量已在设计固化，不以开发后放宽替代：**满足**。
3. 未修改 P0-1/P0-2/P0-3 的业务规则、权限、审核语义、状态机、搜索或 IM：**满足**。
4. P1 全部进入正文、约束、接口或验收标准：**满足**。
5. 真实 COS 策略负向测试是开发第一门禁：**满足**。
6. P0-5 未启动：**满足**。

## 7. 开发准入与停线条件

架构复审结论为 PASS 后，只允许按 v1.1 开始 P0-4 开发，并必须先执行 Gate-0。出现以下任一情况立即停线并重新评审：

- 客户端必须持有可自行签名的临时 Secret 才能完成上传；
- COS 无法强制精确 Content-Length/MD5，且团队拟改回事后删除；
- final 允许客户端写入或允许读取“当前最新版本”而非固定版本；
- A/B/C 无法在父对象同事务写 Outbox；
- 删除/替换无法先设置 fail-closed revocation fence；
- 需要修改冻结权限、审核、社区、交易、搜索、状态机或 IM 业务规则；
- 试图提前启动 P0-5。

## 8. 最终判定

- P0 高风险：0 项未关闭；
- P1 中风险：0 项未进入设计；
- P2 低风险：0 项未进入设计；
- P0-4 Architecture Review：**PASS（设计阶段）**；
- 是否允许进入 P0-4 开发：**是，但 Gate-0 必须先通过**；
- 是否表示 P0-4 专项验收通过：**否**；
- 是否允许进入 P0-5：**否**。
