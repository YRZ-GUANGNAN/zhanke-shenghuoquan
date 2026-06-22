# P0-1 登录与权限体系专项验收报告 v1.0

验收日期：2026-06-18  
验收范围：JWT、微信登录降级策略、封禁、禁言、管理员隔离、超级管理员体系  
范围约束：未推进社区、交易、图片上传或运维 P0；未新增范围外业务能力

## 1. 验收结论

**P0-1 代码级验收通过（PASS）**。

原报告确认的默认 JWT 密钥、生产 mock 管理员接管、旧 token 不受封禁/角色变化影响、禁言不生效、管理员可互相操作、缺少超级管理员等风险已经关闭。系统整体仍为 **NO-GO**，只有剩余 P0-2 至 P0-5 全部通过后才可申请 MVP GO 验收。

真实 MySQL 迁移尚未在本机执行：当前环境没有 Docker CLI 或可用的 MySQL 验收实例。因此 `004_auth_hardening.sql` 必须在预发布库完成迁移、回滚和账号抽样后，才能视为部署级通过。

## 2. 整改结果

### 2.1 登录与生产配置

- 生产环境必须显式提供长度不少于 32 字符且非默认值的 `JWT_SECRET`，否则进程拒绝启动。
- `WECHAT_APP_ID` 与 `WECHAT_APP_SECRET` 必须成对配置；生产环境缺少任一项时拒绝启动。
- mock 微信登录默认关闭，只能在非生产环境通过 `ALLOW_MOCK_WECHAT_LOGIN=true` 显式开启。
- 删除了 `code=admin` 自动创建管理员的逻辑；即使本地 mock code 为 `admin`，新用户也只能获得 `user` 角色。
- 开发种子中的 `mock_openid_admin` 已降为普通用户；迁移会撤销历史 mock 管理员角色并使其旧 token 失效。
- Compose 改为通过必填环境变量注入 JWT 与微信双密钥，不再使用硬编码生产凭据。

### 2.2 JWT 与即时撤销

- 原自定义单段 token 已替换为标准三段式 HS256 JWT。
- Claims 固定包含 `iss`、`sub`、`iat`、`exp`、随机 `jti` 与 `ver`，严格校验算法、类型、签名、签发时间和过期时间。
- access token 有效期从 30 天缩短为 15 分钟。
- token 不再携带或信任角色快照；每个受保护请求都会从用户域 repository 读取当前用户状态和角色。
- `users.token_version` 作为撤销版本。封禁、解封、禁言、解除禁言和角色变化都会原子递增版本，所有旧 token 立即失效。
- banned 用户无法重新登录，持有旧 token 也会在实时状态检查时立即被拒绝。

### 2.3 禁言边界

muted 用户允许读取公开内容和本人数据，但不能执行会产生或修改用户内容的操作：

- 修改个人展示资料；
- 发帖、评论；
- 发布或编辑商品；
- 提交举报。

点赞、收藏、读取会话、联系卖家和管理本人已有商品状态未被归类为新增内容，保持原有权限。muted 管理员不能进入任何后台管理接口。

### 2.4 管理员与超级管理员隔离

| 操作 | user | admin | superadmin |
| --- | --- | --- | --- |
| 普通受保护接口 | 是 | 是 | 是 |
| 后台审核/管理 | 否 | 是（仅 active） | 是（仅 active） |
| 封禁/禁言普通用户 | 否 | 是 | 是 |
| 操作 admin/superadmin 状态 | 否 | 否 | 是 |
| 修改用户角色 | 否 | 否 | 是 |
| 修改自身管理状态/角色 | 否 | 否 | 否 |

- 新增 `superadmin` 角色及独立中间件。
- 新增 `PATCH /api/v1/admin/users/:id/role`，路由和 service 双层校验 superadmin。
- 普通 admin 对 admin/superadmin 的状态操作在 service 层拒绝，不能依赖前端隐藏按钮绕过。
- 管理员不能修改自己的状态或角色。
- 最后一个 active superadmin 的降级/停用保护位于数据库事务内，并对 active superadmin 集合加 `FOR UPDATE` 锁，避免并发操作清空最高权限主体。
- 后台用户管理页已接入 active/muted/banned 与 user/admin/superadmin 操作；服务端仍是最终权限裁决者。
- 首个真实 superadmin 必须按 `docs/runbooks/SUPERADMIN_BOOTSTRAP.md` 由 DBA 一次性引导，禁止 mock OpenID；之后统一走受控 API。

## 3. 数据库变更

- `users` 新增 `token_version BIGINT UNSIGNED NOT NULL DEFAULT 1`。
- 新增迁移 `004_auth_hardening.sql`：补列、修复零版本、清理历史 mock 管理员并撤销旧 token。
- 角色/状态写入与 token 撤销为同一条 `UPDATE`；最后超管保护使用事务与行锁。

预发布迁移验收必须执行：

1. 备份 `users` 表并记录 admin/superadmin 数量；
2. 执行迁移并确认所有 `token_version >= 1`；
3. 确认 `mock_openid_%` 不含 admin/superadmin；
4. 抽样验证旧 token 返回 401、banned 返回 403、muted 内容写入返回 403；
5. 验证唯一 active superadmin 无法被停用或降级；
6. 按回滚预案恢复备份并再次演练升级。

## 4. 自动化验收证据

| 检查 | 结果 |
| --- | --- |
| `npm run verify` | 通过：边界、两端 TypeScript、Go 全量测试 |
| `go vet ./...` | 通过 |
| `go test ./...` | 通过 |
| 用户域专项测试 | 通过；覆盖 JWT 篡改/过期、实时角色、封禁、版本撤销、mock 提权、管理员越权、禁言边界 |
| 配置专项测试 | 通过；覆盖默认/短 JWT、微信 Secret 缺失、生产 mock 开关 |
| 用户域语句覆盖率 | 39.7%（包含未被单元测试覆盖的 Gorm repository 与 HTTP handler） |
| `npm run build:admin` | 通过 |
| `npm run build:miniapp` | 通过 |
| `go test -race` | 未执行：当前 Windows Go 未启用 CGO |
| MySQL 8 迁移/并发集成测试 | 未执行：当前环境无 Docker/MySQL 实例 |

## 5. 风险关闭映射

| 原风险 | 状态 | 证据 |
| --- | --- | --- |
| 默认 JWT + mock 管理员导致后台接管 | 已关闭 | 生产 fail-fast、mock 显式开关、删除 admin 特判、种子降权 |
| 30 天角色快照 token | 已关闭 | 15 分钟 JWT、token 不含角色、实时查库 |
| banned 旧 token 继续访问 | 已关闭 | 实时状态校验 + token_version 撤销 |
| muted 与普通用户相同 | 已关闭 | 内容写路由统一增加 `UnmutedRequired` |
| admin 可操作其他管理员 | 已关闭 | service 层目标角色隔离 |
| 无 superadmin | 已关闭 | 角色、路由、service、后台 UI、首次引导齐备 |
| 并发移除最后超管 | 已关闭 | 事务 + `FOR UPDATE` + 数量保护 |

## 6. 遗留项与下一门禁

以下项目不属于 P0-1，未在本次并行推进：

- P0-2：评论、点赞、收藏、关注、通知完整接通；
- P0-3：搜索、腾讯 IM 真实接入、售出闭环；
- P0-4：COS 临时签名、图片校验与内容审核；
- P0-5：管理员不可变审计日志、备份恢复、监控告警、发布回滚；
- 原总报告中的限流、HTTP 超时、安全头、生产 CORS、依赖漏洞和审核并发等其他上线门禁，仍按对应专项阶段处理。

下一步只允许进入 P0-2。进入 P0-2 不改变当前系统整体 **NO-GO** 状态。
