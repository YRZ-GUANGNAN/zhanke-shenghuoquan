# P0-3 Transaction Acceptance Report v1.0

版本：1.0  
验收日期：2026-06-18  
验收范围：C 交易域与 E 域腾讯云 IM 适配链路  
冻结边界：未修改 P0-1 权限体系、P0-2 社区闭环、统一审核流；未进入 P0-4 COS 图片链路或 P0-5 运维专项

## 1. 验收结论

P0-3 的代码建设、真实 Meilisearch 联调、真实 MySQL 8.4 迁移和并发测试已完成。商品发布、审核后展示、浏览、全文搜索、收藏、联系关系、状态流转及售出展示均已形成代码闭环；搜索不再使用 MySQL `LIKE`，后端不存储聊天消息，也不存在本地 Mock IM 回退。

当前交易系统结论为 **MVP NO-GO（仅剩腾讯云生产环境联调门禁）**。

原因：验收环境未提供 `TEST_TENCENT_IM_SDK_APP_ID`、`TEST_TENCENT_IM_SECRET_KEY`、`TEST_TENCENT_IM_ADMIN_USER`，因此无法诚实声明腾讯云线上 `Account Import → UserSig 登录 → SDK_READY → C2C 收发消息` 已在真实云租户跑通。官方 SDK、REST Account Import、UserSig、C2C 会话和页面链路均已实现并通过本地自动化或编译验证，但这不能替代云端端到端验收。

解除 NO-GO 的唯一 P0-3 条件：使用正式或预发布腾讯云 IM 应用执行第 12 节的云端验收，全部通过后可将 P0-3 改判为 MVP GO。不得用 Mock、HTTP 200 假响应或本地伪会话替代。

## 2. 闭环验收矩阵

| 链路 | 实现状态 | 验收证据 | 结论 |
| --- | --- | --- | --- |
| 商品发布 | 创建 `selling + submitted` 商品及图片关系，提交既有统一审核流 | Go 全量测试通过；边界检查通过 | 通过 |
| 审核通过 | 沿用统一审核回调；商品状态与搜索 outbox 同事务更新 | 审核目标接线及 outbox 收敛测试 | 通过 |
| 商品浏览 | 无关键词时按状态分页；详情支持 selling/reserved/sold | TypeScript 检查及小程序构建通过 | 通过 |
| 全文搜索 | 标题、描述均由 Meilisearch 检索；无 MySQL LIKE 回退 | Meilisearch v1.46.0 真实进程集成测试 | 通过 |
| MySQL 二次校验 | 按命中 ID 再校验 approved、未删除、请求状态，并保持搜索顺序 | 服务测试覆盖陈旧索引过滤 | 通过 |
| 商品收藏 | `(product_id,user_id)` 唯一关系；商品行锁；仅真实变更更新计数 | MySQL 8.4 十路并发测试 | 通过 |
| 联系卖家 | 校验审核、删除、售出、本人联系等条件；导入买卖双方 IM 账号后落关系 | 服务测试及 REST 协议测试 | 通过 |
| 腾讯云真实会话 | 官方 SDK、UserSig、Account Import、C2C API 均已实现 | 本地协议测试与构建通过；无云凭证 | **云端未验收** |
| 商品状态流转 | CAS 更新；仅卖家；selling/reserved/sold 合法矩阵 | 单元测试与 MySQL 并发 CAS 测试 | 通过 |
| 售出展示 | 列表状态筛选、详情状态标签、卖家商品入口 | 小程序类型检查和生产构建 | 通过 |
| 售出限制 | sold 禁止编辑、删除、新增收藏和新建联系关系 | 服务自动化测试 | 通过 |
| 索引同步 | 创建、编辑、审核、删除、收藏和状态变化均写 outbox；worker 以 MySQL 当前事实收敛 | 单元测试、迁移核验、worker 自动重启 | 通过 |

因此，除腾讯云租户端到端验证外，交易闭环没有“伪完成”项。

## 3. 搜索验收结果

### 3.1 架构

- 使用 `github.com/meilisearch/meilisearch-go v0.36.3` 对接 Meilisearch。
- 验收实例为官方 Meilisearch Windows v1.46.0。
- 可搜索字段为 `title`、`description`。
- 可过滤字段包含 `status`、`reviewStatus`、`category`、`campus`、`sellerId`。
- 有关键词时必须走 Meilisearch；不可用时返回 503，不静默退化为 `%LIKE%`。
- Meilisearch 只返回候选 ID；最终展示必须由 MySQL 复核 `approved + 未删除 + 状态匹配`。
- 写操作通过 `trade_search_outbox` 与业务数据同事务登记，worker 等待 Meilisearch 异步任务完成后才标记处理成功。
- worker 异常退出后 5 秒自动重启并重新排队事实快照，避免同步永久静默停止。

### 3.2 实测

| 用例 | 结果 |
| --- | --- |
| 中文标题“高等数学教材”按“数学”命中 | 通过 |
| 中文描述“适合宿舍书桌”按“宿舍”命中 | 通过 |
| selling/sold 状态过滤 | 通过 |
| 删除文档后不可检索 | 通过 |
| 陈旧索引结果经 MySQL 二次校验剔除 | 通过 |
| 交易域静态扫描无 `LIKE` 搜索 | 通过 |

真实集成命令：

```text
go test ./internal/module/trade -run TestMeilisearchTitleDescriptionAndDeleteAgainstConfiguredInstance -count=1 -v
PASS
```

## 4. 腾讯云 IM 验收结果

### 4.1 已完成

- 小程序使用官方 `@tencentcloud/chat v3.6.6`，等待 `SDK_READY` 后进入会话。
- UserSig 仅由后端签发；后端先执行真实 REST `account_import`，失败时直接返回错误，不生成本地伪成功。
- 稳定用户标识为 `campus_<user_id>`。
- C2C 会话使用腾讯云约定的 `C2C<peerUserID>`；同一买卖双方可按不同商品维护多条业务关系。
- 小程序使用 SDK 的 `getConversationProfile`、`getMessageList`、`createTextMessage`、`sendMessage` 和 `MESSAGE_RECEIVED`。
- 数据库只保存用户映射和商品会话关系；代码及迁移中不存在消息内容表。
- sold 商品禁止创建新的联系关系。

### 4.2 自动化结果

| 用例 | 结果 |
| --- | --- |
| UserSig 生成及稳定用户 ID | 通过 |
| 买卖双方 Account Import 调用顺序 | 通过 |
| 官方 REST 路径、管理员 identifier、UserSig、请求体 | 通过（本地 HTTP 协议测试） |
| Account Import 失败时不落本地伪会话 | 通过 |
| 买卖双方会话视角返回正确 peer ID | 通过 |
| 官方 JS SDK 类型检查与生产构建 | 通过 |
| 腾讯云真实租户 Account Import | 未执行：缺少验收凭证 |
| 微信开发者工具/真机 C2C 双向收发 | 未执行：依赖真实租户登录 |

### 4.3 数据最小化

允许持久化：`user_id ↔ im_user_id` 映射、买方、卖方、商品引用、会话标识及时间戳。  
禁止持久化：消息正文、图片消息、语音、文件、消息历史副本。当前实现符合该边界。

## 5. 状态机验收结果

| 当前状态 | 目标状态 | 结果 |
| --- | --- | --- |
| selling | reserved | 允许 |
| selling | sold | 允许 |
| reserved | selling | 允许 |
| reserved | sold | 允许 |
| sold | 任意状态 | 拒绝，终态 |
| selling | selling | 拒绝 |
| reserved | reserved | 拒绝 |

附加约束均已验证：

- 仅商品卖家可以修改状态。
- 仅审核通过、未删除商品可以流转。
- 数据库更新携带旧状态条件；并发竞争只有一个请求成功，失败方返回冲突。
- 状态变化与 outbox 写入同事务。
- sold 商品仍可按 sold 筛选和详情展示，但禁止新增收藏、联系、编辑和删除。

## 6. 并发一致性验收结果

真实 MySQL 8.4.0 集成用例使用真实卖家、买家和外键关系执行：

| 场景 | 预期 | 结果 |
| --- | --- | --- |
| 同一用户十路并发切换收藏 | 关系数与计数最终一致，不漂移 | 0 条关系、计数 0，通过 |
| selling 同时竞争 reserved 与 sold | 仅一个 CAS 成功 | 通过 |
| 重复联系同一商品 | 复用唯一业务关系 | 仓储幂等约束通过 |
| 多个索引事件乱序 | 以 MySQL 当前事实 upsert/delete | 服务测试通过 |

命令：

```text
go test ./internal/module/trade -run TestMySQL -count=1 -v
PASS
```

## 7. 权限与安全验收结果

下表路径均以 `/api/v1` 为前缀。

| 接口 | 鉴权 | 业务校验 |
| --- | --- | --- |
| `GET /trade/products` | 公开 | 仅返回审核通过、未删除及请求状态商品 |
| `GET /trade/products/:id` | 公开 | 仅返回审核通过、未删除商品 |
| `GET /trade/my-products` | 用户 | 仅当前用户商品 |
| `POST /trade/products` | 用户、未禁言 | 卖家取自 token，不接受客户端冒充 |
| `PUT /trade/products/:id` | 用户、未禁言 | 仅卖家；sold 禁止编辑；重新进入审核 |
| `DELETE /trade/products/:id` | 用户 | 仅卖家；sold 禁止删除 |
| `PATCH /trade/products/:id/status` | 用户 | 仅卖家；审核状态和状态机校验；CAS |
| `POST /trade/products/:id/favorite` | 用户 | 与当前用户绑定；sold 禁止新增 |
| `POST /trade/products/:id/contact` | 用户 | 非卖家本人；approved、未删除、非 sold |
| `GET /im/usersig` | 用户 | 只签发当前用户身份并执行账号导入 |
| `GET /im/conversations` | 用户 | 仅返回当前用户作为买方或卖方的关系 |

P0-1 的 JWT、封禁、禁言及管理员权限实现未被本次修改。P0-2 和统一审核流亦未被改写。

## 8. 自动化测试与构建结果

| 检查 | 结果 |
| --- | --- |
| `go test ./...` | 通过 |
| `go vet ./...` | 通过 |
| `npm run check:boundaries` | 通过，A-F 边界检查为 `ok` |
| 管理端 TypeScript | 通过 |
| 小程序 TypeScript | 通过 |
| `npm run verify` | 通过 |
| 小程序生产构建 | 通过，有 bundle 体积告警 |
| 管理端生产构建 | 通过 |
| Meilisearch 真实集成测试 | 通过 |
| MySQL 8.4 真实迁移/并发测试 | 通过 |
| 腾讯云真实租户测试 | 跳过，凭证未配置 |

## 9. 数据库迁移结果

迁移文件：`backend/migrations/006_transaction_closure.sql`

新增或调整内容：

- 新建 `trade_search_outbox` 及 pending/product 索引。
- 新增商品公开查询复合索引：`review_status,status,deleted_at,created_at,id`。
- 新增用户收藏查询索引：`user_id,created_at`。
- IM 会话 `conversation_id` 改为普通索引，业务唯一约束为买方、卖方、引用类型、商品 ID。
- 新增买方/卖方会话列表复合索引。
- 将旧 IM 用户 ID 收敛为 `campus_<user_id>`。
- 为现存商品写入初始重建事件。

真实升级路径在 MySQL 8.4.0 执行：

```text
001_init.sql → 003_domain_convergence.sql → 004_auth_hardening.sql
→ 005_community_loop.sql → 006_transaction_closure.sql
PASS
```

验收过程中首次发现 006 对“旧库尚无 IM 表”的场景缺少保护，已改为检查表存在性并在创建过程前清理残留 procedure；修复后完整迁移链通过。迁移后核验：IM ID 异常数为 0，核心唯一约束和热点索引均存在，现有商品已排队重建索引。

## 10. 剩余风险清单

| 级别 | 风险 | 影响 | 整改/门禁 |
| --- | --- | --- | --- |
| P0 | 未用真实腾讯云租户完成 Account Import、SDK 登录和 C2C 双向收发 | 无法证明线上 IM 闭环 | 按第 12 节完成云端验收；通过前保持 NO-GO |
| P1 | 小程序 `vendors.js` 约 731 KiB，高于构建工具 244 KiB 建议值 | 首包加载和弱网体验风险 | 上线打包阶段评估分包；不得在本 P0-3 内扩展业务 |
| P1 | npm 审计仍报告既有依赖漏洞 | 供应链风险未在本专项消除 | 依赖专项逐项确认可利用性并升级，避免盲目 major 升级 |
| P1 | 搜索 outbox 为最终一致，故障恢复期间索引可能暂时滞后 | 搜索短暂漏结果 | MySQL 二次校验已防止越权暴露；监控/告警留给 P0-5 |
| P2 | 索引陈旧时 Meilisearch `estimatedTotalHits` 可能高于 MySQL 最终可见条数 | 分页总数短暂偏大 | 后续可增加校正策略；不影响数据安全 |
| P2 | 多实例 worker 可能重复处理同一 outbox 事件 | 增加 Meilisearch 写入量 | 操作幂等且以当前事实收敛；规模扩大后再引入 claim/lease |

## 11. 范围合规结论

本次没有引入支付、订单、物流、担保交易、自建聊天、图片上传或图片审核；没有修改被冻结的社区互动、通知、审核流和权限体系。Docker 仅增加 P0-3 必需的 Meilisearch 服务及配置，没有推进备份、监控、告警或回滚等 P0-5 内容。

## 12. MVP GO 前必须完成的 P0-3 门禁

1. 在隔离的腾讯云 IM 预发布应用配置 `TEST_TENCENT_IM_SDK_APP_ID`、`TEST_TENCENT_IM_SECRET_KEY`、`TEST_TENCENT_IM_ADMIN_USER`。
2. 执行 `go test ./internal/module/im -run TestTencentAccountImportAgainstConfiguredApplication -count=1 -v`，确认真实 Account Import 成功。
3. 使用两个真实测试用户在微信开发者工具或真机完成：买家联系卖家、双方账号导入、UserSig 登录、等待 SDK_READY、进入同一 C2C、双向发送与接收文本消息。
4. 验证腾讯云故障或错误凭证时接口失败且数据库不产生伪会话。
5. 验证 sold 后旧会话仍可按腾讯云策略查看，但 API 禁止新建商品联系关系；数据库仍无消息内容。
6. 保存脱敏验收证据并补发报告 v1.1，将唯一 P0 项改为通过后，P0-3 方可判定 MVP GO。

在该门禁完成前，不得进入 P0-4。
