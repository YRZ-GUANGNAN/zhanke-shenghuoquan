# 小程序回归故障修复 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 恢复首页原生滚动与翻页、帖子图片发布、真实私信连接和聊天页主页入口，不改变现有整体视觉方案。

**Architecture:** 首页改用微信原生 `ScrollView` 的触底事件，避免普通 `View + overflow-y` 在真机失效。社区帖子媒体接入现有 Module 10 核心媒体绑定并保留旧媒体回退，与二手商品的混合媒体策略一致。腾讯 IM 的登录和历史消息读取增加有界超时，页面在订阅成功后即可进入可用状态。聊天主页入口只替换错误文字图标，不调整页面结构。

**Tech Stack:** Taro 3、React、TypeScript、SCSS、Go、Gin、GORM、MySQL、Tencent Cloud Chat SDK。

---

## 文件结构

- `apps/miniapp/src/pages/home/index.tsx`：原生滚动容器与触底翻页。
- `apps/miniapp/src/pages/home/index.scss`：滚动容器高度约束。
- `apps/miniapp/src/pages/secondhand-chat/index.tsx`：主页图标与私信就绪顺序。
- `apps/miniapp/src/pages/secondhand-chat/index.scss`：圆形主页按钮和文字禁用态。
- `apps/miniapp/src/services/e-im.ts`：Tencent IM 调用超时边界。
- `backend/internal/module/community/media_adapter.go`：帖子媒体的新旧双栈适配器。
- `backend/internal/module/community/media_adapter_test.go`：核心媒体、旧媒体回退和错误映射测试。
- `backend/cmd/server/main.go`：社区服务和 outbox 接入混合媒体适配器。
- `scripts/check-miniapp-regression-contract.cjs`：小程序回归静态契约。
- `package.json`：暴露回归检查命令。

### Task 1：首页恢复真机滚动

- [x] 新增失败契约：要求首页使用 `ScrollView`、`scrollY` 和 `onScrollToLower`，并禁止 `createIntersectionObserver`。
- [x] 将 `.student-home-content` 从普通 `View` 改为原生 `ScrollView`。
- [x] 删除 IntersectionObserver 类型、effect 和 sentinel，触底时仅在“发现”、非加载中且仍有下一页时请求。
- [x] 为滚动容器增加 `height: 0`，保留现有内容和布局。

### Task 2：恢复帖子图片发布

- [x] 为社区混合媒体适配器编写核心媒体成功、旧媒体回退、冲突映射测试。
- [x] 使用 `BindingReserveInput{Purpose: post}` 绑定 Module 10 上传的图片。
- [x] 对旧图片集合回退到 legacy media，保持历史数据可发布。
- [x] 将社区创建和 outbox processor 都切换到同一混合适配器。
- [x] 运行社区适配器与 keyed create/outbox 测试。

### Task 3：私信连接不再无限等待

- [x] 新增失败契约：Tencent IM 登录、会话资料、历史消息必须经过 `withTimeout`。
- [x] 为 SDK login、conversation profile、message list、read receipt 设置明确超时。
- [x] 订阅成功即把聊天状态切到 ready，历史消息失败时显示可重试错误而不是永久“正在连接”。

### Task 4：修复聊天页主页按钮

- [x] 新增失败契约：可访问主页时必须使用 `AppIcon name='home'`。
- [x] 将“主页”文字替换为固定尺寸圆形图标；交易中状态继续使用文字胶囊。

### Task 5：验证

- [x] 运行 `npm.cmd run check:miniapp-regression-fixes`。
- [x] 运行 `npm.cmd --workspace apps/miniapp run typecheck`。
- [x] 运行 `npm.cmd --workspace apps/miniapp run build:weapp`。
- [x] 运行相关 Go 单元测试和 `go test ./internal/module/community ./cmd/server`。
- [x] 记录微信开发者工具与真机仍需执行的验收边界。
