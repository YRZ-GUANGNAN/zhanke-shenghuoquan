# 系统健康与管理后台状态加固设计

日期：2026-10-24  
基线：`main` @ `c1e62f5`

## 目标

在不改变业务 API、路由、数据模型、支付开关或外部平台接入的前提下，修复骑手/商家工作台的手势边界，恢复三个独立检查脚本与当前代码的契约，并让管理后台在支付能力关闭时明确表达“不可真实打款”的状态。

## 范围与边界

包含：

1. 从已验证的 `dfbca09` 局部移植 `DeliveryStagePager` 及工作台接入所需的最小代码，使阶段横滑不冒泡到工作区频道导航；保留原生横滑、刷新、身份守卫和现有事件隔离。
2. 修复 `check-public-profile-gestures.cjs`、`check-sheet-dismiss.cjs`、`check-admin-identity-applications-contract.cjs` 的测试桩或路径断言，使它们检查当前实现，而非旧目录/旧 API。
3. 在 `apps/admin/src/DeliveryConsole.tsx` 的钱包/支付展示中增加关闭态说明、线下清账边界和不可用操作反馈；不注册新支付路由，不改变 `ENABLE_DELIVERY_PAYMENT` 默认值。

不包含：

- 真实微信支付、自动打款、支付回调或任何外部订单 Provider。
- 数据库 migration（包括共享 MySQL 的 migration 047）或线上数据写入。
- 微信上传、审核、部署、GitHub push。
- 与本次问题无关的页面、视觉主题、路由、接口和业务逻辑重构。

## 设计

### 工作台手势边界

`DeliveryStagePager` 包装骑手和商家工作台的阶段 `ChannelPager`。它在触摸开始/移动/结束时停止事件冒泡，并用独立的水平导航跟踪器判断是否处于阶段列表边缘。阶段内部滑动由 `ChannelPager` 消费；只有从首项向左侧或末项向右侧的有效边缘滑动，才调用外层 `workspace.step`。页面隐藏、身份失效或上下文 key 变化时取消未完成手势，避免旧触摸跨页面或跨账号串联。

### 独立检查脚本

- 公开主页脚本的 Taro 桩补齐 `useDidHide`。
- sheet 脚本的模块加载器把 `@tarojs/taro` 解析为最小可用桩，并继续保留现有拖拽断言。
- 身份申请脚本从 `apps/admin/src/admin-navigation.ts` 读取 `identities` 导航契约，同时保留 `App.tsx` 实际渲染组件的断言。

### 管理后台支付关闭态

管理端继续展示订单、钱包和报酬信息，但当支付能力未开通时：

- 顶部状态明确写出“支付能力未开通”；
- 骑手报酬操作明确写出“当前仅支持线下清账，不会发起真实打款”；
- 真实支付/提现入口不可误触，保留现有权限与 API 行为；
- 若用户尝试不可用操作，使用现有统一提示组件说明开通条件，不伪造余额、流水或成功结果。

## 验证

代码变更完成后运行：

```text
npm run verify
npm run build
node scripts/check-public-profile-gestures.cjs
node scripts/check-sheet-dismiss.cjs
node scripts/check-admin-identity-applications-contract.cjs
git diff --check
```

另外检查 `git status --short`：除本次源码/脚本和本设计、计划文档外，不得出现新的改动；不得触碰 `.env`、共享数据库或外部订单分支。

结果分类：代码与本地测试为 PASS；migration 047、真实支付、外部平台资质和真实微信设备行为保持 PENDING/UNVERIFIED，不在本批次宣称完成。
