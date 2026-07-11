# UI-21a Runtime Login and Identity Foundation Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 建立可信的小程序启动登录、服务端身份持久化、资格校验和角色页面深链守卫，同时保持新版静态 UI 视觉不变。

**Architecture:** Domain A 持有当前身份及 GET/POST API；cmd/server 组合层通过 Delivery 开关和骑手只读窄接口计算资格。小程序以 Zustand session 串联 token 恢复、微信登录、profile/identity hydration，并由页面守卫阻止身份不匹配的深链。

**Tech Stack:** Go 1.x、Gin、GORM、MySQL 8.4、Taro 4、React 18、TypeScript、Zustand 5。

---

### Task 1: Domain A 身份模型与迁移

**Files:**
- Create: `backend/migrations/029_user_current_identity.sql`
- Create: `backend/migrations/rollback/029_user_current_identity.sql`
- Create: `backend/internal/module/user/identity.go`
- Modify: `backend/internal/module/user/model.go`
- Modify: `backend/internal/module/user/repository.go`
- Modify: `backend/internal/module/user/service.go`
- Test: `backend/internal/module/user/service_test.go`

- [ ] 写失败测试：student 永远可用、不可用目标返回 `ErrIdentityUnavailable`、成功切换只更新 `CurrentIdentity` 而不更新 `Role`。
- [ ] 运行 `cd backend; go test ./internal/module/user -count=1`，确认因缺少身份类型/接口失败。
- [ ] 新增 `Identity` 三值、`IdentityEligibility`、`IdentitySummary`、`SwitchIdentity`，并为 Repository 增加：

```go
UpdateCurrentIdentity(context.Context, uint, Identity) error
```

- [ ] 为新用户显式写入 `IdentityStudent`，`User` JSON 透出 `currentIdentity`。
- [ ] 新增迁移 029 与 rollback，使用 CHECK 且零外键。
- [ ] 运行 user 单元测试并确认 PASS。
- [ ] 提交 `feat(identity): 建立服务端身份状态模型`。

### Task 2: Delivery 资格适配与身份 HTTP

**Files:**
- Create: `backend/cmd/server/identity_eligibility.go`
- Create: `backend/cmd/server/identity_eligibility_test.go`
- Create: `backend/cmd/server/identity_http_mysql_test.go`
- Modify: `backend/internal/module/user/handler.go`
- Modify: `backend/cmd/server/main.go`

- [ ] 写资格适配器失败测试，覆盖 Delivery disabled、merchant enabled、rider missing/resting/active。
- [ ] 实现组合层适配器；只允许骑手适配器 import takeout，共享商家不读取 `takeout_merchants`。
- [ ] 注册：

```text
GET  /api/v1/auth/identity
POST /api/v1/auth/identity
```

- [ ] 在构造 `user.Service` 时注入 `user.WithIdentityEligibility(...)`；旧外卖 runtime 和 gate 不变。
- [ ] 改写真实 HTTP/MySQL 测试：商家无需旧外卖夹具，骑手仍以共享 `takeout_riders` active 状态验收。
- [ ] 运行 `cd backend; go test ./internal/module/user ./cmd/server -count=1`。
- [ ] 提交 `feat(identity): 接通配送身份资格与HTTP接口`。

### Task 3: API 错误与 session bootstrap

**Files:**
- Modify: `apps/miniapp/src/services/taro.ts`
- Modify: `apps/miniapp/src/taro-shim.d.ts`
- Modify: `apps/miniapp/src/services/api.ts`
- Modify: `apps/miniapp/src/services/a-user.ts`
- Modify: `apps/miniapp/src/stores/session.ts`
- Modify: `apps/miniapp/src/app.tsx`
- Modify: `apps/miniapp/src/app.scss`
- Modify: `apps/miniapp/src/types.ts`

- [ ] 为 Taro facade 增加 `login/removeStorageSync/reLaunch` 的准确签名。
- [ ] 新增 `ApiError`：

```ts
class ApiError extends Error {
  constructor(public statusCode: number, public code: number, message: string) { super(message) }
}
```

- [ ] 让 `wechatLogin` 返回 token 与 profile，由 session 统一持久化 token；增加 `currentIdentity/switchIdentity` 客户端。
- [ ] 实现 session `bootstrap/retry/logout/refreshIdentity/switchIdentity`，合并并发 bootstrap，401 后只重登一次。
- [ ] App 根组件仅在 authenticated 时渲染 children；boot/error/anonymous 使用最小状态页与重试按钮。
- [ ] 运行 `npm --workspace apps/miniapp run typecheck` 与 `npm run build:miniapp`。
- [ ] 提交 `feat(miniapp): 建立登录会话运行时`。

### Task 4: Profile 与角色页面接线

**Files:**
- Create: `apps/miniapp/src/hooks/use-identity-guard.ts`
- Modify: `apps/miniapp/src/components/IdentitySwitchModal.tsx`
- Modify: `apps/miniapp/src/components/IdentitySwitchModal.scss`
- Modify: `apps/miniapp/src/pages/profile/index.tsx`
- Modify: `apps/miniapp/src/pages/delivery-rider/index.tsx`
- Modify: `apps/miniapp/src/pages/merchant-home/index.tsx`

- [ ] Profile 从 session 读取真实 profile/identity；不可用角色不发切换请求并显示后端原因。
- [ ] Modal 增加 loading/error，提交中禁止重复点击，不改变原面板尺寸、色彩和排版体系。
- [ ] 新增 `useIdentityGuard(expected)`；authenticated 后身份不符时不渲染业务内容，并按服务端当前身份 `reLaunch` 到对应首页。
- [ ] 骑手和商家切回学生均先调用 `switchIdentity('student')`，成功后跳转。
- [ ] 运行 typecheck、build 与 `npm run check:miniapp-static-routes`。
- [ ] 提交 `feat(miniapp): 接通真实身份切换与页面守卫`。

### Task 5: 真实环境验收与交付

**Files:**
- Create: `docs/UI-21A-RUNTIME-IDENTITY-DEVELOPMENT.md`
- Create: `docs/UI-21A-RUNTIME-IDENTITY-ACCEPTANCE.md`
- Create: `docs/UI-21A-RUNTIME-IDENTITY-RESIDUE.md`

- [ ] 从 `backend/.env` 只读取 DSN 到进程环境，不在日志或报告中输出密码，运行迁移 apply→rollback→apply 测试。
- [ ] 运行真实 HTTP/MySQL 身份测试，证明 401、403、active rider、merchant、持久化和 role 不变。
- [ ] 运行：

```powershell
cd backend; go test ./... -count=1
npm run check:boundaries
npm run check:miniapp-static-routes
npm run verify
npm run build
rg "db\.AutoMigrate\(" backend/cmd/server
```

- [ ] 核实 `backend/internal/module/takeout` 业务逻辑与旧外卖路由无无关改动，旧分支仍保留。
- [ ] 写入 Development/Acceptance/Residue 报告，注明真实环境证据与任何 Conditional 项。
- [ ] 提交最终报告并确认 `git status` 干净；停在 `feat/ui21a-runtime-identity` 等 owner 验收。
