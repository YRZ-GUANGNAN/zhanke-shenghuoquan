# 2026-11-17 「众创」频道上线记录

首页新增「众创」频道（外接小程序入口），后台「服务入口」页新增众创入口管理。本次不涉及后端二进制改动（用户端 `/navigation/categories` 已返回 `systemKey`，功能开关与工具频道共用 `campus_tools`）。

## 本次内容

- **迁移** `backend/migrations/064_campus_maker_category.sql`：幂等种入「众创」系统分类（`system_key=campus_maker`，排序 30）；回滚 `rollback/064_campus_maker_category.sql`（删除分类及其条目）；迁移测试 `migration_064_test.go`。
- **小程序**：首页分段「发现 / 二手 / 工具 / 众创」；众创频道与工具同为外接小程序跳转（`navigateToMiniProgram`），数据按 `systemKey` 分流（`listCampusMakerItems`），工具频道自动排除众创分类；`ToolDirectory` 支持众创文案与空态；加载/刷新/切号清理与工具频道一致。
- **后台**：运行入口为 `main.tsx → preview/App` 的运营后台壳，「服务入口」页（`preview/views/SettingsView.tsx` 的 `NavigationSettings`）新增「众创入口」面板：条目新增/编辑/删除、启停与排序；旧壳 `App.tsx` 同步新增 `CampusMakerEntries` 组件保持一致。

## 部署记录（2026-11-17）

- 代码：`ee8124d` 已推送到 origin/main（其上叠加了并行会话的 `16b6659`、`9ed060f`）。
- 生产库迁移 064 已用维护账号（debian-sys-maint）执行；迁移前备份 `/var/backups/campus-circle/pre-migration-064-20261117T072940Z.sql.gz`；执行后 `navigation_categories` 含「众创」（id=2，`system_key=campus_maker`，sort=30）。
- 后台静态：线上 `current` 构建（`index-CKjAXSW3.js` + `SettingsView-B3HGaudY.js`，与本地当前源码构建结果一致）已包含众创面板（chunk 内含 `campus_maker`），本次未重复部署。
- 后端：`campus-api` 未改动、未重启（版本仍为 `2026.11.16.candidate.1`）。
- 线上验证：`GET /api/v1/navigation/categories` 返回 `{"id":2,"name":"众创","systemKey":"campus_maker","sort":30,"items":[]}`。

## 待办（人工）

- 小程序上传体验版并发版后，「众创」分段才对用户可见。
- 在后台「服务入口 → 众创入口」新增实际的外接小程序条目。
- 微信公众平台「小程序跳转 AppID 白名单」需加入各外接小程序 AppID，否则点击跳转会被微信拦截。
- 若工具频道不需要展示众创以外分类的行为变化无需操作（工具=除外卖商家与众创外的全部分类）。

## 回滚

- 代码：回退 `ee8124d` 相关文件（首页分段、`tool-directory.tsx`、`d-navigation.ts`、后台面板）。
- 数据：执行 `backend/migrations/rollback/064_campus_maker_category.sql`（删除众创分类及其条目；纯新增，回滚代码时可暂不删数据）。
