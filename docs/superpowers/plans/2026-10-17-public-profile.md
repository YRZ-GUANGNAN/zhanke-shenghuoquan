# 公开个人主页统一 Implementation Plan

> 本次沿用用户已授权的隔离分支自主实施，在当前任务逐项完成并验证，不启动额外代理。

**Goal:** 帖子作者与二手卖家均可进入同一带真实封面的个人主页，入口默认选中相应内容频道。

**Architecture:** 延用 `/pages/user-profile/index`、已有权限及模块守卫；公开 DTO 加入已存在的 `cover` 字段。公开主页复用 `profile-cover-surface.scss` 的裁切比例与头像跨封面底边规则，保留分页、滑动、关注、分享、举报、拉黑与自己的编辑入口。

**Tech Stack:** Taro / React / TypeScript / SCSS / Go；组件行为回归与微信构建验证。

## 设计决定与自审

- 首页、帖子详情、二手列表及详情中的头像昵称均为主页入口，入口阻止冒泡，避免同时打开详情；点击防重，失败可重试。
- 同一公开主页展示同一真实封面、头像、昵称及用户号，不再显示已取消的学校资料。帖子默认 `tab=posts`，二手默认 `tab=secondhand`。
- 复用“我的”封面 0.64 宽高比例、轻渐变、144rpx 头像和白底深色资料区，不改现有主题色，不加入生成图片。
- 后台没有封面或图片失败时使用现有主题浅底，头像失败显示昵称首字。加载及错误时也保留返回，单页进入可返回首页。
- 用户主页仍受 forum 开关控制；二手关闭后不请求商品。原有行为不因 UI 统一而扩大权限。
- 不改原项目、不迁移数据库、不发布；本地预览使用已有授权摄影素材，仅位于可视化目录。

## 实施步骤

- [x] 在 `backend/internal/module/user/model.go` 的 `Profile` 和 `User.Profile()` 增加公开封面映射：`Cover string json:"cover"`、`Cover: u.Cover`；关闭账号继续返回脱敏资料。扩展 community public profile 和 user 投影回归。
- [x] 在 `apps/miniapp/src/services/b-community.ts` 的 `PublicCommunityProfile.profile` 添加兼容旧接口的 `cover?: string`。
- [x] 在 `apps/miniapp/src/utils/navigation.ts` 增加校验 ID、默认频道和防连点的公开主页导航。入口格式：`/pages/user-profile/index?userId=7&tab=posts`。
- [x] 补齐 home、post-detail、student-secondhand/content 和 secondhand-detail 的作者入口；事件先执行 `event.stopPropagation()`，再检查模块、滑动状态和用户 ID。头像与昵称是同一点击区域，关注和内容卡片仍独立。
- [x] 修改 `pages/user-profile/index.tsx` / `index.scss`，复用封面样式，加入图片错误回退，保留导航、统计、操作及内容功能。错误状态使用独立重试按钮。顶部内容在短屏也可滚动，频道内容滚动位置分别保存。
- [x] 对真实组件验证：头像昵称导航、无效 ID / 重复点击 / 失败重试、两种默认频道、封面缺失 / 失败 / 更新、自己与他人操作、分页与返回。
- [x] 执行 `npm --workspace apps/miniapp run typecheck`、相关 profile/post/secondhand/channel/navigation 检查及 `npm run build:miniapp`；Go 只运行不依赖生产数据的定向包测试。
- [x] 用真实页面组件制作本地预览，检查 320 / 390 / 430px 与短屏，截图主页帖子和二手两种状态，确认封面裁切一致、无横向溢出、入口可点。

## 交付边界

当前基线 `343ad6f`，分支 `codex/public-profile-20261017`。仅提交本次明确文件；已有 artifacts 与无关计划不纳入。说明后端新增投影需随版本同步，微信真机验收和发布保持待执行。
