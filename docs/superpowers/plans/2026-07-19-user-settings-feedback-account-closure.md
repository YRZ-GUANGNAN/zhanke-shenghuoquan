# 用户设置、反馈与账号注销 Implementation Plan

**Goal:** 将设置页、黑名单、手机号绑定、反馈、关于和直接注销从静态演示改为真实服务闭环，并在注销时保留历史内容、匿名作者、下架在售二手，同时阻止有未结业务或余额的账号注销。

**Architecture:** Domain A 负责设置、手机号、黑名单、反馈与注销 API；`internal/app/accountclosure` 作为应用编排仓储，在同一数据库事务中复查跨域未结业务并执行最小匿名化，不向任何领域暴露跨域模型。反馈图片扩展独立私有 `feedback` media purpose。客户端只在服务端写成功后更新状态；注销成功后才清理本地 token。

**Tech Stack:** Go 1.25、Gin、GORM、MySQL 8.4、Taro 4、React 18、TypeScript

---

### Task 1: 向前迁移与静态迁移测试

- [x] 新增 `033_user_settings_feedback_account_closure.sql`：`users.phone/closed_at`、`user_settings`、`user_blocks`、`user_feedbacks`、`user_feedback_images`，并扩展 media reference type 为 `feedback`。
- [x] 新增 `migration_033_test.go`，断言无 rollback、唯一键/索引/默认值和 media guard 顺序。
- [x] 运行 `go test ./migrations -run TestMigration033 -count=1`。

### Task 2: Domain A 设置、手机号、黑名单与反馈

- [x] 先写 service 测试：设置默认值和覆盖、手机号冲突、黑名单成功后删除、反馈 5–1000 字且最多 3 图。
- [x] 增加 optional repository interfaces，避免破坏现有测试替身。
- [x] 实现微信手机号 code 服务端换取、脱敏返回；凭据和明文手机号不得进日志。
- [x] 实现 `GET/PATCH /auth/settings`、`POST /auth/me/phone`、`GET/POST/DELETE /auth/blocks`、`POST /auth/feedbacks`。
- [x] Admin 增加只读反馈查询接口。

### Task 3: 私有反馈图片 purpose 与绑定

- [x] core media 增加私有 `PurposeFeedback`，允许上传和绑定但禁止公共 resolve。
- [x] MiniApp media client 增加 `feedback` purpose。
- [x] Domain A 反馈创建先 reserve，写入反馈后 commit；失败取消并保留客户端输入。

### Task 4: 账号注销应用编排

- [x] 先写 blocker/匿名化测试。
- [x] 阻断：配送/外卖非终态订单，进行中支付/退款/提现，商家或骑手可用余额，待结骑手收益；管理员账号先移交权限。
- [x] 同一事务重检并执行：释放 openID/phone、状态 closed、tokenVersion+1、资料匿名；删除设置/黑名单/搜索历史；下架非 sold 二手；暂停商家/骑手身份。
- [x] 帖子、评论、已售商品和审计事实不物理删除；用户目录对 closed 用户统一返回“账号已注销”。
- [x] 实现 `DELETE /auth/me`，409 返回具体 blockers。

### Task 5: MiniApp 服务、设置和黑名单真实化

- [x] `a-user.ts` 增加 settings/phone/blocks/feedback/close account 客户端。
- [x] 设置页真实加载并保存开关，失败回滚；显示真实手机号、身份与黑名单人数。
- [x] 手机号按钮使用 `open-type=getPhoneNumber`，只提交一次性 code。
- [x] 黑名单真实加载、空态、失败重试，解除成功后才移除。
- [x] 缓存只清理白名单可重建项；更新检查接入 `getUpdateManager`。
- [x] 注销双确认；仅服务端成功后 logout/reLaunch。

### Task 6: 反馈与关于页面

- [x] 新增 feedback 页面：分类、5–1000 字、最多 3 图、图片审核、提交中防重复、成功反馈编号。
- [x] 新增 about 页面：产品介绍、运行时版本、协议、隐私、反馈真实入口。
- [x] 注册路由，修正 profile 静态 Toast 与静态缓存/版本文案。

### Task 7: 自动门禁与验证

- [x] 新增 `check:miniapp-settings-contract`，禁止写死手机号、静态黑名单、静态缓存/版本和 Toast 假入口。
- [x] 运行 Domain A、accountclosure、media、migration tests。
- [x] 运行 boundaries、所有 MiniApp 契约、typecheck、Admin/MiniApp build。
- [ ] 独立数据库 migration 033 真实验证不可用时保持 `PENDING (permission blocked)`，不得破坏共享数据库。
- [x] 提交时排除既有部署、微信项目配置和本地环境文件。

### Task 8: 真机待验

- [ ] 微信手机号授权、UpdateManager、反馈图片审核、注销后的重新注册、真实未结订单阻断、黑名单跨设备与作者匿名均保持 `PENDING`，直至迁移后的真实后端和真机完成。
