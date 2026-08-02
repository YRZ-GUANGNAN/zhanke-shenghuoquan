# 个人中心信息层级优化 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan.

**Goal:** 将学生端“我的”页重构为紧凑、真实、状态清晰的个人中心，并保持全部现有业务行为。

**Architecture:** 仅在 Profile 页面展示层增加头像与身份状态映射，数据继续来自 session store、`getMyCommunitySummary()` 与 `getMyProductsPage()`；身份点击继续复用现有审核门禁和 `IdentitySwitchModal`。通过静态契约锁定真实头像、状态文案、统计路由与设置路由。

**Tech Stack:** React、Taro、TypeScript、SCSS、Node.js 静态契约脚本

---

## 任务一：建立个人中心静态契约

**Files:**
- Create: `scripts/check-profile-center-ui.cjs`
- Modify: `package.json`

1. 断言 Profile 使用真实 `profile.avatar`，无头像时保留首字回退。
2. 断言四项内容统计仍指向 `my-content` 对应 Tab。
3. 断言身份入口展示状态文案并保留申请、切换与门禁逻辑。
4. 断言四条账户与支持路由未丢失。
5. 将检查接入现有 `verify` 链路或独立可执行脚本。

## 任务二：重构 Profile 信息结构

**Files:**
- Modify: `apps/miniapp/src/pages/profile/index.tsx`

1. 引入真实头像渲染与昵称首字回退。
2. 增加学校缺省文案与当前身份标签。
3. 将统计、身份、设置整理为三个明确分组。
4. 基于 `IdentityAvailability` 派生身份状态标签，但不改变原点击行为。
5. 保留编辑资料、内容统计、身份申请／切换、设置与底部导航的全部路由。

## 任务三：重做 Profile 视觉层级

**Files:**
- Modify: `apps/miniapp/src/pages/profile/index.scss`

1. 删除渐变头部、装饰圆形和悬浮重叠布局。
2. 建立白色资料头、浅色内容卡、紧凑身份卡和统一设置列表。
3. 为头像、状态标签、点击态、长昵称和小屏宽度补充约束。
4. 保留胶囊安全行与共享 `StudentBottomNav`，不在页面覆盖底栏样式。

## 任务四：验证与候选包

1. 运行个人中心静态契约。
2. 运行 miniapp TypeScript 检查与仓库全量 `npm run verify`。
3. 运行 `npm run build:miniapp`，确认没有新增告警。
4. 将生产构建复制到独立候选目录，逐文件校验哈希并生成 ZIP 与 SHA-256。
5. 不执行微信上传，仅交付给用户最终审查。
