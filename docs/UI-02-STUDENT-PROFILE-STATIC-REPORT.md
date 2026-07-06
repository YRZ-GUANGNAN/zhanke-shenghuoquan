# UI-02 学生端“我的”静态页复刻报告

## 1. 范围与结论

本次任务基于 `master@847b4220`（已含 `ui-student-home-baseline`），在 `feat/ui-student-profile` 分支完成学生端“我的”一级页静态复刻。

结论：已将 `apps/miniapp/src/pages/profile` 旧页面整体替换为 Claude Design 截图对应的静态 UI，包括蓝色资料头、统计卡、身份切换卡、设置列表、底部导航“我的”高亮。页面不接后端、不读取真实登录态、不执行真实身份切换，仅保留静态占位提示。

## 2. 前置核实

- 页面入口：`apps/miniapp/src/pages/profile/index.tsx`
- 页面样式：`apps/miniapp/src/pages/profile/index.scss`
- 页面配置：`apps/miniapp/src/pages/profile/index.config.ts`
- UI-01 设计令牌：`apps/miniapp/src/styles/design-tokens.scss`
- UI-01 底部导航当前为页面内实现，并非共享组件；本次按同一视觉规格在 profile 页内复用结构与样式，激活“我的”。

## 3. 实现内容

### 3.1 资料头

- 自定义导航样式，保留截图中的整屏沉浸式蓝色头图；
- 静态用户信息：
  - 昵称：阿泽
  - 身份：学生
  - 湛科号：`zk_aze233`
  - 学校：湛江科技学院
- “编辑资料”按钮为静态占位 toast。

### 3.2 数据统计

复刻截图中的白色悬浮统计卡：

| 项 | 值 |
|---|---:|
| 收藏 | 23 |
| 帖子 | 8 |
| 评论 | 41 |
| 宝贝 | 2 |

### 3.3 身份切换

复刻三身份卡片：

- 学生：当前态，高亮蓝色边框和“当前”角标；
- 骑手：静态卡片；
- 商家：静态卡片。

点击身份仅切换前端高亮并显示 toast，不触发任何业务路由或后端请求。

### 3.4 设置列表

复刻截图中的设置卡：

- 设置
- 用户协议与隐私政策
- 清理缓存：`12.6MB`
- 检查更新：`v2.4.0`
- 关于湛科生活圈

所有行均为静态占位，不接真实功能。

### 3.5 底部导航

复刻 UI-01 底部导航结构，并将“我的”置为 active：

- 首页
- 外卖
- 中央发布按钮
- 二手
- 我的

非当前项仅显示静态占位 toast，不跳转旧页面，避免把已废弃的旧 UI 再带回。

## 4. 修改文件

- `apps/miniapp/src/pages/profile/index.tsx`
- `apps/miniapp/src/pages/profile/index.scss`
- `apps/miniapp/src/pages/profile/index.config.ts`
- `docs/UI-02-STUDENT-PROFILE-STATIC-REPORT.md`

未修改后端、接口、数据库、迁移、路由契约或其它业务域。

## 5. 验收记录

已执行：

```powershell
npm --workspace apps/miniapp run typecheck
npm run build:miniapp
npm run check:boundaries
npm run verify
```

结果：

- `typecheck`：PASS
- `build:miniapp`：PASS（仅既有 `vendors.js` 体积告警）
- `check:boundaries`：PASS
- `verify`：PASS（含 admin/miniapp typecheck 与 backend `go test ./...`）

## 6. 残留与边界

- 本次为纯静态前端 UI 复刻；
- 未引入新依赖；
- 未改 `package-lock.json`；
- 未改任何后端文件；
- 未接真实登录态、真实身份切换、真实设置页或缓存清理；
- `node_modules` 仅为 worktree 本地验收依赖，不入库。
