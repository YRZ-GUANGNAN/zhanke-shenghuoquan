# 静态假页面与失效入口收口 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 让已注册小程序页面只展示真实业务数据、真实可执行入口或诚实空状态。

**Architecture:** 以 `app.config.ts` 的已注册页面为用户可达边界，删除未引用的静态夹具，移除缺少真实后端契约的交互，并让热门标签完全由现有聚合接口驱动。新增独立静态契约脚本并接入根级 `verify`，防止固定业务内容和失效入口回流。

**Tech Stack:** Taro 4、React 18、TypeScript、SCSS、Node.js 合约检查脚本。

---

### Task 1: 建立失败的静态假页面契约检查

**Files:**
- Create: `scripts/check-miniapp-no-fake-pages.cjs`
- Modify: `package.json`

- [ ] **Step 1: 写入失败检查**

检查脚本必须读取相关页面源码，并断言不存在 `FALLBACK_TAGS`、`二手私信图片发送尚未开放`、`卖家未提供微信联系方式`、`当前没有可安全清理的缓存` 和 `delivery-static-data`。

```js
assert.doesNotMatch(publishPost, /FALLBACK_TAGS/)
assert.doesNotMatch(secondhandChat, /二手私信图片发送尚未开放/)
assert.doesNotMatch(secondhandDetail, /卖家未提供微信联系方式/)
assert.doesNotMatch(settings, /当前没有可安全清理的缓存/)
assert.equal(fs.existsSync(deliveryStaticDataPath), false)
```

- [ ] **Step 2: 运行检查并确认失败**

Run: `node scripts/check-miniapp-no-fake-pages.cjs`

Expected: FAIL，指出当前固定标签或失效入口仍存在。

- [ ] **Step 3: 接入根级验证命令**

在 `package.json` 添加 `check:miniapp-no-fake-pages`，并将其加入 `verify` 的小程序检查序列。

- [ ] **Step 4: 提交测试护栏**

```powershell
git add -- scripts/check-miniapp-no-fake-pages.cjs package.json
git commit -m "测试：阻止静态假页面回流"
```

### Task 2: 让发布标签完全使用真实聚合数据

**Files:**
- Modify: `apps/miniapp/src/pages/publish-post/index.tsx`
- Modify: `apps/miniapp/src/pages/publish-post/index.scss`

- [ ] **Step 1: 删除固定热门标签**

将热门标签状态初始化为空数组，并保留现有 `listHotCommunityTags()` 加载逻辑。

```tsx
const [hotTags, setHotTags] = useState<string[]>([])
```

- [ ] **Step 2: 添加诚实空状态**

当真实接口返回空数组时显示“暂无热门标签，可在下方自定义”，仍允许用户创建合法自定义标签。

- [ ] **Step 3: 运行定向验证**

Run: `npm run check:miniapp-publish-contract && npm run typecheck`

Expected: PASS。

- [ ] **Step 4: 提交真实数据改造**

```powershell
git add -- apps/miniapp/src/pages/publish-post/index.tsx apps/miniapp/src/pages/publish-post/index.scss
git commit -m "修复：热门标签仅使用真实聚合数据"
```

### Task 3: 移除无真实能力支撑的可见入口

**Files:**
- Modify: `apps/miniapp/src/pages/secondhand-chat/index.tsx`
- Modify: `apps/miniapp/src/pages/secondhand-chat/index.scss`
- Modify: `apps/miniapp/src/pages/secondhand-detail/index.tsx`
- Modify: `apps/miniapp/src/pages/settings/index.tsx`
- Delete: `apps/miniapp/src/pages/delivery-static-data.ts`

- [ ] **Step 1: 移除二手私信图片按钮**

删除图片按钮 JSX 和专用样式，文本私信输入区保持完整宽度；只有在独立的二手私信媒体权限、上传和解析契约全部实现后才能恢复。

- [ ] **Step 2: 移除无数据来源的微信联系选项**

联系面板只保留现有平台私信入口和交易安全提示。

- [ ] **Step 3: 移除无实际效果的清理缓存行**

设置页保留检查更新、意见反馈等真实操作，不再展示固定 `0 B` 假操作。

- [ ] **Step 4: 删除静态配送订单夹具**

删除 `apps/miniapp/src/pages/delivery-static-data.ts`，不迁移其中任何订单、电话、费用或状态数据。

- [ ] **Step 5: 运行新契约与现有业务闭环检查**

Run: `npm run check:miniapp-no-fake-pages && npm run check:miniapp-business-closure && npm run check:miniapp-static-routes`

Expected: PASS。

- [ ] **Step 6: 提交入口收口**

```powershell
git add -- apps/miniapp/src/pages/secondhand-chat/index.tsx apps/miniapp/src/pages/secondhand-chat/index.scss apps/miniapp/src/pages/secondhand-detail/index.tsx apps/miniapp/src/pages/settings/index.tsx apps/miniapp/src/pages/delivery-static-data.ts
git commit -m "修复：移除无真实能力支撑的入口"
```

### Task 4: 全量验证交付边界

**Files:**
- Verify only

- [ ] **Step 1: 运行全量验证**

Run: `npm run verify`

Expected: 全部检查、前端类型检查和安全后端测试 PASS。

- [ ] **Step 2: 运行生产构建**

Run: `npm run build`

Expected: 小程序与管理后台构建 PASS；允许记录已有体积警告，但不得出现新的编译错误。

- [ ] **Step 3: 检查工作区边界**

Run: `git status --short`

Expected: 只剩用户原有未跟踪计划文件；不包含微信上传、生产配置或数据库写入。
