# 湛科生活圈后台管理补全 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 在不扩展后端接口和权限边界的前提下，修复后台布局错位并接入现有分页、反馈和导航管理能力，形成可供最终审查的完整后台。

**Architecture:** 保持 React + Zustand + 现有 `adminRequest` 数据流，将页面级重复结构收敛到公共后台组件；列表页统一使用服务端分页查询，写操作继续走现有接口和确认弹窗。CSS 以单一最终视觉层覆盖并删除冲突规则，不改变媒体、配送、外卖、身份和审核业务服务。

**Tech Stack:** React 18、TypeScript、Zustand、Vite、Go/Gin 现有管理 API、Node 静态契约检查、Codex in-app browser。

---

## 文件结构

- Create: `apps/admin/src/components/AdminPanel.tsx` — 页面内容容器与工具区。
- Create: `apps/admin/src/components/AdminToolbar.tsx` — 列表结果、刷新与操作行。
- Create: `apps/admin/src/components/AdminPagination.tsx` — 服务端分页控件。
- Create: `apps/admin/src/components/AdminStatusBadge.tsx` — 后端状态中文映射。
- Create: `apps/admin/src/components/AdminRowActions.tsx` — 行操作分组。
- Modify: `apps/admin/src/components/index.ts` — 导出新增组件。
- Modify: `apps/admin/src/types.ts` — 增加反馈与导航写入类型，扩展 `AdminSection`。
- Modify: `apps/admin/src/services/f-admin.ts` — 为列表接入分页参数、反馈和导航 CRUD。
- Modify: `apps/admin/src/admin-navigation.ts` — 增加用户反馈入口。
- Modify: `apps/admin/src/App.tsx` — 重写统计、用户、帖子、商品、举报、反馈和导航页面组合。
- Modify: `apps/admin/src/styles.css` — 收敛壳层、公共组件、表格、操作区和响应式样式。
- Modify: `scripts/check-admin-ui-system.cjs` — 增加补全功能的静态契约检查。

### Task 1: 公共组件契约

**Files:**
- Create: `apps/admin/src/components/AdminPanel.tsx`
- Create: `apps/admin/src/components/AdminToolbar.tsx`
- Create: `apps/admin/src/components/AdminPagination.tsx`
- Create: `apps/admin/src/components/AdminStatusBadge.tsx`
- Create: `apps/admin/src/components/AdminRowActions.tsx`
- Modify: `apps/admin/src/components/index.ts`
- Modify: `scripts/check-admin-ui-system.cjs`

- [ ] **Step 1: 先扩展静态契约检查**

在组件必需清单加入新增组件，并检查分页器、反馈入口与导航写操作：

```js
for (const name of ['AdminPanel', 'AdminToolbar', 'AdminPagination', 'AdminStatusBadge', 'AdminRowActions']) {
  assert.ok(fs.existsSync(path.join(componentRoot, `${name}.tsx`)), `${name}.tsx is required`)
}
assert.match(appSource, /function Feedbacks\(/)
assert.match(adminAPI, /feedbacks:\s*\(query/)
assert.match(adminAPI, /createNavigationCategory:/)
assert.match(adminAPI, /deleteNavigationItem:/)
```

- [ ] **Step 2: 运行检查并确认失败**

Run: `node scripts/check-admin-ui-system.cjs`

Expected: FAIL，提示 `AdminPanel.tsx is required`。

- [ ] **Step 3: 实现公共组件**

组件接口固定为：

```tsx
export function AdminPanel(props: { title: string; description?: string; actions?: ReactNode; children: ReactNode }): JSX.Element
export function AdminToolbar(props: { summary?: ReactNode; actions?: ReactNode; children?: ReactNode }): JSX.Element
export function AdminPagination(props: { page: number; pageSize: number; total: number; loading?: boolean; onPageChange(page: number): void }): JSX.Element
export function AdminStatusBadge(props: { value: string }): JSX.Element
export function AdminRowActions(props: { children: ReactNode }): JSX.Element
```

`AdminPagination` 使用 `Math.max(1, Math.ceil(total / pageSize))` 计算总页数；前后按钮在边界和加载时禁用。`AdminStatusBadge` 对 `active/muted/banned/user/admin/superadmin/submitted/reviewing/approved/rejected` 等现有枚举提供中文映射，未知值保留原值并使用中性色。

- [ ] **Step 4: 导出并运行静态检查**

Run: `node scripts/check-admin-ui-system.cjs`

Expected: 后续页面契约仍失败，但新增组件存在性检查通过。

- [ ] **Step 5: 提交公共组件**

```powershell
git add -- apps/admin/src/components scripts/check-admin-ui-system.cjs
git commit -m "界面：补齐后台公共列表组件"
```

### Task 2: 服务与类型契约

**Files:**
- Modify: `apps/admin/src/types.ts`
- Modify: `apps/admin/src/services/f-admin.ts`
- Modify: `scripts/check-admin-ui-system.cjs`

- [ ] **Step 1: 定义分页查询与反馈类型**

```ts
export interface PageQuery { page?: number; pageSize?: number }
export interface FeedbackRow {
  id: number
  userId: number
  category: string
  content: string
  status: string
  images?: Array<{ assetId: string }>
  createdAt: string
}
export interface NavigationCategoryInput { name: string; enabled: boolean }
export interface NavigationItemInput { categoryId: number; title: string; appId: string; path: string; enabled: boolean }
```

同时将 `AdminSection` 增加 `'feedbacks'`。

- [ ] **Step 2: 为列表增加查询参数**

使用一个通用查询函数：

```ts
function pageQueryString(query: PageQuery = {}) {
  const params = new URLSearchParams()
  if (query.page) params.set('page', String(query.page))
  if (query.pageSize) params.set('pageSize', String(query.pageSize))
  const value = params.toString()
  return value ? `?${value}` : ''
}
```

将 `users/posts/products/reports/feedbacks` 改为接受 `PageQuery`，请求现有接口。

- [ ] **Step 3: 接入导航 CRUD**

```ts
createNavigationCategory: (input: NavigationCategoryInput) => api<NavigationCategory>('/admin/navigation/categories', { method: 'POST', body: JSON.stringify(input) })
updateNavigationCategory: (id: number, input: NavigationCategoryInput) => api<{ updated: boolean }>(`/admin/navigation/categories/${id}`, { method: 'PUT', body: JSON.stringify(input) })
deleteNavigationCategory: (id: number) => api<{ deleted: boolean }>(`/admin/navigation/categories/${id}`, { method: 'DELETE' })
createNavigationItem: (input: NavigationItemInput) => api<NavigationItem>('/admin/navigation/items', { method: 'POST', body: JSON.stringify(input) })
updateNavigationItem: (id: number, input: NavigationItemInput) => api<{ updated: boolean }>(`/admin/navigation/items/${id}`, { method: 'PUT', body: JSON.stringify(input) })
deleteNavigationItem: (id: number) => api<{ deleted: boolean }>(`/admin/navigation/items/${id}`, { method: 'DELETE' })
```

- [ ] **Step 4: 类型检查**

Run: `npm --prefix apps/admin run typecheck`

Expected: PASS。

- [ ] **Step 5: 提交服务契约**

```powershell
git add -- apps/admin/src/types.ts apps/admin/src/services/f-admin.ts scripts/check-admin-ui-system.cjs
git commit -m "功能：接入后台分页反馈与导航接口"
```

### Task 3: 核心管理页面补全

**Files:**
- Modify: `apps/admin/src/App.tsx`
- Modify: `apps/admin/src/admin-navigation.ts`

- [ ] **Step 1: 增加统一列表加载模式**

每个列表保存 `page`、`data`、`loading`、`error`、`notice`，以 `pageSize=20` 请求现有 API。刷新后若 `items.length === 0 && page > 1`，回退一页。

```tsx
const PAGE_SIZE = 20
const [page, setPage] = useState(1)
const [data, setData] = useState<Page<Row> | null>(null)
const [loading, setLoading] = useState(false)
```

- [ ] **Step 2: 重做用户管理**

用 `AdminStatusBadge` 显示角色和状态；用 `AdminRowActions` 分组状态与角色动作。当前状态按钮禁用，禁言、封禁、降权及超级管理员提升先调用 `dialog.confirm`。写操作成功后显示 `AdminNotice` 并刷新当前页。

- [ ] **Step 3: 补全帖子、商品和举报**

三个页面接入统一分页、状态映射、空态和错误重试。帖子和商品保留带原因下架；待审商品提供 `setSection('moderation')`。举报页面不新增处置接口，只提供前往统一审核入口。

- [ ] **Step 4: 新增用户反馈页面**

`Feedbacks` 调用 `adminAPI.feedbacks({ page, pageSize: PAGE_SIZE })`，展示用户 ID、类别、内容摘要、图片数量、状态与时间。页面不渲染任何后端不存在的处理按钮。

- [ ] **Step 5: 完成数据统计快捷入口**

基于现有 `Stats` 渲染真实指标，并用按钮切换到统一审核、用户管理和导航管理；不计算趋势。

- [ ] **Step 6: 更新导航与内容分发**

在“内容治理”增加 `feedbacks`，并在 `Content` 中返回 `<Feedbacks />`。

- [ ] **Step 7: 运行静态检查与类型检查**

Run: `node scripts/check-admin-ui-system.cjs`

Expected: `admin ui system: ok`。

Run: `npm --prefix apps/admin run typecheck`

Expected: PASS。

- [ ] **Step 8: 提交核心页面**

```powershell
git add -- apps/admin/src/App.tsx apps/admin/src/admin-navigation.ts
git commit -m "功能：补全后台核心管理页面"
```

### Task 4: 普通导航完整管理

**Files:**
- Modify: `apps/admin/src/App.tsx`

- [ ] **Step 1: 分类编辑表单**

使用 `AdminDialog.prompt` 获取名称、`AdminDialog.confirm` 确认删除；启用状态通过编辑时的当前值切换。提交字段仅为 `name` 和 `enabled`。

- [ ] **Step 2: 入口编辑表单**

入口编辑采用页面内紧凑表单，字段严格为 `categoryId/title/appId/path/enabled`。创建和编辑共享同一状态对象，取消时清空编辑状态。

- [ ] **Step 3: 写操作反馈**

所有删除操作二次确认；提交期间禁用当前按钮；成功后重新调用 `adminAPI.navigation()`，失败显示 `AdminNotice tone="danger"`。

- [ ] **Step 4: 保留外部目录边界**

`ExternalTakeoutMerchants` 继续独立使用现有接口，不与普通导航表单混用。其运行错误以模块级错误状态展示，不阻断普通导航管理。

- [ ] **Step 5: 类型检查并提交**

Run: `npm --prefix apps/admin run typecheck`

Expected: PASS。

```powershell
git add -- apps/admin/src/App.tsx
git commit -m "功能：补齐普通导航增删改管理"
```

### Task 5: 样式系统收敛

**Files:**
- Modify: `apps/admin/src/styles.css`

- [ ] **Step 1: 删除冲突壳层规则**

保留一套 `html/body/.app-shell/.sidebar/.main/.topbar/.main__content` 定义；桌面列宽为 `252px minmax(0, 1fr)`，正文 `max-width: 1480px`，所有内容容器 `min-width: 0`。

- [ ] **Step 2: 统一公共组件样式**

为 `admin-panel/admin-toolbar/admin-pagination/admin-row-actions/admin-status-badge` 定义边界、间距、交互和焦点状态。危险操作只使用红色文本或边框，不使用大面积高饱和背景。

- [ ] **Step 3: 收敛表格**

表格统一 `min-width: 760px`、表头、行高、首列标题/元数据、悬停与操作区；按钮触控高度至少 34px。操作列不再因按钮换行造成每行高度随机。

- [ ] **Step 4: 响应式规则**

`<=900px` 时侧栏转顶部导航、顶部栏换行；`<=560px` 时单列表单。页面本身不得产生水平溢出，只有 `.admin-table-frame` 和 `.table-scroll` 可横向滚动。

- [ ] **Step 5: 静态与构建验证**

Run: `node scripts/check-admin-ui-system.cjs`

Expected: `admin ui system: ok`。

Run: `npm --prefix apps/admin run build`

Expected: TypeScript 和 Vite build 均成功。

- [ ] **Step 6: 提交样式**

```powershell
git add -- apps/admin/src/styles.css
git commit -m "界面：收敛后台布局与表格视觉系统"
```

### Task 6: 本地运行条件与浏览器验收

**Files:**
- No project code changes expected.

- [ ] **Step 1: 核对迁移状态**

使用只读查询确认 `external_takeout_merchants` 是否存在。缺失时，仅对本地 `campus_circle` 应用仓库已有 `backend/migrations/031_external_takeout_merchant_directory.sql`，记录该操作，不创建假商家。

- [ ] **Step 2: 运行完整静态验证**

Run: `node scripts/check-admin-ui-system.cjs`

Run: `npm --prefix apps/admin run typecheck`

Run: `npm --prefix apps/admin run build`

Run: `git diff --check`

Expected: 全部 PASS。

- [ ] **Step 3: 桌面浏览器逐页检查**

使用真实管理员会话依次打开媒体、配送、审核、帖子、商品、举报、身份、反馈、统计、用户和导航。每页不得出现意外 `role=alert`；GET 数据、分页控件和空态应正常。

- [ ] **Step 4: 响应式验证**

在真实 900px 和 430px viewport 检查：

```js
({ innerWidth: window.innerWidth, scrollWidth: document.documentElement.scrollWidth })
```

Expected: `scrollWidth === innerWidth`；宽表仅在表格容器内部滚动。

- [ ] **Step 5: 精确提交与状态报告**

确认提交只包含 `apps/admin`、检查脚本和计划文档；四个小程序脏文件与 `artifacts/` 保持未提交。报告分支、提交、构建、浏览器结果、本地迁移以及未合并/未部署状态。
