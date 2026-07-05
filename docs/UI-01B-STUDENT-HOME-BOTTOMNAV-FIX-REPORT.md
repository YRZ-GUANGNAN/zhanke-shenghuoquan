# UI-01b 学生端首页底部导航修正报告

## 范围

本次为 `UI-01` 学生端首页静态界面的受控视觉修正，仅调整首页底部导航：

- 中央发布按钮从过度悬浮改为半嵌入底栏；
- 四个底栏图标从字体字符替换为更清晰、更接近设计图的实线图标；
- 保留首页瀑布流、顶部 tab、卡片样式与静态数据；
- 不修改任何后端、API、服务、状态管理、迁移或其它页面。

## 基线说明

当前主仓库 `master` 尚未合并 `UI-01` 第一阶段，因此本次实际基于已完成的新学生端首页分支：

- 基线提交：`ec1e3375 feat(ui): 按新设计重做学生端首页静态界面`
- 本次分支：`feat/ui-student-home-bottomnav`

## 修改内容

### 1. FAB 嵌入底栏

修改文件：

- `apps/miniapp/src/pages/home/index.scss`

关键调整：

- `.student-bottom-nav__fab` 的 `margin-top` 从 `-40rpx` 调整为 `-24rpx`；
- 底栏高度从 `118rpx` 调整为 `124rpx`；
- 底栏增加 `overflow: visible`，确保按钮阴影与白色描边自然露出；
- 视觉目标为：按钮轻微上浮，下半部分嵌入底栏，接近设计图中“一体化”的发布按钮。

### 2. 底栏图标重绘

修改文件：

- `apps/miniapp/src/pages/home/index.tsx`
- `apps/miniapp/src/pages/home/index.scss`

关键调整：

- 移除旧字符图标：`⌂`、`▱`、`▢`、`♙`；
- 改为语义 icon key：`home`、`takeaway`、`market`、`profile`；
- 使用 CSS `::before` / `::after` 绘制首页、外卖、二手、我的四个图标；
- 图标颜色继承导航项状态：激活态 `#2E7DD4`，未激活态 `#93A2B4`。

## 边界确认

- 未修改 `backend/**`；
- 未修改 API client；
- 未修改路由注册；
- 未修改其它小程序页面；
- 未修改迁移、数据库、配置；
- 未接入真实数据；
- 未恢复旧 UI。

## 验收命令

已通过以下命令：

```powershell
npm --workspace apps/miniapp run typecheck
npm run build:miniapp
npm run check:boundaries
npm run verify
```

验收结果：

- `npm --workspace apps/miniapp run typecheck`：PASS；
- `npm run build:miniapp`：PASS，仅保留既有 `vendors.js` 体积告警；
- `npm run check:boundaries`：PASS；
- `npm run verify`：PASS，包含 admin/miniapp typecheck 与 backend `go test ./...`。

## 结论

本次修正只针对 `UI-01` 新学生端首页底部导航，使中央发布按钮与四个导航图标更贴近最新 Claude Design 截图。页面仍为静态首页，不引入任何接口或业务逻辑变化。
