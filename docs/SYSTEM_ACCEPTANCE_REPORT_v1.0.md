# 系统验收报告 v1.0

> 项目：Campus Circle 六域系统  
> 审计日期：2026-06-18  
> 审计阶段：Phase 2 工程验收与风险排查  
> 结论：**不通过（NO-GO），当前不满足 MVP 上线标准**

## 1. 执行摘要

领域收敛本身已经成立：A–F 目录边界清晰，`npm run check:boundaries` 通过，业务数据库访问只出现在各域 repository。TypeScript 类型检查、Go 测试和 `go vet` 均通过。

但当前仍是“可编译的工程骨架”，不是可安全上线的 MVP。主要阻断原因如下：

1. Docker 部署会使用默认 JWT 密钥，并因缺少微信 Secret 进入 mock 登录；开发种子同时创建 `mock_openid_admin`，形成可直接获取管理员权限的完整利用链。
2. JWT 携带 30 天有效的角色快照，封禁、禁言或角色变更不会使已签发 token 失效。
3. 社区、交易和后台三个核心闭环均未完整接通前端，腾讯云 IM 只创建会话关系，没有真正启动 SDK 会话。
4. 审核状态更新跨服务且无事务/版本控制，并发审核可能产生对象状态与审核任务不一致。
5. 无限流、无 HTTP 服务端超时、无请求体上限、无 TLS、安全响应头和生产密钥管理。
6. `npm audit` 检出 34 项依赖告警；生产依赖口径仍有 22 项，其中 3 critical。Docker 使用已过期的 Go 1.22 构建线。
7. 绝大多数后端包测试覆盖率为 0%，没有数据库集成测试、API 鉴权测试、E2E、迁移演练或负载测试。
8. 没有备份、恢复、监控、告警、结构化日志或可回滚发布基线；当前 Git 仓库没有任何提交，全部文件均未跟踪。

### 1.1 风险统计

| 级别 | 数量 | 上线要求 |
| --- | ---: | --- |
| 高风险 | 15 | 全部关闭后才允许上线 |
| 中风险 | 18 | P1 项上线前关闭，其余必须有书面接受与监控 |
| 低风险 | 8 | 纳入首个稳定性迭代 |

## 2. 审计范围、方法与限制

### 2.1 已执行

- 全量扫描 Go、TypeScript、SQL、Docker、Nginx、环境变量和脚本。
- 提取并核对 41 个 HTTP 接口及其中间件。
- 执行 `npm run verify`：领域边界、TypeScript、Go 测试全部通过。
- 执行 `go vet ./...`：通过。
- 执行 `go test -cover ./...`：仅 `admin/review` 26.9%、`trade` 1.5%，其余业务包均为 0%。
- 执行 `npm audit --json`：34 项（3 critical、11 high、20 moderate）。
- 执行 `npm audit --omit=dev --json`：22 项（3 critical、3 high、16 moderate）。
- OSV 版本查询：四个直接 Go 三方模块未返回版本匹配公告；Go 1.22.12 stdlib 返回多项后续公告。

### 2.2 限制

- 本机没有 Docker CLI，因此未实际启动 Compose、执行 MySQL 迁移或做容器健康检查。
- `govulncheck` 因 Go 模块代理连续 EOF 未完成；因此不能声明 Go 调用链“无漏洞”。
- 未获得真实生产数据量、DAU、峰值 QPS 和服务器规格；性能结论为基于代码路径的容量预测，不是压测结果。
- 当前仓库无 commit，无法给报告绑定不可变 commit SHA；这是报告本身的可追溯性限制。

## 3. 总风险清单

### 3.1 高风险

| ID | 问题 | 证据/影响 | 整改方案 | 优先级 |
| --- | --- | --- | --- | --- |
| H-01 | 生产 JWT 使用默认密钥 | Compose 未注入 `JWT_SECRET`；配置默认 `dev-secret-change-me`。攻击者可离线伪造任意用户/管理员 token。 | 启动时若 `APP_ENV=prod` 且密钥缺失、过短或等于默认值则拒绝启动；密钥进入 Secret Manager，立即轮换。 | P0 |
| H-02 | mock 管理员登录后门 | `002_seed_dev.sql` 创建 `mock_openid_admin`；Compose 仅设置 `WECHAT_APP_ID`，未设置 Secret，代码退回 `mock_openid_<code>`。`code=admin` 可登录种子管理员。 | 生产镜像/迁移禁止执行开发 seed；生产缺少微信双密钥必须拒绝启动；彻底移除生产可达的 mock 分支。 | P0 |
| H-03 | JWT 角色与状态 30 天不刷新 | token 内嵌 role，有效期 30 天；中间件不查用户状态。已封禁、禁言、降权用户继续拥有原权限。 | 短期 access token（建议 15–30 分钟）+ refresh；每次敏感操作读取当前用户状态/权限或校验 tokenVersion；封禁立即撤销会话。 | P0 |
| H-04 | `muted` 状态完全未执行 | 仅登录时拒绝 `banned`；所有写接口只验 token。禁言用户仍可发帖、评论、发布商品和举报。 | A 域输出当前访问主体和状态；写接口统一应用 active/write 权限中间件。 | P0 |
| H-05 | 无限流、无服务端超时和请求体上限 | 登录、发帖、评论、点赞、举报、usersig、联系卖家均可无限调用；`router.Run` 使用默认 HTTP Server。可被刷库、刷外部微信 API或 Slowloris。 | 网关和应用双层限流；设置 ReadHeader/Read/Write/IdleTimeout、MaxHeaderBytes、JSON body 上限；按 IP+用户分级配额。 | P0 |
| H-06 | 审核流并发与一致性不安全 | `setTarget` 先修改业务对象，再单独 `Save` 审核任务；无事务、乐观锁或 CAS。两个管理员可同时 start/decide。 | 审核任务增加 version；状态更新使用 `WHERE id=? AND status=?` CAS；采用 outbox/补偿任务保证跨域最终一致，并记录不可变审核事件。 | P0 |
| H-07 | 管理员体系无超级管理员隔离 | 只有 `user/admin`；任一 admin 可封禁自己或其他 admin、审核全部内容、改全部导航。无双人复核和权限审计。 | 增加明确的 superadmin 管理边界或将管理员账号生命周期移出业务 API；普通管理员不得操作管理员账号；关键操作审计。 | P0 |
| H-08 | 举报处置闭环无效 | 举报允许任意 targetType/targetID；审核通过只修改 report 自身状态，不隐藏目标、不触发封禁/处置。 | 校验目标存在且类型受白名单约束；定义“举报审核结论→目标处置”的明确服务命令和审计记录；处置与账号状态立即生效。 | P0 |
| H-09 | 点赞计数存在并发漂移 | 两个并发请求均可能未查到 like；`ON CONFLICT DO NOTHING` 后仍各自 `liked_count + 1`。计数与关系表不一致。 | 用原子插入的 RowsAffected 决定是否增量；取消操作同理；定期以关系表对账修复计数。 | P0 |
| H-10 | 生产供应链存在 critical 依赖 | `swiper@11.1.15` 命中 GHSA-hmx5-qpq5-p643 原型污染；Taro/Webpack 构建链还有 high/critical。 | 不使用 npm 提议的盲目降级；统一升级到已修复且相互兼容的 Taro 版本，使 swiper ≥12.1.2，并重跑构建、E2E、audit。 | P0 |
| H-11 | Docker Go 构建基线过期 | `golang:1.22-alpine` 为已停止维护的分支；OSV 对 1.22.12 返回大量 stdlib 公告，且镜像 tag 可漂移。 | 升级到当前受支持 Go 版本，固定 patch 和 digest；用可用网络完成 `govulncheck` 并纳入 CI。 | P0 |
| H-12 | 部署没有 TLS、秘密管理与网络隔离 | Nginx 只监听 HTTP；MySQL 以固定弱口令并映射宿主端口；后端 8080 也直接暴露。 | TLS 终止、HSTS；数据库只在内部网络；随机凭据由 Secret 管理；防火墙仅暴露 443。 | P0 |
| H-13 | 无备份/恢复/监控/告警 | 无数据库备份脚本、恢复演练、指标、错误告警或容量监控。故障后 RPO/RTO 不可承诺。 | 定义 RPO/RTO；自动全量+增量备份、异地副本、季度恢复演练；接入指标、日志、追踪和告警。 | P0 |
| H-14 | 核心业务闭环未完成 | 社区关注/通知不存在，评论/点赞/收藏 UI 未接；交易搜索、IM SDK、售出 UI 未接；后台举报处置不闭环。 | 不在本验收阶段开发；上线前逐项接通已有能力，缺失能力必须获得明确范围授权或从 MVP 声明中移除。 | P0 |
| H-15 | 无可回滚版本与有效测试网 | Git 无任何 commit、所有文件未跟踪；业务包几乎无测试，且无 API/DB/E2E/迁移/负载测试。 | 立即建立受保护主干和发布 tag；补齐 P0 路径测试、MySQL 集成测试、鉴权矩阵、迁移回滚和最小压测门槛。 | P0 |

### 3.2 中风险

| ID | 问题 | 影响与整改 | 优先级 |
| --- | --- | --- | --- |
| M-01 | 热点查询缺组合索引 | feed、商品列表、评论、审核队列均会 filesort/扫描。按第 4.4 节添加组合索引并用真实 `EXPLAIN ANALYZE` 验证。 | P1 |
| M-02 | `%keyword%` 搜索无法使用 B-tree | 帖子和商品搜索随数据量线性退化。上线前至少评估 MySQL FULLTEXT；20k 规模以上准备独立搜索方案。 | P1 |
| M-03 | OFFSET + 每页 COUNT | 深页越来越慢，每次列表执行 COUNT。公共 feed/商品/会话改游标分页；后台总数可异步或按需。 | P1 |
| M-04 | 无缓存层 | 导航、统计、首页 feed、用户展示信息每次查库。引入前先测量；建议导航 5–30 分钟、统计 30–60 秒、feed 短 TTL。Redis 当前不存在。 | P1 |
| M-05 | 无外键且清理不完整 | 删除/软删除可能留下图片、收藏、会话和审核孤儿。保持领域边界的前提下增加应用级清理、约束检查和定期孤儿审计。 | P1 |
| M-06 | 软删除与唯一键冲突 | `users.open_id` 全局唯一；软删除用户无法重新注册。需要明确恢复策略或采用包含删除标识的唯一策略。 | P1 |
| M-07 | 审核任务覆盖历史 | `(object_type,object_id)` 唯一，重新提交会覆盖原任务状态、操作员和理由；无法形成完整审计轨迹。增加不可变 review_events。 | P1 |
| M-08 | 内容创建与审核任务非原子 | 先创建帖子/评论/商品/举报，再创建审核任务；后者失败会留下永远不可展示的孤儿对象。使用 outbox/补偿扫描。 | P1 |
| M-09 | 外部安全检查与客户端超时冲突 | 后端微信安全检查可等 5 秒，小程序默认超时 3 秒；客户端可能报失败后重试并产生重复提交。统一超时预算并提供幂等键。 | P1 |
| M-10 | 输入缺少长度、数量和枚举约束 | TEXT、图片数组、举报详情无应用层上限；状态/类型主要靠字符串。加入 validator、图片数量限制、DB CHECK 和请求体限制。 | P1 |
| M-11 | 图片 URL 任意且无图片审核 | 服务端只存客户端 URL；无域名白名单、文件类型/大小/内容审核。可能加载追踪或违规资源。若保留图片，必须采用受控对象存储和图片审核。 | P1 |
| M-12 | CORS 配置不正确 | `Allow-Origin:*` 与 credentials 同时设置；所有来源均可携带 Authorization 发起跨域 API 请求。使用生产来源白名单并区分后台域名。 | P1 |
| M-13 | 管理 token 存于 localStorage | 任意后台 XSS 可窃取长期管理员 token。结合短 token、CSP、严格依赖和受控会话存储降低风险。 | P1 |
| M-14 | 无安全响应头 | 缺少 CSP、frame-ancestors/X-Frame-Options、nosniff、Referrer-Policy、Permissions-Policy。由 Nginx 统一设置。 | P1 |
| M-15 | IM 会话并发创建存在竞态 | 先查后插；唯一键冲突会直接报错而不是返回既有关系。改为原子 upsert 后读取。 | P1 |
| M-16 | 自动迁移与 SQL 迁移并存 | 启动执行 AutoMigrate，同时 seed 工具重复执行所有 SQL，无 schema version 表。生产变更不可审计、不可可靠回滚。统一版本化迁移工具，生产禁用 AutoMigrate。 | P1 |
| M-17 | 健康检查过浅 | `/healthz` 永远返回 ok，不验证数据库；Compose 也没有 backend/admin healthcheck。区分 liveness/readiness 并验证关键依赖。 | P1 |
| M-18 | 旧 mock API 与真实契约漂移 | `dev:api` 仍返回旧 `/market`、`/tools` 和数组结构，可能让开发/验收跑错系统。移除脚本或更新为由正式 OpenAPI 生成的 mock。 | P2 |

### 3.3 低风险

| ID | 问题 | 整改 | 优先级 |
| --- | --- | --- | --- |
| L-01 | 自定义 token 格式非标准 JWT | 目前 HMAC 比较正确，但缺 iss/aud/iat/jti/kid 和标准库生态。迁移到成熟 JWT/PASETO 库。 | P2 |
| L-02 | 错误信息不统一且可能泄漏内部错误 | 多个 handler 直接返回 `err.Error()`。生产返回稳定错误码，详细错误只进服务端日志。 | P2 |
| L-03 | 无 OpenAPI/接口版本文档 | 当前只能从代码提取契约。生成 OpenAPI 并在 CI 做兼容检查。 | P2 |
| L-04 | 日志无 request_id、actor、审计字段 | Gin 默认日志不够关联一次请求或管理员动作。采用结构化日志并脱敏。 | P2 |
| L-05 | 无压缩与公共缓存头 | 导航和公开列表可配置 gzip/brotli、ETag/Cache-Control，但需避免缓存私有接口。 | P2 |
| L-06 | 今日统计按 `Truncate(24h)` | 在 Asia/Shanghai 下不是本地零点，日报口径会偏移。使用明确业务时区的当天起点。 | P2 |
| L-07 | 导航更新使用 Gorm struct Updates | `false`、`0` 等零值可能不会更新，导致无法禁用或清零排序。改为显式字段 map/Select。 | P2 |
| L-08 | Docker/依赖 tag 未固定 digest | `mysql:8.0`、`alpine:3.20`、`nginx:1.27-alpine`、Node/Go tag 都可漂移。固定 patch、digest 并定期更新。 | P2 |

## 4. 数据库审计

### 4.1 表结构清单

| 域 | 表 | 主用途 | 审计结论 |
| --- | --- | --- | --- |
| A | `users` | 微信身份、资料、角色、状态 | `open_id` 唯一有效；角色/状态无 DB CHECK；软删除与唯一键冲突 |
| B | `posts` | 帖子与公开计数 | 有软删除；缺公开 feed 组合索引 |
| B | `post_images` | 帖子图片 | `(post_id,sort)` 唯一合理；无 FK/URL 约束 |
| B | `comments` | 评论 | 缺 `(post_id,review_status,created_at)` 组合索引；无删除策略 |
| B | `post_likes` | 点赞关系 | 复合主键合理；代码存在并发计数漂移 |
| B | `post_favorites` | 帖子收藏 | 复合主键合理；没有按用户反查索引，但当前无“我的收藏”接口 |
| C | `products` | 商品、业务状态、审核状态 | 两类状态已分离；缺组合索引与 DB CHECK |
| C | `product_images` | 商品图片 | `(product_id,sort)` 唯一合理；商品软删后保留 |
| C | `product_favorites` | 商品收藏 | 复合主键合理；无用户反查索引 |
| D | `navigation_categories` | 导航分类 | 结构符合纯数据域；缺 enabled+sort 组合索引 |
| D | `navigation_items` | appid/path 跳转项 | 字段边界正确；缺 category+enabled+sort 组合索引 |
| E | `im_user_mappings` | 本地用户到腾讯 IM 用户 | user_id、im_user_id 双唯一合理 |
| E | `im_conversations` | 商品会话关系 | 两个唯一约束合理；OR 查询与排序索引不足 |
| F | `review_tasks` | 当前审核状态 | 对象唯一避免重复任务，但覆盖历史；status+created_at 索引不足 |
| F | `reports` | 举报 | targetType/ID 无引用验证；列表和审核缺组合索引 |

### 4.2 唯一约束

有效约束包括用户 openID、帖子/商品图片顺序、互动复合主键、IM 用户映射、IM 会话关系和审核对象。主要风险：

- review_tasks 的对象唯一约束把“当前状态”和“审计历史”混为一张表。
- soft-deleted user 仍占用 openID。
- 没有数据库外键，跨表完整性完全依赖应用代码。
- 商品图片、收藏和会话关系不会随软删除自动处理。

### 4.3 软删除策略

仅 `users`、`posts`、`products` 使用 Gorm `DeletedAt`。其余表为硬删除或没有删除路径。当前策略缺少恢复 API、级联清理、保留周期和匿名化规范。上线前必须明确：

1. 用户封禁、注销、软删除是三种不同状态；注销后 PII 何时匿名化。
2. 帖子/商品删除后，图片、收藏、审核任务、会话关系如何处理。
3. 软删除数据是否进入后台统计和备份保留。

### 4.4 查询热点与索引建议

以下索引需以真实数据执行 `EXPLAIN ANALYZE` 后确认，不能直接盲加：

- posts：`(review_status, deleted_at, pinned, created_at, id)`；话题 feed 增加 topic 维度。
- comments：`(post_id, review_status, created_at, id)`。
- products：`(review_status, status, deleted_at, created_at, id)`；分类列表增加 category。
- review_tasks：`(status, created_at, id)`。
- reports：`(review_status, created_at, id)` 和 `(target_type,target_id)`。
- navigation_categories：`(enabled,sort,id)`；items：`(category_id,enabled,sort,id)`。
- im_conversations：分别建立 `(buyer_id,updated_at,id)`、`(seller_id,updated_at,id)`，或将 OR 改写为 UNION。
- users/posts/products：统计若保留实时查询，需要 created_at 索引；更优方案是异步聚合。

### 4.5 潜在慢查询

1. `title LIKE '%keyword%' OR content/description LIKE '%keyword%'`：必然全表扫描。
2. feed 先 COUNT 再 ORDER/OFFSET：深页双重开销。
3. popular 排序涉及多计数字段，数据增长后 filesort 明显。
4. IM `buyer_id=? OR seller_id=? ORDER BY updated_at`：容易 index merge + filesort。
5. 后台 stats 每次发起 7 个独立 COUNT，且无缓存。

## 5. API 审计

### 5.1 接口清单与鉴权

| 分组 | 方法与路径 | 鉴权 | 权限/说明 |
| --- | --- | --- | --- |
| 健康 | `GET /healthz` | 无 | 仅进程存活 |
| A | `POST /api/v1/auth/wechat-login` | 无 | 微信或 mock 登录；当前生产配置危险 |
| A | `GET /api/v1/auth/me` | 用户 | 只读当前 token 用户 |
| A | `PATCH /api/v1/auth/me` | 用户 | 更新本人资料 |
| B | `GET /community/posts` | 无 | 仅 approved，page/pageSize/sort/topic/keyword |
| B | `GET /community/posts/:id` | 无 | 仅 approved |
| B | `GET /community/posts/:id/comments` | 无 | 仅 approved，分页 |
| B | `POST /community/posts` | 用户 | 发帖并提交审核 |
| B | `POST /community/posts/:id/comments` | 用户 | 评论并提交审核 |
| B | `POST /community/posts/:id/like` | 用户 | toggle |
| B | `POST /community/posts/:id/favorite` | 用户 | toggle |
| C | `GET /trade/products` | 无 | 默认 selling+approved，分页/分类/搜索 |
| C | `GET /trade/products/:id` | 无 | approved，但不限制业务状态 |
| C | `POST /trade/products` | 用户 | 发布并审核 |
| C | `PUT /trade/products/:id` | 用户+所有者 | 编辑并重新审核 |
| C | `DELETE /trade/products/:id` | 用户+所有者 | 软删除 |
| C | `PATCH /trade/products/:id/status` | 用户+所有者 | 状态机校验 |
| C | `POST /trade/products/:id/favorite` | 用户 | toggle |
| C | `POST /trade/products/:id/contact` | 用户 | 联系卖家关系；未阻止 sold 商品 |
| D | `GET /navigation/categories` | 无 | 仅 enabled 数据 |
| E | `GET /im/usersig` | 用户 | 腾讯 IM UserSig |
| E | `GET /im/conversations` | 用户 | 仅 buyer/seller 本人的关系，分页 |
| F | `POST /reports` | 用户 | 未校验目标类型/存在性 |
| F | `GET /admin/stats` | admin | 实时统计 |
| F | `GET /admin/reviews` | admin | 分页/状态过滤 |
| F | `POST /admin/reviews/:id/start` | admin | 开始审核 |
| F | `POST /admin/reviews/:id/approve` | admin | 通过 |
| F | `POST /admin/reviews/:id/reject` | admin | 拒绝 |
| F | `GET /admin/reports` | admin | 举报分页 |
| F | `GET /admin/users` | admin | 用户分页 |
| F | `PATCH /admin/users/:id/status` | admin | 任意用户状态，包括管理员 |
| F | `GET /admin/posts` | admin | 全状态帖子分页 |
| F | `GET /admin/products` | admin | 全状态商品分页 |
| F | `GET /admin/navigation` | admin | 全导航树 |
| F | `POST/PUT/DELETE /admin/navigation/categories[/:id]` | admin | 分类管理 |
| F | `POST/PUT/DELETE /admin/navigation/items[/:id]` | admin | 导航项管理 |

### 5.2 分页机制

- 所有列表将 page 归一到 ≥1、pageSize 默认 20、最大 100，基础防护合理。
- 使用 OFFSET 分页，公共高频列表缺少稳定 cursor；相同 created_at 下也没有 id 次级排序，翻页期间可能重复/漏项。
- 导航树无分页，当前数据量小可接受。

### 5.3 越权检查

已确认的正向控制：商品编辑、删除、状态变更均校验 sellerID；个人资料固定为 token userID；IM 会话列表仅 buyer/seller；公开内容只展示 approved。

未通过项：管理员权限完全依赖 token 内旧 role；封禁/禁言不生效；举报目标任意；管理员之间无权限隔离；无 superadmin。

### 5.4 限流建议

当前限流策略：**无**。建议初始基线（最终以压测和业务数据调整）：

- 登录：IP 10 次/分钟，设备 20 次/小时。
- 发帖/商品：用户 10 次/小时；评论 30 次/分钟。
- 点赞/收藏：用户 60 次/分钟，接口需幂等。
- 搜索/feed：IP 120 次/分钟。
- 举报/联系卖家/usersig：用户 10 次/分钟。
- 管理后台写操作：管理员 30 次/分钟，并记录审计日志。

## 6. 安全审计

### 6.1 JWT 与权限

自定义 HMAC-SHA256 签名使用常量时间比较，未发现直接签名绕过；但默认密钥、mock 管理员、长寿命角色快照和无撤销机制使整体鉴权评级为高风险。

### 6.2 SQL 注入

当前查询参数均通过 Gorm 占位符绑定，排序字符串来自服务端固定分支，未发现可确认的 SQL 注入点。仍应为输入做长度/枚举校验，并避免未来把 query 参数拼接进 Order/Raw。

### 6.3 XSS

前端未使用 `dangerouslySetInnerHTML`，React/Taro 文本默认转义，当前直接存储型 XSS 风险较低。后台 token 在 localStorage，且没有 CSP，因此一旦未来出现 XSS，影响会被放大为管理员会话泄露。

### 6.4 文件上传与 COS

当前没有服务端上传接口、COS 临时密钥或签名实现，因此不存在“当前 COS 签名可被绕过”的结论；但图片发布闭环也因此不完整。系统只是接收任意 URL，缺少域名、扩展名、MIME、大小、数量和图片安全审核。

上线选择必须二选一：MVP 明确不支持图片并隐藏入口；或在下一获批范围中使用短期临时凭证/服务端签名、固定 key 前缀、content-length-range、MIME 白名单、回调校验和内容审核。严禁前端持有永久 Secret。

### 6.5 Redis

当前代码和部署均没有 Redis，因此无 Redis 暴露或弱口令问题；同时也没有分布式限流、token 撤销和缓存能力。若后续引入，必须仅内网监听、启用 ACL/TLS、禁用危险命令并设置内存淘汰策略。

## 7. 性能审计

### 7.1 预测前提

用户总数不能直接等价于并发量。以下预测假设：日活 10%–20%，峰值读写比约 9:1，单实例后端、单 MySQL、连接池 20、无缓存。

| 注册用户 | 预测瓶颈 | 结论 |
| ---: | --- | --- |
| 5,000 | 安全检查外部调用延迟、stats 多 COUNT、无缓存；数据表较小时全扫描尚可掩盖 | 功能与安全整改后可做灰度；必须先压测 |
| 10,000 | feed/商品 COUNT+OFFSET+filesort、LIKE 搜索、IM OR 查询、20 连接池排队 | 需组合索引、缓存、限流和游标分页，否则 p95 明显抬升 |
| 20,000 | 搜索全扫描、热门排序、热点计数行锁、实时统计、单实例和 20 连接成为集中瓶颈 | 当前架构参数下不建议上线至该规模；需完成 P1 性能项并重新压测 |

### 7.2 热点接口

1. `GET /community/posts`：最高频，COUNT、排序、图片预加载、用户批量读取。
2. `GET /trade/products`：COUNT、LIKE 搜索、排序和卖家资料读取。
3. 点赞/收藏：写热点、事务和计数一致性。
4. `POST /posts|comments|products`：同步调用微信安全 API。
5. `GET /navigation/categories`：高读低写，最适合缓存。
6. `GET /admin/stats`：单请求执行多次 COUNT。
7. `POST /trade/products/:id/contact`、`GET /im/usersig`：容易被刷并创建持久关系/签名。

### 7.3 缓存建议

- L1 进程缓存：导航配置，TTL 1–5 分钟，写入后主动失效。
- 分布式缓存：导航 5–30 分钟、统计 30–60 秒、热门 feed 10–30 秒、公开用户展示资料 5 分钟。
- 不缓存管理员写接口、个人 token、审核决定。
- 缓存只是索引和查询治理后的优化，不能用来掩盖 `%LIKE%` 或错误分页。

## 8. 权限审计

| 主体 | 当前能力 | 预期边界 | 结论 |
| --- | --- | --- | --- |
| 游客 | 浏览 approved 社区、商品、导航 | 只读公开数据 | 基本通过 |
| 普通用户 | 资料、发帖评论互动、商品、举报、IM | 只能操作本人资料/商品/关系 | 商品所有权通过；禁言/封禁令牌失效不通过 |
| muted 用户 | 与普通用户完全相同 | 禁止产生内容，通常允许读取和申诉 | 不通过 |
| banned 用户 | 重新登录被拒，但旧 token 全部有效 | 立即失去所有受保护访问 | 不通过 |
| 管理员 | 全量审核、封禁任意账号、导航管理 | 运营权限受限且全审计 | 无细粒度权限、可互相操作，不通过 |
| 超级管理员 | 不存在 | 管理管理员生命周期和高危配置 | 缺失，不通过 |

建议权限模型：`user`、`content_admin`、`ops_admin`、`superadmin`，但是否引入新角色需另行授权；在获得授权前，至少应把管理员账号管理从普通 admin API 移除，并采用线下受控流程。

## 9. 依赖审计

### 9.1 Node/npm

- 依赖总数：1301（npm audit 元数据口径）。
- 全量：3 critical、11 high、20 moderate。
- `--omit=dev`：3 critical、3 high、16 moderate；由于 Taro 将部分构建工具放在 dependencies，此口径仍混有构建链。
- 可确认运行依赖风险：`@tarojs/components@4.2.0 -> swiper@11.1.15`，命中 [GHSA-hmx5-qpq5-p643](https://github.com/advisories/GHSA-hmx5-qpq5-p643)。
- 构建/供应链重点：webpack 5.91.0、webpack-dev-server 4.15.2、serialize-javascript 6.0.2、html-minifier、esbuild 等存在公告。
- `npm outdated` 显示 Babel、SWC、React、TypeScript 等有主版本更新；不能为追新而盲升，应优先安全修复并锁定兼容矩阵。

疑似可精简项需通过 Taro 官方依赖要求和构建验证确认：miniapp 的 `react-dom`、若干 Taro helper/shared/runtime 直接声明，以及根级 `@swc/core`。当前不应直接删除。

### 9.2 Go

- 直接依赖均有源码引用，未发现明确无用依赖。
- OSV 对 Gin 1.10.0、godotenv 1.5.1、Gorm 1.25.11、mysql driver 1.5.7 的精确版本查询未返回公告。
- 这不等于调用链安全；`govulncheck` 未成功完成。
- Docker Go 1.22 已停止获得安全修复，必须升级后重跑官方扫描。

## 10. 部署审计

### 10.1 正向项

- Go 和管理端使用多阶段构建。
- Go 最终镜像使用非 root 用户。
- MySQL 有基础 healthcheck，数据目录使用 named volume。
- Nginx 代理路径结构清楚。

### 10.2 缺口

- 生产秘密未注入，且存在默认值/硬编码凭据。
- 生产数据库初始化会执行开发 seed。
- 后端启动 AutoMigrate；迁移脚本没有版本表和回滚机制。
- 管理端 Docker 使用 `npm install`，没有复制 lock 后 `npm ci`，构建不可完全复现。
- 镜像未固定 digest；Go/Alpine/Nginx 基线过期或接近过期。
- 无 restart policy、资源限制、只读文件系统、capabilities 收缩。
- MySQL 和 backend 均映射到宿主端口。
- 无 TLS、安全头、访问日志轮转、WAF 或网关限流。
- 无 readiness、依赖健康、指标端点、追踪、错误告警。
- 无备份、恢复、灾备、发布回滚和数据库变更演练。

## 11. 业务闭环完整性

### 11.1 社区闭环

| 环节 | 后端 | 前端 | 结论 |
| --- | --- | --- | --- |
| 发帖 | 有，进入审核 | publish 页面已调用；登录和图片未闭环 | 部分完成 |
| 评论 | 有，进入审核 | 详情页仍是 toast，占位按钮 | 未闭环 |
| 点赞 | 有 toggle API/client | 页面未调用 | 未闭环 |
| 收藏 | 有 toggle API/client | 页面未调用 | 未闭环 |
| 关注 | 无模型/API | 首页仅有静态“关注”标签 | 缺失 |
| 通知 | 无模型/API | 无 | 缺失 |

社区闭环结论：**不完整**。如关注/通知不是本次授权范围，必须从 MVP 对外承诺中移除，不能以静态 UI 暗示可用。

### 11.2 交易闭环

| 环节 | 后端 | 前端 | 结论 |
| --- | --- | --- | --- |
| 发布商品 | 有，进入审核 | 已调用，图片未上传 | 部分完成 |
| 浏览/详情 | 有 | 已接 | 完成 |
| 搜索 | `%LIKE%` API 有 | 搜索框未绑定 keyword | 未闭环 |
| 收藏 | 有 | 商品详情已调用 | 基本完成 |
| 联系卖家 | 创建 IM 关系 | 只显示 conversationId toast | 未真正进入腾讯 IM 会话 |
| 售出 | 状态机 API 有 | 无卖家管理/标记售出入口 | 未闭环 |

交易闭环结论：**不完整**，不能宣称“发布到成交”可用。

### 11.3 后台闭环

| 环节 | 现状 | 结论 |
| --- | --- | --- |
| 审核 | 后端和后台 UI 已接 | 基本可操作，但并发/一致性/历史审计不合格 |
| 举报 | 后端可提交、后台可列出/审核；用户端无入口 | 未闭环 |
| 封禁 | 后台可改状态 | 旧 token 不失效，实际封禁不成立 |
| 举报→目标处置 | 不存在 | 缺失 |

后台闭环结论：**不完整**。

## 12. MVP 上线判定

### 12.1 判定

**NO-GO。** 当前不满足最小安全、功能闭环、可恢复和可观测要求。

### 12.2 上线前必须完成的任务清单

#### P0：任何联网环境前

1. 删除生产 mock 登录和开发 seed；生产缺微信 Secret/JWT Secret 时拒绝启动。
2. 轮换 JWT，实施短 token、撤销/版本校验，确保 banned/muted/admin 变更即时生效。
3. 建立 admin/superadmin 或等价的受控管理员生命周期；禁止普通 admin 操作管理员账号。
4. 修复审核并发、一致性和不可变审计历史；修复点赞计数竞态。
5. 加入限流、HTTP 超时、body 上限、TLS、安全响应头、生产 CORS 白名单。
6. 升级 Go 构建线和 Taro/Swiper 安全版本，`npm audit`/`govulncheck` 达到批准阈值。
7. 建立 Git commit/tag、可复现镜像和数据库版本化迁移；完成迁移与回滚演练。
8. 建立备份、恢复、监控、告警；至少完成一次恢复演练。
9. 完成登录、社区互动、交易 IM/售出、举报处置的端到端闭环；或书面缩减 MVP 范围并移除误导 UI。
10. 补齐鉴权矩阵、核心 service、MySQL repository、HTTP API、E2E 测试；覆盖 P0 利用链和状态机。

#### P1：正式放量前

1. 用生产级样本执行 `EXPLAIN ANALYZE`，补组合索引、游标分页和搜索方案。
2. 加入导航/统计/热门读取缓存和缓存失效策略。
3. 解决孤儿数据、软删除恢复/匿名化、举报目标验证和图片安全链。
4. 完成 5k/10k/20k 三档压测，给出 p50/p95/p99、错误率、DB CPU、连接池和慢查询结果。
5. 建立灰度、回滚、容量阈值和故障演练流程。

### 12.3 建议验收门槛

- P0 高风险为 0；P1 中风险有明确 owner、截止时间和监控。
- 核心 API p95 < 300 ms（不含明确异步任务），错误率 < 0.1%；具体阈值需产品/运维确认。
- 峰值目标 QPS 2 倍压测持续 30 分钟，无数据错乱、连接池耗尽或明显内存增长。
- 备份恢复演练满足已批准 RPO/RTO。
- 所有管理员动作可追踪到 actor、对象、前后状态、时间和 request_id。
- 发布物可绑定 commit SHA、依赖锁文件、镜像 digest 和迁移版本。

## 13. 最终意见

六域架构边界可以作为后续工程基础保留；当前阶段不应继续扩展业务域。下一阶段应严格围绕 P0 安全闭环、现有功能接线、数据一致性、测试、迁移和运维准备工作推进。只有在上述上线门槛被证据化验证后，项目才应从 NO-GO 重新申请 MVP 上线验收。
