# 个人中心参考改造实施计划

> 按 `writing-plans` 拆分，在本会话内顺序实施与自审，不委派子代理。不安装当前环境未提供的执行技能。

**目标：** 不改主题、功能与接口，在隔离分支改进“我的”及其已有内容栏目的排版和状态可靠性。

**架构：** 页面样式局部修改；独立计数与按栏列表状态由页面/专用 Hook 持有；现有服务、权限、导航和会话仍是权威来源。

**技术栈：** Taro、React、TypeScript、Sass、现有 Node/JSDOM 隔离行为检查。

---

## 任务 1：检查点与范围

- [x] 核对当前分支、原项目及参考资料。
- [x] 保存 `9d485c3` 检查点并建立 `codex/profile-reference-ui-20261014`。
- [x] 记录设计与不改范围，自审没有不确定的待办设计。
- [x] 单独提交设计与本计划，不纳入两份已有微信配置。

## 任务 2：先建立行为测试

文件：新增 `scripts/check-profile-presentation.cjs`；修改 `package.json` 的检查入口。

- [x] 使用与 `scripts/check-post-presentation.cjs` 相同的真实 TSX 转译/React 挂载方式。服务全部为内存 deferred promise，禁止网络。
- [x] 先确认原实现无法通过以下断言：

```js
assert.equal(communityCalls.length, 1, '首次展示不重复请求计数')
assert.equal(counts.items, '7', '社区失败不吞掉二手真实计数')
assert.equal(counts.posts, '—', '失败不是零内容')
assert.equal(navigations.length, 1, '同帧连续点击只导航一次')
assert.equal(content.props('.my-content-body').scrollTop, 640, '切栏恢复滚动位置')
assert.equal(postRequests.length, 2, '重复触底只请求一次下一页')
```

- [x] 增加关闭栏目/账号变化时旧响应无效、刷新失败保留内容、空列表/真实 0 的回归测试。

## 任务 3：个人中心

文件：`apps/miniapp/src/pages/profile/index.tsx`、`index.scss`。

- [x] 删除合并两类计数的全有或全无等待；各自保存 `{ data, loading, error }` 和请求代次/同步锁。
- [x] 保留真正的数字与未知态区分：`data === null ? '—' : String(data)`。
- [x] 导航统一经过本页锁；成功由 `useDidShow` 解锁，失败立即解锁并提示，不修改路径。
- [x] 头像失效按当前 URL 记忆，换头像后可重新加载；会话作用域变化重建本页局部状态。
- [x] 使用动态列数 `repeat(${stats.length}, minmax(0, 1fr))`，按资料/内容/角色/支持顺序布局，44px 以上触控区，长昵称和学校截断。
- [x] 角色可用判断、确认对话框和后台开关表达式保留；只调整工作台入口的行式布局。

## 任务 4：栏目状态与返回

文件：`apps/miniapp/src/pages/my-content/index.tsx`、`index.scss`，新增该目录 `use-content-tabs.ts`。

- [x] 为每个既有栏目维护独立 `items/page/total/phase/error/loadingMore/loadMoreError` 和滚动 ref。
- [x] 读取入口保持 `getMyFavoritePosts/getMyPosts/getMyComments/getMyProductsPage`、页大小 20；既有路由参数不变。
- [x] 同步 busy 锁拦重复触底；请求代次与可见性检查屏蔽旧响应；卸载使任务失效。
- [x] 首读为空才显示整页状态；已有内容的刷新失败显示小提示并保留列表。
- [x] 详情返回刷新当前已加载页，并按 ID 合并；切栏单独恢复 scrollTop，账号变化清理页面会话。
- [x] 保留原主次文案、图片字段、商品状态、评论来源和日期格式，不改公开作者页/咨询/主题。

## 任务 5：验证与交付

- [x] `node scripts/check-profile-presentation.cjs`：所有隔离行为断言通过。
- [x] `npm.cmd run verify`：已有前后端安全回归检查全部通过。
- [x] `npm.cmd run build:miniapp`：构建成功，区分原有体积提示与新增问题。
- [x] `npm.cmd run check:campus-info`：保护咨询/校园指南与开关、分享契约。
- [x] 复核变更文件白名单、`git diff --check`、原项目状态、既有两份配置。
- [x] 保存本轮代码提交，报告检查点、分支、完成项和真机待验收项；不推送/不部署。

## 验收记录与边界

个人中心实现结束时，完整 verify、构建、校园指南检查与差异检查均通过；继续搜索前重新运行窄回归。构建仍有既有 vendors.js 731 KiB 和 NoAsyncChunksWarning 提示。设计提交为 b14c4d0，本计划随个人中心代码精确保存，不纳入两份既有微信配置。

用户截图显示新版首屏已生效，但真机底部滚动、全部入口往返和触控尚未完成验收。上一轮用户停止电脑界面操作后已停止，不把脚本结果当作真机验收。
