# 小程序功能可见性中心 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 在隔离本地环境实现后台可配置的小程序功能显隐与服务端权限保护。

**Architecture:** 后端以持久化 feature policy 为唯一事实源，提供管理员读写和用户能力查询；社区、交易、IM、外卖、履约写入口统一调用 policy guard。管理端提供分组开关界面，小程序启动/回前台读取能力并隐藏入口，同时旧链接由后端和页面空态双重拦截。

**Tech Stack:** Go、Gin、GORM、MySQL migration、React/TypeScript、Taro、小程序现有 API/CSRF/审计组件。

---

## 文件边界

- 后端新建 feature-policy model/repository/service、migration 与测试；修改 admin/community/trade/im/takeout/delivery 注册和写入口。
- 管理端修改 `apps/admin/src/App.tsx`、`admin-navigation.ts`、`services/f-admin.ts`、`types.ts`，新增 `FeaturePolicyConsole.tsx` 及样式。
- 小程序新增 `services/feature-policy.ts`、`stores/feature-policy.ts`，修改首页、搜索、发布入口、底部导航、社区/二手/外卖/工作台入口与页面空态。
- 不触碰当前已修改的底部导航、student-takeout 四个文件和 `artifacts/`，除非先逐段合并并保留其内容。

### Task 1: 后端数据模型与迁移

**Files:** `backend/migrations/034_feature_policy.sql`, `backend/migrations/rollback/034_feature_policy.sql`, `backend/internal/module/featurepolicy/model.go`, `repository.go`, `service.go`, `service_test.go`

- [ ] 写迁移测试/仓储测试，覆盖默认值、唯一键、row version、审计字段。
- [ ] 创建 `feature_policies` 单行表，字段包含 key、enabled、row_version、updated_by、updated_at；创建 `feature_policy_audit_logs`。
- [ ] 实现白名单 key 常量、默认值、`GetAll`、`Update`、`CanUse`；未知 key、版本冲突、数据库错误返回稳定错误。
- [ ] 用独立测试数据库运行迁移与回滚，确认不触碰现有表。

### Task 2: 后端管理 API 与用户能力 API

**Files:** `backend/internal/module/admin/handler.go`, 新建 `backend/internal/module/featurepolicy/handler.go`, server wiring 文件、HTTP 测试

- [ ] 为超级管理员注册 `GET/PATCH /api/v1/admin/feature-policy`，复用现有 admin auth、`user.SuperAdminRequired`、CSRF 与响应格式。
- [ ] 为登录用户注册只读 `GET /api/v1/feature-policy/capabilities`，返回可见模块与 `canPost/canComment`，不返回管理信息。
- [ ] 编写普通管理员 403、超级管理员成功、未知字段 400、版本冲突 409、匿名能力接口 401 的 HTTP 测试。
- [ ] 确认配置读取失败不会返回全开结果。

### Task 3: 社区与业务写入口统一拦截

**Files:** `backend/internal/module/community/handler.go`, `service.go`, `trade`/`im`/`takeout`/`delivery` 写入口及测试

- [ ] 在 CreatePost、CreatePostKeyed、AddComment、AddCommentKeyed 的最终写入前调用 feature policy；继续执行登录、禁言、封禁和内容安全校验。
- [ ] 对二手发布/聊天、通用私信、关注、外卖新订单、跑腿新订单、商家/骑手新业务操作接入对应 key；订单读取、已开始订单、取消退款售后保留原鉴权。
- [ ] 对论坛/二手/外卖关闭返回统一业务错误；幂等重放、旧客户端和直接 API 均测试。
- [ ] 运行后端全量测试，确保用户 role 不因开关变化。

### Task 4: 管理端功能设置页面

**Files:** `apps/admin/src/FeaturePolicyConsole.tsx`, `App.tsx`, `admin-navigation.ts`, `services/f-admin.ts`, `types.ts`, 样式文件及测试

- [ ] 增加分组开关 UI：社区、交易与服务、身份工作台、交流关系；显示说明、当前值、修改人、时间和本地实验提示。
- [ ] 对普通管理员渲染只读状态；超级管理员保存时带 rowVersion、禁用重复提交、处理 403/409/网络错误。
- [ ] 关闭论坛/二手/外卖时显示影响说明；不得出现“所有用户都是管理员”文案。
- [ ] 增加管理端类型检查与组件测试，验证保存结果以服务端返回为准。

### Task 5: 小程序能力读取与入口过滤

**Files:** `apps/miniapp/src/services/feature-policy.ts`, `stores/feature-policy.ts`, `pages/home/index.tsx`, `pages/search/index.tsx`, `pages/publish-entry/index.tsx`, `components/student-bottom-nav.tsx`

- [ ] 实现能力请求、缓存、失败拒绝策略和 `useDidShow` 刷新；能力字段包含模块状态与发帖评论权限。
- [ ] 首页按 forum/secondhand/takeout 过滤内容与入口；搜索按模块裁剪 scope；发布入口按 post/secondhand_publish 显示。
- [ ] 底部导航仅渲染可见项并保持发布按钮对齐；关闭所有可选模块时保留个人与必要服务入口。
- [ ] 修改当前脏文件时先保存 diff，使用最小补丁合并，不覆盖用户内容。

### Task 6: 页面旧链接、空态与订单边界

**Files:** 社区/二手/外卖/商家/骑手相关页面与服务 client；必要时新增共享 `FeatureDisabledState`

- [ ] 关闭模块后直接 URL、分享深链、通知入口显示统一“暂未开放”并提供返回，不白屏、不循环跳转、不发起写请求。
- [ ] 论坛详情在评论加载前检查 comment/forum；二手详情/发布/聊天分别检查 secondhand、secondhand_publish、secondhand_chat。
- [ ] `student-takeout` 仅隐藏外卖商家区，保留校园工具；不替换用户已有未提交改动。
- [ ] 验证订单查询、履约、取消、退款、售后和必要参与方沟通不被模块开关切断。

### Task 7: 集成验证与本地预览

**Files:** 新增/修改测试脚本、docs 验证记录；不生成图片资产

- [ ] 执行 14 个模块开关的逐项测试、论坛发言模式叠加测试及代表性组合（全部开、全部关、论坛只读、二手关闭、外卖关闭）。
- [ ] 执行匿名/普通/禁言/封禁/管理员/超级管理员矩阵、旧链接、直接请求、幂等重放、降权旧会话和并发保存测试。
- [ ] 运行 `npm run typecheck`、`npm run build:miniapp`、`npm run build:admin`、相关 `npm run check:*` 与 `npm run test:backend`。
- [ ] 在仅 loopback 的实验服务和独立测试数据库启动，打开管理页验证开关，再打开小程序验证入口与受限提示；记录未验证项。
- [ ] 最后检查 `git diff`，确保用户既有四个修改文件和 `artifacts/` 未被覆盖，提交仅包含本功能文件。

## 自审

- 设计中的所有开关均有对应 Task 3/5/6；基础不可关闭能力和订单边界由 Task 6 覆盖。
- 无 `TBD`、`TODO` 或“适当处理”等空泛步骤；每个步骤包含目标文件和验证动作。
- `rowVersion`、key 白名单、`canPost/canComment` 命名在后端、管理端和小程序中保持一致。
- 计划只面向本地隔离环境，不包含部署、提审或生产变更。
