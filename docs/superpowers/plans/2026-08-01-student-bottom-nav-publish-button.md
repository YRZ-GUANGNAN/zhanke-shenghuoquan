# Student Bottom Navigation Publish Button Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Replace the oversized circular plus button in the shared student bottom navigation with a compact inline “发布” button and remove page-level style drift.

**Architecture:** Keep `StudentBottomNav` as the single implementation for all four student main pages. Encode the publish button contract in the existing UI-system check, then remove duplicated page-local navigation rules so only the shared component owns layout and interaction styling.

**Tech Stack:** React, Taro 4, TypeScript, SCSS, Node.js contract tests

---

### Task 1: Lock the shared publish-button contract

**Files:**
- Modify: `scripts/check-miniapp-ui-system.cjs`

- [ ] **Step 1: Add failing contract assertions**

Append checks that load `components/student-bottom-nav.tsx`, `components/student-bottom-nav.scss`, and the four student page styles. Require the shared component to render a text label, prohibit the old add icon, require the compact dimensions, and prohibit page-level ownership:

```js
const studentBottomNavSource = fs.readFileSync(
  path.join(root, 'apps', 'miniapp', 'src', 'components', 'student-bottom-nav.tsx'),
  'utf8',
)
const studentBottomNavStyle = fs.readFileSync(
  path.join(root, 'apps', 'miniapp', 'src', 'components', 'student-bottom-nav.scss'),
  'utf8',
)

assert.match(studentBottomNavSource, /student-bottom-nav__publish/)
assert.match(studentBottomNavSource, />发布<\/Text>/)
assert.doesNotMatch(studentBottomNavSource, /AppIcon name='add'/)
assert.match(studentBottomNavStyle, /\.student-bottom-nav__publish[\s\S]*width:\s*84rpx/)
assert.match(studentBottomNavStyle, /\.student-bottom-nav__publish[\s\S]*height:\s*64rpx/)
assert.doesNotMatch(studentBottomNavStyle, /\.student-bottom-nav__publish[\s\S]*border-radius:\s*50%/)

for (const pageName of ['home', 'profile', 'student-secondhand', 'student-takeout']) {
  const pageStyle = fs.readFileSync(path.join(pagesRoot, pageName, 'index.scss'), 'utf8')
  assert.doesNotMatch(
    pageStyle,
    /\.student-bottom-nav(?:--|__|\s*\{)/,
    `${pageName} must not override the shared student bottom navigation`,
  )
}
```

- [ ] **Step 2: Run the contract and verify failure**

Run:

```powershell
npm run check:miniapp-ui-system
```

Expected: FAIL because the component still renders `AppIcon name='add'`, the shared style still uses a `96rpx` circle, and page styles still contain duplicate navigation selectors.

### Task 2: Implement the compact shared publish entry

**Files:**
- Modify: `apps/miniapp/src/components/student-bottom-nav.tsx`
- Modify: `apps/miniapp/src/components/student-bottom-nav.scss`

- [ ] **Step 1: Replace the icon-only FAB markup**

Replace the central block with a semantic text action while preserving its route:

```tsx
<View
  className='student-bottom-nav__publish'
  onClick={() => void Taro.navigateTo({ url: '/pages/publish-entry/index' })}
>
  <Text className='student-bottom-nav__publish-label'>发布</Text>
</View>
```

- [ ] **Step 2: Replace circular FAB styles**

Use a compact inline control owned entirely by the shared component:

```scss
.student-bottom-nav__publish {
  align-self: end;
  justify-self: center;
  display: flex;
  align-items: center;
  justify-content: center;
  width: 84rpx;
  height: 64rpx;
  margin-bottom: 4rpx;
  border: 1rpx solid rgba($brand-primary, .12);
  border-radius: 18rpx;
  background: $brand-tint;
  box-shadow: 0 8rpx 20rpx -14rpx rgba(33, 104, 184, .35);
  box-sizing: border-box;
  color: $brand-primary-strong;
  transition: transform $duration-fast $ease-standard,
    background-color $duration-fast $ease-standard;
}

.student-bottom-nav__publish:active {
  background: #D8EAF9;
  transform: scale(.97);
}

.student-bottom-nav__publish-label {
  font-size: 23rpx;
  font-weight: 700;
  letter-spacing: 2rpx;
  line-height: 1;
}
```

- [ ] **Step 3: Run the contract and observe the remaining page-override failure**

Run:

```powershell
npm run check:miniapp-ui-system
```

Expected: FAIL only on page-local `.student-bottom-nav*` selectors.

### Task 3: Remove page-level navigation style drift

**Files:**
- Modify: `apps/miniapp/src/pages/home/index.scss:489-631`
- Modify: `apps/miniapp/src/pages/profile/index.scss:443-585`
- Modify: `apps/miniapp/src/pages/student-secondhand/index.scss:368-510`
- Modify: `apps/miniapp/src/pages/student-takeout/index.scss:284-426`

- [ ] **Step 1: Delete duplicated navigation blocks**

Delete each page’s trailing `.student-bottom-nav`, `.student-bottom-nav--secondhand`, `.student-bottom-nav__item`, legacy pseudo-icon, active-state, and `.student-bottom-nav__fab` rules. Do not modify page-specific content styles above those blocks.

- [ ] **Step 2: Run focused checks**

Run:

```powershell
npm run check:miniapp-ui-system
npm run check:miniapp-capsule-layout
npm --workspace apps/miniapp run typecheck
```

Expected: all commands PASS.

- [ ] **Step 3: Build the WeChat miniapp**

Run:

```powershell
npm --workspace apps/miniapp run build:weapp
```

Expected: build succeeds. The existing `vendors.js` size and async-chunk recommendations may remain as non-blocking warnings.

- [ ] **Step 4: Check diff hygiene**

Run:

```powershell
git diff --check
git status --short
```

Expected: only the implementation plan, shared navigation component/styles, UI contract, and four page SCSS files are changed; the three existing user-owned untracked plan files remain untouched.

- [ ] **Step 5: Commit the implementation**

```powershell
git add -- scripts/check-miniapp-ui-system.cjs apps/miniapp/src/components/student-bottom-nav.tsx apps/miniapp/src/components/student-bottom-nav.scss apps/miniapp/src/pages/home/index.scss apps/miniapp/src/pages/profile/index.scss apps/miniapp/src/pages/student-secondhand/index.scss apps/miniapp/src/pages/student-takeout/index.scss
git commit -m "优化：轻量化学生端发布入口"
```
