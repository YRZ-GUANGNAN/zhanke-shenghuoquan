# 正常小程序体验包发布计划

> 用户已授权直接执行，本会话完成，不另派代理。只上传正常开发代码供体验，不提交审核、不发布。

**Goal:** 用真实后端、完整页面的版本替换当前演示包，保留封面细线与主页局部横滑修复。

**Architecture:** 从正常版本 `b8bf4f6` 建立 `codex/profile-interactions-release-20261017`，只迁入 `ed4f2e6`。不带入 `src/demo` 或演示编译适配器。正式接口、业务开关、身份守卫不变。

**Tech Stack:** Taro 4、React、TypeScript、官方微信开发者工具 CLI。

## 1. 正常代码基线

- [x] `git switch -c codex/profile-interactions-release-20261017 b8bf4f6`。
- [x] `git cherry-pick ed4f2e6`，仅迁入主页布局、共用封面样式与配套检查。
- [x] 核对 `apps/miniapp/src/app.config.ts` 包含 30 页面，底部服务路由 `/pages/student-takeout/index` 确实打包。
- [x] 核对 `apps/miniapp/config/prod.ts` 连接 `https://zhanduiwang.top/api/v1`，无演示分支、模拟用户与本地服务地址。

## 2. 验证与冻结

- [x] 运行 `npm.cmd run typecheck --workspace @campus-circle/miniapp`。
- [x] 运行 `node scripts/check-external-takeout-directory.cjs`、`node scripts/check-student-entry-avatar-contract.cjs`、`node scripts/check-miniapp-static-routes.cjs`、`node scripts/check-miniapp-back-navigation.cjs`、`node scripts/check-public-profile-gestures.cjs`，以及导航防重与 320–768px 样式检查。
- [x] 运行 `npm.cmd run build:weapp --workspace @campus-circle/miniapp`；冻结完整产物到 `.codex-tmp/release-normal-20261017-5/`，验证所有 app.json 路由的 js/json/wxml 文件以及服务页、共用样式存在，记录源码与文件树散列。
- [x] 真实后端健康正常，最新配置为 `standard` / `rowVersion: 21`；服务目录返回 200，目前没有可用合作商家。官方开发者工具正常包验证“服务 → 首页 → 服务 → 首页”成功，无请求模拟、身份注入或生产开关修改。

## 3. 交付

- [x] 官方 CLI 上传 `2026.10.17.5`，备注 `优化体验`，保存回执与手机预览码，上传包 1,971,073 字节。
- [x] 推送独立正常发布分支到现有 GitHub 仓库并核对远端提交，保留演示分支和原项目。
- [ ] 尝试在微信公众平台将该开发版本设为体验版并核验；若无可用的平台控制连接，明确报告尚需执行该步骤，不将开发上传冒称体验版设置成功。

平台状态：本会话仅有官方开发工具 CLI，已检查浏览器与桌面控制能力但所需连接工具不可用。已请求打开公众平台版本管理页；该工具返回 queued，不能作为平台已经切换体验版的证据。已生成真实正常包开发预览码供手机测试。

自审：任务只涉及正常包发布与已确认修复的迁入；不恢复模拟数据、不修改生产业务开关、不改支付配置。原项目的现有脏文件不动。
