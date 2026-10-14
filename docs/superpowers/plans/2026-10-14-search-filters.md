# 搜索筛选实施计划

> 使用 writing-plans，按任务在本会话顺序实施并自审，不委派代理；当前未提供的执行技能不安装。

**Goal:** 在隔离分支完成真实帖子/二手搜索筛选，不改咨询、主题、既有权限或线上环境。

**Architecture:** 页内草稿面板与已提交查询分离；两个现有搜索接口接收可选条件，数据库在分页前过滤；旧接口调用兼容，以响应版本防止旧后端静默忽略筛选。

**Tech Stack:** Taro/React/TypeScript/Sass、Go/Gin/GORM/MySQL、Node/JSDOM、无网络 GORM SQL 检查。

---

## 任务 1：保存基线和设计

- [x] 个人中心窄回归通过后精确保存为 `84eabda`，排除两份用户微信配置。
- [x] 建立 `codex/search-filters-ui-20261014`，阅读最新手册细节和真实服务端查询。
- [ ] 提交本计划与设计；允许路径只包含搜索相关页面、服务、后端查询和验证。

## 任务 2：服务端查询先测试后实现

新增 `backend/internal/module/community/search_filters.go`、`search_filters_test.go`；二手目录下同名文件。修改两域 search/service 入口和 repository 搜索方法。

- [ ] 用表驱动测试断言非法 sort/type/day、负价格、倒置价格拒绝；空条件保持有效。
- [ ] 用无网络 GORM DryRun 检查实际查询构造，断言审核、屏蔽、删除条件仍在；条件出现在 count/find，ORDER BY 不使用用户表达式，价格以整数分绑定。
- [ ] 新增 `SearchFilters`，服务与仓储采用可选参数，使现有三参数前端请求与既有 Go 调用不变。
- [ ] Handler 在 metrics.Confirm 前验证 filters；返回原 Page 字段和 `filterVersion: 1`。

```go
// 仅展示兼容性接口约定；实际结构分别保存在两个业务域。
type SearchFilters struct {
    Sort string `json:"sort"`
}
// 现有 page/size 参数及默认行为保留，options ...SearchFilters 为增量。
```

- [ ] `npm.cmd run test:backend` 应通过；不使用裸 `go test ./...` 触碰共享数据库。

## 任务 3：客户端传参与行为测试

修改 `apps/miniapp/src/services/b-community.ts`、`c-trade.ts`，新增 `apps/miniapp/src/pages/search/search-filters.ts`、`search-filter-panel.tsx` 和 `scripts/check-search-filters.cjs`。

- [ ] 服务第四可选参数 filters，只有非默认条件时发送；读取 filterVersion 后再交出结果，缺版本时抛可读错误。
- [ ] 单测将完整真实服务文件转译，以内存 request 捕获参数；断言默认 payload 不变、0 分不遗漏、旧后端被拒绝。
- [ ] 真实 TSX/JSDOM 行为测试先断言面板取消不发请求，确认才查询，未提交输入不覆盖已提交关键词。
- [ ] 测试新筛选隔离旧第一页/下一页，分页重试携带原条件，同帧连续触底仅请求一次；空态保留条件。

```js
assert.equal(requests.length, beforeOpen, '面板草稿不查询')
assert.equal(requests.at(-1).keyword, '已提交的关键词')
assert.equal(requests.at(-1).page, 1)
assert.equal(requests.at(-1).filters.minPrice, 0)
```

## 任务 4：页面与局部样式

修改 `apps/miniapp/src/pages/search/index.tsx`、`index.scss`。

- [ ] 查询 key 包含 scope/keyword/filters；保存 filters 快照供分页、重试使用，换条件回顶部。
- [ ] 工具行不随列表滚动；面板草稿重置/取消/确认语义清楚，44PX 触控区、减少动态效果偏好、焦点/按压反馈。
- [ ] 二手展示真实首图和商品元数据；无图/失效图明确区分，禁止新增媒体资源。
- [ ] 账号变化重建本页状态，现有 feature gate 保留；页内返回先关闭面板。
- [ ] 现有帖子/外卖、历史热搜、路由和返回测试继续通过；仅更新确有结构调整的测试定位，不削弱业务断言。

## 任务 5：整体验证与交付

- [ ] 将新检查加入 package.json 的 preverify。
- [ ] `npm.cmd run verify`、`npm.cmd run build:miniapp`。
- [ ] `npm.cmd run check:campus-info`、`npm.cmd run check:share-cards`、`git diff --check`。
- [ ] 复核原项目、两份既有配置、没有咨询或全局主题变化。
- [ ] 精确提交搜索代码，报告完成、未验收和上线需前后端配套更新的边界，不部署。
