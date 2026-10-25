# 身份切换与统一相册入口 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 将用户批准的弹窗和发布入口落地到正式 Taro 代码，保持我的内容及原图标不变。

**Architecture:** AppDialog 仅新增可选 className 用于身份专属样式，不修改共享布局。媒体服务按选择结果分流至原图片/视频上传实现；页面保留审核与提交状态机。测试使用 Taro/API 桩，无生产写入。

**Tech Stack:** Taro 4.2、React 18、TypeScript、SCSS tokens、Node assert/JSDOM。

---

## Task 1: 统一媒体选择服务及行为测试

**Files:**
- Modify: `apps/miniapp/src/services/media.ts`
- Create: `scripts/test-post-media-picker.cjs`

- [x] 新增失败测试覆盖新接口：

```ts
export type UploadedPostSelection =
  | { kind: 'images'; images: UploadedMedia[] }
  | { kind: 'motion'; motion: UploadedPostMotion }

export async function chooseAndUploadPostMedia(
  count: number, allowMotion: boolean,
  onProgress: (message: string) => void, shouldContinue: () => boolean
): Promise<UploadedPostSelection>

export async function chooseAndAttachPostLiveClip(
  poster: UploadedMedia, onProgress: (message: string) => void,
  shouldContinue: () => boolean
): Promise<UploadedPostMotion>
```

测试使用真实 media.ts 编译模块，桩化 Taro 文件读取/相册和 API 上传，断言：空选择无上传；混选/多视频/未知类型先拒绝；能力关闭只图片；已选文件不重复弹相册；0.5–60 秒/30MiB/MP4 校验；单图配片段复用 poster；取消与卸载不继续上传。

- [x] 执行 `node scripts/test-post-media-picker.cjs --baseline` 确认旧版本缺少接口时失败。
- [x] 原 `chooseAndUploadPostMotion` 取出选择逻辑后委托私有 `uploadPostMotionClip`，保留原验证、上传、complete 超时恢复。新选择函数先验证整个选择集合，再上传图片（并发上限 3）或单视频。配片段路径只选择一次 video，复用 poster。
- [x] 再运行新测试及 `npm run test:post-motion`，全部 PASS。

## Task 2: 发帖页面接入

**Files:**
- Modify: `apps/miniapp/src/pages/publish-post/index.tsx`
- Modify: `apps/miniapp/src/pages/publish-post/index.scss`
- Modify: `scripts/check-post-presentation.cjs`

- [x] 页面调用 `chooseAndUploadPostMedia(MAX_IMAGES - images.length, motionEnabled && images.length === 0, progress, () => alive.current)`；按返回 kind 分别更新 images/motion。选择前锁定，取消不报错，finally 解锁，所有异步更新校验 alive。
- [x] 只在单图且动态能力开启后显示「添加动态片段」，调用 `chooseAndAttachPostLiveClip(images[0], progress, () => alive.current)`；成功后原图片迁入 motion.poster，失败保留 images。
- [x] 去除常驻两类媒体按钮和长说明，上传状态使用独立提示，+ 使用浅底、圆角与按下缩放；保留四列、正文/标签/提交顺序，不添加发布确认层。
- [x] 增加页面测试：双击只打开一次相册、照片/视频显示分支、配片段成功/取消、能力关闭、上传取消、错误草稿保留、卸载后不得写入。
- [x] 执行 `node scripts/check-post-presentation.cjs` 与 `node scripts/check-miniapp-publish-contract.cjs`，全部 PASS。

## Task 3: 身份弹窗

**Files:**
- Modify: `apps/miniapp/src/components/IdentitySwitchModal.tsx`
- Modify: `apps/miniapp/src/components/IdentitySwitchModal.scss`
- Modify: `apps/miniapp/src/components/ui/app-dialog.tsx`

- [x] AppDialog 增加 `className?: string`（默认空），仅附加到 layer。IdentitySwitchModal 传 `identity-switch-dialog`；不改 onConfirm/onCancel/confirming/error。
- [x] 使用原 AppIcon，将名称/状态合为右侧文字组。角色卡浅灰/淡蓝、无额外白色图标底与重阴影；44px 以上按钮、320px 换行、退出期间交互隔离继续由 AppDialog 处理。
- [x] 所有 .app-dialog/.app-button 覆盖均位于 `.identity-switch-dialog` 作用域内，原公共样式不变。
- [x] 执行 `npm --workspace apps/miniapp run typecheck`、`node scripts/check-layer-presence.cjs`，全部 PASS。

## Task 4: 回归与交付

- [x] 运行 `npm run verify`、`npm run build:miniapp`、`git diff --check`；全仓验证与构建均通过，构建仅有既有 webpack 体积/样式顺序警告。
- [x] 检查 `git diff --name-only`，确认没有 profile、SVG、后端、业务开关改动；保留原有三份未跟踪文档。
- [x] 只提交本次源文件、测试及两份说明；中文提交 `优化：收敛身份弹窗与帖子统一相册入口`，不 push。
- [x] 回报本地构建/提交与微信上传、审核、发布之间的区别；原生实况动态段与真机交互仍待验证。

## 执行方式

当前环境未提供计划中引用的 subagent-driven-development / executing-plans 技能，按本计划在当前会话执行，弹窗与测试可独立并行，主代理集成并检查范围。用户已确认设计并要求落地，不再重复询问执行方式。
