# 全页面参考规范实施计划

> **For agentic workers:** 按三个互不重叠的页面组实施；主代理审查公共规范、路由覆盖与回归。既有封面 release 代理继续在独立工作区，不混合前端发布。

**Goal:** 将参考库 59f33ad 的已确认排版规律应用到全部在用小程序页面，保留咨询例外、颜色和功能。

**Architecture:** 纯 Sass 的 reference-layout.scss 提供 type、wrap、page-foundation 三个 mixin。各页面显式引用；不改变全局 page、现有 design-tokens 色值与共享咨询组件。页面局部 selectors 负责语义映射。

**Tech Stack:** Taro / React / SCSS / Node 静态回归 / Chrome 布局验证。

## 1. 公共规范与覆盖测试

- [ ] 新增 `apps/miniapp/src/styles/reference-layout.scss`，type($role) 以 map 选择 title/body/comment/author/meta/label；wrap 保留长文本；page-foundation 仅作用域内设置状态/表单/按钮的宽度与行高，不改变颜色。
- [ ] 新增 `scripts/check-reference-layout.cjs`：读取 app.config.ts，27 个目标路由必须主动引用，campus-guide 不得引用；编译每份 Sass 验证无未定义 mixin。检查当前蓝色 token、评论无新增折叠、布局只用实际容器宽。
- [ ] 注册 `check:reference-layout` 并纳入 preverify。先观察未接入页面时失败，再完成各页面适配。

## 2. 内容页面

- [ ] home、search、student-secondhand、post-detail、secondhand-detail、user-profile、my-content：逐页引用规范，分别处理标题/作者/辅助信息、长文本、窄屏可点区域。咨询 home 分支不接入。
- [ ] 复用现有 post/secondhand/gallery/search/profile 测试，不能改变请求、排序、过滤、计数、权限和路由。

## 3. 工作台与服务

- [ ] student-takeout、secondhand-chat、merchant-home、merchant-publish、merchant-wallet、merchant-messages、merchant-order-detail、delivery-rider、delivery-conversation：逐页映射标签/金额/说明/消息，修复长字段挤出和输入触控高度。
- [ ] 保持订单和消息业务代码不变；执行现有 role-workbench、rider/merchant、publish-takeout、clipboard 检查。

## 4. 账户与发布

- [ ] profile、edit-profile、settings、blacklist、feedback、about、legal、identity-application、publish-entry、publish-post、publish-secondhand：引用规范，按原语义改标签/正文/说明/表单尺寸与安全区。
- [ ] 完成既有 `profile-cover-upload-repair.md` 的前端修复，让两项已失败的封面/资料测试转绿。

## 5. 交付

- [ ] 320/390/430px 实际布局抽取检查，不调用接口、不添加生产测试记录、不生成照片。
- [ ] `npm.cmd run verify`、`npm.cmd run build:miniapp`、`git diff --check`。
- [ ] 审查差异：原项目、咨询、角色/路由/API/功能开关、两份用户配置保持不变。只暂存本轮明确文件并中文提交，不自动推送或发布微信。
- [ ] 在设计说明中逐路由记录应用内容、验证通过/未验证边界；生产封面发布结果独立报告。
