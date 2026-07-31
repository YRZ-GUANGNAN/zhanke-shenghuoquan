# 湛科生活圈发布候选收敛 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 将当前小程序、后端和管理后台收敛为不含已知源码阻断、可由用户最终上传的发布候选版本。

**Architecture:** 保留现有 Taro + React、Go + Gin/GORM、React + Vite 架构；以现有业务合同为准，不重写模块。先固定现有业务成果，再修复边界检查和共享样式顺序，最后执行分层验证并生成交付记录。

**Tech Stack:** Taro 4.2、React 18、TypeScript、SCSS、Go、Gin、GORM、Vite、Node.js 合同检查脚本。

---

### Task 1: 固定发布候选边界

**Files:**
- Create: `docs/RELEASE-CANDIDATE-2026-07-31.md`
- Modify: `.gitignore`

- [ ] **Step 1: 记录工作区基线**

运行：

```powershell
git status --short
git worktree list
git log -8 --oneline
```

预期：当前分支为 `master`；保留所有用户已有改动，不执行 reset、checkout 或整仓暂存。

- [ ] **Step 2: 只忽略本地临时目录**

确保 `.gitignore` 包含：

```gitignore
.codex-tmp/
.superpowers/
.tmp/
```

不得忽略业务源码、迁移、规格或验收文档。

- [ ] **Step 3: 创建发布候选记录骨架**

`docs/RELEASE-CANDIDATE-2026-07-31.md` 写明：

```markdown
# 发布候选 2026-07-31

- 微信上传：未执行，由用户最终操作
- 生产数据写入：未执行
- 小程序构建：PENDING
- 管理后台构建：PENDING
- 后端聚焦测试：PENDING
- 双账号双真机：PENDING
```

- [ ] **Step 4: 检查暂存范围**

运行：

```powershell
git diff --check
git status --short
```

预期：没有空白错误；未出现密钥文件。

### Task 2: 收敛二手留言与自动审核成果

**Files:**
- Modify: `backend/internal/module/admin/review/*.go`
- Modify: `backend/internal/module/community/*.go`
- Modify: `backend/internal/module/trade/*.go`
- Create: `backend/migrations/036_trade_comments_auto_review.sql`
- Create: `backend/migrations/rollback/036_trade_comments_auto_review.sql`
- Modify: `apps/miniapp/src/pages/secondhand-detail/index.tsx`
- Modify: `apps/miniapp/src/pages/secondhand-detail/index.scss`
- Create: `apps/miniapp/src/pages/secondhand-detail/product-comments.tsx`
- Create: `apps/miniapp/src/pages/secondhand-detail/product-comments.scss`
- Modify: `apps/miniapp/src/pages/notifications/index.tsx`
- Modify: `apps/miniapp/src/services/c-trade.ts`
- Modify: `apps/miniapp/src/types.ts`
- Modify: `apps/admin/src/App.tsx`
- Modify: `apps/admin/src/services/f-admin.ts`
- Modify: `apps/admin/src/types.ts`
- Test: `scripts/check-secondhand-comments-contract.cjs`

- [ ] **Step 1: 运行现有失败优先检查**

运行：

```powershell
npm.cmd run check:secondhand-comments-contract
go test ./internal/module/admin/review ./internal/module/trade ./internal/module/community ./cmd/server -count=1
```

预期：全部 PASS；如果失败，只修改本任务列出的留言、审核或通知合同。

- [ ] **Step 2: 检查审核失败关闭**

确认 `AutomaticReviewer` 的结果映射满足：

```go
switch outcome {
case OutcomeApproved:
    // automatic approved
case OutcomeRejected:
    // automatic rejected
default:
    // remain submitted for manual review
}
```

网络失败、超时、未配置与未知结果不得进入自动通过分支。

- [ ] **Step 3: 检查后台人工改判**

确认管理后台调用：

```ts
overrideReview: (id, status, reason) =>
  api(`/admin/reviews/${id}/override`, {
    method: 'POST',
    body: JSON.stringify({ status, reason }),
  })
```

`reason` 必须为非空；成功后刷新审核列表，失败后保留选中任务。

- [ ] **Step 4: 检查真实数据渲染**

商品详情只渲染服务端返回的：

```ts
comments.items
viewerState.isSeller
viewerState.favorited
product.status
```

不得在失败路径插入本地留言、用户、评分或商品。

- [ ] **Step 5: 提交业务成果**

使用精确路径暂存，运行：

```powershell
git diff --cached --check
git commit -m "功能：完成二手留言与自动审核闭环"
```

不得包含 `.env`、部署文件、微信项目私有配置或其他页面的无关改动。

### Task 3: 修复架构边界检查

**Files:**
- Modify: `scripts/check-boundaries.cjs`
- Test: `scripts/check-boundaries.cjs`

- [ ] **Step 1: 写入明确模块分类**

把允许模块改为：

```js
const allowedDomains = new Set([
  'user', 'community', 'trade', 'navigation', 'im', 'admin', 'media',
  'takeout', 'delivery', 'adminauth', 'identityapplication',
])
```

- [ ] **Step 2: 为事务编排建立窄白名单**

加入精确相对路径集合：

```js
const transactionOrchestrators = new Set([
  path.join('identityapplication', 'media.go'),
  path.join('identityapplication', 'service.go'),
])
```

数据库访问规则只允许 repository、测试文件和该集合；不得按整个目录放行。

- [ ] **Step 3: 运行边界检查**

运行：

```powershell
npm.cmd run check:boundaries
```

预期：

```text
ok A-G boundaries plus independent delivery and isolated P0-4 media domains
```

- [ ] **Step 4: 提交**

```powershell
git add -- scripts/check-boundaries.cjs
git diff --cached --check
git commit -m "架构：更新身份申请事务边界检查"
```

### Task 4: 消除小程序 CSS 顺序警告

**Files:**
- Modify: `apps/miniapp/src/pages/profile/index.tsx`

- [ ] **Step 1: 统一共享组件导入顺序**

将同时使用两个组件的页面统一为：

```ts
import StudentBottomNav from '../../components/student-bottom-nav'
import { CapsuleSystemRow } from '../../components/capsule-page-header'
```

该顺序与首页、外卖和二手首页保持一致。

- [ ] **Step 2: 运行类型检查**

```powershell
npm.cmd --workspace apps/miniapp run typecheck
```

预期：PASS。

- [ ] **Step 3: 运行生产构建并检查日志**

```powershell
npm.cmd run build:miniapp
```

预期：构建 PASS，日志不再出现 `Conflicting order`；允许保留 `vendors.js` 大小建议。

- [ ] **Step 4: 提交**

```powershell
git add -- apps/miniapp/src/pages/profile/index.tsx
git diff --cached --check
git commit -m "构建：统一小程序共享样式导入顺序"
```

### Task 5: 完成全层验证

**Files:**
- Modify: `docs/RELEASE-CANDIDATE-2026-07-31.md`

- [ ] **Step 1: 运行小程序合同**

```powershell
npm.cmd run check:miniapp-static-routes
npm.cmd run check:miniapp-business-closure
npm.cmd run check:miniapp-regression-fixes
npm.cmd run check:secondhand-comments-contract
npm.cmd run check:miniapp-capsule-layout
npm.cmd run check:miniapp-publish-contract
npm.cmd run check:miniapp-domain-search
npm.cmd run check:miniapp-settings-contract
npm.cmd run check:identity-application-contract
npm.cmd run check:home-real-tools-contract
```

预期：全部 PASS。

- [ ] **Step 2: 运行安全与角色工作台合同**

```powershell
npm.cmd run check:admin-password-session-contract
npm.cmd run check:real-wechat-secret-hygiene
npm.cmd run check:merchant-home-clarity
npm.cmd run check:rider-workbench-clarity
npm.cmd run check:external-takeout-directory
npm.cmd run check:boundaries
```

预期：全部 PASS。

- [ ] **Step 3: 运行类型检查和构建**

```powershell
npm.cmd run typecheck
npm.cmd run build:admin
npm.cmd run build:miniapp
```

预期：全部 PASS；产物位于 `apps/admin/dist` 与 `apps/miniapp/dist`。

- [ ] **Step 4: 运行后端非破坏性回归**

```powershell
go test ./internal/module/admin/review ./internal/module/trade ./internal/module/community ./cmd/server -count=1
```

预期：PASS。不得为使共享 MySQL 集成测试变绿而清空业务数据库。

- [ ] **Step 5: 更新发布候选记录**

把每条命令、结果、已知非阻断警告和真机边界写入 `docs/RELEASE-CANDIDATE-2026-07-31.md`。双账号双真机没有重新执行时必须保持 `PENDING`。

- [ ] **Step 6: 提交验证记录**

```powershell
git add -- docs/RELEASE-CANDIDATE-2026-07-31.md
git diff --cached --check
git commit -m "验证：记录发布候选检查结果"
```

### Task 6: 交付但不上传

**Files:**
- No source changes

- [ ] **Step 1: 确认微信项目配置**

```powershell
Select-String -Path apps/miniapp/project.config.json -Pattern 'wx0dacef976daa8e55|dist/'
```

预期：AppID 与 `miniprogramRoot` 正确。

- [ ] **Step 2: 确认构建产物**

```powershell
Get-Item apps/miniapp/dist/app.json
Get-Item apps/admin/dist/index.html
```

预期：两个文件存在且时间为本轮构建时间。

- [ ] **Step 3: 给出最终上传信息**

建议：

```text
版本号：0.2.1
项目备注：发布候选：二手留言、自动审核、后台改判与页面稳定性收敛
```

只指导用户点击微信开发者工具“上传”；本任务不代替用户上传，也不修改体验版或审核状态。

