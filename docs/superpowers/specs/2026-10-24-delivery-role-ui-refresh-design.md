# 骑手与商家工作区 UI 完整落地设计

## 背景

当前仓库的骑手工作台与商家工作台已经在 `0393fd5` 落地 v10 组件样式：骑手按“接单 → 配送 → 消息 → 我的”组织，商家按“订单 → 发单 → 消息 → 我的”组织，商家订单保持双列。离线总览已经用真实 TSX/SCSS 验证了两种角色的 8 个一级页面，并确认 B 方向的“我的”布局可以进入真实代码。

本次目标是把 B 方向的角色资料页落地，并对现有工作台做契约级回归确认。工作台的业务逻辑、接口、路由、开关、支付门禁和数据来源不变。

## 范围

### 本次修改

- `apps/miniapp/src/pages/delivery-profile/index.tsx`
  - 将个人封面纳入可滚动内容的顶部，使封面、身份卡和资料内容成为连续页面。
  - 将“编辑资料与封面”入口放入身份卡右侧，保留原 `/pages/edit-profile/index` 路由和点击保护。
  - 角色名称、头像、封面、资料字段继续读取当前 session 与 `getIdentityApplication` 的真实返回。
  - 将审核状态从资料事实列表的第一行提升为资料标题旁的轻量状态 chip，避免重复信息。
  - 保留商家钱包入口、支付能力开关、骑手收入说明、资料失败重试、身份确认弹窗和 `useIdentityGuard`。

- `apps/miniapp/src/pages/delivery-profile/index.scss`
  - 落地已确认的 B 方向：封面与身份区衔接、身份区内编辑按钮、角色资料分组、商家资金入口、骑手收入说明和底部账号切换。
  - 保留品牌蓝 token、系统胶囊安全区、底部导航、触控尺寸、320px 窄屏和 `prefers-reduced-motion` 适配。
  - 不引入玻璃变体、新的远程装饰素材或生成图片；真实用户头像和封面继续使用原 profile 资源，封面缺失仍收起媒体区。

- `scripts/test-delivery-workspace.cjs` 与 `scripts/check-role-workbench-ui.cjs`
  - 新增 B 布局、图片失败回退及手势点击保护的回归用例，保留现有业务断言。
  - 将旧的渐变封面边界检查替换为 B 方向实色身份区衔接检查，不放宽无缝边界要求。

### 明确不修改

- `apps/miniapp/src/pages/delivery-rider/**` 与 `apps/miniapp/src/pages/merchant-home/**` 的业务逻辑、DOM 合同、订单双列结构、接口调用、状态文案和导航关系。
- `merchant-publish`、`merchant-messages`、学生端、后端、支付接口、外部平台订单接口和管理后台。
- 不新增依赖、路由、数据字段、假账户、假收入、钱包余额或提现入口。
- 不上传微信体验版、不提交审核、不 push GitHub、不部署。

## 页面结构

两种角色共用同一资料页结构，按角色显示已有能力：

1. 系统胶囊安全区和可选封面。
2. 身份区：头像、角色工作区、真实名称、编辑资料按钮（无障碍标签仍说明可编辑头像、昵称与封面）。旧的角色说明保留在树内但按批准稿隐藏，避免重复。
3. 资金区：商家显示已有钱包入口及支付开关说明；骑手仅显示后端结算说明。
4. 资料区：标题旁显示真实审核状态；联系人、电话及商家取货地址按行显示。资料未读取时显示读取中，失败时显示重试入口，字段缺失显示 `未填写`。
5. 账号与身份区：保留现有说明和切换到学生端按钮，切换仍经过确认、并发锁、账号/token/可见性校验。
6. 角色底部导航：沿用现有 `DeliveryWorkspaceNav` 和页面手势保护。

## 数据与交互边界

- 页面不创建新的 API 请求。`getIdentityApplication(role)`、session profile、`useDeliveryPayment` 和身份切换服务保持原调用方式。
- 个人封面和头像由当前 profile 读取；图片加载失败后保持现有回退逻辑，不把失败解释为审核通过。
- 审核状态只映射现有 `approved`、`pending` 与其它状态；不得在离线或错误状态显示“已通过”。
- 编辑入口只负责导航到既有编辑页，不在资料页保存或上传媒体。
- 商家钱包仍受支付能力门禁；骑手不展示余额或提现。
- 订单、消息和发单页继续使用当前 v10 实现；本次以类型检查、现有行为脚本和构建验证为准。

## 视觉与响应式规则

- 沿用 `$brand-primary` / `$brand-primary-strong`、`$brand-tint`、`$bg-app`、`$bg-surface` 等现有 token，字号和间距保持 v10 的 8px 节奏。
- 有封面时，身份区以负边距与封面相接；无封面时不保留空白媒体块。
- 编辑入口最小触控高度 44px；身份、资金、资料和账号区具有清晰分组，但不增加重复卡片。
- 长昵称、电话、地址和审核文本可换行；320px 宽度下缩小头像和左右内边距，不能产生横向溢出。
- 仅使用短时 opacity/transform 按下反馈和入场反馈，尊重 `prefers-reduced-motion`。

## 验收标准

1. `npm --workspace apps/miniapp run typecheck` 通过。
2. `node scripts/check-role-workbench-ui.cjs`、`node scripts/check-delivery-stage-workbench.cjs`、`npm run test:delivery-stage-workbench` 和 `node scripts/check-private-messaging.cjs` 全部通过。
3. `npm run build:miniapp` 通过；允许既有 webpack 体积/顺序警告。
4. `git diff --check` 通过。
5. 资料页行为脚本继续验证：身份守卫、失败重试、账号/token/身份/隐藏/卸载中断保护、钱包入口与身份切换防重。
6. `git status` 中产品改动只限于本设计的资料页文件；已有未跟踪计划文件保持不动。
7. 浏览器离线总览中的 8 个一级页与二级页仍可正常打开；该总览不作为线上验收证据。
