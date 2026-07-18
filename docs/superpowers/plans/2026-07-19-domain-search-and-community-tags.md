# 分域搜索与帖子标签 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 为帖子、二手和外卖建立互不混合的搜索、历史和近 7 天热词，并为帖子增加最多 3 个自定义标签与热门标签。

**Architecture:** 三个领域分别暴露搜索路由并持有独立 history/event 表；共享包只提供显式表配置、关键词规范化和确定性热词排序，不存在默认或全局 scope。帖子标签以 `posts.tags` JSON 数组保存，由社区服务统一规范化、参与幂等哈希、搜索与 7 天热榜。前端复用搜索页面，但非法或缺失 scope 失败关闭。

**Tech Stack:** Go 1.25、Gin、GORM、MySQL 8.4、Taro 4、React 18、TypeScript、Node.js 静态契约检查

---

## 文件结构

- Create: `backend/migrations/032_domain_search_and_community_tags.sql`
- Create: `backend/migrations/migration_032_test.go`
- Create: `backend/internal/pkg/searchmetrics/metrics.go`
- Create: `backend/internal/pkg/searchmetrics/metrics_test.go`
- Create: `backend/internal/module/community/search.go`
- Create: `backend/internal/module/community/search_test.go`
- Create: `backend/internal/module/trade/search_api.go`
- Create: `backend/internal/module/trade/search_api_test.go`
- Create: `backend/internal/module/navigation/search.go`
- Create: `backend/internal/module/navigation/search_test.go`
- Modify: `backend/internal/module/community/model.go`
- Modify: `backend/internal/module/community/handler.go`
- Modify: `backend/internal/module/community/repository.go`
- Modify: `backend/internal/module/community/service.go`
- Modify: `backend/internal/module/trade/repository.go`
- Modify: `backend/cmd/server/main.go`
- Modify: `apps/miniapp/src/types.ts`
- Modify: `apps/miniapp/src/services/b-community.ts`
- Modify: `apps/miniapp/src/services/c-trade.ts`
- Modify: `apps/miniapp/src/services/d-navigation.ts`
- Modify: `apps/miniapp/src/pages/search/index.tsx`
- Modify: `apps/miniapp/src/pages/search/index.scss`
- Modify: `apps/miniapp/src/pages/home/index.tsx`
- Modify: `apps/miniapp/src/pages/student-secondhand/index.tsx`
- Modify: `apps/miniapp/src/pages/publish-post/index.tsx`
- Create: `scripts/check-miniapp-domain-search.cjs`
- Modify: `package.json`

### Task 1: 建立向前迁移和静态迁移测试

**Files:**
- Create: `backend/migrations/032_domain_search_and_community_tags.sql`
- Create: `backend/migrations/migration_032_test.go`

- [ ] **Step 1: 先写迁移静态测试并确认失败**

测试读取 `032_domain_search_and_community_tags.sql`，逐项断言：

```go
required := []string{
	"ADD COLUMN tags JSON NULL",
	"CREATE TABLE community_search_histories",
	"CREATE TABLE community_search_events",
	"CREATE TABLE trade_search_histories",
	"CREATE TABLE trade_search_events",
	"CREATE TABLE navigation_search_histories",
	"CREATE TABLE navigation_search_events",
}
```

同时断言 `backend/migrations/rollback/032_domain_search_and_community_tags.sql` 不存在，且每个 history 表包含 `(user_id, normalized_keyword)` 唯一键、每个 event 表包含热榜与防刷所需复合索引。

Run: `Push-Location backend; go test ./migrations -run TestMigration032 -count=1; Pop-Location`

Expected: FAIL，提示迁移文件不存在。

- [ ] **Step 2: 创建迁移**

迁移必须包含以下等价结构，三个领域使用独立表：

```sql
ALTER TABLE posts ADD COLUMN tags JSON NULL AFTER topic;

CREATE TABLE community_search_histories (
  id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  user_id BIGINT UNSIGNED NOT NULL,
  normalized_keyword VARCHAR(100) NOT NULL,
  keyword VARCHAR(100) NOT NULL,
  updated_at DATETIME(3) NOT NULL,
  PRIMARY KEY (id),
  UNIQUE KEY uq_community_search_history_user_keyword (user_id, normalized_keyword),
  KEY idx_community_search_history_user_updated (user_id, updated_at)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;
```

每个 `*_search_events` 使用 `id`、`user_id`、`normalized_keyword`、`keyword`、`created_at`，并建立 `(normalized_keyword, created_at)` 与 `(user_id, normalized_keyword, created_at)` 索引。复制结构时只替换 `community` 为 `trade`、`navigation`，不加入 scope 列。

- [ ] **Step 3: 运行迁移测试**

Run: `Push-Location backend; go test ./migrations -run TestMigration032 -count=1; Pop-Location`

Expected: PASS。

### Task 2: 实现确定性的关键词统计工具

**Files:**
- Create: `backend/internal/pkg/searchmetrics/metrics.go`
- Create: `backend/internal/pkg/searchmetrics/metrics_test.go`

- [ ] **Step 1: 写规范化与排行失败测试**

覆盖：空白合并、英文小写、1–50 字符、30 秒重复、每天每用户每词最多 5 次、7/6/5/4/3/2/1 日权重、至少 3 次且至少 2 个用户、稳定排序、前 10 个。

- [ ] **Step 2: 定义无默认 scope 的存储接口**

```go
type Tables struct { History, Events string }
type HotKeyword struct { Keyword string `json:"keyword"`; Count int `json:"count"`; Score int `json:"score"` }
type Metrics interface {
	Confirm(context.Context, uint, string) (string, error)
	History(context.Context, uint) ([]string, error)
	ClearHistory(context.Context, uint) error
	Hot(context.Context) ([]HotKeyword, error)
}
```

`NewStore(db, tables, now)` 必须拒绝空表名和不符合 `^[a-z_]+$` 的表名；调用者必须显式传入两个表名。

- [ ] **Step 3: 实现规范化、确认和个人历史**

`Normalize` 使用 `strings.Fields` 合并空白、`strings.ToLower` 形成规范键、`utf8.RuneCountInString` 限制 1–50。`Confirm` 在一个事务中 upsert 当前领域 history；30 秒内已有事件则不新增，超过每日 5 次也不新增，但正常返回规范后的展示词。`History` 只查当前表、当前用户，按 `updated_at DESC` 返回 20 条；`ClearHistory` 只删除 history 表，不删除 event 表。

- [ ] **Step 4: 实现热词排行**

只加载最近 7 个上海自然日事件；当天权重 7，前 6 天权重 1。按 `score DESC, count DESC, latest_at DESC, normalized_keyword ASC` 排序；不足 3 次或不足 2 个用户的词排除；默认返回 10，最多 20。

- [ ] **Step 5: 运行包测试**

Run: `Push-Location backend; go test ./internal/pkg/searchmetrics -count=1; Pop-Location`

Expected: PASS。

### Task 3: 社区搜索与帖子标签

**Files:**
- Create: `backend/internal/module/community/search.go`
- Create: `backend/internal/module/community/search_test.go`
- Modify: `backend/internal/module/community/model.go`
- Modify: `backend/internal/module/community/handler.go`
- Modify: `backend/internal/module/community/repository.go`
- Modify: `backend/internal/module/community/service.go`

- [ ] **Step 1: 写标签规范化、搜索可见性和热榜失败测试**

测试必须覆盖：去除前导 `#`、去重、最多 3 个、单标签 1–12 字符；创建请求含标签时进入 v2 幂等哈希、无标签仍使用 v1 哈希；搜索只返回 `approved`；标题完全匹配优先于前缀、正文和标签；标签热榜排除非公开帖子。

- [ ] **Step 2: 扩展帖子模型与输入**

```go
type Post struct {
  // existing fields
  Tags []string `json:"tags" gorm:"serializer:json"`
}

type CreatePostInput struct {
  UserID uint
  Title, Content, Topic string
  Tags []string
  ImageAssetIDs []string
}
```

- [ ] **Step 3: 实现标签规范化并兼容幂等 v1**

`normalizeTags` 返回稳定顺序的 0–3 个无 `#` 标签。无标签时沿用 `community-post-create:v1` 原哈希结构；非空标签使用 `community-post-create:v2` 并把规范化 tags 写入哈希，保证旧请求重放不冲突。普通创建和 keyed 创建都把同一规范化 tags 写入 `Post.Tags`。

- [ ] **Step 4: 扩展创建 handler**

JSON 请求增加：

```go
Tags []string `json:"tags"`
```

传给 `CreatePostInput`；没有 `tags` 时允许把旧 `topic` 作为兼容展示字段，但不伪造多个标签。

- [ ] **Step 5: 实现数据库搜索与热门标签**

新增 `GormRepository.SearchPosts`，使用标题完全匹配、标题前缀、标题包含、正文包含、`CAST(tags AS CHAR)` 包含的 CASE 排名；强制 `review_status = approved`，排序最后使用 `created_at DESC`、互动合计 DESC、`id DESC`。新增 `ListRecentTagPosts` 只读取最近 7 天已批准帖子，服务层按使用分优先、互动分次之、最近时间与标签文本稳定排序，返回前 12 个。

- [ ] **Step 6: 注册社区独立搜索路由**

`SearchHandler` 在鉴权组注册：

```text
POST   /community/searches
GET    /community/search-history
DELETE /community/search-history
GET    /community/hot-searches
GET    /community/hot-tags
```

`POST /searches` 先 `Confirm`，再执行社区数据库搜索，返回现有 `Page[PostView]`。其他接口只使用 community metrics 实例。

- [ ] **Step 7: 运行社区测试**

Run: `Push-Location backend; go test ./internal/module/community -count=1; Pop-Location`

Expected: PASS。

### Task 4: 二手独立搜索

**Files:**
- Create: `backend/internal/module/trade/search_api.go`
- Create: `backend/internal/module/trade/search_api_test.go`
- Modify: `backend/internal/module/trade/repository.go`

- [ ] **Step 1: 写二手搜索失败测试**

覆盖：只搜索 title/description/category；只返回 `review_status=approved AND status=selling`；已售、下架／删除、未审核不返回；完全匹配优先；history/hot 使用 trade metrics 而不是 community metrics。

- [ ] **Step 2: 实现数据库回退搜索**

新增 `GormRepository.SearchProducts`，使用 CASE 排名与稳定分页；不依赖 Meilisearch 可用性。新增 `Service.SearchProducts` 专供确认搜索路由，现有列表和 Meilisearch 索引链路保持不变。

- [ ] **Step 3: 注册二手独立搜索路由**

```text
POST   /trade/searches
GET    /trade/search-history
DELETE /trade/search-history
GET    /trade/hot-searches
```

请求与社区一致，但只返回 `ProductView` 页面。

- [ ] **Step 4: 运行二手测试**

Run: `Push-Location backend; go test ./internal/module/trade -count=1; Pop-Location`

Expected: PASS。

### Task 5: 外卖独立搜索

**Files:**
- Create: `backend/internal/module/navigation/search.go`
- Create: `backend/internal/module/navigation/search_test.go`

- [ ] **Step 1: 写外卖搜索失败测试**

覆盖：只搜索商家 title/subtitle；目录关闭时返回空；隐藏商家不返回；营业、休息状态均可返回；完全匹配优先，随后前缀、简介、`sort`、ID；history/hot 只使用 navigation metrics。

- [ ] **Step 2: 实现目录内搜索**

服务使用 `PublicTree` 的已启用分类和条目，定位 `SystemKeyExternalTakeoutMerchants`，在内存中做有界匹配与稳定分页。不得调用旧外卖订单、支付或配送领域。

- [ ] **Step 3: 注册外卖独立搜索路由**

```text
POST   /navigation/external-takeout/searches
GET    /navigation/external-takeout/search-history
DELETE /navigation/external-takeout/search-history
GET    /navigation/external-takeout/hot-searches
```

- [ ] **Step 4: 运行导航测试**

Run: `Push-Location backend; go test ./internal/module/navigation -count=1; Pop-Location`

Expected: PASS。

### Task 6: 在服务器中显式装配三个指标实例

**Files:**
- Modify: `backend/cmd/server/main.go`

- [ ] **Step 1: 创建三个独立 Store**

```go
communityMetrics := searchmetrics.NewStore(db, searchmetrics.Tables{History: "community_search_histories", Events: "community_search_events"}, time.Now)
tradeMetrics := searchmetrics.NewStore(db, searchmetrics.Tables{History: "trade_search_histories", Events: "trade_search_events"}, time.Now)
navigationMetrics := searchmetrics.NewStore(db, searchmetrics.Tables{History: "navigation_search_histories", Events: "navigation_search_events"}, time.Now)
```

- [ ] **Step 2: 在各自领域 group 注册 SearchHandler**

社区、二手、导航分别接收自己的 metrics；不得创建全局 `/search` API，也不得把 scope 作为后端路由参数。

- [ ] **Step 3: 运行 server 与边界测试**

Run: `Push-Location backend; go test ./cmd/server ./internal/module/community ./internal/module/trade ./internal/module/navigation ./internal/pkg/searchmetrics -count=1; Pop-Location`

Expected: PASS。

Run: `npm run check:boundaries`

Expected: PASS。

### Task 7: 接入 Miniapp 分域搜索服务与页面

**Files:**
- Modify: `apps/miniapp/src/types.ts`
- Modify: `apps/miniapp/src/services/b-community.ts`
- Modify: `apps/miniapp/src/services/c-trade.ts`
- Modify: `apps/miniapp/src/services/d-navigation.ts`
- Modify: `apps/miniapp/src/pages/search/index.tsx`
- Modify: `apps/miniapp/src/pages/search/index.scss`
- Modify: `apps/miniapp/src/pages/home/index.tsx`
- Modify: `apps/miniapp/src/pages/student-secondhand/index.tsx`
- Create: `scripts/check-miniapp-domain-search.cjs`
- Modify: `package.json`

- [ ] **Step 1: 写前端静态契约检查并确认失败**

检查：home 使用 `scope=community`；secondhand 使用 `scope=secondhand`；搜索页只接受三个合法 scope；缺失 scope 不调用服务；三个服务分别使用各自后端路径；禁止出现“搜帖子、二手好物”全局文案和静态数组。

- [ ] **Step 2: 增加前端类型与服务函数**

`CommunityPost` 增加 `tags: string[]`。三个服务分别实现 `search*`、`list*History`、`clear*History`、`list*HotSearches`；community 额外实现 `listHotCommunityTags`。禁止新增接受任意 scope 的通用 API 服务函数。

- [ ] **Step 3: 重写搜索页状态机**

搜索页读取并校验 `scope`；合法时加载当前模块 history/hot。用户键盘确认、点击搜索按钮、历史词或热词时调用当前模块 `POST searches`。结果分别渲染帖子、商品或商家简洁卡片并进入对应详情／商家入口；加载、空结果、失败、重试和清空历史均有真实状态。

- [ ] **Step 4: 修正入口路由**

```ts
// home
void Taro.navigateTo({ url: '/pages/search/index?scope=community' })

// student-secondhand
void Taro.navigateTo({ url: '/pages/search/index?scope=secondhand' })
```

外卖入口在下一阶段动态目录页面中接入 `scope=takeout`。

- [ ] **Step 5: 运行前端检查**

Run: `npm run check:miniapp-domain-search`

Expected: PASS。

Run: `npm --prefix apps/miniapp run typecheck`

Expected: PASS。

### Task 8: 接入帖子自定义标签

**Files:**
- Modify: `apps/miniapp/src/services/b-community.ts`
- Modify: `apps/miniapp/src/pages/publish-post/index.tsx`

- [ ] **Step 1: 扩展创建请求**

```ts
export interface CreateCommunityPostInput {
  title: string
  content: string
  topic?: string
  tags?: string[]
  imageAssetIds?: string[]
  clientRequestId?: string
}
```

请求体发送 `tags: input.tags ?? []`；`normalizePost` 读取后端 tags。

- [ ] **Step 2: 将固定话题改为热门加自定义**

页面加载 `listHotCommunityTags()`，失败时使用现有固定话题作为不计热度的回退展示。增加一个自定义标签 Input；提交时去除 `#`、校验 1–12 字符、去重并限制总数 3。发布请求发送全部 tags，`topic` 使用第一个标签或“湛科日常”保持旧展示兼容。

- [ ] **Step 3: 运行发布与搜索契约检查**

Run: `npm run check:miniapp-publish-contract`

Expected: PASS。

Run: `npm run check:miniapp-domain-search`

Expected: PASS。

### Task 9: 阶段总验证与提交

**Files:**
- Verify all files above

- [ ] **Step 1: 运行后端目标测试**

Run: `Push-Location backend; go test ./migrations ./internal/pkg/searchmetrics ./internal/module/community ./internal/module/trade ./internal/module/navigation ./cmd/server -count=1; Pop-Location`

Expected: PASS。

- [ ] **Step 2: 运行前端验证与构建**

Run: `npm run check:miniapp-publish-contract`

Expected: PASS。

Run: `npm run check:miniapp-domain-search`

Expected: PASS。

Run: `npm run check:miniapp-static-routes`

Expected: PASS。

Run: `npm run typecheck`

Expected: PASS。

Run: `npm run build:miniapp`

Expected: PASS。

- [ ] **Step 3: 检查迁移与共享数据库边界**

只在可创建的独立临时数据库执行迁移 032。若当前 MySQL 用户没有 `CREATE DATABASE` 权限，记录 `PENDING (permission blocked)`；不得在共享 `campus_circle` 执行回滚或破坏性替代验证。

- [ ] **Step 4: 检查差异并提交**

Run: `git diff --check`

Expected: 无新增空白错误。

只暂存本计划列出的文件：

```powershell
git add -- backend/migrations/032_domain_search_and_community_tags.sql backend/migrations/migration_032_test.go backend/internal/pkg/searchmetrics backend/internal/module/community backend/internal/module/trade/search_api.go backend/internal/module/trade/search_api_test.go backend/internal/module/trade/repository.go backend/internal/module/navigation/search.go backend/internal/module/navigation/search_test.go backend/cmd/server/main.go apps/miniapp/src/types.ts apps/miniapp/src/services/b-community.ts apps/miniapp/src/services/c-trade.ts apps/miniapp/src/services/d-navigation.ts apps/miniapp/src/pages/search apps/miniapp/src/pages/home/index.tsx apps/miniapp/src/pages/student-secondhand/index.tsx apps/miniapp/src/pages/publish-post/index.tsx scripts/check-miniapp-domain-search.cjs package.json docs/superpowers/plans/2026-07-19-domain-search-and-community-tags.md
git commit -m "功能：增加分域搜索与帖子标签"
```

- [ ] **Step 5: 记录真机待验**

三个模块的历史／热词隔离、中文搜索、搜索清空、标签发布与热榜更新需要在迁移后的真实后端和微信真机复测；未执行前保持 `PENDING`。
