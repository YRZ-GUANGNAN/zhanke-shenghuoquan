# UI-01 学生端首页静态还原报告

## 结论

本次按最新 `UI-01-STUDENT-HOME-STATIC-AUTHORIZATION.md` 与 owner 新截图重新实现学生端首页静态 UI。上一版 `feat/ui-student-home` 的错误尝试未复用；本次从 `master@501c6250` 新开 `feat/ui-student-home-v2` 重做。

范围严格限定为小程序首页静态还原：

- 仅改 `apps/miniapp/src/pages/home/*`
- 新增 `apps/miniapp/src/styles/design-tokens.scss`
- 不接后端、不改接口、不改后端、不改其它页面
- 不复用旧首页视觉，不显示“广海空间”旧首页标题/工具宫格/公告卡/旧帖子列表

## 前置核实

- `master` 基线：`501c6250 fix(media): 接入真实微信图片审核适配器`
- `real-wechat-image-review-adapter-baseline` 标签存在
- `backend/cmd/server` 下 `db.AutoMigrate(`：0 命中
- 主仓库工作区：干净
- 旧错误 worktree：`C:\Users\33141\Documents\ui-student-home [feat/ui-student-home]`，本次未继续使用

## 实现说明

### design-tokens 地基

新增 `apps/miniapp/src/styles/design-tokens.scss`，沉淀 Claude Design 给出的品牌色、文字色、背景色、阴影、占位斜纹、功能色等 token。由于当前 Taro 项目配置为 `designWidth: 750`，实现中按 `1px ≈ 2rpx` 映射到小程序样式。

### 首页静态 UI

`apps/miniapp/src/pages/home/index.tsx` 已替换为纯静态结构：

- 顶栏：`私信 / 发现 / 工具` 三 tab，默认激活 `发现`
- 搜索：右侧圆形搜索按钮
- 发现：双列错落瀑布流，占位图 + 标题 + 作者 + 点赞
- 私信：静态会话列表，含未读红点
- 工具：静态 3 列工具宫格
- 底部导航：首页 / 外卖 / 发布 FAB / 二手 / 我的
- 交互：tab 切换、点赞计数、点击 toast 占位

底部导航按授权在 home 内局部实现，未改共享 `BottomNav`，避免影响其它页面。

## 旧 UI 清理证据

已对 home 与 tokens 执行精确旧痕迹搜索，以下旧标识均无命中：

- `广海空间`
- `home-header__name`
- `tool-grid__`
- `notice__`
- `feed-tabs`
- `BottomNav`
- `listCommunityPosts`
- `listFollowingPosts`
- `listNavigationItems`
- `pages/publish/index?type=post`

## 验收命令

全部通过：

```bash
npm --workspace apps/miniapp run typecheck
npm run build:miniapp
npm run check:boundaries
npm run verify
```

`npm run build:miniapp` 仅保留既有 `vendors.js` 体积告警 / `NoAsyncChunksWarning`，无本次新增构建错误。

`npm run verify` 覆盖：

- boundary check
- admin typecheck
- miniapp typecheck
- backend `go test ./...`

## 残留与边界

- 后端改动：0
- API/接口改动：0
- 迁移改动：0
- 其它页面改动：0
- 临时进程：无
- 测试数据残留：无

最终 commit hash 以本分支 `git log --oneline -1` 与交付回复为准。
