# 新版商家工作台与审核门禁实施计划

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 将审核通过后的商家入口切换到新版工作台，禁止未审核用户通过身份切换、页面路径或配送接口进入商家能力，并修正搜索与资料编辑头部按钮。

**Architecture:** 身份资格由后端 `user.Service` 统一判断，商家配送接口增加服务端身份中间件；小程序的身份守卫同时检查当前身份和可用资格。新版商家页面只消费真实配送、钱包、会话与身份申请数据，不注册静态样例业务页面。

**Tech Stack:** Go、Gin、React 18、Taro 4、TypeScript、SCSS、Node.js 合同检查脚本。

---

### Task 1: 建立审核门禁失败用例

**Files:**
- Modify: `backend/internal/module/user/middleware_test.go`
- Modify: `scripts/check-miniapp-business-closure.cjs`

- [ ] **Step 1: 写后端身份中间件失败测试**

```go
router.GET(
    "/merchant",
    service.AuthRequired(),
    service.IdentityRequired(IdentityMerchant),
    func(c *gin.Context) { response.OK(c, gin.H{"allowed": true}) },
)
```

测试未审核、审核后未切换身份返回 `403`，审核且当前身份为商家返回 `200`。

- [ ] **Step 2: 写小程序路由合同失败断言**

```js
assert routeForMerchant === '/pages/merchant-home/index'
assert everyMerchantPageUsesIdentityGuard === true
assert registeredMerchantPagesContainNoStaticFixtures === true
```

- [ ] **Step 3: 运行测试并确认失败**

Run: `go test ./internal/module/user ./cmd/server`

Expected: FAIL，提示 `IdentityRequired` 尚未定义。

Run: `node scripts/check-miniapp-business-closure.cjs`

Expected: FAIL，提示商家路由仍指向旧工作台。

### Task 2: 实现三层商家审核门禁

**Files:**
- Modify: `backend/internal/module/user/middleware.go`
- Modify: `backend/cmd/server/main.go`
- Modify: `apps/miniapp/src/hooks/use-identity-guard.ts`
- Modify: `apps/miniapp/src/pages/profile/index.tsx`

- [ ] **Step 1: 实现服务端身份中间件**

```go
func (s *Service) IdentityRequired(expected Identity) gin.HandlerFunc {
    return func(c *gin.Context) {
        summary, err := s.IdentitySummary(c, CurrentUserID(c))
        if err != nil {
            response.Fail(c, http.StatusInternalServerError, "get identity failed")
            c.Abort()
            return
        }
        if summary.CurrentIdentity != expected || !identityAvailable(summary.Identities, expected) {
            response.Fail(c, http.StatusForbidden, "approved identity required")
            c.Abort()
            return
        }
        c.Next()
    }
}
```

- [ ] **Step 2: 保护配送商家接口**

```go
merchantRoutes := api.Group(
    "/delivery/merchant",
    userService.AuthRequired(),
    userService.IdentityRequired(user.IdentityMerchant),
)
```

- [ ] **Step 3: 收紧前端页面守卫**

`useIdentityGuard('merchant')` 仅在 `currentIdentity === 'merchant'` 且对应 `IdentityAvailability.available === true` 时渲染页面；否则返回学生页。

- [ ] **Step 4: 修正未审核商家提示**

`missing`、`pending`、`rejected` 分别引导申请、等待审核和重新提交，不弹出“可直接切换”的确认框。

- [ ] **Step 5: 运行门禁测试**

Run: `go test ./internal/module/user ./cmd/server`

Expected: PASS。

### Task 3: 将新版商家工作台接入真实数据

**Files:**
- Modify: `apps/miniapp/src/app.config.ts`
- Modify: `apps/miniapp/src/hooks/use-identity-guard.ts`
- Modify: `apps/miniapp/src/pages/merchant-home/index.tsx`
- Modify: `apps/miniapp/src/pages/merchant-publish/index.tsx`
- Modify: `apps/miniapp/src/pages/merchant-wallet/index.tsx`
- Modify: `apps/miniapp/src/pages/merchant-messages/index.tsx`
- Modify: `apps/miniapp/src/pages/merchant-order-detail/index.tsx`

- [ ] **Step 1: 注册新版商家页面并替换入口**

```ts
merchant: '/pages/merchant-home/index'
```

注册 `merchant-home`、`merchant-publish`、`merchant-wallet`、`merchant-messages` 和 `merchant-order-detail`，旧 `delivery-merchant` 不再作为身份首页。

- [ ] **Step 2: 工作台读取真实订单、钱包与商家资料**

使用 `listMerchantDeliveryOrders()`、`getDeliveryWallet()`、`getIdentityApplication('merchant')` 和登录资料构建订单分组、今日单量、配送中数量、余额、商家名称与地址。

- [ ] **Step 3: 新版发布页提交真实配送单**

使用 `createMerchantDeliveryOrder()` 创建订单；选择钱包或微信后分别调用 `payDeliveryOrderWithWallet()` 或 `payDeliveryOrderWithWechat()`，成功后进入真实订单详情。

- [ ] **Step 4: 新版钱包页读取真实资金数据**

使用 `getDeliveryWallet()`、`listDeliveryWalletLedger()` 和 `startDeliveryWalletRecharge()`；不得本地伪造余额、充值成功或流水。

- [ ] **Step 5: 新版消息页读取真实会话**

使用 `listDeliveryConversations()` 展示真实配送会话；点击后将真实 `deliveryOrderPublicId` 传入 `delivery-conversation`。

- [ ] **Step 6: 新版详情页读取真实订单与凭证**

使用 `getMerchantDeliveryOrder()`、`getDeliveryOrderFunds()` 和 `loadPrivateDeliveryProofs()`，删除 `delivery-static-data` 依赖。

- [ ] **Step 7: 为全部新版商家子页增加 `RuntimeSessionGate` 和 `useIdentityGuard('merchant')`**

直接输入页面路径也必须先经过登录态和审核身份检查。

### Task 4: 修复搜索与资料编辑头部按钮

**Files:**
- Modify: `apps/miniapp/src/pages/home/index.scss`
- Modify: `apps/miniapp/src/pages/edit-profile/index.tsx`
- Modify: `apps/miniapp/src/pages/edit-profile/index.scss`
- Modify: `scripts/check-miniapp-capsule-layout.cjs`

- [ ] **Step 1: 重绘搜索图标**

统一使用像素尺寸绘制 14px 圆环和 8px 手柄，避免 `rpx` 与 `px` 混用造成斜杠穿过圆心。

- [ ] **Step 2: 扩大取消与保存按钮**

左右按钮改为 `View` 点击容器，宽度至少 `64px`、高度 `44px`，文字使用 `white-space: nowrap`，保存中也不得折行。

- [ ] **Step 3: 增加布局合同断言**

检查搜索图标不混用尺寸单位，资料编辑按钮包含不折行和 64px 点击区。

### Task 5: 全量验证与独立提交

**Files:**
- Modify: `docs/superpowers/plans/2026-07-23-merchant-review-new-workbench.md`

- [ ] **Step 1: 运行小程序合同检查**

Run: `npm.cmd run check:miniapp-business-closure`

Expected: PASS。

Run: `npm.cmd run check:miniapp-capsule-layout`

Expected: PASS。

- [ ] **Step 2: 运行类型检查与生产构建**

Run: `npm.cmd --workspace apps/miniapp run typecheck`

Expected: PASS。

Run: `$env:NODE_ENV='production'; $env:TARO_APP_API_BASE='https://zhanduiwang.top/api/v1'; npm.cmd --workspace apps/miniapp run build:weapp`

Expected: `Compiled successfully`。

- [ ] **Step 3: 运行后端测试**

Run: `Set-Location backend; go test ./internal/module/user ./cmd/server`

Expected: PASS。

- [ ] **Step 4: 在微信开发者工具验收**

未审核账号点击商家后进入申请/审核状态页；已审核账号切换后进入新版工作台；首页搜索图标正常；资料编辑页取消和保存均可点击且不折行。

- [ ] **Step 5: 只提交本任务文件**

```powershell
git add backend/internal/module/user/middleware.go backend/internal/module/user/middleware_test.go backend/cmd/server/main.go apps/miniapp/src scripts/check-miniapp-business-closure.cjs scripts/check-miniapp-capsule-layout.cjs
git commit -m "修复：启用新版商家工作台与审核门禁"
```

不得暂存现有 `.env.example`、部署文件、快速验证文档或用户拥有的配置改动。
