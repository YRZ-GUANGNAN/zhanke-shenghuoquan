# 小程序运行时稳定与外部商家入口 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 移除旧的内置商家编译链路，恢复稳定的底部导航/背景淡化/弹窗与刷新组件，并用未做上传前激进压缩的正常 Taro 构建产物解决体验版白屏和钱包打不开。

**Architecture:** `student-takeout` 只保留 `d-navigation` 的外部商家目录和 `navigateToMiniProgram` 跳转；旧 `ServiceMerchantCatalog` 及其 `g-takeout-buyer` 依赖保留在源码中但不再从可达页面导入。全局 UI 恢复到当前工作树的稳定契约：原生刷新反馈、原有 sheet 拖拽和半透明背景淡化，不在首屏路径挂载实验性 Taro UX 组件。包体通过减少可达模块解决，不使用上传前 `unsafe/toplevel/drop_console` 二次压缩。

**Tech Stack:** Taro 4.2 + React 18 + TypeScript + SCSS；微信开发者工具 CLI；Node.js 回归脚本。

---

### Task 1: 锁定外部商家唯一入口并覆盖回归契约

**Files:**
- Modify: `apps/miniapp/src/pages/student-takeout/index.tsx`
- Modify: `scripts/check-external-takeout-directory.cjs`
- Test: `scripts/check-external-takeout-directory.cjs`

- [ ] **Step 1: 写失败检查，禁止学生端服务页引用内置目录组件**

在 `scripts/check-external-takeout-directory.cjs` 中读取 `apps/miniapp/src/pages/student-takeout/index.tsx`，加入以下断言：

```js
assert.match(page, /listExternalTakeoutMerchants/)
assert.match(page, /navigateToMiniProgram/)
assert.doesNotMatch(page, /ServiceMerchantCatalog|g-takeout-buyer|merchantId/)
```

同时保留现有的 `external_takeout_merchants`、有效 AppID/路径和跳转失败提示检查。

- [ ] **Step 2: 运行检查确认当前实现失败**

运行：

```powershell
node scripts/check-external-takeout-directory.cjs
```

预期：FAIL，原因是页面当前仍包含 `ServiceMerchantCatalog` 和 `merchantId` 商品目录分支。

- [ ] **Step 3: 移除内置商品目录分支，保留外部商家列表**

在 `student-takeout/index.tsx` 中：

```tsx
// 删除 ServiceMerchantCatalog import
// 删除 merchantId 读取
// 删除 merchantId 的“商家商品”早返回分支
// 删除普通页面中的 <ServiceMerchantCatalog />
```

保留 `listExternalTakeoutMerchants()`、营业状态筛选、`Taro.navigateToMiniProgram({ appId, path })`、空态/错误态和 `StudentBottomNav active='takeout'`。页面标题文案继续说明“商品、订单与售后由商家小程序提供”。

- [ ] **Step 4: 运行外部商家检查确认通过**

运行：

```powershell
node scripts/check-external-takeout-directory.cjs
```

预期：PASS；服务页只保留外部商家目录契约。

- [ ] **Step 5: 提交入口隔离变更**

```powershell
git add apps/miniapp/src/pages/student-takeout/index.tsx scripts/check-external-takeout-directory.cjs
git commit -m "调整：服务页仅保留外部商家入口"
```

### Task 2: 恢复稳定的全局 UI 组件，解除钱包对白屏的依赖

**Files:**
- Modify: `apps/miniapp/src/components/surface-enhancements.scss`
- Modify: `apps/miniapp/src/components/refresh-scroll-view.tsx`
- Modify: `apps/miniapp/src/components/refresh-scroll-view.scss`
- Modify: `apps/miniapp/src/components/ui/app-action-sheet.tsx`
- Modify: `apps/miniapp/src/components/ui/app-action-sheet.scss`
- Modify: `apps/miniapp/src/components/ui/app-state.tsx`
- Modify: `apps/miniapp/src/components/student-bottom-nav.tsx`
- Test: `scripts/check-refresh-feedback.cjs`, `scripts/check-sheet-dismiss.cjs`, `scripts/check-miniapp-ui-system.cjs`, `scripts/test-wallet-pages.cjs`

- [ ] **Step 1: 恢复稳定 surface 规则**

将 `surface-enhancements.scss` 恢复为稳定契约：导航和刷新层使用 `var(--glass-nav, rgba(255,255,255,.94))` 与 `blur(18PX)`，弹窗和 action sheet 使用 `var(--glass-sheet, rgba(255,255,255,.96))`，刷新反馈使用 `var(--glass-refresh, rgba(255,255,255,.92))` 与 `blur(10PX)`；保留 `page .student-bottom-nav`、`page .delivery-workspace-nav`、`page .app-dialog-layer .app-dialog`、`page .app-action-sheet-layer .app-action-sheet` 的选择器，不添加透明全覆盖规则。

- [ ] **Step 2: 恢复原生刷新反馈实现**

在 `refresh-scroll-view.tsx` 中移除 `TaroUxLoading`、`taroUxBlurStyle`、`vibrateShort` 和 `blur` 状态，仅保留 `phase`、`triggered`、`generation`、`locked`、`onRefresherPulling`、`onRefresherRefresh`、`onRefresherRestore`、`onRefresherAbort`。恢复如下渲染契约：

```tsx
<ScrollView {...props}
  refresherEnabled={enabled && (!busy || locked.current)}
  refresherThreshold={60}
  refresherDefaultStyle='none'
  refresherBackground='transparent'
  refresherTriggered={triggered}
  onRefresherRefresh={() => { void refresh() }}
  onRefresherRestore={resting}
  onRefresherAbort={resting}
>
  <Slot name='refresher' className={`refresh-feedback refresh-feedback--${phase}`}>
    <StatusGlyph small kind={phase === 'refreshing' ? 'loading' : phase === 'success' ? 'success' : phase === 'error' ? 'offline' : 'pull'} />
    <Text>{labels[phase]}</Text>
  </Slot>
  {children}
</ScrollView>
```

删除 `taro-ux-refresh` 类和对应 SCSS 规则，保留原有刷新状态文字、请求锁和错误状态。

- [ ] **Step 3: 恢复原有 action sheet 拖拽与样式**

在 `app-action-sheet.tsx` 中重新使用 `useSheetDismiss`，将面板内联样式恢复为：

```tsx
const drag = useSheetDismiss(visible && phase === 'open' && !loading, handleClose)
// ...
style={drag.dragging ? { transform: `translateY(${drag.offset}px)` } : undefined}
```

在 `app-action-sheet.scss` 中恢复 `transform: translateY(...)` 的进入/退出动画、原有深色半透明遮罩与 `background: $bg-surface`；移除 `--taro-ux-sheet-bottom`、透明面板和 `blur(64px)` 覆盖。这样商家钱包和骑手钱包仍可使用原有 action sheet，不加载实验性拖拽工具。

- [ ] **Step 4: 恢复登录状态与底部导航的稳定内容**

在 `app-state.tsx` 将登录连接提示恢复为现有的 `<View className='app-state__connection-dot' />`，在 `student-bottom-nav.tsx` 删除仅用于实验 hover 的 `hoverClass='student-bottom-nav__item--pressed'`；保留 active 状态、发布按钮和导航锁。这样“我的”与其他学生页继续使用同一套底部布局。

- [ ] **Step 5: 运行组件与钱包检查**

运行：

```powershell
node scripts/check-refresh-feedback.cjs
node scripts/check-sheet-dismiss.cjs
node scripts/check-miniapp-ui-system.cjs
node scripts/test-wallet-pages.cjs
```

预期：全部 PASS；钱包测试应覆盖重新进入页面自动加载，不再要求“刷新资金记录”。

- [ ] **Step 6: 提交 UI 稳定性变更**

```powershell
git add apps/miniapp/src/components/surface-enhancements.scss apps/miniapp/src/components/refresh-scroll-view.tsx apps/miniapp/src/components/refresh-scroll-view.scss apps/miniapp/src/components/ui/app-action-sheet.tsx apps/miniapp/src/components/ui/app-action-sheet.scss apps/miniapp/src/components/ui/app-state.tsx apps/miniapp/src/components/student-bottom-nav.tsx
git commit -m "修复：恢复稳定底部导航与钱包弹窗"
```

### Task 3: 构建正常包并核对包体与真机启动入口

**Files:**
- Modify: none in source; inspect generated `apps/miniapp/dist/**`
- Test: `scripts/check-miniapp-package-size.cjs`, `npm --workspace apps/miniapp run typecheck`, `npm run build:miniapp`

- [ ] **Step 1: 类型检查**

运行：

```powershell
npm --workspace apps/miniapp run typecheck
```

预期：exit 0。

- [ ] **Step 2: 使用正常 Taro 构建**

运行：

```powershell
npm run build:miniapp
```

不要调用任何临时目录、Terser `unsafe`、`toplevel`、`drop_console`、WXSS/WXML 二次压缩脚本；`apps/miniapp/dist` 直接作为微信开发者工具的 `miniprogramRoot`。

- [ ] **Step 3: 检查主包大小和入口**

运行：

```powershell
node scripts/check-miniapp-package-size.cjs
node -e "const fs=require('fs'); const a=JSON.parse(fs.readFileSync('apps/miniapp/dist/app.json','utf8')); if(!a.pages.includes('pages/student-takeout/index')||!a.pages.includes('pages/rider-wallet/index')||!a.pages.includes('pages/merchant-wallet/index')) process.exit(1); console.log('route-contract: PASS')"
```

预期：包体小于 `2 * 1024 * 1024`，并且外部商家、骑手钱包、商家钱包路由仍存在。

- [ ] **Step 4: 执行既有业务回归**

运行：

```powershell
node scripts/check-miniapp-business-closure.cjs
node scripts/check-role-workbench-ui.cjs
node scripts/check-home-real-tools-contract.cjs
node scripts/check-private-messaging.cjs
git diff --check
```

预期：全部 exit 0；不修改后端或数据库。

### Task 4: 微信开发者工具本地预览（不上传）

**Files:**
- Inspect: `apps/miniapp/project.config.json`, `apps/miniapp/dist/app.json`
- Test: 微信开发者工具模拟器/预览日志

- [ ] **Step 1: 打开正常 dist 项目**

```powershell
& 'D:\3314\微信web开发者工具\cli.bat' open --project 'C:\Users\33141\.codex\worktrees\native-theme-motion\New project\apps\miniapp'
```

- [ ] **Step 2: 在模拟器确认启动路径**

确认首屏能渲染；依次打开“我的”、骑手钱包、商家钱包和“服务”外部商家列表。确认没有 `Page is not found`、`module is not defined`、`Cannot read properties of undefined` 或模板解析错误。

- [ ] **Step 3: 记录结果并停止**

只在本地预览和日志均通过后向用户汇报；本计划不执行微信上传、体验版覆盖、正式发布或 GitHub push。

