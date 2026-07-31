# 管理后台 UI 细节系统升级 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 建立管理后台统一的按钮、字段、状态、图标和异步对话框系统，移除浏览器原生弹窗并收敛所有运营模块的布局与交互状态。

**Architecture:** 将无业务依赖的 React 组件放入 `apps/admin/src/components`，通过 `AdminDialogProvider` 向现有业务组件提供 Promise 风格的 `confirm` 与 `prompt`，以最小业务改动替换原生 API。全局样式以 CSS 变量为 token，保留现有模块结构和接口调用。

**Tech Stack:** React 18、TypeScript、Vite、CSS、Node.js `assert` 合同检查。

---

## 文件结构

- Create: `apps/admin/src/components/AdminButton.tsx` — 统一按钮语义和加载状态。
- Create: `apps/admin/src/components/AdminField.tsx` — 输入、选择和错误说明。
- Create: `apps/admin/src/components/AdminSelect.tsx` — 筛选与表单选择器。
- Create: `apps/admin/src/components/AdminIcon.tsx` — 内置线性 SVG 图标。
- Create: `apps/admin/src/components/AdminState.tsx` — 加载、空、错误和无权限状态。
- Create: `apps/admin/src/components/AdminBadge.tsx` — 审核、业务和系统状态徽标。
- Create: `apps/admin/src/components/AdminNotice.tsx` — 页面内成功、警告与错误反馈。
- Create: `apps/admin/src/components/AdminDialog.tsx` — 对话框视图与焦点管理。
- Create: `apps/admin/src/components/AdminDialogProvider.tsx` — Promise 风格确认与原因输入服务。
- Create: `apps/admin/src/components/index.ts` — 公共导出。
- Create: `scripts/check-admin-ui-system.cjs` — 禁止原生弹窗和字母图标的静态合同。
- Modify: `apps/admin/src/main.tsx` — 注入 `AdminDialogProvider`。
- Modify: `apps/admin/src/App.tsx` — 迁移导航图标、共享状态和审核/下架弹窗。
- Modify: `apps/admin/src/AccountPanel.tsx`、`Login.tsx` — 统一账户表单和结果反馈。
- Modify: `apps/admin/src/DeliveryConsole.tsx`、`TakeoutConsole.tsx`、`ExternalTakeoutMerchants.tsx`、`IdentityApplications.tsx` — 替换原生确认与原因输入。
- Modify: `apps/admin/src/MediaConsole.tsx` — 收敛状态反馈与表格操作。
- Modify: `apps/admin/src/styles.css` — token、组件、响应式和焦点样式。
- Modify: `package.json` — 将后台 UI 合同加入 `verify`。

### Task 1: 锁定后台 UI 合同

**Files:**
- Create: `scripts/check-admin-ui-system.cjs`
- Modify: `package.json`

- [ ] **Step 1: 编写失败合同检查**

```js
const assert = require('node:assert/strict')
const fs = require('node:fs')
const path = require('node:path')

const root = path.join(__dirname, '..')
const src = path.join(root, 'apps', 'admin', 'src')
const files = fs.readdirSync(src, { withFileTypes: true })
  .filter(entry => entry.isFile() && entry.name.endsWith('.tsx'))
  .map(entry => path.join(src, entry.name))

const componentRoot = path.join(src, 'components')
for (const name of ['AdminButton', 'AdminField', 'AdminSelect', 'AdminIcon', 'AdminState', 'AdminBadge', 'AdminNotice', 'AdminDialog', 'AdminDialogProvider']) {
  assert.ok(fs.existsSync(path.join(componentRoot, `${name}.tsx`)), `${name}.tsx is required`)
}

const source = files.map(file => fs.readFileSync(file, 'utf8')).join('\n')
assert.doesNotMatch(source, /window\.(confirm|prompt|alert)\s*\(/)
assert.doesNotMatch(fs.readFileSync(path.join(src, 'App.tsx'), 'utf8'), /icon:\s*'[A-Z]'/)

const styles = fs.readFileSync(path.join(src, 'styles.css'), 'utf8')
for (const token of ['--color-primary', '--radius-control', '--shadow-dialog', '--duration-fast']) assert.match(styles, new RegExp(token))
console.log('admin ui system: ok')
```

- [ ] **Step 2: 运行并确认失败**

Run: `node scripts/check-admin-ui-system.cjs`

Expected: FAIL，首个缺失项为 `AdminButton.tsx is required`。

- [ ] **Step 3: 注册检查并提交**

在根 `package.json` 增加 `"check:admin-ui-system": "node scripts/check-admin-ui-system.cjs"`，并加入 `verify`。

```powershell
git add scripts/check-admin-ui-system.cjs package.json
git commit -m "测试：锁定管理后台 UI 组件边界"
```

### Task 2: 建立后台 token、按钮、字段、状态与图标

**Files:**
- Create: `apps/admin/src/components/AdminButton.tsx`
- Create: `apps/admin/src/components/AdminField.tsx`
- Create: `apps/admin/src/components/AdminSelect.tsx`
- Create: `apps/admin/src/components/AdminIcon.tsx`
- Create: `apps/admin/src/components/AdminState.tsx`
- Create: `apps/admin/src/components/AdminBadge.tsx`
- Create: `apps/admin/src/components/AdminNotice.tsx`
- Create: `apps/admin/src/components/index.ts`
- Modify: `apps/admin/src/styles.css`

- [ ] **Step 1: 在 `:root` 建立语义 token**

```css
:root {
  --color-primary: #2e7dd4;
  --color-primary-hover: #256fbd;
  --color-primary-pressed: #1f61a6;
  --color-success: #168a5b;
  --color-warning: #b7791f;
  --color-danger: #c43d4b;
  --color-text: #172033;
  --color-text-secondary: #607087;
  --color-border: #dfe6ef;
  --color-surface: #ffffff;
  --color-canvas: #f4f7fb;
  --radius-control: 10px;
  --radius-card: 14px;
  --radius-dialog: 18px;
  --shadow-card: 0 10px 30px rgba(22, 39, 64, .06);
  --shadow-dialog: 0 24px 80px rgba(13, 27, 48, .22);
  --duration-fast: 120ms;
  --duration-standard: 200ms;
}
```

- [ ] **Step 2: 实现按钮与字段**

```tsx
export type AdminButtonTone = 'primary' | 'secondary' | 'ghost' | 'danger' | 'text'
export interface AdminButtonProps extends React.ButtonHTMLAttributes<HTMLButtonElement> { tone?: AdminButtonTone; loading?: boolean }
export function AdminButton({ tone = 'secondary', loading = false, disabled, children, className = '', ...props }: AdminButtonProps) {
  return <button {...props} className={`admin-button admin-button--${tone} ${className}`.trim()} disabled={disabled || loading}>{loading ? '处理中…' : children}</button>
}
```

`AdminField` 支持 `label`、`description`、`error`、`required` 与原生 `input/select/textarea` props，错误使用 `role="alert"`。

`AdminSelect` 支持 `label`、`description`、`error` 和原生 `select` props，外观与 `AdminField` 共用 `.admin-control`；下拉箭头由 CSS 绘制，不使用平台不一致的字符。

- [ ] **Step 3: 实现状态和 SVG 图标**

`AdminState` 接口固定为：

```tsx
export interface AdminStateProps { kind: 'loading' | 'empty' | 'error' | 'forbidden'; title: string; description?: string; actionLabel?: string; onAction?: () => void }
```

`AdminIcon` 使用内部 SVG path 映射，名称覆盖 `media`、`delivery`、`dashboard`、`users`、`identities`、`moderation`、`community`、`trade`、`reports`、`navigation`、`search`、`close`。

`AdminBadge` 接口固定为 `tone: 'neutral' | 'info' | 'success' | 'warning' | 'danger'` 与文本子节点；所有审核和业务状态先映射为 tone，再渲染统一徽标。

`AdminNotice` 接口固定为 `tone: 'info' | 'success' | 'warning' | 'danger'`、可选标题和文本子节点；页面级 API 结果使用 `role="status"`，危险错误使用 `role="alert"`。

- [ ] **Step 4: 验证并提交**

Run: `npm --workspace apps/admin run typecheck`

Expected: PASS。

```powershell
git add apps/admin/src/components apps/admin/src/styles.css
git commit -m "界面：建立管理后台基础组件"
```

### Task 3: 建立异步 `AdminDialogProvider`

**Files:**
- Create: `apps/admin/src/components/AdminDialog.tsx`
- Create: `apps/admin/src/components/AdminDialogProvider.tsx`
- Modify: `apps/admin/src/components/index.ts`
- Modify: `apps/admin/src/main.tsx`
- Modify: `apps/admin/src/styles.css`

- [ ] **Step 1: 定义 Promise 风格接口**

```tsx
export interface ConfirmOptions { title: string; description: string; confirmLabel?: string; tone?: 'default' | 'danger' }
export interface PromptOptions extends ConfirmOptions { inputLabel: string; placeholder?: string; requiredMessage?: string }
export interface AdminDialogAPI {
  confirm(options: ConfirmOptions): Promise<boolean>
  prompt(options: PromptOptions): Promise<string | null>
  notify(options: Omit<ConfirmOptions, 'tone'>): Promise<void>
}
```

- [ ] **Step 2: 实现 Provider 状态机**

Provider 保存单个活动请求和触发控件引用；确认、取消和卸载时只结算一次 Promise。`prompt` 在空值时显示 `requiredMessage || '请输入原因'`，不关闭弹窗。

- [ ] **Step 3: 实现对话框视图**

`AdminDialog` 使用 `role="dialog"`、`aria-modal="true"`、`aria-labelledby`；打开后聚焦输入或确认按钮，Escape 只在非提交状态关闭，遮罩不可误触提交。

- [ ] **Step 4: 注入根节点**

```tsx
ReactDOM.createRoot(document.getElementById('root')!).render(
  <React.StrictMode>
    <AdminDialogProvider><App /></AdminDialogProvider>
  </React.StrictMode>,
)
```

- [ ] **Step 5: 验证并提交**

Run: `npm --workspace apps/admin run typecheck`

Expected: PASS。

```powershell
git add apps/admin/src/components apps/admin/src/main.tsx apps/admin/src/styles.css
git commit -m "界面：建立管理后台异步对话框"
```

### Task 4: 替换全部原生确认、输入和提示弹窗

**Files:**
- Modify: `apps/admin/src/App.tsx`
- Modify: `apps/admin/src/AccountPanel.tsx`
- Modify: `apps/admin/src/DeliveryConsole.tsx`
- Modify: `apps/admin/src/TakeoutConsole.tsx`
- Modify: `apps/admin/src/ExternalTakeoutMerchants.tsx`
- Modify: `apps/admin/src/IdentityApplications.tsx`

- [ ] **Step 1: 迁移统一审核与内容下架**

在组件内使用：

```tsx
const dialog = useAdminDialog()
const confirmed = await dialog.confirm({ title: `通过${label}？`, description: `通过后${label} #${id} 将按业务规则公开展示。`, confirmLabel: '通过并公开' })
const reason = await dialog.prompt({ title: `拒绝${label}`, description: `拒绝后内容不会公开展示。`, inputLabel: '拒绝原因', requiredMessage: '拒绝原因不能为空', confirmLabel: '确认拒绝', tone: 'danger' })
```

将顶层 `takedownReason` 改为组件内异步函数，原因输入与危险确认合并为一次 `prompt`。

- [ ] **Step 2: 迁移配送、外卖与身份审核**

所有费率更新、特殊价停用、线下清账、骑手状态、提现处理、商家删除和身份通过动作改用 `dialog.confirm/prompt`；确认文案必须包含具体用户、商家、订单或金额。

- [ ] **Step 3: 迁移账户结果提示**

密码修改成功使用 `await dialog.notify({ title: '密码已修改', description: '所有管理网页会话已退出，请使用新密码重新登录。', confirmLabel: '重新登录' })`，删除 `window.alert`。

- [ ] **Step 4: 运行静态合同**

Run: `npm run check:admin-ui-system`

Expected: `admin ui system: ok`。

- [ ] **Step 5: 提交迁移**

```powershell
git add apps/admin/src/App.tsx apps/admin/src/AccountPanel.tsx apps/admin/src/DeliveryConsole.tsx apps/admin/src/TakeoutConsole.tsx apps/admin/src/ExternalTakeoutMerchants.tsx apps/admin/src/IdentityApplications.tsx
git commit -m "界面：统一后台风险操作弹窗"
```

### Task 5: 收敛后台导航、表格、筛选和账户页面

**Files:**
- Modify: `apps/admin/src/App.tsx`
- Modify: `apps/admin/src/Login.tsx`
- Modify: `apps/admin/src/AccountPanel.tsx`
- Modify: `apps/admin/src/MediaConsole.tsx`
- Modify: `apps/admin/src/DeliveryConsole.tsx`
- Modify: `apps/admin/src/TakeoutConsole.tsx`
- Modify: `apps/admin/src/ExternalTakeoutMerchants.tsx`
- Modify: `apps/admin/src/IdentityApplications.tsx`
- Modify: `apps/admin/src/styles.css`

- [ ] **Step 1: 替换侧栏字母图标**

`nav` 删除 `icon:string`，渲染 `<AdminIcon name={item.key === 'takeout' ? 'delivery' : item.key} />`；品牌标识从字母 `F` 改为“湛”字标，不新增图片资源。

- [ ] **Step 2: 统一状态和操作组件**

加载、空和错误分支改为 `AdminState`；主要、次要和危险按钮改用 `AdminButton`；状态使用 `AdminBadge` 或既有 `.status` 的统一 token 版本。

- [ ] **Step 3: 收敛表格和筛选布局**

所有表格放入 `.table-scroll`，表头保持可读对比度；筛选区允许换行，行内操作使用 `.table-actions`；空数据在表格外显示 `AdminState kind="empty"`。

- [ ] **Step 4: 收敛登录和账户安全**

登录表单使用统一字段、按钮和错误提示；密码弹窗保留三字段业务规则但改用共享对话框表面和 token。

- [ ] **Step 5: 增加响应式与减少动态效果**

```css
@media (max-width: 1180px) {
  body { min-width: 960px; }
  .app-shell { grid-template-columns: 216px minmax(0, 1fr); }
  .main { padding: 24px; }
  .metric-grid { grid-template-columns: repeat(2, minmax(0, 1fr)); }
}

@media (prefers-reduced-motion: reduce) {
  *, *::before, *::after { scroll-behavior: auto !important; transition-duration: .01ms !important; animation-duration: .01ms !important; }
}
```

- [ ] **Step 6: 验证并提交**

Run: `npm --workspace apps/admin run typecheck && npm run build:admin`

Expected: 两项 PASS。

```powershell
git add apps/admin/src
git commit -m "界面：完成管理后台细节收敛"
```

### Task 6: 后台与全项目验收

**Files:**
- Modify: `scripts/check-admin-ui-system.cjs`（只在发现遗漏约束时）
- Modify: `docs/RELEASE-CANDIDATE-2026-07-31.md` — 记录 UI 验收范围和上传边界。

- [ ] **Step 1: 执行后台聚焦检查**

Run: `npm run check:admin-ui-system && npm --workspace apps/admin run typecheck && npm run build:admin`

Expected: 全部 PASS。

- [ ] **Step 2: 执行全项目验证**

Run: `npm run verify`

Expected: 所有合同、前端类型检查和后端安全测试 PASS。

- [ ] **Step 3: 执行双前端生产构建**

Run: `npm run build`

Expected: 小程序与管理后台构建成功；不上传微信版本。

- [ ] **Step 4: 检查差异和秘密文件**

Run: `git diff --check && git status --short && npm run check:real-wechat-secret-hygiene`

Expected: 无空白错误和密钥泄漏；旧计划继续保持未跟踪，不混入提交。

- [ ] **Step 5: 更新发布候选说明并提交**

发布说明记录完成的 UI 组件、原生弹窗清零、构建结果、已知 `vendors.js` 警告和仍需用户执行的微信上传。

```powershell
git add docs/RELEASE-CANDIDATE-2026-07-31.md scripts/check-admin-ui-system.cjs
git commit -m "交付：补充 UI 细节系统验收证据"
```
