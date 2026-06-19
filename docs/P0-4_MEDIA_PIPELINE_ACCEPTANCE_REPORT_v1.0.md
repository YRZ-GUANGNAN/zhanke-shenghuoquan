# P0-4 Media Pipeline Acceptance Report v1.0

验收日期：2026-06-19  
专项范围：社区帖子图片、二手商品图片、用户主动修改头像  
冻结范围：P0-1 权限、P0-2 社区闭环、P0-3 交易状态机/搜索/腾讯云 IM；未进入 P0-5

## 0. 验收结论

**P0-4 结论：CONDITIONAL PASS（条件通过，尚未正式关闭）。**

代码、本地 MySQL 8.4 迁移、并发一致性、类型检查和小程序构建均通过。当前已经形成真实的本地闭环：

`选择图片 → 客户端限边压缩/WebP 优先 → 精确对象级 STS → COS 直传 → 服务端 HEAD/Range/魔数/解码尺寸复验 → 微信异步内容安全 → 统一审核流 → 审核通过后绑定与展示 → 拒绝/删除后逻辑删除 → 定时物理删除 → 全状态审计`。

不能判定正式 PASS 的原因：当前环境没有腾讯云 COS、微信正式/测试环境凭证，真实 STS、真 COS 上传和微信内容安全通过/拒绝回调未完成云端联调；同时小程序现有 Taro 依赖树仍有 `swiper` Critical 公告。二者均不得用本地 Fake、HTTP 200 或代码存在性替代。

P0-4 正式关闭门禁：完成第 12 节的真实云联调，处理或书面豁免 Taro/Swiper Critical 风险，补齐媒体票据配额/限流，并补发报告 v1.1。此前禁止进入 P0-5。

## 1. 实施清单与边界检查

| 项目 | 结果 | 证据/说明 |
|---|---|---|
| 独立媒体域 | PASS | `internal/pkg/media` 独立拥有凭证、对象校验、审核、访问与回收；A/B/C 仅依赖 `MediaGateway` |
| 前端直传 | PASS | 小程序使用 `cos-wx-sdk-v5` 直传；后端不接收图片文件体 |
| 首次登录头像 | PASS | 登录请求不发送头像；后端创建用户时忽略 `LoginInput.Avatar`，仅客户端缓存预览 |
| 主动修改头像 | PASS | 必须走 `avatar` 资产上传、微信审核、批准、绑定后才更新正式头像 |
| 业务层 COS 隔离 | PASS | Gateway 只返回 `/api/v1/media/assets/{id}/content` 稳定媒体地址，不返回对象 Key、COS URL 或签名 |
| 社区/商品图片 | PASS | 业务仅提交 `imageAssetIds`；任意外部 URL 不再作为创建输入 |
| 冻结边界 | PASS | A-F 边界脚本通过；未修改交易状态机、Meilisearch 或腾讯云 IM 业务逻辑 |
| P0-5 | NOT STARTED | 未增加监控、备份、告警或回滚能力 |

主要实现：

- `backend/internal/pkg/media`：媒体状态、COS STS/对象访问、微信内容安全、审核回调、引用、审计和 GC；
- `backend/migrations/007_media_pipeline.sql`：媒体资产及 A/B/C 引用列；
- `backend/migrations/008_media_dimensions_and_audit.sql`：服务端尺寸与追加式审计表；
- `apps/miniapp/src/services/media.ts`：选择、压缩、直传、完成确认、审核轮询和取消；
- 社区、交易、用户模块只加入媒体引用接入，不改变冻结业务规则。

## 2. 一、上传链路验收

| 验收项 | 结果 | 验收结论 |
|---|---|---|
| 客户端选择 | PASS | 仅选择图片；帖子/商品最多 9 张，头像 1 张 |
| 客户端压缩 | PASS | 帖子/商品最长边 1920，头像 1024，质量 78；优先 WebP，不支持时回退运行时原生压缩格式并读取真实 MIME |
| 临时凭证签发 | CODE PASS / CLOUD PENDING | 腾讯云 STS 15 分钟，仅授权服务端生成的单一 Key 执行 `PutObject`；真实云未联调 |
| COS 上传 | CODE PASS / CLOUD PENDING | `cos-wx-sdk-v5 1.7.1` 前端直传；后端不做中转；真实 Bucket 未联调 |
| 完成确认 | PASS | 后端对 COS 执行 HEAD 与最多 1 MiB Range 读取，验证大小、Content-Type、文件魔数、实际解码格式及尺寸 |
| 回调处理 | PROTOCOL PASS / CLOUD PENDING | 支持微信签名校验及 JSON/XML 回调；真实回调尚无云端证据 |
| 失败重试 | PASS | 同一幂等键重试会刷新上传截止时间；审核失败指数退避；客户端最长轮询约 60 秒，之后提示稍后重试 |

后端硬限制：

| 用途 | 数量 | 单文件大小 | 最长边 | 格式 |
|---|---:|---:|---:|---|
| `post_image` | 9 | 10 MiB | 1920 px | JPEG/PNG/WebP |
| `product_image` | 9 | 10 MiB | 1920 px | JPEG/PNG/WebP |
| `avatar` | 1 | 5 MiB | 1024 px | JPEG/PNG/WebP |

前端限制用于节省流量，后端限制才是安全边界。SVG、GIF、视频、空文件、扩展名与 MIME 不匹配、Header 与魔数不一致、不能解码或超限尺寸均拒绝并进入删除队列。

## 3. 二、COS 验收

### 3.1 存储结构

对象 Key 仅由服务端生成：

`media/{purpose}/{ownerUserId}/{yyyy-mm}/{128-bit-random}.{ext}`

客户端不能指定目录或覆盖其他用户对象。数据库保存 Object Key，但业务模型只保存 `asset_id` 与 MediaGateway 稳定访问地址。

### 3.2 权限与访问

- STS Policy 只允许一个完整对象资源的 `name/cos:PutObject`；不授予 List、Get、Delete 或目录级写权限；
- 业务展示请求先查询媒体资产，只有 `approved + 已绑定 + 未请求删除 + 未物理删除` 才签发 5 分钟读取 URL；
- 未审核、拒绝、孤儿、已删除及越权对象不公开；
- MediaGateway 未泄露 COS URL，为未来 CDN、私有化存储或其他 ObjectStore 留出替换边界。

### 3.3 生命周期

- 上传票据数据库有效期 10 分钟，STS 15 分钟；
- 每分钟执行维护任务；
- 超时未上传立即进入删除；未提交/卡住审核、拒绝、批准但未引用等孤儿超过 24 小时进入删除；
- 业务删除或引用替换先标记 `deleted/delete_requested_at`，随后异步调用 COS Delete 并记录 `storage_deleted_at`；
- COS 控制台 Bucket Lifecycle 尚未配置/验证。应用 GC 是主路径，Bucket Lifecycle 应作为最终兜底但不能先行替代应用一致性。

## 4. 三、微信内容安全验收

| 项目 | 结果 | 说明 |
|---|---|---|
| 审核触发 | PASS（协议） | 完成对象复验后创建 `media_asset` 统一审核任务，进入 reviewing，再调用 `mediaCheckAsync` |
| 审核输入 | PASS（协议） | 使用 10 分钟 COS 签名读取 URL及当前用户 OpenID，不暴露永久公开地址 |
| Access Token | PASS（代码） | 服务端缓存并在到期前刷新；密钥只从环境变量读取 |
| 回调签名 | PASS | `token + timestamp + nonce` 排序后 SHA-1 校验；伪造签名拒绝 |
| 通过 | PASS（本地） | 微信结果驱动统一审核流 `reviewing → approved`；批准前不能 Reserve/展示 |
| 拒绝 | PASS（本地） | `reviewing → rejected → deleted`，不允许绑定，进入物理删除队列 |
| 延迟/失败 | PASS（本地） | 指数退避重试，维护任务恢复 `uploaded/submitted/reviewing` 无 trace 的任务 |
| 真实微信云 | PENDING | 缺少 AppID/Secret/回调环境，不得判为真实通过 |

当前回调采用微信明文签名模式，未实现 AES 安全模式。生产控制台必须与该模式一致；若强制启用安全模式，需要在 P0-4 关闭前补充解密支持并重新验收。

## 5. 四、数据一致性验收

### 5.1 媒体与业务引用

- `media_assets` 是生命周期真相源；`post_images.asset_id`、`product_images.asset_id`、`users.avatar_asset_id` 保存真实引用；
- Reserve 使用 MySQL 行锁、所有权/用途/状态/重复校验及 15 分钟租约；
- Commit 再校验 owner、token、approved、未绑定，并按业务引用顺序写入；
- 新头像或新商品图片绑定时，旧媒体引用在媒体域同一事务内标记删除；
- 用户资料跨域提交失败会回滚旧头像；社区/商品若在业务落库后媒体 Commit 遭遇数据库故障，内容仍停留为未审核、不可公开，但可能留下待人工清理的提交记录，列为中风险。

### 5.2 状态与审计

媒体主状态：

`pending_upload → uploaded → submitted → reviewing → approved`  
`pending_upload/reviewing → rejected → deleted`  
`任意未公开/失去引用状态 → deleted`

`media_asset_audits` 为追加式审计表，记录：`asset_id/from_status/to_status/action/actor_type/actor_id/detail/created_at`。已覆盖票据创建、上传完成、送审、审核状态、引用绑定/替换、删除请求和物理删除；审计数据不含 COS 签名或临时凭证。

### 5.3 数据库迁移

- `007_media_pipeline.sql` 已在 MySQL 8.4 执行；
- `008_media_dimensions_and_audit.sql` 已在 MySQL 8.4 实际执行；
- 已核验 `width/height` 列、`media_asset_audits` 表和 `(asset_id,created_at)`、`(action,created_at)` 索引；
- 已核验幂等唯一索引、审核扫描、删除扫描及引用复合索引，无 GORM 同义重复索引。

## 6. 五、并发验收

| 场景 | 结果 | 证据 |
|---|---|---|
| 重复上传 | PASS | 同一 owner + 幂等键 20 并发，只创建 1 条资产、返回 1 个 ID、产生 1 条创建审计 |
| 重复提交 | PASS | 完成接口按状态幂等；审核任务 `(object_type,object_id)` 唯一；状态 CAS/行锁阻止重复推进 |
| 并发绑定 | PASS | MySQL 两个并发 Reserve 只有 1 个成功；绑定再校验租约和状态 |
| 多图上传 | PASS | 客户端最多 3 路并发，返回顺序与选择顺序一致；业务端最多 9 张 |
| 异常中断 | PASS | 上传重试刷新票据截止时间；未完成对象由 GC 处理；审核调用失败由 worker 重试 |
| 引用替换 | PASS | 新引用绑定和旧引用删除在媒体域单事务完成，MySQL 集成测试已验证 |

Windows 当前 `CGO_ENABLED=0` 且无 GCC，因此未执行 Go `-race`。数据库并发测试使用真实 MySQL 行锁和唯一约束，不是内存 Fake；`-race` 仍建议在 CI Linux 工具链补跑。

## 7. 六、垃圾回收验收

| 类型 | 触发规则 | 结果 |
|---|---|---|
| 未完成上传 | `pending_upload` 且票据过期 | PASS |
| 未完成审核 | uploaded/submitted/reviewing 无法完成且超过 24 小时 | PASS |
| 审核拒绝 | 回调拒绝后立即请求删除 | PASS |
| 孤儿资产 | approved 但 24 小时未绑定 | PASS |
| 失去引用 | 业务删除、审核拒绝或引用替换 | PASS |
| 物理删除 | 每分钟批量 100 个；失败保留并记录错误，后续重试 | PASS |
| 审计 | 请求删除与物理删除分别落审计 | PASS |

真实 COS Delete、权限拒绝与 Bucket Lifecycle 尚待云端验收；本地 Fake 验证了任务选择、删除调用和 `storage_deleted_at` 落库。

## 8. 七、成本评估

估算假设：注册用户 10,000/20,000，日活 20%，每个日活每天 0.3 次图片上传，压缩后均值 400 KiB，未计历史存量和 CDN 命中。

| 规模 | 日上传数 | 月新增存储 | 若原图均值 2 MiB | 压缩节省 |
|---:|---:|---:|---:|---:|
| 10,000 用户 | 600 | 约 7.0 GiB | 约 35 GiB/月 | 约 80% |
| 20,000 用户 | 1,200 | 约 14.1 GiB | 约 70 GiB/月 | 约 80% |

成本构成：标准存储容量、PUT/HEAD/Range/GET/DELETE 请求、微信审核拉取与用户访问产生的下行流量。每次完成验收多 1 次 HEAD 和最多 1 MiB Range GET；换来服务端真实类型/尺寸校验。客户端压缩减少上传流量、COS 存储、审核拉取与展示下行四项成本。

实际人民币金额依赖 COS 地域、存储类型、流量包/CDN、免费额度和真实图片均值，当前无生产 Bucket 计费配置，故不伪造固定价格。上线前应使用 7 天预发布样本统计 `平均压缩前后字节、审核拒绝率、孤儿率、展示 GET/图片` 后代入腾讯云控制台价格计算。

生命周期建议：应用 GC 保持 10 分钟/24 小时规则；Bucket 增加未完成分片和 `media/` 临时孤儿的更长期兜底；正式展示接入 CDN 后仅修改 MediaGateway，不改 A/B/C 业务模型。

## 9. 八、安全验收

| 项目 | 结果 | 说明 |
|---|---|---|
| 越权上传/查询/删除 | PASS | owner 从 JWT 当前用户取得；查询、完成、取消均带 owner 条件 |
| 越权展示 | PASS | 仅 approved 且已绑定媒体可获取短时签名 URL |
| 路径覆盖 | PASS | Key 由服务端随机生成，STS 精确到单对象 |
| 类型绕过 | PASS | 扩展名、声明 MIME、COS Content-Type、魔数、实际解码格式五重一致性校验 |
| 尺寸/解码炸弹基础防护 | PASS | 仅读取最多 1 MiB 头部解析配置，限制总字节与最长边；解码失败即删除 |
| 恶意 SVG/脚本 | PASS | 不接受 SVG/GIF/HTML；展示前必须微信审核通过 |
| 临时凭证 | CODE PASS / CLOUD PENDING | 15 分钟、单 Key、仅 PutObject；真实越权 Key 拒绝测试待云端凭证 |
| 回调伪造 | PASS | token 签名校验；未知 trace/非法状态拒绝 |
| 敏感信息 | PASS | API 业务响应不含 SecretKey/ObjectKey/COS 签名；审计不记录凭证 |

依赖安全处理：

- `cos-wx-sdk-v5 1.8.0` 因 `fast-xml-parser` Critical 公告已移除，锁定 `1.7.1`，依赖为 `@xmldom/xmldom 0.8.13`；
- Go 首轮漏洞扫描发现 4 条可达漏洞，已升级至 Go 1.25、`x/image v0.39.0`、`x/net v0.55.0` 等公告修复版本；升级后测试和 vet 通过；
- 升级后的 `govulncheck` 复扫因 `vuln.go.dev` 多次返回 `unexpected EOF` 未取得完整报告，不能写成“零漏洞”；
- `npm audit --omit=dev` 仍报告 23 项：17 Moderate、3 High、3 Critical。Critical 来自 Taro 组件链的 `swiper 11.1.15`；不能在 P0-4 内盲目强升主版本，必须在正式关闭前完成兼容升级或安全负责人书面风险接受。

## 10. 九、自动化测试结果

| 命令/测试 | 结果 |
|---|---|
| `go test ./...` | PASS |
| `go vet ./...` | PASS |
| P0-4 媒体专项测试 | PASS：上传→审核→绑定→访问、MIME 伪造、超尺寸、幂等、重试、GC、微信协议 |
| MySQL 8.4 集成测试 | PASS：20 并发幂等、审计唯一、并发 Reserve、引用替换、GC 查询、索引 |
| 腾讯云 COS 真实集成测试 | SKIPPED：未配置 `TEST_COS_*` 凭证 |
| 微信内容安全真实云测试 | NOT RUN：未配置真实 AppID/Secret/回调环境 |
| `npm run check:boundaries` | PASS：A-F module boundaries |
| Admin + Miniapp TypeScript | PASS |
| `npm run build:miniapp` | PASS |
| 小程序构建警告 | `vendors.js` 约 977 KiB，超过 244 KiB 建议值；无异步分包 |
| Go `-race` | NOT RUN：Windows CGO/GCC 条件不具备 |
| 迁移 007/008 | PASS：真实 MySQL 8.4 执行与结构核验 |

## 11. 十、剩余风险清单

### 高风险

| ID | 风险 | 影响 | 整改/关闭条件 |
|---|---|---|---|
| H1 | COS STS、真实上传、越权 Key 拒绝、微信 pass/reject 回调没有真实云证据 | 代码正确不等于云配置可用，P0-4 不能正式关闭 | 使用隔离测试 Bucket/小程序执行第 12 节并保存脱敏证据 |
| H2 | Taro 依赖树含 `swiper 11.1.15` Critical 原型污染公告 | 小程序运行时供应链风险 | 评估升级/override 后做完整小程序回归；无法升级须安全负责人书面接受且限定影响面 |
| H3 | 媒体票据接口只有鉴权/禁言校验，尚无按用户/IP 的速率与小时配额 | 恶意登录用户可批量申请单 Key STS 并制造 COS 请求、存储和审核成本 | 在媒体接口增加可多实例一致的限流/配额并做突发并发测试；这属于 P0-4 安全关闭项，不是 P0-5 监控功能 |

### 中风险

| ID | 风险 | 影响 | 整改建议 |
|---|---|---|---|
| M1 | 微信回调仅支持明文签名模式 | 控制台若启用 AES 安全模式，回调失败 | 固定明文模式并验收，或在 P0-4 v1.1 前补 AES 解密 |
| M2 | 业务落库与 Media Commit 是跨域 Saga | 极端数据库故障可能留下不可公开的 pending 帖子/商品记录 | 增加媒体提交补偿任务/人工清理查询；不得跨模块直写表 |
| M3 | `vendors.js` 约 977 KiB | 首包和弱网体验下降 | 上线打包阶段做分包/按需加载，仅做工程优化，不扩业务 |
| M4 | 升级后的 Go 漏洞复扫受官方数据库 EOF 阻断 | 无法提供“零可达漏洞”最终证据 | CI 网络稳定后重跑 `govulncheck ./...` 并归档 |
| M5 | COS Bucket Lifecycle 未真实配置 | 应用 GC 长期失效时残留增加 | 云验收时增加保守兜底规则，不替代应用引用一致性 |
| M6 | 统一审核任务与媒体状态分属两个事务 | 极端数据库故障可能出现媒体已变更、审核任务保存失败 | 保持 P0-2 审核流冻结；在后续获准的审核一致性修复中增加幂等补偿，本阶段通过审计定位 |

### 低风险

| ID | 风险 | 影响 | 整改建议 |
|---|---|---|---|
| L1 | WebP 输出依赖微信运行时能力 | 老运行时可能回退 JPEG/PNG，压缩率略低 | 已按实际格式上传；真机统计 WebP 命中率 |
| L2 | 当前尺寸解析只读取对象前 1 MiB | 携带超大元数据的合法 JPEG 可能被拒绝 | 作为安全取舍保留；若有真实误拒样本再评估，不放宽到全量下载 |
| L3 | `media_asset_audits` 当前无归档策略 | 长期审计表增长 | 留待 P0-5 之后的数据保留策略评审，本阶段不删除审计 |

## 12. 十一、P0-4 正式 PASS 必须完成的真实云验收

1. 配置隔离 COS Bucket、Region、AppID、SecretId/SecretKey、微信 AppID/Secret 和回调 Token；
2. 真机获取 STS，上传授权 Key 成功；使用同一 STS 上传其他 Key 必须被拒绝；
3. 分别上传 JPEG、PNG、WebP，HEAD/Range、尺寸与 MIME 校验全部成功；伪 MIME、超尺寸、超大小全部拒绝并删除；
4. 微信内容安全至少准备一张 pass 和一张 reject 测试图片，保存 trace、回调、统一审核任务和媒体审计的脱敏证据；
5. pass 图片绑定帖子、商品、头像后能展示；未绑定、reviewing、reject、deleted 地址均不能展示；
6. 删除/替换后确认 COS 对象被物理删除，失败后重试可恢复；
7. 断网/杀进程后使用相同文件重试，资产 ID 不重复，GC 能回收残留；
8. 真机验证首次登录头像只在客户端缓存，数据库 `avatar/avatar_asset_id` 不落该头像；主动修改后才产生正式资产；
9. 处理 H2 供应链风险和 H3 媒体配额/限流，并在稳定网络补跑 `govulncheck`；
10. 补发 v1.1，将所有 CLOUD PENDING 改为 PASS 后方可正式关闭 P0-4。

## 13. P0-3 独立待办与项目状态

P0-3 继续保持 Conditional PASS；腾讯云 IM 双账号真机 `Account Import → UserSig → SDK_READY → 双向消息 → 断线重连 → 重新登录` 待办已独立记录在 `docs/P0-3_IM_CLOUD_VALIDATION_TODO.md`。P0-4 没有修改该链路。

**最终项目判定：P0-4 CONDITIONAL PASS；P0-5 禁止启动。**
