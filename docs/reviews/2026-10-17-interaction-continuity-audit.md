# 横滑连续性与发布入口自审

基线：`bde6a01`；方案提交：`3704cf5`；隔离分支：`codex/loading-feedback-polish-20261017`。

## 本轮体验调整

1. **横滑后切后台再回来可继续操作。** 共享 ChannelPager 在页面隐藏、恢复及卸载时清理旧手势状态。隐藏时到达的切换事件、恢复后迟到的 transition/change/animationfinish 不会继续切栏目或重新锁住点击；新横滑可以继续使用。首页、他人主页、我的内容共用此实现，仍保留原生 Swiper、200ms 时长、既有子树及各栏目滚动位置。
2. **发布选择页连续点击只进入一个编辑页。** “发帖子”和“卖闲置”共用同步请求锁；打开失败提示并允许重试，返回后解锁，旧失败回执不影响新的打开操作。请求期间返回不与打开编辑页并发，普通返回沿用首页兜底。
3. **减少动态效果模式不再高速播放旧动画。** 发布卡片取消按压缩放与过渡；后台旧样式层和补全层都直接禁用 animation/transition，取消按钮装饰性位移，保留功能开关位置及静态加载状态。

原有配色、页面布局、功能、业务接口、权限和开关均未调整。未新增图片、业务数据或依赖。未触及咨询页。

## 回归证据

- 先在旧实现复现：隐藏期的滑动回调错误地将首页切至工具（`2 !== 0`）；发布页同帧点击不同入口发出三次导航（`10 !== 8`）。修复后对应真实组件回归通过。
- `check:channel-pager` 覆盖缺少 touchend 的隐藏/恢复、旧事件、新手势、卸载回调、滑后误触保护、栏目保活、账号及功能开关变化。
- `check:tactile-navigation` 覆盖连点、同步/异步失败重试、返回解锁、旧失败不解锁新请求、成功后的过渡锁、隐藏/卸载、返回互斥与关闭功能。原有导航热区经 Sass 和 Taro PostCSS 在 320–768px 尺寸计算中保持至少 44px。
- `check:post-presentation`、`check:profile-presentation`、`check:miniapp-regression-fixes`、`check:student-entry-avatar-contract` 通过。帖子检查装配真实 ProductPrice、纯函数和 ChannelPager；RefreshScrollView/ToolDirectory 等非本检查目标的展示组件使用隔离替身，相关滚动/工具逻辑由原有专项检查覆盖。
- 旧检查更新为真实结构：首页检查共享 RefreshScrollView 最终使用原生 ScrollView 且透传滚动/分页属性；校园服务检查实际骨架屏、空态、分类空态和错误重试，不再要求已不使用的旧加载图标。
- `preverify` 全部检查通过一次；本轮产品修改后，受影响的频道、发布、帖子、个人主页检查已分别重跑。项目 `verify` 主链在修正旧断言后继续执行尾部；除了下面的基线问题，其余检查、两端 TypeScript 类型检查和安全模式下的 Go 单元测试均通过。Go 检查清空 MySQL 测试连接，不等同于数据库集成验收。
- `npm.cmd run build` 通过，小程序与后台均生成产物。产物包含 30 个业务页面，无临时检查页；使用原默认 `https://zhanduiwang.top/api/v1`，包内默认分享图存在。发布页 reduced-motion 样式和后台两层 `animation: none !important` 已核验进入构建。
- `git diff --check` 通过；后端、生产构建配置和校园指南页面相对基线无差异。

## 仍需单独处理或验收

- **基线遗留检查失败：** `check:boundaries` 报 `database access outside repository: backend/internal/module/delivery/external_intake.go`。此文件持有 `*gorm.DB` 并在 Receipt/Retry 中直接查询；基线 `bde6a01` 的源码和检查器已有同样内容。本轮后端文件无差异，也未放宽边界检查。后续应独立整理持久化职责并做数据库集成回归，不能将总验证标成全部通过。
- **既有构建建议：** 小程序仍报告公共 CSS 导入顺序冲突及 `vendors.js` 约 732 KiB 的建议警告；构建退出码为 0。本轮未通过忽略警告或修改分包来掩盖它们。
- **真机待验：** 微信切后台再回来、系统边缘返回、连续快速横滑、减少动态效果、低端设备手感。本轮证据为源组件行为测试与编译产物，没有把浏览器模拟或构建成功当作真机通过。

## 隔离与交付

原项目 `C:/Users/33141/Documents/New project` 仍为 `e79e168`，没有已跟踪文件修改，仅有原先三个未跟踪计划。当前工作区既有 artifacts 和配送迁移计划保持原状，均不纳入本轮提交。

本轮只保存本地代码和审查记录；未上传微信、推送 GitHub、部署服务、执行数据库迁移或修改线上配置。
