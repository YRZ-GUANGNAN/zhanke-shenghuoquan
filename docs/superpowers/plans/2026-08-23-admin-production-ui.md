# Admin Production UI Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 将现有湛科生活圈后台的全部真实模块统一为可上线审查的运营管理网站，同时保持业务接口、权限、功能开关和写操作语义不变。

**Architecture:** 保留现有 React + Zustand 单页架构和各领域服务层，在 `apps/admin/src` 内增加少量展示型公共组件与导航元数据。全局壳层和设计令牌负责一致性，简单列表页使用公共页头与表格容器，复杂控制台仅整理结构与样式，不迁移业务逻辑。

**Tech Stack:** React 18、TypeScript、Zustand、Vite、原生 CSS、Node 静态契约脚本。

---

## 文件结构

- Create: `apps/admin/src/admin-navigation.ts` — 后台导航分组、页面标题和说明元数据。
- Create: `apps/admin/src/components/AdminPageHeader.tsx` — 页面标题、说明和操作槽位。
- Create: `apps/admin/src/components/AdminTableFrame.tsx` — 可横向滚动、带语义标题的表格容器。
- Modify: `apps/admin/src/components/index.ts` — 导出新增公共组件。
- Modify: `apps/admin/src/App.tsx` — 使用导航元数据、统一壳层，并整理数据/用户/审核/内容/举报/导航页面。
- Modify: `apps/admin/src/Login.tsx` — 登录页内容层级与状态语义。
- Modify: `apps/admin/src/AccountPanel.tsx` — 账户摘要和密码弹窗结构。
- Modify: `apps/admin/src/IdentityApplications.tsx` — 接入统一页头、工具栏和状态容器。
- Modify: `apps/admin/src/MediaConsole.tsx` — 接入统一模块页头和控制台分区。
- Modify: `apps/admin/src/DeliveryConsole.tsx` — 接入统一模块页头、金额与长编号可读布局。
- Modify: `apps/admin/src/TakeoutConsole.tsx` — 保留功能开关，统一分区和状态。
- Modify: `apps/admin/src/ExternalTakeoutMerchants.tsx` — 合作商家目录视觉层级与危险操作反馈。
- Modify: `apps/admin/src/styles.css` — 设计令牌、壳层、表格、控制台、状态和响应式系统。
- Modify: `scripts/check-admin-ui-system.cjs` — 锁定公共组件、导航分组、焦点与响应式契约。

### Task 1: 扩展后台 UI 静态契约

**Files:**
- Modify: `scripts/check-admin-ui-system.cjs`
- Test: `scripts/check-admin-ui-system.cjs`

- [ ] **Step 1: 写入会失败的新设计系统断言**

```js
for (const name of ['AdminPageHeader', 'AdminTableFrame']) {
  assert.ok(fs.existsSync(path.join(componentRoot, `${name}.tsx`)), `${name}.tsx is required`)
}
const navigationSource = fs.readFileSync(path.join(src, 'admin-navigation.ts'), 'utf8')
for (const group of ['运营中心', '内容治理', '平台管理']) {
  assert.match(navigationSource, new RegExp(group))
}
for (const contract of ['focus-visible', '@media (max-width: 900px)', 'overflow-x: auto']) {
  assert.match(styles, new RegExp(contract.replace(/[()]/g, '\\$&')))
}
```

- [ ] **Step 2: 运行检查并确认失败原因是缺少新文件**

Run: `npm run check:admin-ui-system`

Expected: FAIL，提示 `AdminPageHeader.tsx is required` 或 `admin-navigation.ts` 不存在。

- [ ] **Step 3: 保留原有审核分页和禁止原生弹窗断言**

确认 `window.confirm/prompt/alert`、`reviewStatus`、`reviewPage`、服务端 `total` 和上一页/下一页断言仍在脚本中；不得删除来换取通过。

### Task 2: 建立导航元数据和展示型公共组件

**Files:**
- Create: `apps/admin/src/admin-navigation.ts`
- Create: `apps/admin/src/components/AdminPageHeader.tsx`
- Create: `apps/admin/src/components/AdminTableFrame.tsx`
- Modify: `apps/admin/src/components/index.ts`
- Test: `scripts/check-admin-ui-system.cjs`

- [ ] **Step 1: 创建分组导航与页面说明**

```ts
import type { AdminIconName } from './components'
import type { AdminSection } from './types'

export interface AdminNavigationItem {
  key: AdminSection
  label: string
  description: string
  icon: AdminIconName
  group: '运营中心' | '内容治理' | '平台管理'
}

export const adminNavigation: AdminNavigationItem[] = [
  { key: 'media', label: '媒体运营', description: '检查素材状态、引用关系与失败任务', icon: 'media', group: '运营中心' },
  { key: 'delivery', label: '配送运营', description: '处理配送订单、资金与异常记录', icon: 'delivery', group: '运营中心' },
  { key: 'moderation', label: '统一审核', description: '按状态处理内容审核与人工改判', icon: 'moderation', group: '内容治理' },
  { key: 'community', label: '帖子管理', description: '查看已提交帖子并执行合规下架', icon: 'community', group: '内容治理' },
  { key: 'trade', label: '商品管理', description: '查看商品状态并衔接审核或下架', icon: 'trade', group: '内容治理' },
  { key: 'reports', label: '举报审核', description: '查看用户举报及其审核状态', icon: 'reports', group: '内容治理' },
  { key: 'identities', label: '身份审核', description: '处理骑手与商家身份申请', icon: 'identities', group: '内容治理' },
  { key: 'dashboard', label: '数据统计', description: '查看平台总量、今日新增与待处理内容', icon: 'dashboard', group: '平台管理' },
  { key: 'users', label: '用户管理', description: '管理用户状态和管理员角色', icon: 'users', group: '平台管理' },
  { key: 'navigation', label: '导航管理', description: '维护普通导航入口与合作商家目录', icon: 'navigation', group: '平台管理' },
]
```

外卖运营项在 `App.tsx` 中继续由 `legacyTakeoutEnabled` 插入，不能把 feature flag 写死为开启。

- [ ] **Step 2: 创建统一页头组件**

```tsx
import type { ReactNode } from 'react'

export function AdminPageHeader({ eyebrow, title, description, actions }: {
  eyebrow: string
  title: string
  description: string
  actions?: ReactNode
}) {
  return <header className="admin-page-header">
    <div>
      <span className="admin-page-header__eyebrow">{eyebrow}</span>
      <h1>{title}</h1>
      <p>{description}</p>
    </div>
    {actions && <div className="admin-page-header__actions">{actions}</div>}
  </header>
}
```

- [ ] **Step 3: 创建统一表格框架**

```tsx
import type { ReactNode } from 'react'

export function AdminTableFrame({ label, children }: { label: string; children: ReactNode }) {
  return <div className="admin-table-frame" role="region" aria-label={label} tabIndex={0}>
    {children}
  </div>
}
```

- [ ] **Step 4: 导出组件并运行静态检查**

Run: `npm run check:admin-ui-system`

Expected: 仍可能因样式契约失败，但不再提示公共组件或导航文件缺失。

### Task 3: 重构后台壳层和简单业务页

**Files:**
- Modify: `apps/admin/src/App.tsx`
- Test: `scripts/check-admin-ui-system.cjs`

- [ ] **Step 1: 用导航分组替换扁平数组**

```tsx
const visibleNavigation = legacyTakeoutEnabled
  ? [
      ...adminNavigation,
      { key: 'takeout', label: '外卖运营', description: '处理商家、菜品、订单与结算', icon: 'delivery', group: '运营中心' } as const,
    ]
  : adminNavigation

const navigationGroups = ['运营中心', '内容治理', '平台管理'] as const
```

侧栏按 `navigationGroups` 渲染标题与按钮，button、key、`setSection(item.key)` 和 active 判断保持不变。

- [ ] **Step 2: 统一顶层页头**

```tsx
const active = visibleNavigation.find(item => item.key === section)
<AdminPageHeader
  eyebrow={active?.group || '校园业务'}
  title={active?.label || '运营管理'}
  description={active?.description || '管理湛科生活圈的真实业务数据'}
/>
```

账户区仍位于全局顶栏，不移动退出或修改密码逻辑。

- [ ] **Step 3: 将四项真实统计改为语义指标带**

```tsx
const metrics = [
  { label: '注册用户', total: data.totals.users, today: data.today.users },
  { label: '社区帖子', total: data.totals.posts, today: data.today.posts },
  { label: '二手商品', total: data.totals.products, today: data.today.products },
  { label: '待处理审核', total: data.totals.pendingReview, today: null },
]
```

使用 `font-variant-numeric: tabular-nums`，`today === null` 时显示“等待处理”而非伪造数值。

- [ ] **Step 4: 为用户、帖子、商品、举报表格套用 `AdminTableFrame`**

```tsx
<AdminTableFrame label="用户管理列表">
  <table>...</table>
</AdminTableFrame>
```

不删除任何列、状态按钮、超管角色按钮、下架操作或“前往审核”入口。

- [ ] **Step 5: 运行类型和静态检查**

Run: `npm --workspace apps/admin run typecheck && npm run check:admin-ui-system`

Expected: TypeScript PASS；静态检查仅允许剩余样式契约失败。

### Task 4: 统一登录、账户和危险操作层级

**Files:**
- Modify: `apps/admin/src/Login.tsx`
- Modify: `apps/admin/src/AccountPanel.tsx`
- Modify: `apps/admin/src/styles.css`
- Test: `scripts/check-admin-password-session-contract.cjs`

- [ ] **Step 1: 保留登录字段和安全契约，只增加结构类名**

```tsx
<div className="login-card__identity">
  <div className="login-card__mark" aria-hidden="true">湛</div>
  <div><p className="login-card__eyebrow">湛科生活圈</p><span>运营管理系统</span></div>
</div>
```

用户名仍为只读 `administrator`，密码仍为 14–72 字符，登录按钮 busy/disabled 和错误提示保持不变。

- [ ] **Step 2: 为账户按钮提供清晰焦点与弹窗标题**

保持 `logout()`、`changePassword()`、所有会话退出提示和密码字段不变；只增加 `account-panel__identity`、`account-panel__actions` 等布局包装。

- [ ] **Step 3: 运行密码会话契约**

Run: `npm run check:admin-password-session-contract`

Expected: `admin password session contract: ok`。

### Task 5: 收敛审核、身份、媒体、配送、外卖与导航控制台

**Files:**
- Modify: `apps/admin/src/App.tsx`
- Modify: `apps/admin/src/IdentityApplications.tsx`
- Modify: `apps/admin/src/MediaConsole.tsx`
- Modify: `apps/admin/src/DeliveryConsole.tsx`
- Modify: `apps/admin/src/TakeoutConsole.tsx`
- Modify: `apps/admin/src/ExternalTakeoutMerchants.tsx`
- Modify: `apps/admin/src/styles.css`

- [ ] **Step 1: 审核页保持状态机，统一工具栏和详情区**

保留 `submitted → reviewing → approved/rejected`、`manual_override`、分页回退、拒绝原因和确认弹窗。将筛选、总量、分页和 selected detail 使用 `admin-toolbar`、`admin-summary-strip`、`admin-detail-panel` 类名收敛。

- [ ] **Step 2: 身份审核使用同一状态和详情语言**

保留 role/status 查询、证明材料、rowVersion、批准/拒绝和事件时间线；只把筛选与详情容器映射到统一类名，不改变请求对象。

- [ ] **Step 3: 媒体和配送控制台统一数据密度**

所有资产 ID、任务 ID、订单 public ID、用户 ID、金额和 rowVersion 使用：

```css
.admin-mono,
.media-console code,
.delivery-console code {
  font-family: ui-monospace, "SFMono-Regular", Consolas, monospace;
  font-variant-numeric: tabular-nums;
  overflow-wrap: anywhere;
}
```

不删除任何查询条件、修复操作、资金操作或会话信息。

- [ ] **Step 4: 导航管理保持两个业务域分离**

`ExternalTakeoutMerchants` 继续独立呈现；普通导航只显示普通分类。保留 category visibility、merchant visibility、operating status、create/update/delete 和 AppID/Path 字段。

- [ ] **Step 5: 外卖功能开关回归**

分别在默认环境和显式关闭环境检查 `legacyTakeoutEnabled`；关闭时侧栏和 Content 都不能出现外卖运营。

### Task 6: 完成上线级 CSS 系统和响应式行为

**Files:**
- Modify: `apps/admin/src/styles.css`
- Test: `scripts/check-admin-ui-system.cjs`

- [ ] **Step 1: 收敛根令牌**

```css
:root {
  --color-primary: #2e6fae;
  --color-primary-hover: #245f97;
  --color-primary-soft: #eaf2fa;
  --color-canvas: #f3f6f9;
  --color-surface: #ffffff;
  --color-surface-subtle: #f7f9fb;
  --color-text: #172433;
  --color-text-secondary: #536578;
  --color-text-muted: #77889a;
  --color-border: #dce4ec;
  --radius-control: 9px;
  --radius-card: 14px;
  --radius-dialog: 18px;
  --shadow-dialog: 0 28px 72px rgba(24, 43, 65, .22);
  --duration-fast: 140ms;
}
```

- [ ] **Step 2: 实现稳定宽屏壳层**

```css
.app-shell { min-height: 100dvh; display: grid; grid-template-columns: 248px minmax(0, 1fr); background: var(--color-canvas); }
.sidebar { position: sticky; top: 0; height: 100dvh; overflow-y: auto; }
.main { min-width: 0; padding: 0 32px 48px; }
.main__content { width: min(1500px, 100%); margin: 0 auto; }
.topbar { position: sticky; top: 0; z-index: 20; }
```

- [ ] **Step 3: 实现表格、工具栏和状态反馈**

```css
.admin-table-frame { overflow-x: auto; border: 1px solid var(--color-border); border-radius: var(--radius-card); background: var(--color-surface); }
.admin-table-frame:focus-visible { outline: 3px solid rgba(46, 111, 174, .24); outline-offset: 2px; }
table { width: 100%; min-width: 760px; border-collapse: collapse; }
table tbody tr:hover { background: #f7fafc; }
button:focus-visible, a:focus-visible, input:focus-visible, select:focus-visible, textarea:focus-visible { outline: 3px solid rgba(46, 111, 174, .24); outline-offset: 2px; }
```

- [ ] **Step 4: 实现 1180px 和 900px 响应式**

```css
@media (max-width: 1180px) {
  .app-shell { grid-template-columns: 220px minmax(0, 1fr); }
  .main { padding-inline: 24px; }
}
@media (max-width: 900px) {
  .app-shell { display: block; }
  .sidebar { position: static; width: 100%; height: auto; }
  .sidebar nav { display: grid; grid-template-columns: repeat(2, minmax(0, 1fr)); }
  .main { padding-inline: 16px; }
  .admin-page-header { align-items: flex-start; flex-direction: column; }
}
```

- [ ] **Step 5: 保留 reduced-motion 并运行静态检查**

Run: `npm run check:admin-ui-system`

Expected: `admin ui system: ok`。

### Task 7: 全量验证和最终预览

**Files:**
- Verify only: `apps/admin/**`
- Verify only: `scripts/check-admin-ui-system.cjs`

- [ ] **Step 1: 运行后台相关契约**

Run:

```powershell
npm run check:admin-ui-system
npm run check:admin-password-session-contract
npm run check:admin-identity-applications-contract
```

Expected: 全部 PASS。

- [ ] **Step 2: 运行类型检查与正式构建**

Run:

```powershell
npm --workspace apps/admin run typecheck
npm --workspace apps/admin run build
```

Expected: TypeScript PASS，Vite 输出 `dist/index.html`、CSS 和 JS 资产，无构建错误。

- [ ] **Step 3: 检查变更边界**

Run:

```powershell
git diff --check
git status --short
git diff -- apps/admin scripts/check-admin-ui-system.cjs
```

Expected: 后台实现只触及计划内文件；已有 `apps/miniapp` 修改保持原样且未被暂存。

- [ ] **Step 4: 打开登录页真实预览**

启动 `npm --workspace apps/admin run dev`，在 1440px 视口检查登录页。不得在代码中加入绕过认证、假管理员或静态 token。

- [ ] **Step 5: 检查已登录页面或诚实预览边界**

如果本机已有有效管理员会话，检查数据统计、统一审核、用户管理、配送运营和导航管理；如果没有，保留真实登录守卫，只提供不提交到生产代码的空态结构预览，并将真实数据验收标记为 `PENDING`。

- [ ] **Step 6: 生成最终截图并保持预览打开**

至少提供登录页、数据统计空态/真实态、统一审核和导航管理四张截图；不使用生成图片或虚构业务数据。
