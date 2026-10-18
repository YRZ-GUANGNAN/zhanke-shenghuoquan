# 商家端与动态媒体整合上传 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.
> 用户已经授权等待商家端完成后合并并上传开发版本。附属执行技能不可用，本任务在当前会话内执行；不另派代理，不重复请求授权。

**Goal:** 等待商家端当前任务完成后，将完整可验证提交与帖子视频/实况功能合并，上传微信开发版本 `2026.10.18.2`，备注 `优化体验`。

**Architecture:** 商家任务继续在 `codex/delivery-workspace-redesign-20261018` 内完成自己的测试与预览；本任务只在 `codex/post-motion-preview-20261017` 接收完成后的提交并制作冻结包。生产后端沿用已部署的 `2026.10.18.1`，本轮没有后端变更时不重新部署。上传开发版与提交审核、发布上线分开记录。

**Tech Stack:** Git、Taro 4、React 18、Node.js、微信官方开发者工具 CLI。

---

## 文件与工作区

- 源工作区：`C:/Users/33141/.codex/worktrees/delivery-workspace-redesign/New project`。
- 整合工作区：`C:/Users/33141/.codex/worktrees/a494/New project`。
- 需要保留：所有原有未跟踪发布产物、原项目和其他工作区。
- 运维记录：`docs/operations/2026-10-18-merchant-motion-integration-release.md`。
- 最终冻结目录：`.codex-tmp/release-merchant-post-motion-20261018-2-final/`。
- 上传回执：`.codex-tmp/upload-2026.10.18.2.json`。

## Task 1: 确认商家任务真正完成

- [x] 找到源任务 `01a14d4a-5356-7931-ab05-d68af7d7f48d`，标题“审查并总结待优化项”。
- [x] 发现初步提交 `d34dcc3` 和合并 `4831758` 后，源任务仍在编写身份申请测试及调整样式。已生成的非 final 冻结包没有上传，不能作为本轮最终交付。
- [ ] 使用 `wait_threads` 等待当前轮完成或需要处理的问题；等待期间不再修改源任务的文件和 Git 索引。
- [ ] 读取完成报告及 `docs/reviews/2026-10-18-identity-application-ui-refresh.md`，核对终稿提交和验证边界。
- [ ] 在源工作区执行 `git status --short --branch`、`git log -3 --oneline`，确认待交付源码已经提交。未提交代码不直接纳入发布。

## Task 2: 完成最终合并

- [ ] 在整合工作区将本计划提交为文档检查点。
- [ ] 记录源提交完整 SHA，再执行 `git merge --no-ff codex/delivery-workspace-redesign-20261018 -m "整合商家端完成版与动态媒体功能"`。若存在冲突，保留视频链路与商家端各自业务语义后逐项处理。
- [ ] 执行 `git diff b21b97c HEAD --name-only -- backend apps/miniapp/src/services apps/miniapp/src/stores apps/miniapp/src/app.config.ts apps/miniapp/project.config.json`；本轮商家 UI 整合预期这些接口、后端与路由文件无变化。
- [ ] 确认 `post-motion-media.tsx`、`motion-preview.ts`、`047_post_motion_media.sql` 仍保留，执行 `git diff --check`。

## Task 3: 在合并提交上验证

- [ ] 执行 `npm run check:identity-application-ui`、`npm run check:identity-application-contract`、`npm run check:profile-presentation`，预期退出码 0。
- [ ] 执行 `npm run check:delivery-stage-workbench`、`npm run check:merchant-home-clarity`、`npm run check:rider-workbench-clarity`、`npm run check:role-workbench-ui`，预期退出码 0。
- [ ] 执行 `npm run test:post-motion`、`npm run check:media-review-policy`、`npm run check:post-presentation`，预期退出码 0。
- [ ] 执行 `npm run check:miniapp-static-routes`、`npm run check:miniapp-publish-contract`、`npm run check:miniapp-business-closure`、`npm run check:profile-center-ui`、`npm run check:feature-policy`、`npm run typecheck`，预期退出码 0。
- [ ] 执行 `npm run build:miniapp`，预期退出码 0；记录 CSS 顺序与包体警告，不把构建成功当作真机验收。

## Task 4: 冻结并上传

- [ ] 从已验证提交的生产构建制作新 final 冻结目录，不覆盖 `2026.10.18.1` 或此前的初步包。校验 AppID、30 条路由、真实 API `https://zhanduiwang.top/api/v1`、身份入口和动态媒体组件均存在，且没有演示数据、模拟登录或密钥。
- [ ] 记录文件数、字节数、完整提交 SHA 和文件树 SHA-256。
- [ ] 执行 `& 'D:/3314/微信web开发者工具/wechatidecli.cmd' islogin`。有效登录继续使用；仅当需要扫码时请求用户身份验证。
- [ ] 执行 `& 'D:/3314/微信web开发者工具/wechatidecli.cmd' upload --project 'C:/Users/33141/.codex/worktrees/a494/New project/.codex-tmp/release-merchant-post-motion-20261018-2-final' --version '2026.10.18.2' --desc '优化体验' --info-output 'C:/Users/33141/.codex/worktrees/a494/New project/.codex-tmp/upload-2026.10.18.2.json'`。必须收到成功退出码及新回执后才能声明上传成功。
- [ ] 复核冻结清单未改变，保存上传结果、最终合并提交和各项测试证据到运维记录，单独提交文档。
- [ ] 向用户报告开发版本号、包含的商家修改及仍需手机验证的项目。本轮不自动提交审核、发布上线或变更功能开关。

## 自审

完成信号来自源任务报告和最终提交，不以短暂干净工作区或类型检查代替。用户要求等待完整修改，因此初步冻结包保留仅用于追踪，不提前上传。合并后的跨模块验证覆盖商家、身份申请、个人主页和动态媒体；云授权、生产数据库迁移和审核策略不再重复执行。
