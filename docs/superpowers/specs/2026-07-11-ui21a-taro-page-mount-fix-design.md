# UI-21a Taro 页面挂载修复设计

## 背景与问题

UI-21a 已完成登录、服务端身份状态和身份切换地基，但微信开发者工具真实冷启动时页面为空。自动化证据显示：

- App 已启动，当前路由为 `pages/home/index`；
- Zustand session 已进入 `authenticated`，JWT、用户资料和身份摘要均有效；
- 页面根节点却只有 `<page></page>`，没有任何 `View` 或 `Text` 子节点；
- 关闭后重启开发者工具、重新编译、重新 `reLaunch` 均不能恢复页面。

根因位于 `apps/miniapp/src/app.tsx`：首次渲染处于 `idle/booting` 时，App 返回登录状态页而没有把 `props.children` 纳入 React 树。异步认证完成后，Taro 没有重新创建当前页面实例，因此 session 正常但页面根节点永久为空。

## 目标

在不改后端、API、身份契约和冻结静态视觉的前提下：

1. 确保 Taro 页面从 App 首次渲染起就被挂载；
2. 启动中或登录失败时不暴露依赖身份的页面内容；
3. 保留一次性 bootstrap、401 清 token、网络错误保留 token、身份恢复和非法深链 fail-closed 等安全行为；
4. 完成微信开发者工具真实冷启动与五项 CONDITIONAL 验收。

## 方案选择

### 采用：App 永久挂载页面，页面级运行时 Gate 控制身份相关内容

- `App` 始终返回 `props.children`，只负责调用 `bootstrap()` 和在认证后恢复当前身份首页；
- 新增可复用的 `RuntimeSessionGate`，负责显示“正在登录”、登录失败、错误信息和“重试”；
- 学生首页与“我的”页在 Gate 通过后才显示身份相关内容；
- 商家首页和骑手首页同时经过 Runtime Gate 与既有 `useIdentityGuard`，只有 session 已认证且服务端当前身份匹配时才渲染业务页面；
- UI-21a 范围外的静态二级页面不新增运行时请求，也不承载服务端身份敏感数据；受限身份页仍由守卫 fail-closed。

该方案直接满足 Taro “页面必须在首次 App 渲染时进入组件树”的生命周期要求，修改面集中，也不会依赖强制刷新。

### 不采用：App 全局遮罩包裹所有页面

让 App 返回页面与全局遮罩的并列节点可能再次触发 Taro App 根节点兼容问题，并且遮罩只能防止视觉暴露，不能保证受限组件没有执行副作用。

### 不采用：认证完成后强制 `reLaunch` 或重新加载

这种方式会产生闪烁、循环跳转和重复 bootstrap 风险，网络错误恢复时也更难保持 token 与当前路由。

## 组件与职责

### `apps/miniapp/src/app.tsx`

- 首次渲染即返回 `props.children`；
- 保留一次性 `bootstrap()`；
- 保留认证完成后的身份首页恢复逻辑；
- 移除 App 级登录状态页面和重试按钮，不再让 App 替换页面根节点。

### `apps/miniapp/src/components/runtime-session-gate.tsx`

公开一个职责单一的 Gate：

- 输入：`children`；
- 读取：session `status`、`error`、`retry`；
- `idle/booting`：显示最小“正在登录”状态；
- `authenticated`：渲染 `children`；
- `anonymous/error`：显示失败信息和重试按钮；
- 重试按钮只调用现有 `retry()`，不自行清 token、不重复实现登录流程。

### 身份入口页

以下页面的现有主体由 `RuntimeSessionGate` 包裹：

- `apps/miniapp/src/pages/home/index.tsx`；
- `apps/miniapp/src/pages/profile/index.tsx`；
- `apps/miniapp/src/pages/merchant-home/index.tsx`；
- `apps/miniapp/src/pages/delivery-rider/index.tsx`。

商家与骑手页继续使用 `useIdentityGuard`。执行顺序为：session Gate 先确认认证状态，身份守卫再确认服务端当前身份。未通过任一层时，不渲染受限业务内容。

### 样式

运行时状态样式从 App 全局样式迁入 Gate 专属样式或继续复用等价类名。业务页面视觉、布局、静态卡片与底部导航不改。

## 状态与数据流

1. 微信启动 App；
2. App 立即挂载当前路由页面；
3. 页面中的 Runtime Gate 看到 `idle/booting`，仅显示登录状态；
4. App 的 effect 调用合并后的唯一 `bootstrap()`；
5. session 成功后 Gate 渲染页面主体；
6. 若当前服务端身份不是学生，App 在路由实例就绪后恢复到对应身份首页；
7. 商家或骑手深链先由 Gate 等待认证，再由身份守卫校验；身份不匹配时不渲染受限主体并跳回正确身份首页。

## 错误与并发不变量

- 模块级 `bootstrapInFlight` 继续合并并发启动调用；
- 无 token 时一轮 bootstrap 最多调用一次 `Taro.login`；
- 已有 token 遇到网络或 5xx 错误时进入可重试状态且保留 token；
- 只有 401 清理 token；
- 重试不新建第二套登录逻辑；
- 身份切换按钮现有 loading/禁用逻辑保持不变；
- 未认证、身份不匹配或身份不可用时，商家/骑手业务主体不得短暂出现。

## 测试与验收

### 自动化测试

1. Runtime Gate：
   - `idle/booting` 显示登录状态且不显示 children；
   - `authenticated` 显示 children；
   - `error/anonymous` 显示错误与重试；
   - 连续点击重试不会绕过 session store 的 in-flight 合并。
2. App：首次渲染始终包含 `props.children`，bootstrap 仍只从 effect 发起。
3. 身份页：未认证不显示主体；认证且身份不匹配时商家/骑手内容不可见。
4. 既有 session、identity、路由和身份切换测试继续通过。

### 微信开发者工具真实验收

真实后端、MySQL、JWT 中间件和微信开发者工具下验证：

1. 无 token 冷启动只触发一次微信登录，首页不再白屏；
2. 持久化 `rider` 或 `merchant` 冷启动恢复到对应首页；
3. 快速重复点击身份切换只发送一次请求；
4. 网络或 5xx 显示重试且不删除 token，恢复后可继续；
5. 非法商家/骑手深链在任何时刻都不显示受限内容；
6. 自动化检查页面 WXML 不再是空 `<page></page>`；
7. `npm run typecheck`、`npm run build:miniapp`、仓库级 verify/boundary/build 均通过。

## 范围边界

- 不改后端、迁移、API 或数据库；
- 不改 session 的 token 生命周期与身份服务端契约；
- 不改冻结静态 UI；
- 不合并或删除 `feat/p0-identity-switch`；
- 不提前合并 UI-21a；真实验收全部通过并更新证据后，才交由 owner 串行 fast-forward 与打 baseline tag。
