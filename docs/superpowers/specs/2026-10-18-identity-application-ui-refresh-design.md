# 商家与骑手身份申请界面重做设计

日期：2026-10-18  
状态：用户已批准，准备实施

## 目标

优化商家端和骑手端身份申请的入口弹窗与申请详情页，让用户在提交、审核中、通过、拒绝和读取失败时都能看懂当前状态、下一步动作和资料是否可编辑。改造只调整 MiniApp 的表现层与交互承载方式，保留现有身份申请 API、媒体隐私规则、身份守卫和后端状态机。

## 范围与边界

本轮只处理以下用户路径：

1. 用户在“我的”点击未开通或需要完善的商家/骑手身份；
2. 入口状态弹窗展示申请摘要并引导继续申请或查看详情；
3. 身份申请详情页展示资料、证明图片、审核事件和底部操作；
4. 详情页中的加载、空申请、错误、上传、审核中、成功和失败反馈。

允许修改的文件范围：

- `apps/miniapp/src/pages/profile/index.tsx`：只调整商家/骑手身份卡片的入口分流；
- `apps/miniapp/src/components/IdentityApplicationEntryModal.tsx`；
- `apps/miniapp/src/components/IdentityApplicationEntryModal.scss`；
- `apps/miniapp/src/components/identity-application-summary.tsx` / `.scss`：申请专属状态展示；
- `apps/miniapp/src/pages/identity-application/index.tsx`；
- `apps/miniapp/src/pages/identity-application/index.scss`；
- `scripts/check-identity-application-ui.cjs`；
- `scripts/test-identity-application-ui.cjs`：真实组件的隔离行为回归；
- `package.json`：只在已有 `verify` 中加入新的 UI 合同检查。

不会修改后端、迁移、身份 API、媒体服务、共享设计 token、学生内容页面、学生底部导航、配送工作台和当前学生端进行中的工作树。当前实现使用独立工作树，构建产物和预览文件不写回主工作树。

## 现状问题

- “我的”对 `missing` 身份直接导航，对 `pending`/`rejected` 只显示 Toast，入口没有解释申请状态和下一步。
- 申请页使用渐变头图、状态卡、资料卡和证明卡的旧式堆叠，状态信息没有统一的时间线和详情层级。
- 旧加载状态只有一行文字，错误、重试和提交结果的视觉承载不一致。
- 历史证明使用文字占位，未把私有媒体限制表达为稳定的锁定图标容器。
- 提交操作随页面内容滚动，长地址、长审核原因和窄屏下主操作不稳定。

## 设计方案

### 1. 入口状态弹窗

新增 `IdentityApplicationEntryModal`，使用现有 `AppDialog` 作为遮罩、层级、关闭和按钮容器。弹窗打开时按目标角色请求 `getIdentityApplication(role)`：

| API 状态 | 弹窗内容 | 主动作 |
| --- | --- | --- |
| 404 | “还没有申请”与角色说明 | 开始申请 |
| pending | 审核中徽标、申请编号、提交时间、等待说明 | 查看详情 |
| rejected | 需完善徽标、拒绝原因、最近审核时间 | 修改资料 |
| approved | 已通过徽标、审核时间、身份已开通说明 | 查看详情 |
| 网络/服务错误 | 已知身份状态、错误说明 | 重试 |

角色图标放在固定尺寸容器内，图标和标题使用同一光学中心；状态徽标同时使用颜色、文字和图标语义。弹窗内容允许自然换行，最大高度受视口限制，窄屏仍保持上下留白。读取和导航期间锁定重复点击；接口错误留在弹窗中，不用平台级 `Taro.showModal`。

若入口接口返回 `404`，主动作进入详情页并以空申请状态展示表单；若已有申请，主动作同样进入详情页，详情页根据状态决定读写权限。直接访问详情路由不依赖入口弹窗。

### 2. 详情页信息架构

详情页采用单一滚动容器，顶部保留 `CapsulePageHeader`，内容顺序固定为：

1. **身份状态头部**：角色图标、角色名称、状态徽标、申请编号和提交时间；
2. **审核反馈区**：pending 显示等待说明，approved 显示开通说明，rejected 显示审核原因和审核时间，读取失败显示重试；
3. **本次申请进度**：使用 `submittedAt` 与 `reviewedAt` 展示当前提交、等待审核或审核结果。用户接口没有历史事件列表，不展示推测的重新提交历史；
4. **申请资料区**：姓名、手机号，商家额外显示商家名称和地址；pending/approved 只读，rejected/空申请可编辑；
5. **证明材料区**：最多 6 张，2 列网格，上传中的图片显示处理中状态，失败显示可理解的原因；历史私有媒体使用锁定占位，禁止猜测或拼接预览 URL；
6. **底部操作栏**：安全区感知的主按钮和次按钮，主按钮为“提交申请”或“重新提交申请”，审核中/通过时显示“返回我的”。

详情页只显示真实的 `applicationId`、`submittedAt`、`reviewedAt`、`reviewReason` 和 `assetIds`；不加入虚构审核人、评分、进度百分比、预计审核时间或业务数据。

### 3. 状态与交互

- `loading`：使用与内容形状接近的轻量骨架，避免整页跳动；
- `empty`：展示角色说明和空申请表单；
- `pending`：资料和证明只读，隐藏重复提交入口；
- `approved`：资料和证明只读，提供返回“我的”；
- `rejected`：保留原资料和审核原因，可编辑并重新提交；
- `error`：保留已加载的内容，显示页面内错误和重试动作；首次读取失败且无内容时使用统一错误状态。

上传、提交和加载继续使用现有 `operationGenerationRef`、锁和取消恢复逻辑。失败时不清空用户输入；提交前仍逐个调用真实媒体状态接口，按当前 `purpose === 'identity_application' && isMediaUsable(status)` 合同校验。材料上传完成可提交，申请仍由管理员人工核验，不恢复旧的微信图片审核阻塞逻辑。所有异步回调在页面卸载、离开、账号变化或新一轮操作开始后失效。

### 4. 视觉与响应式规则

- 继续消费现有 `design-tokens.scss`，只使用现有品牌蓝、语义状态色、圆角和动效时长；不添加新的渐变、图标字体或图片资源。
- 状态头部、资料卡和证明卡使用细边框与轻阴影，避免每一块都使用强阴影。
- 所有图标使用 `AppIcon`，角色图标、锁定图标、时间线节点和返回按钮放在固定画布中，检查视觉中心和点击区域。
- 主次按钮和关闭按钮最小触控尺寸为 44px 等效值；底部操作栏使用 `env(safe-area-inset-bottom)`。
- 320px、390px、430px 宽度下，角色标题、审核原因、商家地址和错误文案允许换行；按钮不被压缩、不出屏、不与胶囊区域重叠。
- 仅保留轻量 opacity/transform 过渡；`prefers-reduced-motion: reduce` 下关闭弹窗和证明状态动画。

## 文件职责

- `IdentityApplicationEntryModal.tsx`：读取申请摘要、映射状态文案、处理重试和进入详情，不拥有表单状态。
- `IdentityApplicationEntryModal.scss`：弹窗内角色摘要、状态徽标、摘要信息和窄屏布局。
- `profile/index.tsx`：根据身份卡点击打开申请弹窗；已有可用身份继续走身份切换弹窗。
- `identity-application/index.tsx`：保留 API、状态机和媒体上传行为，重排详情视图并接入统一状态组件/按钮/字段。
- `identity-application/index.scss`：详情页布局、状态区、时间线、证明网格和底部操作栏。
- `check-identity-application-ui.cjs`：锁定入口弹窗状态映射、禁止平台级业务弹窗、触控尺寸、安全区、关键状态和旧图标残留规则。

## 验证

自动验证顺序：

1. `node scripts/check-identity-application-contract.cjs`；
2. `node scripts/check-identity-application-ui.cjs`；
3. `npm.cmd --workspace apps/miniapp run typecheck`；
4. `npm.cmd run build:miniapp`；
5. `git diff --check`。

视觉预览覆盖 320/390/430px 和以下状态：空申请、加载、网络失败、审核中、已通过、已拒绝、上传中、上传失败、重新提交、减少动效。检查弹窗居中、遮罩范围、图标光学居中、时间线节点、证明网格、底部按钮和安全区是否出现位移或裁切。

## 完成边界

代码、静态合同、类型检查、MiniApp 构建和本地预览通过后，标记为代码交付。微信开发者工具、真机字体倍率、真实审核账号和发布状态仍需单独验收；本轮不上传、不部署、不提交审核、不合并学生端工作。
