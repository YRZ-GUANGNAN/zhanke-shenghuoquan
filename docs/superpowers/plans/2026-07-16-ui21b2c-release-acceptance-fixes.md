# UI-21b-2c Release Acceptance Fixes Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 修复真实微信开发者工具验收发现的商品图片相对 URL 失效和卖家查看自己商品时 `viewer-state` 返回 500 两个上线阻塞。

**Architecture:** 小程序在交易 DTO 归一化边界把根相对媒体 URL 解析到现有 API base 的 origin，绝对 URL 和微信本地协议保持原样。交易服务在完成真实收藏状态读取后，对卖家查看自己商品的场景直接返回 `contactedSeller=false`，不进入 IM 自联系查询；API、数据库和审核链路均不变。

**Tech Stack:** Taro 4、TypeScript、React、Go、Gin、现有 Trade/IM gateway 单元测试。

---

## 文件映射

| 文件 | 职责 |
| --- | --- |
| `apps/miniapp/src/services/resource-url.ts` | 纯函数：只解析根相对资源 URL，保留旧绝对 URL 和特殊协议。 |
| `apps/miniapp/src/services/api.ts` | 导出既有 `API_BASE`，不改变请求行为。 |
| `apps/miniapp/src/services/c-trade.ts` | 在 `normalizeProduct` 集中规范化商品图片 URL。 |
| `backend/internal/module/trade/service.go` | 卖家查看自己商品时跳过 IM 自联系查询。 |
| `backend/internal/module/trade/service_test.go` | 记录会话查询次数并覆盖卖家 viewer-state 回归。 |
| `docs/UI-21B-2C-TRADE-WRITES-ACCEPTANCE.md` | 补记 GUI 证据、修复验证和腾讯云漫游查询限制。 |

### Task 1: 兼容根相对商品图片 URL

**Files:**
- Create: `apps/miniapp/src/services/resource-url.ts`
- Modify: `apps/miniapp/src/services/api.ts`
- Modify: `apps/miniapp/src/services/c-trade.ts`

- [ ] **Step 1: 运行缺失模块检查，确认修复前为红**

Run:

```powershell
Test-Path apps/miniapp/src/services/resource-url.ts
```

Expected: 输出 `False`。

- [ ] **Step 2: 新增无运行时依赖的 URL 解析纯函数**

```ts
const HTTP_ORIGIN = /^(https?:\/\/[^/]+)/i

export function resolveApiResourceUrl(value: string, apiBase: string): string {
  const candidate = value.trim()
  if (!candidate.startsWith('/') || candidate.startsWith('//')) return value
  const origin = apiBase.match(HTTP_ORIGIN)?.[1]
  return origin ? `${origin}${candidate}` : value
}
```

该函数只转换 `/api/v1/...` 一类根相对值；`http://`、`https://`、`//cdn...`、`wxfile://`、`data:`、`blob:`、空字符串和非根相对值均原样返回。

- [ ] **Step 3: 导出 API base 并接入交易 DTO 归一化**

在 `api.ts` 中把常量改为：

```ts
export const API_BASE = (
  typeof TARO_APP_API_BASE !== 'undefined' && TARO_APP_API_BASE
    ? TARO_APP_API_BASE
    : LOCAL_API_BASE
).replace(/\/$/, '')
```

在 `c-trade.ts` 中导入并使用解析器：

```ts
import { API_BASE, request } from './api'
import { resolveApiResourceUrl } from './resource-url'

images: (item.images ?? []).map((value) => resolveApiResourceUrl(value.url, API_BASE))
```

- [ ] **Step 4: 编译纯函数并运行表驱动断言**

Run:

```powershell
Remove-Item -LiteralPath C:\tmp\ui21b2c-url-test -Recurse -Force -ErrorAction SilentlyContinue
npx tsc apps/miniapp/src/services/resource-url.ts --target ES2020 --module commonjs --outDir C:\tmp\ui21b2c-url-test --skipLibCheck
node -e "const {resolveApiResourceUrl:r}=require('C:/tmp/ui21b2c-url-test/resource-url.js'); const b='https://api.example.test/gateway/api/v1'; const cases=[['/api/v1/media/assets/a/content','https://api.example.test/api/v1/media/assets/a/content'],['https://legacy.example/a.jpg','https://legacy.example/a.jpg'],['//cdn.example/a.jpg','//cdn.example/a.jpg'],['wxfile://tmp/a.jpg','wxfile://tmp/a.jpg'],['data:image/png;base64,x','data:image/png;base64,x'],['','']]; for(const [v,w] of cases){const g=r(v,b);if(g!==w)throw new Error(v+' => '+g)}"
npm --workspace apps/miniapp run typecheck
npm --workspace apps/miniapp run build:weapp
```

Expected: Node 无输出且退出码为 0；TypeScript 与微信小程序构建均成功。

- [ ] **Step 5: 提交前端兼容修复**

```powershell
git add apps/miniapp/src/services/resource-url.ts apps/miniapp/src/services/api.ts apps/miniapp/src/services/c-trade.ts docs/superpowers/plans/2026-07-16-ui21b2c-release-acceptance-fixes.md
git commit -m "fix(UI-21b-2c): 兼容商品媒体相对地址"
```

Expected: 提交成功且不包含 `project.config.json` 环境噪声。

### Task 2: 卖家 viewer-state 跳过 IM 自联系查询

**Files:**
- Modify: `backend/internal/module/trade/service_test.go`
- Modify: `backend/internal/module/trade/service.go`

- [ ] **Step 1: 让会话测试桩复现生产自联系拒绝**

```go
type tradeConversations struct {
    calls     int
    findCalls int
    values    []im.ConversationContext
}

func (c *tradeConversations) FindProductConversation(
    _ context.Context,
    productID, buyerID, sellerID uint,
) (im.ConversationContext, bool, error) {
    c.findCalls++
    if buyerID == sellerID {
        return im.ConversationContext{}, false, errors.New("seller cannot contact self")
    }
    for _, value := range c.values {
        if value.ProductID == productID && value.BuyerID == buyerID && value.SellerID == sellerID {
            return value, true, nil
        }
    }
    return im.ConversationContext{}, false, nil
}
```

- [ ] **Step 2: 新增卖家 viewer-state 回归测试并确认修复前失败**

```go
func TestViewerStateForSellerSkipsSelfConversationLookup(t *testing.T) {
    repo := newTradeMemoryRepo(Product{
        ID:           1,
        SellerID:     1,
        Status:       StatusSelling,
        ReviewStatus: review.StatusApproved,
    })
    conversations := &tradeConversations{}
    service := newTradeService(repo, &tradeSearchFake{}, conversations)

    state, err := service.ViewerState(context.Background(), 1, 1)
    if err != nil {
        t.Fatalf("seller viewer state failed: %v", err)
    }
    if state.Favorited || state.ContactedSeller {
        t.Fatalf("unexpected seller viewer state: %+v", state)
    }
    if conversations.findCalls != 0 {
        t.Fatalf("seller viewer state reached self conversation lookup: calls=%d", conversations.findCalls)
    }
}
```

Run:

```powershell
go test ./internal/module/trade -run TestViewerStateForSellerSkipsSelfConversationLookup -count=1
```

Expected: FAIL，错误包含 `seller cannot contact self`。

- [ ] **Step 3: 在收藏状态读取后加入最小短路**

```go
if userID == value.SellerID {
    return ProductViewerState{Favorited: favorited, ContactedSeller: false}, nil
}
```

该分支必须位于 `IsFavorited` 成功之后、`stableProductConversationGateway` 类型断言之前；不得修改 IM 服务的自联系保护。

- [ ] **Step 4: 运行定向和全量后端验证**

Run:

```powershell
go test ./internal/module/trade -run 'TestViewerStateForSellerSkipsSelfConversationLookup|TestProductTargetFavoriteViewerStateAndStableContact' -count=1
go test ./...
go vet ./...
```

Expected: 全部退出码为 0。

- [ ] **Step 5: 提交后端回归修复**

```powershell
git add backend/internal/module/trade/service.go backend/internal/module/trade/service_test.go
git commit -m "fix(UI-21b-2c): 修复卖家商品查看状态"
```

Expected: 提交成功，不包含临时腾讯云漫游测试。

### Task 3: 重启 RC、完成 GUI 回归并收口

**Files:**
- Modify: `docs/UI-21B-2C-TRADE-WRITES-ACCEPTANCE.md`
- Delete: `backend/internal/module/im/gui_roam_temp_test.go`
- Restore: `apps/miniapp/project.config.json`

- [ ] **Step 1: 使用新二进制重启 RC 后端并确认依赖健康**

Run:

```powershell
go build ./cmd/server
Invoke-WebRequest http://127.0.0.1:8080/healthz -UseBasicParsing
Invoke-WebRequest http://127.0.0.1:7700/health -UseBasicParsing
```

Expected: 构建成功，两个健康检查均返回 200。

- [ ] **Step 2: 在微信开发者工具重新编译并复验商品 145**

验收必须同时满足：

1. 封面显示真实图片，不再显示“卖家未上传图片”；
2. 点击封面可预览；
3. 买家 `900000099` 的收藏切换成功，HTTP `PUT/DELETE /trade/products/145/favorite` 返回 200；
4. “联系卖家”进入商品 145 会话，HTTP `PUT /trade/products/145/contact` 与 `GET /im/usersig` 返回 200；
5. 卖家 `900000097` 请求 `/trade/products/145/viewer-state` 返回 200，且不会创建自联系会话。

- [ ] **Step 3: 更新验收报告，诚实记录腾讯云漫游查询限制**

在 `docs/UI-21B-2C-TRADE-WRITES-ACCEPTANCE.md` 增加 2026-07-16 GUI 证据：收藏、建会话、消息界面发送、图片修复、卖家 viewer-state。明确说明 `admin_getroammsg` 对当前消息和已知旧消息均返回空，因此不把该接口结果伪报为通过；GUI/HTTP/数据库证据与该外部限制分开列示。

- [ ] **Step 4: 清理工具噪声并执行最终检查**

Run:

```powershell
Remove-Item -LiteralPath backend/internal/module/im/gui_roam_temp_test.go -Force
git restore -- apps/miniapp/project.config.json
Remove-Item -LiteralPath C:\tmp\ui21b2c-url-test -Recurse -Force -ErrorAction SilentlyContinue
git status --short
npm run check:boundaries
npm run check:miniapp-static-routes
npm run typecheck
npm run build
go test ./...
go vet ./...
```

Expected: 所有检查通过；`git status --short` 最终仅显示尚未提交的验收报告。

- [ ] **Step 5: 提交验收报告并确认可快进合并**

```powershell
git add docs/UI-21B-2C-TRADE-WRITES-ACCEPTANCE.md
git commit -m "docs(UI-21b-2c): 完成真实 GUI 回归验收"
git status --short
git -C "C:\Users\33141\Documents\New project" merge-base --is-ancestor master feat/ui21b2c-trade-writes
```

Expected: 工作树干净，祖先检查退出码为 0；随后才能由 owner 在主工作树执行 fast-forward 合并。

## 自审结果

- 规格覆盖：根相对图片 URL、旧媒体兼容、卖家 self viewer-state、IM 自联系保护、无迁移、真实 GUI 与报告收口均有对应任务。
- 占位扫描：计划中没有 `TBD`、`TODO` 或“稍后实现”式步骤。
- 类型一致性：`resolveApiResourceUrl(value, apiBase)`、`API_BASE`、`findCalls` 与当前代码中的 DTO、gateway 和测试桩签名一致。
