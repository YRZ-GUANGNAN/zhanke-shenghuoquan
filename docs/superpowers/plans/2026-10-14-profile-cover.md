# 个人封面 Implementation Plan

> 按用户已确认设计在当前会话逐项执行；不另开代理、不重复审批。所引用的 executing-plans 技能未安装，使用本计划和逐项测试代替。

**Goal:** “我的”支持真实上传、审核、独立保存与恢复默认封面。

**Architecture:** React/Taro 页面复用上传客户端；新增独立封面接口与用户行锁事务，接入现有媒体绑定服务。旧资料接口保持不变。

**Tech Stack:** Taro、React、Zustand、SCSS、Gin、GORM、MySQL 8。

---

### 1. 后端契约及事务

- [ ] 在 `backend/internal/module/user/profile_cover_test.go` 写请求、用途、事务失败与资料隔离测试。
- [ ] `backend/internal/module/user/profile_cover.go` 定义封面专用服务与数据边界：

```go
type ProfileCover struct {
    URL string
    AssetID *uint64
}
type ProfileCoverRepository interface {
    WithLockedCover(context.Context, uint, func(context.Context, User) (ProfileCover, error)) (User, error)
}
```

- [ ] `repository.go` 用事务与 `dbtx.WithDB` 执行回调，仅写 `cover`、`cover_asset_id`；`model.go` 新增字段。
- [ ] `handler.go` 注册 `PUT /me/cover`，`profile_cover.go` 实现完整输入校验和中文错误映射；`cmd/server/main.go` 注入当前媒体绑定服务。
- [ ] 媒体 `model.go` 添加 `PurposeProfileCover = "profile_cover"`，沿用 10MiB 上限；客户端压缩长边 1920px。
- [ ] `backend/migrations/042_user_profile_cover.sql` 添加字段，原子扩展媒体用途与引用约束；注销协调器清空封面并撤销引用。

### 2. 客户端和个人页

- [ ] `types.ts`、`services/a-user.ts` 增加可选 `cover`，独立调用：

```ts
request('/auth/me/cover', { url: '', method: 'PUT', data: { assetId } })
```

- [ ] `services/media.ts` 接入 `profile_cover`，不改变既有上传用途。
- [ ] `components/profile-cover-editor.tsx` 与其样式：照片选择、审核、独立保存、重试、恢复默认；同步操作锁和会话检查。
- [ ] `pages/profile/index.tsx`、`index.scss` 增加横向媒体区及白色资料衔接；`pages/edit-profile/index.tsx` 接入封面编辑。保留全部原入口。

### 3. 验证与保存

- [ ] 新增前端行为脚本 `scripts/check-profile-cover.cjs`；断言失败不覆盖旧图、恢复默认只发空ID、重复点击不重复提交、旧账号结果不更新新账号。
- [ ] `npm.cmd run test:backend`：清除测试MySQL环境变量后跑单元测试。隔离库测试独立执行，不能把共享库当测试库。
- [ ] `npm.cmd run verify`、`npm.cmd run build:miniapp`，记录已有问题与新问题。
- [ ] 实际组件及样式布局检查覆盖 320/390/430px，无图状态截图；真实微信图片上传与跨设备读回待真机验收。
- [ ] `git diff --check`，精确暂存本轮文件并本地提交，不包含原有配置、不推送、不部署。
