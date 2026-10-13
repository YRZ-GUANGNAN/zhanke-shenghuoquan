# 原版与独立资讯模块实施计划

> 按用户已确认的方案在当前隔离工作区执行，不再请求重复批准。不新增子代理；上传和提交审核由用户完成。

**目标：**恢复原版，提供保存原配置的全局展示模式，交付后台和可上传小程序包。

**架构：**featureflags存储原配置，统一计算有效策略；两个独立首页模块；Admin用现有鉴权、CAS、审计保存。

**技术栈：**Go/Gin/Gorm/MySQL、React/TypeScript、Taro。

## 执行步骤

- [x] 核验两套release与原分支隔离，切回API/Admin前一版本；检查 `/readyz`、`/version`、`/features` 及公网首页，保留数据库和原20项开关。
- [ ] 修改 `backend/internal/module/featureflags/{service,repository,handler}.go`，移除固定release限制，新增 `presentation.go`。新增迁移 `041_feature_policy_presentation.sql`；默认 `standard`，新增字段不更新旧配置。
- [ ] 新行为测试放 `presentation_test.go`，在 `repository_mysql_test.go` 使用真实迁移验证 `standard -> campus_info -> standard`、重启、CAS、审计回滚。示例断言：`Snapshot().Flags[KeyForum]` 保持原值，资讯模式 `Can(KeyForum)` 为false，返回原版后按原值恢复。
- [ ] 从 `d8b175e` 精确恢复原首页和相关契约，用 `apply_patch` 应用；将现资讯首页移到独立组件。首页依据 `visibility.presentationMode` 选择组件，未知/失败策略不挂载业务。MiniApp解析器和关于/协议/设置同步真实模式。
- [ ] `apps/admin/src/{FeaturePolicyConsole.tsx,types.ts,services/f-admin.ts}` 增加类型和模式选择；保存时包含 `presentationMode`，旧服务缺失该字段时禁用模式编辑并说明需更新后端。原开关保留。
- [ ] 恢复原首页契约，并为独立资讯模块、模式解析和分享补充断言。运行 `npm.cmd run check:campus-info`、`npm.cmd run verify`、`node scripts/test-feature-policy-mysql.cjs`、两端build。只使用安全后端测试入口。
- [ ] 用真实开发工具/后台页面验证可达到的交互，无法验证的环节如实记录，不冒充真实登录。
- [ ] 精确提交本轮源码，保留已有两个project配置改动；按现有release脚本模式备份数据库、增加字段、核验哈希、原子发布API/Admin，默认保持原版；检查新模式字段与原开关一致。
- [ ] 生成交付记录、上传目录和模式使用说明，不调用微信上传、提审或发布。

## 失败处理

接口读失败关闭可选能力并允许重试；冲突返回409，不覆盖他人保存；数据库写或审计失败整体回滚。部署失败恢复上一release，新增列保留向后兼容，不能回滚整库或删除新期间业务数据。

## 自审

用户的保留、独立、可切回、后台操作和自行提交分别有对应实现与验收步骤；没有固定发行锁、隐藏审核模式、假数据或无关重构。
