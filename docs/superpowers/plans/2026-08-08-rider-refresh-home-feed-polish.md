# 骑手刷新与首页信息流精修 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 在保留真实数据契约和现有业务能力的前提下，为骑手工作台加入原生下拉刷新，并把首页发现流改成真实媒体双列卡与无图文字卡混排。

**Architecture:** 骑手页继续使用单一订单加载函数，`ScrollView` refresher 只负责触发并显示刷新状态；业务操作仍调用现有订单服务。首页继续合并社区与二手分页数据，但渲染层按媒体真实性分组，无图帖子不再进入图片瀑布流。

**Tech Stack:** React 18、Taro 4.2、TypeScript、SCSS、Node.js 静态契约检查

---

## 文件结构

- 修改 `apps/miniapp/src/pages/delivery-rider/index.tsx`：下拉刷新状态、更新时间和订单工作台结构。
- 修改 `apps/miniapp/src/pages/delivery-rider/index.scss`：统计区、刷新提示、订单区域和空状态视觉。
- 修改 `apps/miniapp/src/pages/home/index.tsx`：发现卡数据语义与混合信息流渲染。
- 修改 `apps/miniapp/src/pages/home/index.scss`：媒体卡、文字卡、状态卡和信息层级。
- 修改 `scripts/check-rider-workbench-clarity.cjs`：把旧手动刷新按钮契约替换为原生 refresher 契约。
- 创建 `scripts/check-home-feed-card-contract.cjs`：约束无图帖子与真实媒体卡的诚实呈现。
- 修改 `package.json`：注册首页信息流契约检查并纳入 `verify`。

### Task 1: 固化骑手原生刷新契约

**Files:**
- Modify: `scripts/check-rider-workbench-clarity.cjs`
- Modify: `apps/miniapp/src/pages/delivery-rider/index.tsx`
- Modify: `apps/miniapp/src/pages/delivery-rider/index.scss`

- [ ] **Step 1: 将测试改为要求原生 refresher**

在 `check-rider-workbench-clarity.cjs` 中要求以下源码契约：

```js
assert.match(source, /refresherEnabled/)
assert.match(source, /refresherTriggered=\{refreshing\}/)
assert.match(source, /onRefresherRefresh=\{\(\) => void handlePullDownRefresh\(\)\}/)
assert.match(source, /rider-grab-refresh-hint/)
assert.doesNotMatch(source, /className=\{`rider-grab-refresh/)
assert.doesNotMatch(style, /\.rider-grab-refresh\s*\{/)
```

- [ ] **Step 2: 运行契约并确认失败**

Run: `node scripts/check-rider-workbench-clarity.cjs`

Expected: FAIL，提示缺少 `refresherEnabled` 或仍存在 `rider-grab-refresh`。

- [ ] **Step 3: 实现刷新状态与更新时间**

在骑手页面增加：

```tsx
const [refreshing, setRefreshing] = useState(false)
const [lastUpdatedAt, setLastUpdatedAt] = useState<Date | null>(null)

const handlePullDownRefresh = async () => {
  if (refreshing || loading) return
  setRefreshing(true)
  try {
    await load()
  } finally {
    setRefreshing(false)
  }
}
```

成功写入订单数据后设置 `setLastUpdatedAt(new Date())`，并在 `ScrollView` 上配置 `refresherEnabled`、`refresherTriggered={refreshing}` 和 `onRefresherRefresh`。删除统计卡内手动刷新按钮，加入只读状态行“下拉刷新订单 · 刚刚更新/HH:mm 更新”。

- [ ] **Step 4: 收敛骑手样式**

统计区保留三列数字层级，删除 `.rider-grab-refresh*` 样式，新增 `.rider-grab-refresh-hint`；刷新提示使用小号中性色文字，不制造新的悬浮按钮。保持 `ScrollView` 本身无 padding，内容 padding 继续放在 `rider-grab-content__inner`。

- [ ] **Step 5: 运行骑手契约**

Run: `node scripts/check-rider-workbench-clarity.cjs && node scripts/check-role-workbench-ui.cjs`

Expected: 输出 `rider workbench clarity contract: ok` 和 `role workbench ui contract: ok`。

### Task 2: 固化首页混合信息流契约

**Files:**
- Create: `scripts/check-home-feed-card-contract.cjs`
- Modify: `package.json`

- [ ] **Step 1: 创建失败的首页卡片契约**

契约读取首页 TSX/SCSS，并要求：

```js
assert.match(source, /kind:\s*'post'\s*\|\s*'product'/)
assert.match(source, /student-text-card/)
assert.match(source, /student-media-grid/)
assert.match(source, /card\.kind === 'post' && !card\.imageUrl/)
assert.doesNotMatch(source, /const heights:/)
assert.doesNotMatch(style, /\$placeholder-stripe/)
assert.match(style, /aspect-ratio:\s*4\s*\/\s*5/)
```

- [ ] **Step 2: 注册脚本并确认失败**

在根 `package.json` 增加：

```json
"check:home-feed-card-contract": "node scripts/check-home-feed-card-contract.cjs"
```

并把它加入 `verify`。运行 `npm run check:home-feed-card-contract`，Expected: FAIL，提示旧首页仍使用随机高度或条纹占位。

### Task 3: 实现首页真实媒体与无图文字混排

**Files:**
- Modify: `apps/miniapp/src/pages/home/index.tsx`
- Modify: `apps/miniapp/src/pages/home/index.scss`

- [ ] **Step 1: 收敛卡片类型**

将 `DiscoverCard` 改为：

```ts
type DiscoverCard = {
  id: string
  target: 'post' | 'secondhand'
  kind: 'post' | 'product'
  tone: 'sky' | 'mint' | 'warm'
  mediaLabel: string
  title: string
  author: string
  avatar: string
  likes: number
  imageUrl?: string
}
```

删除随机 `heights`，只保留稳定色调用于头像和诚实无图商品面。

- [ ] **Step 2: 分组内容并保持交错顺序**

在合并后的 `cards` 上派生：

```ts
const textCards = cards.filter((card) => card.kind === 'post' && !card.imageUrl)
const mediaCards = cards.filter((card) => card.kind === 'product' || Boolean(card.imageUrl))
const mediaColumns = splitCards(mediaCards)
```

无图帖子进入全宽文字区；带图帖子、带图商品和无图商品进入媒体区。图片加载失败后根据 `failedImageKeys` 使用“暂无图片”，不得生成装饰图片。

- [ ] **Step 3: 实现两种卡片组件**

新增 `renderTextCard` 与 `renderMediaCard`。文字卡显示类型标签、最多三行标题、作者和互动数；媒体卡固定 4:5 媒体比例并显示标题、作者、互动数。两个卡片继续使用原 `openCard()` 路由。

- [ ] **Step 4: 更新视觉状态**

加载/空/错误状态改为 `.student-discover-state` 全宽卡；部分源失败保持可点击重试。删除条纹假媒体和随机高低卡样式，统一圆角、细边框、轻阴影和 20rpx 网格间距。

- [ ] **Step 5: 运行首页契约**

Run: `npm run check:home-feed-card-contract && npm run check:miniapp-regression-fixes && npm run check:miniapp-no-fake-pages`

Expected: 所有命令退出码为 0。

### Task 4: 完整验证与提交

**Files:**
- Test: `apps/miniapp/src/pages/delivery-rider/index.tsx`
- Test: `apps/miniapp/src/pages/home/index.tsx`
- Test: `scripts/check-rider-workbench-clarity.cjs`
- Test: `scripts/check-home-feed-card-contract.cjs`

- [ ] **Step 1: 类型检查**

Run: `npm --workspace apps/miniapp run typecheck`

Expected: TypeScript 退出码为 0。

- [ ] **Step 2: 静态业务回归**

Run: `npm run check:rider-workbench-clarity && npm run check:home-feed-card-contract && npm run check:role-workbench-ui && npm run check:miniapp-regression-fixes && npm run check:miniapp-ui-system && npm run check:miniapp-no-fake-pages`

Expected: 六组契约检查全部 PASS。

- [ ] **Step 3: 生产构建**

Run: `npm --workspace apps/miniapp run build:weapp`

Expected: 微信小程序构建成功，`apps/miniapp/dist/app.json` 存在。

- [ ] **Step 4: 审核 Git 差异**

Run: `git diff --check && git status --short`

Expected: 无空白错误；仅出现本计划列出的代码、测试和文档文件，以及原先用户自己的未跟踪文件。

- [ ] **Step 5: 提交实现**

```powershell
git add -- apps/miniapp/src/pages/delivery-rider/index.tsx apps/miniapp/src/pages/delivery-rider/index.scss apps/miniapp/src/pages/home/index.tsx apps/miniapp/src/pages/home/index.scss scripts/check-rider-workbench-clarity.cjs scripts/check-home-feed-card-contract.cjs package.json docs/superpowers/plans/2026-08-08-rider-refresh-home-feed-polish.md
git commit -m "优化：重整骑手刷新与首页信息流"
```
