# 分享卡片实施计划

**Goal:** 管理后台可维护真实分享卡片，小程序有效公开页面可转发且接收方正常进入。

**Architecture:** 独立 sharesettings 模块持久化配置；Admin 复用现有表单、鉴权和请求层；MiniApp 使用可测试的分享模型、配置缓存及统一分享 Hook。

**Tech Stack:** Go / Gin / Gorm / MySQL、React / Vite、Taro / TypeScript。

按用户全自动执行要求在当前会话执行，不另派任务、不重复申请设计许可。

## 1. 持久化与接口

- [ ] 新建 `backend/internal/module/sharesettings/{model,repository,handler}.go` 和测试、`backend/migrations/040_share_settings.sql`。固定单行配置，版本比较更新、同事务审计，拒绝未知字段与非 HTTPS JPG/PNG 图片地址。
- [ ] 在 `backend/cmd/server/main.go` 注册公开 GET `/share-settings`、管理员 GET/PUT `/admin/share-settings`；复用既有管理员鉴权与 CSRF 中间件。
- [ ] 执行 `go test ./internal/module/sharesettings`，覆盖读写、无权限、无效输入、冲突和回滚。

## 2. 管理后台

- [ ] 新建 `apps/admin/src/ShareSettingsConsole.tsx`、样式与服务，接入导航和 section。使用实际 API；表单验证、保存中禁用、冲突不覆盖草稿、真实图片预览。
- [ ] 执行 `npm.cmd --workspace apps/admin run typecheck`、`npm.cmd run build:admin`。

## 3. 小程序

- [ ] 新建 `apps/miniapp/src/services/share-model.ts`、`share-settings.ts`、`hooks/use-page-share.ts`；白名单链接生成，默认与真实内容卡片选取，页面显示时刷新配置。
- [ ] 为首页、二手、校园服务、帖子/商品详情、公开主页加入 Hook 与 `enableShareAppMessage`；消息区域、未审核内容不开放分享。
- [ ] 详情复用登录门控，安全解析帖子参数，“分享主页”改原生分享按钮。
- [ ] 新建 `scripts/check-share-cards.cjs` 运行模型断言与必要配置检查；执行 MiniApp typecheck/build。

## 4. 审查和交付

- [ ] 运行现有边界、路由、模块策略、部署、安全检查与后端测试，记录具体结果。
- [ ] 核对生产版本后备份、增量迁移并发布 API 和 Admin，验证健康、版本、配置接口与鉴权。
- [ ] 构建微信包并上传“优化体验”；核验开发版本，提供预览与后台入口，记录真机/发布待验项。
