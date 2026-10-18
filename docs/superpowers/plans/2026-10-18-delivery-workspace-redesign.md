# 商家与骑手整体改版实施计划

**Goal:** 在独立工作树完成可运行、可预览的商家与骑手工作区整体改版，学生端代码与正在进行的开发不受影响。

**Architecture:** 保留现有 Taro 页面行为与服务合同，重排角色专属 JSX 和样式；以已有分组和真实数量组织任务。只抽出角色专属视觉样式，不改全局 UI 或业务层。

**Tech Stack:** Taro / React / TypeScript / SCSS，既有隔离组件行为测试及布局预览工具。

## 1. 隔离与基线

- [x] 用 `ab08e57996a0cb36e1aa62462e4a0c0d60b0dad5` 创建独立工作树及 `codex/delivery-workspace-redesign-20261018`。
- [x] 核对学生端改动：home/post-detail/publish-post、community/media/taro/types 及后端动态媒体链路，均排除。
- [x] 独立安装锁定依赖，执行三个已有配送行为测试作为基线。

## 2. 商家订单工作区

文件：`apps/miniapp/src/pages/merchant-home/index.tsx`、`index.scss`。

- [x] 将分组文案设为 `pending: 待处理`、`delivering: 配送中`、`completed: 已结束`，请求和服务端 total 不变。
- [x] 重排身份栏、摘要、发单入口及双列队列；订单卡按平台/取餐号、状态、地址摘要、费用/操作排列。
- [x] 所有待付费与支付失败卡仍从详情处理；不把仅打开详情的动作写成直接支付成功。

## 3. 骑手任务工作区

文件：`apps/miniapp/src/pages/delivery-rider/index.tsx`、`index.scss`。

- [x] 用 `activeTab === 'grab'` 表示大厅，其他值表示配送；大厅只展示待抢单，配送显示待取货/待送达。
- [x] 保留底栏本地切换、身份变化保护、原三列表加载与分页，重排卡片识别信息、路线及动作。
- [x] 将危险动作与主动作明确分层，保留订单内进度与结果待确认提示。

## 4. 配套页面与专属样式

文件：`merchant-order-detail/index.tsx`、`index.scss`，`merchant-publish/index.tsx`、`index.scss`，`delivery-profile/index.tsx`、`index.scss`，`components/delivery-workspace-nav.scss`。

- [x] 重排详情首屏摘要、路线、费用、时间线及凭证章节，保留全部展开/预览行为。
- [x] 统一发单页章节、表单和固定提交区，保留输入值与原提交方法。
- [x] 重新组织角色我的页与资金/身份入口；底栏统一触控尺寸、选中态与安全区。

## 5. 回归、范围与视觉验收

- [x] `node scripts/test-delivery-stage-workbench.cjs`：履约、凭证、资格、分页与恢复原用例保持通过。
- [x] `node scripts/test-delivery-publish-feedback.cjs`：创建幂等、支付/导航失败与账号切换保持通过。
- [x] `node scripts/test-delivery-workspace.cjs`：路由、身份、消息与资料保持通过。
- [x] 为两种骑手视图及商家真实分组补充行为检查，更新原布局专用断言，保留原业务断言。
- [x] `npm run typecheck`、相关布局/路由/业务闭环合同、小程序构建、`git diff --check`。
- [x] 将相对基线的改动路径与白名单比对；学生页面、共享服务、全局样式、配置和后端应无差异。
- [x] 使用实际 TSX/SCSS 渲染独立样本，检查 320/390/430 宽度，逐张查看核心页面。保留布局指标及截图。

## 6. 交付

- [x] 本地提交代码及验证记录，打开独立预览，说明本轮结果、遗留问题和真机验收边界。
- [x] 不推送、不部署、不上传微信、不合并原学生端工作树。
