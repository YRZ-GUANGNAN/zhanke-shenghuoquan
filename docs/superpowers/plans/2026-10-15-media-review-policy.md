# 图片用途审核分流实施计划

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.
> 当前没有上述执行技能，按用户已授权范围由本会话逐项执行，不派生额外任务。

**Goal:** 帖子/二手自动检查、个人资料图片直接保存、身份申请由管理员核验。

**Architecture:** 审核投影增加明确的 not_required 策略终态，以服务端 purpose 白名单控制；绑定、展示和回收共用一致的用途判定。新头像保存接入核心媒体域并与资料变更共用事务。客户端使用可用状态，不把技术上传成功误写成内容审核通过。

**Tech Stack:** Go、Gin、GORM、MySQL 8.4、Taro/React/TypeScript；复用 COS 上传与微信自动审核。

---

### 1. 媒体策略与数据库

- [x] 增加 `backend/internal/module/media/review_policy.go`，唯一白名单为 avatar、profile_cover、identity_application；`ReviewAllowsUse(purpose, status)` 仅允许 approved 或白名单用途的 not_required。
- [x] 在 `review_projection_types.go` 增加状态，Terminal 包含它，Decision 不包含它；`review_projection_repository.go` 在验证 final_ready、final 固定对象后，仅为新投影写入该状态及决定时间，旧记录原样返回。
- [x] 增加 `backend/migrations/044_media_review_policy.sql`，使用一个原子 ALTER 扩展 projection CHECK，不修改审核决定表，不批量改历史状态。
- [x] 在 `reference_binding_repository.go`、`gateway_service.go`、`display_repository.go` 和 `identityapplication/media.go` 使用同样的可用条件，保留所有原权限与引用判断。
- [x] 添加用途矩阵、非法用途伪造 not_required、未完成上传、旧拒绝、回调终态和私有材料测试；GC 使用已有终态保留策略，未改变回收期限。

### 2. 真实头像保存与状态

- [x] 在 `user` 域新增核心头像资料更新服务与事务仓储，复用 core Reserve/Commit/Publish/Revoke；PATCH /me 接入该服务，历史旧媒体资产保留原兼容路径。
- [x] `miniappflow/types.go` 和 `service.go` 返回 ready/not_required，而不是批准文案；技术处理、删除或过期资产永远不可用。
- [x] 修改 `services/media.ts`，提供已上传且可用的确认函数，ready 只允许对应用途；帖子/商品原审核等待函数继续只接受 approved。
- [x] 修改编辑资料、封面编辑器、身份申请和上传历史提示；状态失败保留原图和草稿，身份证明上传可用不改变申请 pending。

### 3. 自动审核与管理显示

- [x] `admin/review/safety.go` 必须识别明确 pass，缺失或未知 suggest 走异常状态；帖子/商品不等待常规人工点击。
- [x] 核对自动检查通过/拒绝/异常恢复，补齐所需自动处理与测试，保持身份申请排除在自动决定名单外；帖子审核通过后同步发布核心媒体引用。
- [x] 管理媒体控制台补齐用途筛选和 not_required 文案，身份申请继续使用现有通过/拒绝、冲突保护、审核记录。

### 4. 验证与交付

- [x] 新增/更新聚焦测试先运行失败案例，再实施并复验；使用临时 MySQL 实例，不访问现有业务数据库。
- [x] 运行 `npm run verify`、隔离数据库测试、`npm run build`；修复与本次改动有关的断言和类型问题，不削弱原权限检查。
- [x] 检查 diff 和用户配置隔离，记录迁移顺序、测试结果及尚需真机/生产验证部分；提交本次文件，不自动部署。
