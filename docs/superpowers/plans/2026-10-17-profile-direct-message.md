# 个人主页私信 Implementation Plan

**Goal:** 他人主页一键进入双方私信，列表可重新打开同一会话。

**Architecture:** 扩展现有 IM 关系为用户与商品两种类型；保留 Tencent C2C 及聊天页，新增受鉴权的用户会话创建接口。原用户表与拉黑表提供权限校验。

**Tech Stack:** Go / Gin / GORM、React / Taro / TypeScript、现有隔离 Node 测试与网页预览。

用户已授权自动完成；由当前代理顺序执行，不新增代理任务、不再次要求方案确认。

## 1. 后端

- [x] 在 `backend/internal/module/im/user_conversation.go` 新增 `EnsureUserConversation(ctx, viewerID, peerID)`：最小 ID 存入 buyer，最大 ID 存入 seller，`user/0` 为稳定关系；返回针对调用者的 C2C。
- [x] `repository.go` 支持读取用户关系与按类型分页，仍排除配送关系；不更改 schema。
- [x] `user/private_contact.go` 与 `private_contact_repository.go` 校验存在、active 与双向拉黑。
- [x] `im/handler.go` 注册 `PUT /users/:userId/conversation`，在创建和读取前执行用户权限检查。总开关保留，商品细分开关由已授权关系决定。
- [x] 测试双向复用、不重复导入、第三人拒绝、无效/自己/停用/拉黑/存储故障、接口认证和类型开关。

## 2. 小程序

- [x] 主页增加现有次级按钮样式的“私信”，隐藏自身入口，点击互斥并正确处理跳转失败。小屏触控宽度至少 44px。
- [x] `e-im.ts` 新增 `contactUser`；会话类型保持兼容。发送前复核授权会话与当前对方。
- [x] 聊天页接受唯一的 productId / relationId / userId 之一，类型和对方校验失败时不能发送；用户会话隐藏商品栏、使用普通私信文案。
- [x] 首页私信列表与旧列表正确显示用户头像昵称，跳过商品查询，按 C2C 去重。功能控制按会话类型处理。
- [x] 扩展 `scripts/test-private-messaging.cjs`，执行真实组件处理器，覆盖用户入口、商品回归、权限失败和账号切换；扩展主页手势测试覆盖按钮路径与连点互斥。

## 3. 验证和交付

- [x] 私信、功能开关、类型检查通过；架构边界检查仅报基线已存在的 `delivery/external_intake.go` 数据库访问，不属于本轮修改。
- [x] `npm.cmd run test:backend` 清空 MySQL 测试环境，避免访问共享数据库；微信构建通过。数据库在线与双真机收发未执行。
- [x] 独立预览 320/390/430px 完整路径通过，覆盖主页进入、空聊天、本地模拟发送、返回、列表重进、自身隐藏、反向身份和键盘避让。
- [x] 审查差异，交付说明列明后端同步、微信上传与真机验收尚未执行；仅本地保存本轮文件。
