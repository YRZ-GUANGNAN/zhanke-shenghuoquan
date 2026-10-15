# 个人主页与编辑资料实装对齐计划

**Goal:** 落实用户确认的预览方向，修正体验版封面取景、渐变、昵称及编辑按钮与预览不一致的问题。

**Architecture:** 主页和封面编辑组件共享 SCSS 比例、底部渐变规则。资料仍属于原 hero，但照片区域不再随昵称和资料高度伸长；现有页面导航、接口、账号隔离、上传及独立保存流程保持不变。直接从实际 JSX/SCSS 生成本地验收页面。

**Tech Stack:** Taro、React、SCSS、现有 Node 回归检查与独立浏览器布局检查。

## 已确认设计与依据

- 用户已确认“统一取景、缩短渐变、用户名回到白底、对齐蓝色编辑按钮”。这是本项目的设计选择，不作为第三方产品精确尺寸声明。
- 阅读参考库 4c90abf（2026-10-15 第十轮），结合第九轮个人页/编辑资料/背景图文档与观察数据；采用层级、分组、取消返回原则。不移植 500×400 字面量、第三方素材或未验证的裁剪/吸顶操作。
- 共享照片高宽比 0.64：常见 390px 宽时约 250px 高；编辑区按自身宽度同比缩放，两处保持 `aspectFill` 居中裁切。
- 照片底部 22% 为短渐变；头像 144rpx，跨照片下沿 64rpx；昵称/用户号在白底。胶囊和工具栏使用现有安全区，极端状态栏通过最小间距兜底，不压住按钮。
- 编辑资料按钮使用现有品牌蓝和白字，至少 44px 触控高；保留所有原功能和入口。
- 编辑页按头像、昵称、主页背景排列；资料保存与背景独立保存保持分开。选图预览注明与主页相同取景。

## 实施步骤

- [x] `apps/miniapp/src/styles/profile-cover-surface.scss`：共享 `$height-ratio: .64`、对应 rpx 高度、媒体框 mixin 和 22% 底部淡出。
- [x] `apps/miniapp/src/pages/profile/index.scss`：照片仅占共享媒体框；头像跨下沿；昵称白底；品牌蓝编辑按钮。保留白字标题和无图深色回退，减少状态栏区域的白色蒙层。
- [x] `apps/miniapp/src/components/profile-cover-editor.scss` 与 `.tsx`：使用同一比例和淡出；调整文案及动作区域，保留完整保存/恢复/失败重试行为。
- [x] `apps/miniapp/src/pages/edit-profile/index.tsx` 与 `.scss`：调整字段顺序和间距，让头像昵称优先；使用已有接口，不暴露学校字段。
- [x] 运行 `node scripts/check-profile-cover.cjs`、`node scripts/check-profile-presentation.cjs`、`node scripts/check-profile-center-ui.cjs`、`node scripts/check-student-entry-avatar-contract.cjs` 和小程序 TypeScript；生产构建验证。
- [x] 从真实 JSX/SCSS 生成新本地预览，核对 320/390/430px、不同安全区、长昵称、无图/深浅中性底、44px触控、照片比例一致和无横向越界；不生成假照片。
- [x] 复查本次文件差异与其他脏文件，保存验证结果。只完成本地实现，不在本次自动上传/提审/发布或提交 Git。

## 验收边界

浏览器布局检查不替代微信真机验收。上一版 `2026.10.15.3` 已上传；本计划的修改须明确标注尚未上传。原项目、后端和媒体策略未完成改动不纳入本次。

## 验证结果

- 4 项个人资料行为/契约检查与小程序 TypeScript 全通过。
- 54 组浏览器布局检查通过：320/390/430px × 状态栏24/48/64px × 无图/深浅中性底 × 主页/编辑页。检查包含长昵称不越界、昵称在封面边界下方、按钮对齐且44px触控高、两处媒体比例相同、编辑字段顺序。
- 独立生产构建 `.codex-tmp/profile-layout-build-20261015/apps/miniapp/dist` 成功，335 源文件仅本次6文件相对已上传 `.3` 不同；129 编译文件仅编辑页 JS/WXSS、个人页 WXSS 有差异。AppID、正式API及无本地接口检查通过，旧发布包完好。
- 构建仍有既有 vendors.js 731KiB / NoAsyncChunksWarning 提示。
- 可视化目录 `C:/Users/33141/.codex/visualizations/2026/07/28/019fa89e-0db5-7d30-bbae-fcf9d7f9fcc7/profile-layout-alignment-20261015`：`index.html` 为真实 JSX/SCSS 双屏预览，可选本地照片且不上传；`paired-preview.png` 为无照片总览；`layout-report.json` 保存54项结果与源码摘要。
- 本地查看地址 `http://127.0.0.1:5191/` 已验证HTTP 200。实际照片构图、微信原生字体/相册/安全区仍待真机验收。
- 没有生成图片、上传微信、提审、发布、Git提交或后端部署；原项目仍仅原有3个未跟踪计划文件。
