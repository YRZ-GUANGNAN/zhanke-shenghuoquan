# 小程序 UI 细节系统升级 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 为 27 个正式小程序路由建立统一的 token、按钮、字段、弹窗、底部操作面板和页面状态，并在不改变业务合同的前提下完成全页面视觉收敛。

**Architecture:** 以 `design-tokens.scss` 为唯一视觉变量来源，在 `components/ui` 中建立无业务依赖的 Taro React 原语，再由现有共享组件和页面逐步迁移。通过静态合同脚本锁定组件边界、微信平台弹窗白名单、安全区和禁止假数据规则。

**Tech Stack:** React 18、Taro 4、TypeScript、SCSS、Node.js `assert` 合同检查。

---

## 文件结构

- Create: `apps/miniapp/src/components/ui/app-button.tsx` / `.scss` — 按钮语义与异步状态。
- Create: `apps/miniapp/src/components/ui/app-field.tsx` / `.scss` — 输入框、文本域与错误说明。
- Create: `apps/miniapp/src/components/ui/app-dialog.tsx` / `.scss` — 居中确认弹窗。
- Create: `apps/miniapp/src/components/ui/app-action-sheet.tsx` / `.scss` — 底部操作面板。
- Create: `apps/miniapp/src/components/ui/app-state.tsx` / `.scss` — 加载、空、错误、无权限状态。
- Create: `apps/miniapp/src/components/ui/app-notice.tsx` / `.scss` — 页面内语义提示。
- Create: `apps/miniapp/src/components/ui/app-skeleton.tsx` / `.scss` — 列表、详情和指标骨架。
- Create: `apps/miniapp/src/components/ui/index.ts` — 唯一公共导出入口。
- Create: `scripts/check-miniapp-ui-system.cjs` — 视觉边界与迁移合同。
- Modify: `apps/miniapp/src/styles/design-tokens.scss` — 语义色、字级、间距、圆角、阴影、层级和动效。
- Modify: `apps/miniapp/src/app.scss` — 全局基础样式只消费 token，移除重复旧规则。
- Modify: `apps/miniapp/src/components/app-icon.tsx` — 扩展动作图标并保持统一画布。
- Modify: `apps/miniapp/src/components/IdentitySwitchModal.tsx` / `.scss` — 改用 `AppDialog`。
- Modify: `apps/miniapp/src/components/runtime-session-gate.tsx` — 改用 `AppState`。
- Modify: `apps/miniapp/src/components/capsule-page-header.scss`、`student-bottom-nav.scss` — 统一尺寸、反馈和安全区。
- Modify: `apps/miniapp/src/pages/*/index.tsx` / `index.scss` — 迁移正式路由中的弹层、表单、状态和页面比例。
- Modify: `package.json` — 将 UI 合同加入 `verify`。

### Task 1: 锁定小程序 UI 视觉合同

**Files:**
- Create: `scripts/check-miniapp-ui-system.cjs`
- Modify: `package.json`

- [ ] **Step 1: 编写失败合同检查**

检查脚本读取 token、公共组件和正式路由源码，要求六个 UI 组件存在，禁止页面自定义 `*-modal__mask`，并只允许 `pages/settings/index.tsx` 使用 `Taro.showModal`：

```js
const assert = require('node:assert/strict')
const fs = require('node:fs')
const path = require('node:path')

const root = path.join(__dirname, '..')
const uiRoot = path.join(root, 'apps', 'miniapp', 'src', 'components', 'ui')
for (const name of ['app-button', 'app-field', 'app-dialog', 'app-action-sheet', 'app-state', 'app-notice', 'app-skeleton']) {
  assert.ok(fs.existsSync(path.join(uiRoot, `${name}.tsx`)), `${name}.tsx is required`)
  assert.ok(fs.existsSync(path.join(uiRoot, `${name}.scss`)), `${name}.scss is required`)
}

const tokens = fs.readFileSync(path.join(root, 'apps', 'miniapp', 'src', 'styles', 'design-tokens.scss'), 'utf8')
for (const token of ['$radius-control', '$radius-card', '$duration-fast', '$z-dialog']) assert.match(tokens, new RegExp(token.replace('$', '\\$')))

const pagesRoot = path.join(root, 'apps', 'miniapp', 'src', 'pages')
const sources = fs.readdirSync(pagesRoot, { withFileTypes: true })
  .filter(entry => entry.isDirectory())
  .flatMap(entry => ['index.tsx', 'index.scss'].map(file => path.join(pagesRoot, entry.name, file)))
  .filter(fs.existsSync)
  .map(file => ({ file, source: fs.readFileSync(file, 'utf8') }))

for (const { file, source } of sources) {
  assert.doesNotMatch(source, /[a-z0-9-]+-modal__mask/, `${file} must use AppDialog or AppActionSheet`)
  if (!file.endsWith(path.join('settings', 'index.tsx'))) assert.doesNotMatch(source, /Taro\.showModal/, `${file} must not use platform modal`)
}
console.log('miniapp ui system: ok')
```

- [ ] **Step 2: 运行并确认失败**

Run: `node scripts/check-miniapp-ui-system.cjs`

Expected: FAIL，首个缺失项为 `app-button.tsx is required`。

- [ ] **Step 3: 注册根脚本**

在根 `package.json` 增加：

```json
"check:miniapp-ui-system": "node scripts/check-miniapp-ui-system.cjs"
```

并在 `verify` 的前端合同段加入 `npm run check:miniapp-ui-system`。

- [ ] **Step 4: 提交合同基线**

```powershell
git add scripts/check-miniapp-ui-system.cjs package.json
git commit -m "测试：锁定小程序 UI 组件边界"
```

### Task 2: 建立 token 与按钮、字段原语

**Files:**
- Modify: `apps/miniapp/src/styles/design-tokens.scss`
- Modify: `apps/miniapp/src/app.scss`
- Create: `apps/miniapp/src/components/ui/app-button.tsx`
- Create: `apps/miniapp/src/components/ui/app-button.scss`
- Create: `apps/miniapp/src/components/ui/app-field.tsx`
- Create: `apps/miniapp/src/components/ui/app-field.scss`
- Create: `apps/miniapp/src/components/ui/index.ts`

- [ ] **Step 1: 扩展语义 token**

加入固定语义档位，并让 `page` 同步暴露 CSS 变量：

```scss
$brand-primary: #2e7dd4;
$brand-primary-hover: #256fbd;
$brand-primary-pressed: #1f61a6;
$success: #168a5b;
$warning: #b7791f;
$danger: #c43d4b;
$info: #2e7dd4;
$radius-control: 16rpx;
$radius-card: 24rpx;
$radius-dialog: 32rpx;
$space-1: 8rpx;
$space-2: 16rpx;
$space-3: 24rpx;
$space-4: 32rpx;
$duration-fast: 120ms;
$duration-standard: 200ms;
$duration-layer: 260ms;
$ease-standard: cubic-bezier(.2, 0, 0, 1);
$z-sticky: 20;
$z-overlay: 80;
$z-dialog: 100;
```

- [ ] **Step 2: 实现 `AppButton`**

公共接口固定为：

```tsx
export type AppButtonVariant = 'primary' | 'secondary' | 'ghost' | 'danger' | 'text'
export type AppButtonSize = 'small' | 'regular'

export interface AppButtonProps {
  children: ReactNode
  variant?: AppButtonVariant
  size?: AppButtonSize
  block?: boolean
  loading?: boolean
  disabled?: boolean
  className?: string
  onClick?: () => void
}

export function AppButton({ children, variant = 'primary', size = 'regular', block = false, loading = false, disabled = false, className = '', onClick }: AppButtonProps) {
  const locked = loading || disabled
  return <View className={`app-button app-button--${variant} app-button--${size} ${block ? 'app-button--block' : ''} ${locked ? 'app-button--disabled' : ''} ${className}`.trim()} onClick={() => !locked && onClick?.()}>
    <Text>{loading ? '处理中…' : children}</Text>
  </View>
}
```

SCSS 必须实现最小 88rpx 触控高度、按下态、整行、禁用和五种语义，不使用新的近似品牌蓝。

- [ ] **Step 3: 实现 `AppField`**

接口支持单行和多行，并统一标签、说明和错误：

```tsx
export interface AppFieldProps {
  label?: string
  value: string
  placeholder?: string
  multiline?: boolean
  maxLength?: number
  helperText?: string
  error?: string
  disabled?: boolean
  onInput: (value: string) => void
}
```

使用 Taro `Input` / `Textarea`，错误文本设置 `aria-role='alert'`，字段容器通过 `app-field--error` 和 `app-field--disabled` 表达状态。

- [ ] **Step 4: 收敛全局旧规则**

`app.scss` 只保留 reset、通用页面背景和兼容别名；`.cc-button`、`.cc-field`、`.cc-textarea` 改为消费 token，不再出现 `#1677ff`、`#2f80d6` 等第二主色。

- [ ] **Step 5: 验证并提交**

Run: `npm --workspace apps/miniapp run typecheck`

Expected: PASS。

```powershell
git add apps/miniapp/src/styles/design-tokens.scss apps/miniapp/src/app.scss apps/miniapp/src/components/ui
git commit -m "界面：建立小程序按钮与字段原语"
```

### Task 3: 建立弹窗、底部面板和页面状态

**Files:**
- Create: `apps/miniapp/src/components/ui/app-dialog.tsx` / `.scss`
- Create: `apps/miniapp/src/components/ui/app-action-sheet.tsx` / `.scss`
- Create: `apps/miniapp/src/components/ui/app-state.tsx` / `.scss`
- Create: `apps/miniapp/src/components/ui/app-notice.tsx` / `.scss`
- Create: `apps/miniapp/src/components/ui/app-skeleton.tsx` / `.scss`
- Modify: `apps/miniapp/src/components/ui/index.ts`

- [ ] **Step 1: 实现 `AppDialog`**

```tsx
export interface AppDialogProps {
  visible: boolean
  title: string
  description?: string
  children?: ReactNode
  confirmText?: string
  cancelText?: string
  tone?: 'default' | 'danger'
  loading?: boolean
  error?: string
  onConfirm: () => void
  onCancel: () => void
}
```

组件必须在 `loading` 时阻止遮罩关闭，按钮分别使用 `AppButton` 的 `secondary` 与 `primary/danger`，错误保留在面板内。

- [ ] **Step 2: 实现 `AppActionSheet`**

```tsx
export interface AppActionSheetProps {
  visible: boolean
  title: string
  description?: string
  children: ReactNode
  loading?: boolean
  onClose: () => void
}
```

面板固定底部，圆角只出现在顶部，`padding-bottom` 使用 `calc(24rpx + env(safe-area-inset-bottom))`，遮罩与 `AppDialog` 共用 `$z-overlay`。

- [ ] **Step 3: 实现状态与提示**

```tsx
export type AppStateKind = 'loading' | 'empty' | 'error' | 'offline' | 'forbidden'
export interface AppStateProps { kind: AppStateKind; title: string; description?: string; actionText?: string; onAction?: () => void }
export interface AppNoticeProps { tone?: 'info' | 'success' | 'warning' | 'danger'; title?: string; children: ReactNode }
```

状态组件只使用 CSS 几何图标和文字，不制造插图。

`AppSkeleton` 接口固定为 `variant: 'list' | 'detail' | 'metrics'` 与可选 `rows`；内部只渲染稳定的块级占位，并通过统一的低对比度渐变表现加载，不引用业务图片。

- [ ] **Step 4: 运行类型检查并提交**

Run: `npm --workspace apps/miniapp run typecheck`

Expected: PASS。

```powershell
git add apps/miniapp/src/components/ui
git commit -m "界面：统一小程序弹层与页面状态"
```

### Task 4: 迁移共享组件和全部业务弹层

**Files:**
- Modify: `apps/miniapp/src/components/IdentitySwitchModal.tsx` / `.scss`
- Modify: `apps/miniapp/src/components/runtime-session-gate.tsx`
- Modify: `apps/miniapp/src/components/capsule-page-header.scss`
- Modify: `apps/miniapp/src/components/student-bottom-nav.scss`
- Modify: `apps/miniapp/src/pages/merchant-publish/index.tsx` / `.scss`
- Modify: `apps/miniapp/src/pages/merchant-wallet/index.tsx` / `.scss`
- Modify: `apps/miniapp/src/pages/rider-wallet/index.tsx` / `.scss`
- Modify: `apps/miniapp/src/pages/secondhand-detail/index.tsx` / `.scss`
- Modify: `apps/miniapp/src/pages/user-profile/index.tsx` / `.scss`
- Modify: `apps/miniapp/src/pages/blacklist/index.tsx`
- Modify: `apps/miniapp/src/pages/delivery-rider/index.tsx`
- Modify: `apps/miniapp/src/pages/feedback/index.tsx`

- [ ] **Step 1: 迁移共享会话与身份切换**

`RuntimeSessionGate` 的加载和失败分支改为：

```tsx
if (status === 'idle' || status === 'booting') return <AppState kind='loading' title='正在登录' description='正在安全校验当前微信账号' />
if (status !== 'authenticated') return <AppState kind='error' title='暂时无法登录' description={error || '请重新尝试微信登录'} actionText='重新登录' onAction={() => void retry()} />
```

`IdentitySwitchModal` 保留原有 props，只把面板渲染替换为 `AppDialog`，保证所有调用方不改业务接口。

- [ ] **Step 2: 迁移五套页面弹层**

将支付方式、充值金额、提现、联系卖家和用户操作菜单分别包入 `AppActionSheet`；删除对应页面的 `*-modal__mask`、重复遮罩、重复关闭按钮和重复安全区 SCSS。

- [ ] **Step 3: 迁移业务确认弹窗**

为黑名单移除、骑手订单动作、反馈完成、商品状态、用户拉黑和账户注销在各页面增加受控 `AppDialog` 状态；只保留设置页版本更新使用 `Taro.showModal`。

- [ ] **Step 4: 运行 UI 合同并修正全部命中项**

Run: `npm run check:miniapp-ui-system`

Expected: `miniapp ui system: ok`。

- [ ] **Step 5: 提交弹层迁移**

```powershell
git add apps/miniapp/src/components apps/miniapp/src/pages/merchant-publish apps/miniapp/src/pages/merchant-wallet apps/miniapp/src/pages/rider-wallet apps/miniapp/src/pages/secondhand-detail apps/miniapp/src/pages/user-profile apps/miniapp/src/pages/blacklist apps/miniapp/src/pages/delivery-rider apps/miniapp/src/pages/feedback
git commit -m "界面：迁移小程序业务弹层与状态反馈"
```

### Task 5: 完成正式路由页面比例与组件风格收敛

**Files:**
- Modify: `apps/miniapp/src/pages/{home,search,post-detail,user-profile,profile,my-content}/index.scss`
- Modify: `apps/miniapp/src/pages/{student-secondhand,secondhand-detail,secondhand-chat,publish-secondhand}/index.scss`
- Modify: `apps/miniapp/src/pages/{publish-entry,publish-post,edit-profile,settings,blacklist,feedback,about,legal}/index.scss`
- Modify: `apps/miniapp/src/pages/{student-takeout,merchant-home,merchant-publish,merchant-wallet,merchant-messages,merchant-order-detail,delivery-rider,delivery-conversation,identity-application}/index.scss`

- [ ] **Step 1: 收敛学生端内容页面**

作者区使用 72rpx 头像、28rpx 用户名、24rpx 辅助信息和最小 88rpx 关注按钮；正文标题不超过 40rpx，互动栏固定 88rpx 高，空评论区不再占据大块空白。

- [ ] **Step 2: 收敛二手与私信页面**

商品卡使用 `$radius-card`、统一 1rpx 边框和价格层级；详情底部操作栏使用白色表面和安全区；聊天输入栏保持键盘顶起后可见并与胶囊标题栏不重叠。

- [ ] **Step 3: 收敛表单和设置页面**

发布、资料编辑、反馈和身份申请的字段全部迁移至 `AppField`；主要提交使用 `AppButton`，危险动作使用 `danger`，说明文本使用 `AppNotice`。

- [ ] **Step 4: 收敛商家和骑手页面**

工作台指标保持现有业务内容，统一卡片内边距、状态徽标、订单卡操作区和空状态；不恢复任何已经淘汰的旧工作台版本。

同时扩展 `AppIcon` 的返回、关闭、搜索、收藏、评论、更多、钱包、订单和刷新图标，页面不得再用 `×`、`→` 或表情符号承担主要动作图标。

- [ ] **Step 5: 运行正式路由与胶囊检查**

Run: `npm run check:miniapp-static-routes && npm run check:miniapp-capsule-layout && npm run check:merchant-home-clarity && npm run check:rider-workbench-clarity`

Expected: 四项 PASS。

- [ ] **Step 6: 提交页面收敛**

```powershell
git add apps/miniapp/src/pages apps/miniapp/src/components/capsule-page-header.scss apps/miniapp/src/components/student-bottom-nav.scss
git commit -m "界面：完成小程序正式页面细节收敛"
```

### Task 6: 小程序完整验收

**Files:**
- Modify: `scripts/check-miniapp-ui-system.cjs`（只在发现遗漏约束时）

- [ ] **Step 1: 执行合同和类型检查**

Run: `npm run check:miniapp-ui-system && npm run check:miniapp-capsule-layout && npm --workspace apps/miniapp run typecheck`

Expected: 全部 PASS。

- [ ] **Step 2: 执行生产构建**

Run: `npm run build:miniapp`

Expected: 构建成功；允许既有 `vendors.js` 体积警告，但不得新增 WXSS 编译错误或 CSS 顺序警告。

- [ ] **Step 3: 检查工作区质量**

Run: `git diff --check && git status --short`

Expected: 无空白错误；只显示明确保留的旧计划或当前验收文件。

- [ ] **Step 4: 提交验收补强**

若 Step 1 发现并补充了新合同：

```powershell
git add scripts/check-miniapp-ui-system.cjs
git commit -m "测试：补强小程序 UI 细节验收"
```
