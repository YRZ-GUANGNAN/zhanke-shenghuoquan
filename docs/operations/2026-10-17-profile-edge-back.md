# 公开主页手势返回更新

2026-10-17，基于前端分支 `codex/public-profile-20261017` 增加公开主页边缘手势返回。改动只作用于公开主页展示层，不改接口、身份守卫、内容开关、帖子／二手数据或后端。

## 行为

- 从左侧 24px 内开始，单指向右滑；超过 8px 后锁定方向，横向距离须大于纵向距离的 1.5 倍，完成阈值按窗口宽度取 24%，并限制在 72–120px。
- 短滑、左滑、纵向滚动、多指、触点更换、取消和弹窗期间均不会由应用手势触发返回。透明返回箭头最多跟手 12px，松手或取消后归位。
- Android、开发者工具及分享直达页面使用应用内手势，统一调用已有 `navigateBackOrHome`，无上一页时回首页。iOS 检测到已有页面栈时保留微信系统侧滑，避免应用手势重复退栈。
- 原有帖子／二手中间区域横滑继续由 `ChannelPager` 处理，边缘触摸层位于分页器外。

## 验证

以下检查通过：`npm run check:profile-edge-back`、`npm run check:profile-presentation`、`npm run check:channel-pager`、`npm run check:miniapp-back-navigation`、`npm run check:reference-layout`、`npm --workspace apps/miniapp run typecheck`、`npm run build:miniapp`。

5197 本地真实组件预览已验证 17 个页面场景，以及短滑／纵向／中央横滑／取消／弹窗保护／真实触摸返回／鼠标拖动返回／减少动态效果；无横向溢出、卡片重叠、脚本错误或外部网络请求。预览地址：`http://127.0.0.1:5197/screen.html?page=profile&userId=7&tab=secondhand&rev=edge-back`。

本次只完成隔离分支代码与本地预览验证，微信 iOS／Android 的系统手势真机手感仍待验收。未上传新的微信开发版、未设置体验版、未送审或正式发布；上一版 `2026.10.17.2` 的上传状态不变。
