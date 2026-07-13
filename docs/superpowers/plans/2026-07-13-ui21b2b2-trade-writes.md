# UI-21b-2b-2 Trade Writes Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 在不修改后端、API 契约和依赖的前提下，把二手详情收藏/联系卖家与二手发布从静态占位接到冻结的真实接口，并保证目标态写入、幂等、整数分金额和真实媒体绑定。

**Architecture:** 保持现有 Taro + React 页面结构。`services/c-trade.ts` 负责冻结契约、幂等键和 viewer-state；`pages/secondhand-detail` 只负责目标态交互与跳转现有私信页；`pages/publish-secondhand` 负责表单、整数分换算、媒体上传和稳定请求键。所有写后状态以后端回读为准，网络结果不确定时不在前端猜测成功。

**Tech Stack:** Taro、React、TypeScript、现有 `request` 封装、现有 media 上传链路、现有二手私信页。

---

## 0. Scope and non-negotiable constraints

- [ ] 起点必须含 `ui21b2b1-community-writes-baseline`，先完成社区写交互再开始本计划。
- [ ] 仅修改：
  - `apps/miniapp/src/services/c-trade.ts`
  - `apps/miniapp/src/pages/secondhand-detail/**`
  - `apps/miniapp/src/pages/publish-secondhand/**`
  - 与本模块直接相关的前端测试/静态检查文件
  - `docs/UI-21B-2B-2-TRADE-WRITES-REPORT.md`
- [ ] 不修改 `backend/**`、`apps/admin/**`、迁移、API 路由、请求字段、响应信封、`package.json` 或依赖锁文件。
- [ ] 不新增前端测试框架；使用现有 TypeScript、静态路由、边界、构建和真实环境验收。
- [ ] 所有金额使用整数分提交，不把浮点数传给后端。
- [ ] 每个逻辑提交完成后运行对应验证；最终提交报告并保持 worktree 干净。

## Task 1: Gate checks and isolated worktree

**Files:** None.

- [ ] 在主仓库执行：

```powershell
git log --oneline -1 master
git tag --list ui21b2b1-community-writes-baseline
git status --short
Get-PSDrive C
```

预期：master 含社区写交互冻结提交；标签存在；工作区干净；磁盘空间足够。

- [ ] 创建隔离 worktree：

```powershell
git worktree add ..\ui21b2b2-trade-writes -b feat/ui21b2b2-trade-writes master
```

- [ ] 在新 worktree 再确认：

```powershell
git status --short
git branch --show-current
git rev-parse --short HEAD
```

预期：分支为 `feat/ui21b2b2-trade-writes`，状态干净。

## Task 2: Freeze the trade service contract

**Files:**
- Modify: `apps/miniapp/src/services/c-trade.ts`

- [ ] 保留现有读取接口和 `IMConversationContext` 对外字段，不改调用方已使用的返回结构。
- [ ] 增加 viewer-state 类型与读取函数：

```ts
export interface TradeViewerState {
  favorited: boolean
  contactedSeller: boolean
}

export function getTradeViewerState(productId: string) {
  return request<TradeViewerState>({
    url: `/api/v1/trade/products/${productId}/viewer-state`,
    method: 'GET',
  })
}
```

- [ ] 把收藏改为目标态接口，删除历史 toggle/POST 语义：

```ts
export function setProductFavorited(productId: string, favorited: boolean) {
  return request<void>({
    url: `/api/v1/trade/products/${productId}/favorite`,
    method: favorited ? 'PUT' : 'DELETE',
  })
}
```

- [ ] 把联系卖家接到冻结的 `PUT /api/v1/trade/products/:id/contact`，返回既有 `IMConversationContext`。
- [ ] 明确定义发布请求，字段与后端完全一致：

```ts
export interface CreateTradeProductInput {
  clientRequestId: string
  title: string
  description: string
  category: string
  price: number
  originalPrice: number
  condition: string
  campus: string
  imageAssetIds: string[]
}
```

- [ ] 创建请求同时携带相同幂等键：

```ts
export function createTradeProduct(input: CreateTradeProductInput) {
  return request<TradeProduct>({
    url: '/api/v1/trade/products',
    method: 'POST',
    header: { 'Idempotency-Key': input.clientRequestId },
    data: input,
  })
}
```

- [ ] 增加生成客户端请求键的纯函数；同一份草稿重试必须复用该值，成功或重置草稿后才生成新值。
- [ ] 运行：

```powershell
npm --prefix apps/miniapp run typecheck
git diff --check
```

- [ ] 提交：

```powershell
git add apps/miniapp/src/services/c-trade.ts
git commit -m "feat(UI-21b-2b-2): 固化二手真实写契约"
```

## Task 3: Connect target-state favorite and contact on product detail

**Files:**
- Modify: `apps/miniapp/src/pages/secondhand-detail/index.tsx`
- Modify only when required by current page structure: `apps/miniapp/src/pages/secondhand-detail/index.scss`

- [ ] 页面加载商品详情后并行读取 viewer-state；未登录或读取失败时显示明确状态，不伪造未收藏。
- [ ] 收藏按钮使用 `desiredFavorited` 目标态：
  1. 记录用户最后一次期望状态；
  2. 同一时刻只允许一个收藏请求在途；
  3. 请求完成后若目标又变化，继续写最后目标；
  4. 写入失败或响应不确定时重新读取 viewer-state；
  5. UI 最终以后端回读为准。
- [ ] 禁止保留“收藏功能下一阶段接入”等静态提示。
- [ ] 联系卖家按钮调用 `contactSeller(productId)`；调用期间禁用重复点击并显示 loading。
- [ ] 成功后使用后端返回的 `relationId`、`conversationId`、`peerUserId`、`productId`、`buyerId`、`sellerId` 导航到现有 `pages/secondhand-chat/index`，所有 query 值使用 `encodeURIComponent`。
- [ ] 重复联系同一商品必须复用后端返回的稳定会话，不在前端生成会话 ID。
- [ ] 联系失败时保留当前页面并显示真实错误；不得弹出“已联系”或跳入伪造会话。
- [ ] 运行：

```powershell
npm --prefix apps/miniapp run typecheck
npm run check:miniapp-static-routes
git diff --check
```

- [ ] 提交：

```powershell
git add apps/miniapp/src/pages/secondhand-detail
git commit -m "feat(UI-21b-2b-2): 接通二手收藏与联系卖家"
```

## Task 4: Replace the static second-hand publisher with a real idempotent flow

**Files:**
- Modify: `apps/miniapp/src/pages/publish-secondhand/index.tsx`
- Modify only when required by current page structure: `apps/miniapp/src/pages/publish-secondhand/index.scss`

- [ ] 移除“静态演示，暂未提交数据”分支，保留现有 UI 视觉结构。
- [ ] 表单至少收集并校验：`title`、`description`、`category`、`price`、`originalPrice`、`condition`、`campus`。
- [ ] 使用严格的元转分函数，不允许指数、负数、超过两位小数或非安全整数：

```ts
export function parseYuanToCents(value: string): number | null {
  const normalized = value.trim()
  if (!/^(0|[1-9]\d*)(\.\d{1,2})?$/.test(normalized)) return null

  const [yuan, fraction = ''] = normalized.split('.')
  const cents = Number(yuan) * 100 + Number(fraction.padEnd(2, '0'))
  return Number.isSafeInteger(cents) ? cents : null
}
```

- [ ] `price` 必须大于 0；`originalPrice` 为空时按冻结契约提交 0，非空时同样严格转为整数分。
- [ ] 图片选择改用现有真实媒体链路：

```ts
const uploaded = await chooseAndUploadImages(
  'product_image',
  remainingCount,
  setUploadProgress,
)
```

- [ ] 等待每个 asset 审核完成后再允许发布；提交 `uploaded.map(item => item.assetPublicId)`，禁止发送临时文件路径或空占位数组。
- [ ] 页面初始化时生成 `clientRequestId` 并存入当前草稿状态；网络失败、超时、用户点击重试时复用同一个值。
- [ ] 只有后端确认创建成功后，才清空草稿、生成新的请求键并跳转商品详情或列表。
- [ ] 提交期间禁用发布按钮；错误时保留文字、分类、价格和已上传图片，方便原请求重试。
- [ ] 运行：

```powershell
npm --prefix apps/miniapp run typecheck
npm run check:miniapp-static-routes
git diff --check
```

- [ ] 提交：

```powershell
git add apps/miniapp/src/pages/publish-secondhand
git commit -m "feat(UI-21b-2b-2): 接通二手幂等发布与媒体绑定"
```

## Task 5: Real-environment acceptance

**Files:** No committed temporary files.

- [ ] 启动真实后端、真实 MySQL 和小程序开发态；不得 mock HTTP、绕过路由或直接改库制造成功结果。
- [ ] 验证收藏：
  - 未收藏 → 收藏，刷新后仍为收藏；
  - 已收藏 → 取消，刷新后仍为未收藏；
  - 快速连点时最终状态等于最后一次意图；
  - 网络结果不确定后能通过 viewer-state 收敛。
- [ ] 验证联系卖家：
  - 首次联系返回真实 `IMConversationContext` 并进入现有聊天页；
  - 再次联系同一商品返回同一稳定关系/会话；
  - 越权或商品不存在时不泄露会话信息。
- [ ] 验证二手发布：
  - 上传真实图片并取得 `product_image` asset public id；
  - 数据库中的金额为整数分；
  - 请求 header `Idempotency-Key` 与 body `clientRequestId` 相同；
  - 重放同一请求只生成一个商品；
  - 非法价格在前端阻止，后端资金字段不被浮点污染；
  - 网络失败后草稿、asset id 和请求键仍保留。
- [ ] 清理验收数据并关闭所起进程，确认无临时入口、日志和测试残留。

## Task 6: Full gates, report, and handoff

**Files:**
- Create: `docs/UI-21B-2B-2-TRADE-WRITES-REPORT.md`

- [ ] 运行完整门禁：

```powershell
npm --prefix apps/miniapp run typecheck
npm run check:miniapp-static-routes
npm run check:boundaries
npm run build:miniapp
npm run verify
npm run build
git diff master...HEAD -- backend apps/admin package.json package-lock.json
git diff --check
```

预期：全部 PASS；最后一个范围 diff 为空。

- [ ] 报告记录：
  - 基线和 worktree；
  - 冻结 API 契约；
  - 收藏目标态与 viewer-state 收敛证据；
  - 联系卖家稳定会话证据；
  - 发布幂等、整数分和真实 media asset 证据；
  - 真实 HTTP + MySQL 验收；
  - Boundary、Build、Residue；
  - 最终 commit hash。
- [ ] 提交报告：

```powershell
git add docs/UI-21B-2B-2-TRADE-WRITES-REPORT.md
git commit -m "docs(UI-21b-2b-2): 记录二手真实写交互验收"
```

- [ ] 最终确认：

```powershell
git status --short
git log --oneline --decorate -5
git diff --name-status master...HEAD
```

预期：worktree 干净；改动只在授权前端文件和报告范围内。

- [ ] 停止等待 owner 验收。owner 在主仓库串行 `git merge --ff-only feat/ui21b2b2-trade-writes`，验收通过后打 `ui21b2b2-trade-writes-baseline`。不得自动进入下一模块。
