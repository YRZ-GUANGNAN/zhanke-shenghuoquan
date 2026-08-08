# 私信与二手交易界面一致性 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 统一私信、二手集市、用户主页二手内容和设置页的视觉与交互，同时保持现有真实业务契约。

**Architecture:** 在现有 Taro React 页面和 SCSS 上做定向重构，不迁移框架、不新增依赖。用一份静态契约脚本保护真实接口、关键路由和交互入口，再逐页调整组件层级和样式。

**Tech Stack:** Taro 3、React 18、TypeScript、SCSS、Node.js 契约检查

---

### Task 1: 建立视觉与业务契约

**Files:**
- Create: `scripts/check-secondhand-messaging-ui.cjs`
- Modify: `package.json`

- [ ] **Step 1: 写入失败契约**

检查四个页面仍包含真实服务调用、消息订阅、商品分页、关注、手机号授权、设置 PATCH、重试与底部导航，并检查新视觉类名。

- [ ] **Step 2: 运行契约确认视觉类名尚未全部满足**

Run: `node scripts/check-secondhand-messaging-ui.cjs`
Expected: FAIL，指出尚未实现的统一视觉结构。

- [ ] **Step 3: 将契约加入完整 verify**

在 `package.json` 增加 `check:secondhand-messaging-ui`，并接入 `verify`。

### Task 2: 精修私信会话

**Files:**
- Modify: `apps/miniapp/src/pages/secondhand-chat/index.tsx`
- Modify: `apps/miniapp/src/pages/secondhand-chat/index.scss`

- [ ] **Step 1: 重组顶部与商品摘要**

保留 `contactSeller`、`getIMConversation`、`loadTencentMessages`、`sendTencentText` 和 `subscribeTencentMessages`，只调整可见信息与层级。

- [ ] **Step 2: 重组消息空态和输入区**

保留键盘高度计算、锚点滚动和图片入口，增加可读的禁用态、按压态和会话提示。

- [ ] **Step 3: 运行类型检查与契约**

Run: `npm.cmd --workspace apps/miniapp run typecheck && npm.cmd run check:secondhand-messaging-ui`
Expected: PASS。

### Task 3: 精修二手集市

**Files:**
- Modify: `apps/miniapp/src/pages/student-secondhand/index.tsx`
- Modify: `apps/miniapp/src/pages/student-secondhand/index.scss`

- [ ] **Step 1: 固定商品媒体比例与信息顺序**

移除基于商品 ID 的随机高度，只保留稳定的真实图片或分类占位；卡片继续使用真实分页结果。

- [ ] **Step 2: 压缩标题、筛选与安全提示**

搜索和分类保持可点击，空态、失败重试、加载更多和到底状态保持可用。

- [ ] **Step 3: 验证二手业务契约**

Run: `npm.cmd run check:miniapp-business-closure && npm.cmd run check:secondhand-comments-contract`
Expected: PASS。

### Task 4: 统一用户主页二手展示

**Files:**
- Modify: `apps/miniapp/src/pages/user-profile/index.tsx`
- Modify: `apps/miniapp/src/pages/user-profile/index.scss`

- [ ] **Step 1: 重排身份摘要、统计与动作**

保留关注、分享、举报、拉黑和二手标签切换；自己的主页隐藏关注动作。

- [ ] **Step 2: 对齐商品卡比例**

真实商品图、价格、状态和分页继续来自现有接口。

- [ ] **Step 3: 运行类型检查**

Run: `npm.cmd --workspace apps/miniapp run typecheck`
Expected: PASS。

### Task 5: 精修设置页

**Files:**
- Modify: `apps/miniapp/src/pages/settings/index.tsx`
- Modify: `apps/miniapp/src/pages/settings/index.scss`

- [ ] **Step 1: 调整分组和账号摘要**

保留 `getPhoneNumber`、通知开关、个性化推荐、黑名单、反馈、关于、注销和退出。

- [ ] **Step 2: 强化保存反馈和失败回滚**

继续使用现有 `updateUserSettings`，禁止仅在本地模拟成功。

- [ ] **Step 3: 验证设置契约**

Run: `npm.cmd run check:miniapp-settings-contract`
Expected: PASS。

### Task 6: 完整回归与提交

**Files:**
- Test: `scripts/check-secondhand-messaging-ui.cjs`

- [ ] **Step 1: 运行完整检查**

Run: `npm.cmd run verify`
Expected: PASS。

- [ ] **Step 2: 运行生产构建**

Run: `npm.cmd run build`
Expected: miniapp 与 admin 均构建成功；仅允许既有非阻断包体积提示。

- [ ] **Step 3: 检查差异并提交**

只提交本计划、设计、契约和四组页面文件，不加入用户已有的未跟踪文档。

