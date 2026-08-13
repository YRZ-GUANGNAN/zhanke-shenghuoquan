# 首页顶部导航细节优化 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 将首页“私信 / 发现 / 工具 / 搜索”业务导航改为稳定的等宽触控轨道，同时保留微信胶囊安全区和全部原有入口。

**Architecture:** 不修改共享 `CapsulePageHeader` 的系统行或动态安全区，只在首页传入的 center/right 内容和页面局部 SCSS 上优化。先更新现有静态契约使其描述目标结构，再最小化修改 JSX 与样式，最后运行回归、类型检查和本地构建。

**Tech Stack:** Taro、React、TypeScript、SCSS、Node.js 静态契约检查

---

### Task 1: 更新顶部导航静态契约

**Files:**
- Modify: `scripts/check-miniapp-capsule-layout.cjs`
- Modify: `scripts/check-home-real-tools-contract.cjs`

- [x] **Step 1: 将胶囊检查改为新导航契约**

在读取首页样式的同时读取首页 JSX，并用下列检查替换旧的伪元素几何检查：

```js
const homeSource = fs.readFileSync(path.join(miniappPagesRoot, 'home', 'index.tsx'), 'utf8')
assert.match(homeSource, /<AppIcon name='search' className='student-home-header__search-icon'\s*\/>/)
assert.match(homeSource, /<Button[\s\S]*className=\{`student-home-header__tab/)
assert.match(homeSource, /hoverClass='student-home-header__tab--pressed'/)
assert.match(homeSource, /当前栏目/)
assert.match(homeSource, /<Button[\s\S]*className='student-home-header__search'/)
assert.match(homeSource, /hoverClass='student-home-header__search--pressed'/)
assert.match(homeSource, /<Text className='student-home-header__a11y'>搜索<\/Text>/)
assert.match(homeStyle, /\.student-home-header__tabs[\s\S]*max-width:\s*432rpx/)
assert.match(homeStyle, /\.student-home-header__tab[\s\S]*flex:\s*1 1 0/)
assert.match(homeStyle, /\.student-home-header__tab[\s\S]*min-width:\s*44PX/)
assert.match(homeStyle, /\.student-home-header__search[\s\S]*min-width:\s*44PX/)
assert.match(homeStyle, /\.student-home-header__search--pressed/)
assert.doesNotMatch(homeStyle, /\.student-home-header__search::before/)
```

- [x] **Step 2: 更新首页工具检查中的连续适配约束**

将旧的 `@media (max-width: 360px)` 断言替换为：

```js
assert.match(homeStyle, /\.student-home-header__tabs[\s\S]*max-width:\s*432rpx/)
assert.doesNotMatch(homeStyle, /@media \(max-width: 360px\)/)
```

- [x] **Step 3: 运行检查并确认其先失败**

Run:

```powershell
npm run check:miniapp-capsule-layout
npm run check:home-real-tools-contract
```

Expected: 至少一个检查因首页仍使用 CSS 伪元素搜索图标或旧媒体查询而失败。

### Task 2: 实现语义化等宽导航

**Files:**
- Modify: `apps/miniapp/src/pages/home/index.tsx`
- Modify: `apps/miniapp/src/pages/home/index.scss`

- [x] **Step 1: 为标签和搜索入口补充语义与现有图标**

将标签和搜索入口改为：

```tsx
<Button
  key={tab.key}
  className={`student-home-header__tab ${activeTab === tab.key ? 'student-home-header__tab--active' : ''}`}
  hoverClass='student-home-header__tab--pressed'
  hoverStayTime={80}
  onClick={() => setActiveTab(tab.key)}
>
  <Text>{tab.label}</Text>
  {activeTab === tab.key ? <View className='student-home-header__tab-indicator' /> : null}
  {activeTab === tab.key ? <Text className='student-home-header__a11y'>，当前栏目</Text> : null}
</Button>
```

```tsx
right={(
  <Button
    className='student-home-header__search'
    hoverClass='student-home-header__search--pressed'
    hoverStayTime={80}
    onClick={openSearch}
  >
    <AppIcon name='search' className='student-home-header__search-icon' />
    <Text className='student-home-header__a11y'>搜索</Text>
  </Button>
)}
```

- [x] **Step 2: 将导航改为连续适配的等宽轨道**

使用以下关键样式替换旧的大间距、媒体查询和伪元素：

```scss
.student-home-header__tabs {
  display: flex;
  flex: 0 1 432rpx;
  align-items: stretch;
  width: 100%;
  max-width: 432rpx;
  min-width: 0;
}

.student-home-header__tab {
  flex: 1 1 0;
  min-width: 44PX;
  min-height: 44PX;
  padding: 0;
  font-weight: 600;
}

.student-home-header__tab::after {
  display: none;
  border: 0;
}

.student-home-header__tab-indicator {
  width: 32rpx;
  height: 4rpx;
}

.student-home-header__search {
  display: flex;
  align-items: center;
  justify-content: center;
  flex: 0 0 44PX;
  width: 44PX;
  min-width: 44PX;
  height: 44PX;
  min-height: 44PX;
  border-radius: 16rpx;
}

.student-home-header__search--pressed {
  background: $bg-subtle;
  transform: scale(0.96);
}

.student-home-header__search-icon {
  width: 20PX;
  height: 20PX;
}
```

- [x] **Step 3: 运行专项契约**

Run:

```powershell
npm run check:miniapp-capsule-layout
npm run check:home-real-tools-contract
npm run check:home-feed-card-contract
```

Expected: 三项全部输出 `ok` 并以 0 退出。

### Task 3: 验证与提交

**Files:**
- Verify: `apps/miniapp/src/pages/home/index.tsx`
- Verify: `apps/miniapp/src/pages/home/index.scss`
- Verify: `scripts/check-miniapp-capsule-layout.cjs`
- Verify: `scripts/check-home-real-tools-contract.cjs`

- [x] **Step 1: 运行类型检查**

Run:

```powershell
npm --workspace apps/miniapp run typecheck
```

Expected: 0 个 TypeScript 错误。

- [x] **Step 2: 运行本地小程序构建**

Run:

```powershell
$env:TARO_APP_API_BASE='http://127.0.0.1:8080/api/v1'
$env:TARO_APP_ENABLE_DELIVERY='true'
$env:TARO_APP_ENABLE_DELIVERY_PAYMENT='false'
npm run build:miniapp
```

Expected: 编译成功；允许保留仓库既有的 `vendors.js` 体积和异步分包警告。

- [x] **Step 3: 检查差异并提交**

Run:

```powershell
git diff --check
git add apps/miniapp/src/pages/home/index.tsx apps/miniapp/src/pages/home/index.scss scripts/check-miniapp-capsule-layout.cjs scripts/check-home-real-tools-contract.cjs docs/superpowers/plans/2026-08-11-home-top-navigation-polish.md
git commit -m "优化：首页顶部导航触控与对齐"
```

Expected: 仅提交本任务文件，用户已有未跟踪文档保持未修改。
