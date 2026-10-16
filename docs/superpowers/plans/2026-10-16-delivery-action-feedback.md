# 配送动作结果与恢复实施计划

> 执行方式：在已授权隔离分支按下面任务并行实现，主任务逐项审查与集成；不额外引入不可用的执行技能。

**Goal:** 完善现有配送动作在重复点击、网络未知、订单变更和跳转失败时的安全恢复。

**Architecture:** 骑手保持按单锁及生命周期 guard，通过现有读取接口确认快照；商家以页面内创建尝试保存幂等 key、表单和已创建订单。业务、刷新和导航结果分层；不改服务合同。

**Tech Stack:** Taro、React、TypeScript、现有 CJS 隔离 Hook 行为测试。

## 任务 1：骑手动作恢复

文件：`apps/miniapp/src/pages/delivery-rider/index.tsx`；测试：`scripts/test-delivery-stage-workbench.cjs`。

- [x] 先增加复现断言：写请求超时且读取失败后重复点击，`writeCalls === 1`；释放拒绝后 `dialog.visible === true`；审核期间状态或版本改变后 `proofWriteCalls === 0`。
- [x] 运行 `node scripts/test-delivery-stage-workbench.cjs` 确认新情境在旧代码失败。
- [x] 将动作返回值区分成功、失败、同步/中止；未知写入结果保留同步门，释放失败保留对话框。在审核后/释放前核对既有详情 ID、状态、版本；读取失败不继续写入。
- [x] 覆盖卸载、身份变化及同帧点击，不让旧结果修改新会话。保留旧 harness 导出供任务 2 复用。
- [x] 重跑行为测试及 `npm run check:rider-workbench-clarity`。

## 任务 2：商家发布动作会话

文件：`apps/miniapp/src/pages/merchant-publish/index.tsx`、必要的 `index.scss`；新增测试 `scripts/test-delivery-publish-feedback.cjs`。

- [x] 用导出的 `hookRenderer/loadPage` 挂载真实页面，先复现同帧创建两次、支付成功后导航失败误报、微信取消与 mock 非 paid 假成功。
- [x] 保存提交尝试的 snapshot/key，使用 ref 同步锁；未知创建结果手动重试仍是同一个请求，仅从未未知的明确拒绝后可编辑重建。断言连续点击 `createCalls === 1`，重试请求号及输入完全一致；超时后409仍保留原请求号。
- [x] 创建成功后保存订单，分别判定支付状态；导航放在独立错误处理。导航重试断言 `createCalls === 1 && paymentCalls === 1`。
- [x] 补齐取消、支付待确认/失败、已创建订单恢复入口、表单锁定说明及页面/身份失效检查。对 mock poll 的 failed/closed/pending 断言无成功提示；同账户恢复沿用请求号、换账户清除敏感表单。
- [x] 运行 `node scripts/test-delivery-publish-feedback.cjs`、`npm run check:delivery-order-clipboard` 与 `npm run check:miniapp-business-closure`。

## 任务 3：集成与交付

文件：`package.json`、本计划及设计文档。

- [x] 将新测试接入 `check:delivery-stage-workbench`，现有 `verify` 自动覆盖。
- [x] 独立审查真实错误分类、既有类型和支付响应；检查没有绕过角色、功能开关或 rowVersion。
- [x] 执行 `npm run verify`、`npm run build:miniapp`、`git diff --check`。必要失败仅修本轮直接相关处，不削弱原检查。
- [x] 记录结果和真机待验项，仅暂存本轮源码、测试、文档及 package，提交中文 Git 说明；不加入历史 `artifacts/`。

## 任务 4：追加授权的工作台排版

文件：上述骑手和发布页面，加 `apps/miniapp/src/pages/merchant-home/index.tsx/index.scss`、`apps/miniapp/src/pages/merchant-order-detail/index.tsx/index.scss`。

- [x] 骑手任务同步压缩头部和分离次要操作/履约主操作；商家发布任务同步纵向输入分区及固定反馈。
- [x] 商家布局任务单独维护首页和详情，保留所有入口与状态处理，减少顶部装饰并将阶段分组和订单优先呈现。
- [x] 主任务用真实 TSX 渲染树及编译 SCSS 生成隔离预览，320/390/430 宽度检查。仅测试样本，无真实订单或接口请求。
- [x] 构建及原工作台合同检查一起通过后交付截图与本地版本；不将浏览器布局截图宣称为微信真机截图。

## 自审

三个并行任务各自拥有独立页面；旧测试由骑手任务维护，新测试由商家发布任务维护，商家首页/详情任务负责独立排版，root 维护 package、文档与可视化验收。行为要求均有对应故障注入断言；不需新 API 或后端部署。

## 2026-10-16 验证结果

- `npm run verify`：PASS，包含整仓合同、界面、双前端类型检查及隔离后端测试。
- 配送行为测试：工作台 21 组、发布 21 组，共 42 组 PASS。
- `npm run build:miniapp`：PASS；Webpack 仍提示公共 `vendors.js` 731 KiB 超过其推荐阈值及无异步 chunk，非构建错误。
- `git diff --check`：PASS。
- 可视化：真实 TSX 渲染树与 SCSS，在 320×568、390×844、430×844 验证四页及发布结果反馈，15 组通过，无横向越界、触控不足或四字标签断行。主任务已查看页面截图。
- 预览与 PNG：`C:/Users/33141/.codex/visualizations/2026/07/28/019fa89e-0db5-7d30-bbae-fcf9d7f9fcc7/delivery-action-layout-20261016/`，本地入口 `http://127.0.0.1:5193/`。数据是明确标注的隔离测试样本，不调真实服务，不属于生产构建。
- 真机待验：弱网、系统相册返回、微信付款页取消/返回、不同手机安全区。没有实际付款、生成订单、上传微信、推送 GitHub、送审或发布。
- 原项目及其他 worktree 未变更；历史 `artifacts/` 保留且不纳入提交。
