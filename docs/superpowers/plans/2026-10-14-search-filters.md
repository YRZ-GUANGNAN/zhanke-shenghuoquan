# 搜索筛选实施计划

> 使用 writing-plans，按任务在本会话顺序实施并自审，不委派代理；当前未提供的执行技能不安装。

**Goal:** 在隔离分支完成真实帖子/二手搜索筛选，不改咨询、主题、既有权限或线上环境。

**Architecture:** 页内草稿面板与已提交查询分离；两个现有搜索接口接收可选条件，数据库在分页前过滤；旧接口调用兼容，以响应版本防止旧后端静默忽略筛选。

**Tech Stack:** Taro/React/TypeScript/Sass、Go/Gin/GORM/MySQL、Node/JSDOM、无网络 GORM SQL 检查。

---

## 任务 1：保存基线和设计

- [x] 个人中心窄回归通过后精确保存为 `84eabda`，排除两份用户微信配置。
- [x] 建立 `codex/search-filters-ui-20261014`，阅读最新手册细节和真实服务端查询。
- [x] 提交本计划与设计；允许路径只包含搜索相关页面、服务、后端查询和验证。

## 任务 2：服务端查询先测试后实现

新增 `backend/internal/module/community/search_filters.go`、`search_filters_test.go`；二手目录下同名文件。修改两域 search/service 入口和 repository 搜索方法。

- [x] 用表驱动测试断言非法 sort/type/day、负价格、倒置价格拒绝；空条件保持有效。
- [x] 用无网络 GORM DryRun 检查实际查询构造，断言审核、屏蔽、删除条件仍在；条件出现在 count/find，ORDER BY 不使用用户表达式，价格以整数分绑定。
- [x] 新增 `SearchFilters`，服务与仓储采用可选参数，使现有三参数前端请求与既有 Go 调用不变。
- [x] Handler 在 metrics.Confirm 前验证 filters；返回原 Page 字段和 `filterVersion: 1`。

```go
// 仅展示兼容性接口约定；实际结构分别保存在两个业务域。
type SearchFilters struct {
    Sort string `json:"sort"`
}
// 现有 page/size 参数及默认行为保留，options ...SearchFilters 为增量。
```

- [x] `npm.cmd run test:backend` 应通过；不使用裸 `go test ./...` 触碰共享数据库。

## 任务 3：客户端传参与行为测试

修改 `apps/miniapp/src/services/b-community.ts`、`c-trade.ts`，新增 `apps/miniapp/src/pages/search/search-filters.ts`、`search-filter-panel.tsx` 和 `scripts/check-search-filters.cjs`。

- [x] 服务第四可选参数 filters，只有非默认条件时发送；读取 filterVersion 后再交出结果，缺版本时抛可读错误。
- [x] 单测将完整真实服务文件转译，以内存 request 捕获参数；断言默认 payload 不变、0 分不遗漏、旧后端被拒绝。
- [x] 真实 TSX/JSDOM 行为测试先断言面板取消不发请求，确认才查询，未提交输入不覆盖已提交关键词。
- [x] 测试新筛选隔离旧第一页/下一页，分页重试携带原条件，同帧连续触底仅请求一次；空态保留条件。

```js
assert.equal(requests.length, beforeOpen, '面板草稿不查询')
assert.equal(requests.at(-1).keyword, '已提交的关键词')
assert.equal(requests.at(-1).page, 1)
assert.equal(requests.at(-1).filters.minPrice, 0)
```

## 任务 4：页面与局部样式

修改 `apps/miniapp/src/pages/search/index.tsx`、`index.scss`。

- [x] 查询 key 包含 scope/keyword/filters；保存 filters 快照供分页、重试使用，换条件回顶部。
- [x] 工具行不随列表滚动；面板草稿重置/取消/确认语义清楚，44PX 触控区、减少动态效果偏好、焦点/按压反馈。
- [x] 二手展示真实首图和商品元数据；无图/失效图明确区分，禁止新增媒体资源。
- [x] 账号变化重建本页状态，现有 feature gate 保留；页内返回先关闭面板。
- [x] 现有帖子/外卖、历史热搜、路由和返回测试继续通过；仅更新确有结构调整的测试定位，不削弱业务断言。

## 任务 5：整体验证与交付

- [x] 将新检查加入 package.json 的 preverify。
- [x] `npm.cmd run verify`、`npm.cmd run build:miniapp`。
- [x] `npm.cmd run check:campus-info`、`npm.cmd run check:share-cards`、`git diff --check`。
- [x] 复核原项目、两份既有配置、没有咨询或全局主题变化。
- [x] 精确提交搜索代码，报告完成、未验收和上线需前后端配套更新的边界，不部署。

## 实施记录（2026-10-14）

- 设计与计划已保存为 `1ca8943`；本计划更新随搜索实现精确提交。个人中心回退点为 `84eabda`。
- 查询构造保留在两域 `repository.go` 内；`search_filters.go` 只定义筛选与校验，符合项目边界检查。没有迁移表。
- 补充 `src/taro-shim.d.ts` 的既有 Taro `hideKeyboard` 类型声明，仅用于搜索提交/面板打开时收键盘，不改变全局运行行为。
- HTTP 内存测试验证 Handler→Service→Repository 的 viewer、keyword、page、filters 传递与旧 payload；非法条件在历史记录前拒绝。DryRun SQL 验证数据库查询结构，但不是实际数据库数据验收。
- 发现原 `Order(clause.Expr)` 被当前 GORM 忽略，改为 `clause.OrderBy` 让既有综合相关性规则实际生效；这会改善默认结果顺序，旧客户端字段和调用方式保持兼容。
- 最终 `npm.cmd run verify`、`npm.cmd run build:miniapp`、`check:campus-info`、`check:share-cards`、`git diff --check` 全部通过。构建仍有既有 vendors.js 731 KiB 和 NoAsyncChunksWarning 提示。
- 实际筛选组件和局部 Sass 以无数据方式渲染，320/390 像素宽度下选项不横向溢出、触控区至少 44px、底部操作可见。检查产物保存在工作区外的 visualizations 目录；没有伪造接口内容或登录，没有生成图片素材。
- 原项目仍为 `e79e168`，三份既有未跟踪计划未动。两份用户微信配置与检查点 `9d485c3` 一致，排除提交。

## 尚未验收／发布

- 微信真机触控、系统返回、键盘高度和真实账号联调尚未验收；组件截图不能代替真机。
- 未连接或修改生产数据库，未部署后端，未上传小程序或推送 GitHub。完整筛选需要后端与小程序配套更新；旧后端会提示暂不支持筛选，清除条件后仍可普通搜索。
