# 商家工作台局部精修 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 在不改变商家身份审核、配送单数据、钱包开通状态和消息路由的前提下，把商家首页收口为订单优先、窄屏可靠、层级清晰的经营工作台。

**Architecture:** 保留 `merchant-home` 现有页面、服务层与路由，只调整页面视图模型、结构类名和局部样式。订单卡片改用可随多行地址增长的双节点路线结构；数量仅做真实值的紧凑显示；所有业务写入和身份门禁继续走现有服务。

**Tech Stack:** Taro 3、React、TypeScript、SCSS、Node.js 静态契约、微信小程序构建。

---

### Task 1: 先补商家首页视觉与业务边界契约

**Files:**
- Modify: `scripts/check-merchant-home-clarity.cjs`

- [ ] 增加“绿色在线点已移除”的负向断言。
- [ ] 增加订单编号区、时间区、动态路线节点和零计数样式的结构断言。
- [ ] 保留商家四个真实入口、完整配送单编号详情路由与骑手私信角色参数的断言。
- [ ] 运行 `npm run check:merchant-home-clarity`，确认新断言在实现前失败。

### Task 2: 重组商家首页信息层级

**Files:**
- Modify: `apps/miniapp/src/pages/merchant-home/index.tsx`

- [ ] 移除商家地址前的绿色圆点，保留学校文字。
- [ ] 用真实数量生成紧凑显示值，并为零计数提供弱化状态。
- [ ] 将配送单头部拆为“配送单 / 完整编号 / 时间 / 状态”四级信息。
- [ ] 将取货点、送达点改为两个独立且可增高的路线节点，不截断真实地址和联系人。
- [ ] 保留身份审核、切换、钱包、账单、消息、分页和详情行为不变。

### Task 3: 完成 320–430px 响应式视觉精修

**Files:**
- Modify: `apps/miniapp/src/pages/merchant-home/index.scss`
- Modify: `apps/miniapp/src/assets/icons/merchant-publish.svg`

- [ ] 压缩头部、概览和快捷入口的垂直占用，统一四个快捷图标的线性视觉。
- [ ] 强化订单卡片的编号、路线、备注、费用和详情按钮层级。
- [ ] 所有主要点击区域使用不低于 `44PX` 的命中高度。
- [ ] 为长商家名、超大数量、长地址、长联系人和 320px 小屏增加可靠换行或截断规则。
- [ ] 保持底部安全区和微信胶囊避让逻辑不变。

### Task 4: 构建、回归、独立审查与白名单提交

**Files:**
- Verify only; no extra product files unless a test exposes a scoped defect.

- [ ] 运行 `npm run check:merchant-home-clarity`。
- [ ] 运行 `npm run check:role-workbench-ui`、`npm run check:miniapp-business-closure` 和 `npm run check:miniapp-capsule-layout`。
- [ ] 运行 `npm --workspace apps/miniapp run typecheck` 与 `npm run build:miniapp`。
- [ ] 运行 `npm run verify` 和 `git diff --check`。
- [ ] 对最新构建做一次独立只读审查，确认无 320px、真实数据或业务门禁回归。
- [ ] 仅对白名单文件执行 `git add`，不得使用 `git add .`，并用中文提交信息收口。
