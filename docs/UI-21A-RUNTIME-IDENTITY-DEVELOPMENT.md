# UI-21a 运行时登录与身份地基 Development Report

## 基线与范围

- 开发基线：`master@18c5b7f4`（二手发布静态页路由闭环）。
- 开发分支：`feat/ui21a-runtime-identity`。
- 实现提交：
  - `bb1696b3`：服务端身份状态、资格适配、HTTP 与真实 MySQL 验收；
  - `c5346df7`：小程序登录会话、身份切换与页面守卫；
  - `082dadb2`：补齐 Taro 页面显示生命周期类型。
- 旧参考分支 `feat/p0-identity-switch` 保留原样，未 merge、未 cherry-pick、未删除。

## 后端实现

1. 迁移 `029_user_current_identity.sql` 为 Domain A 的 `users` 增加 `current_identity`，默认 `student`，CHECK 仅允许 `student/rider/merchant`；rollback 对称删除约束与字段，零外键，未使用 AutoMigrate。
2. Domain A 新增 `Identity`、`IdentityAvailability`、`IdentitySummary` 与 `IdentityEligibility` 窄接口，并提供：
   - `GET /api/v1/auth/identity`
   - `POST /api/v1/auth/identity`
3. 身份切换仅更新 `current_identity`，不修改表示后台权限的 `users.role`；同目标重复切换幂等返回。
4. `cmd/server` 组合层按 `ENABLE_DELIVERY` 计算资格：
   - Delivery 关闭时 rider/merchant 均不可用；
   - merchant 对已登录用户可用，不读取封存的 `takeout_merchants`；
   - rider 只通过轻量 `takeout.RiderIdentityService` 读取共享骑手资格，且仅 `active` 可用；
   - 未重新装配封存的旧外卖订单、商家、支付或路由。

## 小程序实现

1. `ApiError` 保留 HTTP status、业务 code 与 message；网络错误为 status 0。只有 401 会清 token，网络/5xx 保留 token 并进入可重试错误态。
2. Zustand session 管理 `idle/booting/authenticated/anonymous/error`、profile 与 identity summary：
   - 恢复 `domainA.token`；
   - 有 token 时按序读取 `/auth/me`、`/auth/identity`；
   - token 失效才执行一次 `Taro.login → /auth/wechat-login`；
   - 模块级 in-flight promise 合并重复 bootstrap；
   - logout 清理 token。
3. App 在 session 可信前不渲染业务页面；冷启动会等待 Taro router 就绪，再按持久化身份恢复到骑手/商家首页，导航失败不锁死恢复机会。
4. Profile 保留冻结视觉结构，只接入真实用户、身份摘要与服务端切换；页面再次显示时刷新资格。切换期间按钮禁用，导航失败时保留弹窗并显示错误。
5. 骑手页、商家页使用相同深链守卫：当前服务端身份不匹配时不渲染业务内容，并回到服务端身份对应首页；切回学生同样先写服务端再跳转。

## 审查修正

- 将 token hydration 从并发改为按序，消除 401 被并发网络错误掩盖的风险。
- 401 身份刷新/切换统一清理 session；非 401 不误删登录态。
- 冷启动路由恢复只在 router path 可用后置 latch，并对 `reLaunch` 失败开放重试。
- Profile/rider/merchant 均在导航成功后关闭切换弹窗。
- Profile `useDidShow` 刷新资格，避免管理员开通骑手后必须重启小程序。

## 非代码环境整理

为解除 C 盘 `ENOSPC`，将当前未被 WindowsApps Claude 进程使用的旧独立 Claude VM 缓存从
`C:\Users\33141\AppData\Local\Claude-3p\vm_bundles`
搬到
`D:\claude-cache\Claude-3p\vm_bundles`，并在原路径保留目录联接。未删除项目源码、用户文档或当前 Claude 数据；C 盘由 0 GB 恢复到可提交状态。

主仓库生成目录 `node_modules` 同步迁移为
`C:\Users\33141\Documents\New project\node_modules → D:\codex-cache\new-project\node_modules`
目录联接。依赖树由本次全新 `npm ci` 产物替换，并重新建立 admin/miniapp workspace 联接；已用 `require('zustand')` 和主仓库全 workspace typecheck 验证可用。该操作只涉及 Git 忽略的生成依赖，不改变源码或 lockfile。

## 明确未做

- 未把订单、钱包、履约照片、聊天、社区或二手静态数据动态化。
- 未修改旧外卖核心、旧外卖路由、社区、二手、media、admin 前端或迁移 001–028。
- 未合并或删除旧身份分支；未合并当前分支，未打 baseline tag。
