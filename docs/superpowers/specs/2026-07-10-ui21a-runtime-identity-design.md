# UI-21a 小程序运行时登录与身份地基 v2 设计

## 目标

在不改变已经冻结的新版静态 UI 视觉结构、不重新启用旧外卖运行时的前提下，建立可信的小程序登录态与服务端身份状态：启动时恢复或获取 JWT，服务端持久化 `student/rider/merchant` 当前身份，页面按真实身份进入，非法深链 fail-closed。

## 边界

- Domain A 拥有 `users.current_identity`、身份摘要与切换命令。
- `users.role` 继续只表示后台权限，身份切换绝不改变管理员权限。
- Delivery 开关关闭时，`rider`、`merchant` 都不可进入。
- 骑手资格复用既有只读 `takeout.RiderIdentityService`；只读取共享骑手身份，不装配旧外卖订单、商家、支付或路由。
- 配送商家当前没有独立准入表；在 `ENABLE_DELIVERY=true` 且用户已登录时可用，不读取封存的 `takeout_merchants`。
- 本模块不把订单、钱包、履约照片、聊天、社区或二手静态数据改为动态数据。

## 后端设计

迁移 029 为 `users` 增加 `current_identity VARCHAR(24) NOT NULL DEFAULT 'student'`，并以 CHECK 冻结三种值。回滚删除 CHECK 和字段，不增加外键，不使用 AutoMigrate。

Domain A 新增：

- `Identity`、`IdentityAvailability`、`IdentitySummary`；
- `IdentityEligibility` 窄接口；
- `GET /api/v1/auth/identity`；
- `POST /api/v1/auth/identity`，请求 `{ "identity": "student|rider|merchant" }`；
- 非法值返回 400，不可用身份返回 403，未登录返回 401。

组合层新增 `deliveryIdentityEligibility`：

- Delivery 关闭：骑手和商家均返回 `status=disabled`；
- Delivery 开启：商家对非零登录用户可用；
- Delivery 开启：骑手仅在共享骑手记录为 `active` 时可用；缺失或非 active 均不可用；
- 适配器不构造或查询旧外卖商家身份。

## 小程序运行时设计

`api.ts` 引入携带 HTTP 状态、业务 code 和 message 的 `ApiError`。网络错误保留为状态 0，401 可被 session 精确识别，禁止把所有错误压平成普通字符串。

Zustand session 状态为 `idle/booting/authenticated/anonymous/error`，并保存 profile、identity summary 和错误信息。启动流程：

1. 读取 `domainA.token`；
2. 有 token 时并发读取 `/auth/me` 与 `/auth/identity`；
3. 只有 401 才清 token，并执行一次 `Taro.login → /auth/wechat-login`；
4. 无 token 时直接执行一次微信登录；
5. 网络、5xx 或微信登录失败进入 error，不自动循环；用户点击重试才重新执行；
6. 登出清理 token，进入 anonymous。

App 根组件在 session 未可信前显示最小运行时状态页；错误或匿名态提供重试。业务页面只在 authenticated 后出现，因此不会展示伪造用户资料。

Profile 保留现有布局、卡片、底栏和静态统计，只把用户姓名、学校、身份及身份切换接真实状态。骑手页和商家页增加深链守卫：当前服务端身份不匹配时不渲染业务内容，提示后跳回 Profile。三个页面的切换确认均调用服务端命令，成功后再跳转。

## 并发与错误处理

- 模块级 bootstrap promise 合并重复启动调用；同一轮不会重复 `Taro.login`。
- identity 切换期间按钮禁用，重复点击不发第二次请求。
- 403 在原页面提示资格不可用；401 交由 session 清理并要求重试；409/5xx 保留后端信息。
- `IdentitySummary` 发现持久化身份已不可用时对外回退为 student，页面仍 fail-closed；不在 GET 请求里隐式写库。

## 验收

- 单元测试覆盖身份校验、商家规则、骑手 active/缺失/非 active、Delivery 关闭、角色不变。
- 真实 MySQL 8.4 执行迁移 apply→rollback→apply，外键数为 0。
- 真实 HTTP 验证 401、403、成功切换、持久化及 role 不变。
- 小程序 typecheck/build 通过；静态壳路由检查通过；现有视觉文件不做无关重写。
- `go test ./... -count=1`、`npm run check:boundaries`、`npm run verify`、`npm run build` 全部通过；`db.AutoMigrate(` 命中为 0。

## 旧分支处理

`feat/p0-identity-switch` 只作为实现参考。它与当前 master 分叉且包含旧 UI 与旧外卖商家资格语义，本模块不 merge、不 cherry-pick；UI-21a 验收冻结前也不删除该分支。
