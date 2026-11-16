# 2026-11-16 学生认证闭环上线（发布准备）

本记录为上线准备稿：代码、迁移与验证已完成，生产部署尚未执行；执行后在本文件追加实际发布身份与验证结果。

## 本次内容

### 学生认证闭环（后端 + 管理后台 + 小程序）

- **数据库**：迁移 `backend/migrations/063_student_verification.sql`
  - 新表：`student_verifications`（一人一条、`status IN ('pending','approved','rejected','cancelled')`、`cancel_requested_at`、`row_version` 乐观锁）、`student_verification_assets`、`student_verification_events`。
  - 媒体用途约束扩展：`media_assets.purpose` 与 `media_asset_references.reference_type` 增加 `student_verification`（保留既有全部用途，含 `profile_cover`）。
  - 回滚：`backend/migrations/rollback/063_student_verification.sql`（恢复约束并删除三张表）。
- **后端**：新模块 `backend/internal/module/studentverification`
  - 用户接口（需登录、未禁言）：`GET /api/v1/auth/student-verification`（无记录返回 404）、`PUT /api/v1/auth/student-verification`（提交/重新提交，带 `expectedRowVersion`）、`POST /api/v1/auth/student-verification/cancel-request`（仅已认证可申请，重复提交幂等）。
  - 管理端接口（管理员会话 + 管理员权限）：`GET /api/v1/admin/student-verifications`、`GET /:applicationId`（含 5 分钟签名照片链接与审核历史）、`POST /:applicationId/approve`、`POST /:applicationId/reject`（原因必填）、`POST /:applicationId/approve-cancel`（处理用户取消申请）。
  - 状态机：`pending → approved | rejected`；`approved + 用户申请取消 → cancelled`；`rejected / cancelled` 可重新提交；管理员不得审批自己的申请；并发以 `row_version` 乐观锁拦截。
  - 学生证照片走既有媒体管线：`student_verification` 用途为上传人直用、无二次内容审核，管理端凭绑定关系签名查看。
- **管理后台**：新导航「学生认证」（内容与用户分组）——列表状态筛选（含「取消申请中」标记）、详情（照片/历史/审核依据）、通过、驳回（必填原因）、解除绑定（仅当用户提交取消申请）；409 冲突自动刷新并锁定操作。
- **小程序**：「学生验证」页由本地占位改为真实接口：提交姓名/学号/学院 + 学生证照片（可裁剪，最多 3 张）；状态回显 待审核 / 已认证 / 未通过（含驳回原因）/ 取消申请中 / 已解除（含解除说明）；未通过与已解除可重新提交；已认证仅可「申请取消认证」（确认弹窗使用 AppDialog）。

### 企微订单卡片跳转小程序（后端）

- 订单通知的企微群模板卡片底部动作从「站点 URL」改为优先跳转小程序（`card_action.type=2`，携带 `WECHAT_APP_ID` 与页面路径）：商家群 → `pages/merchant-home/index`，骑手群 → `pages/delivery-rider/index`。
- 桌面端企业微信无法直接打开小程序，卡片保留站点 URL 兜底；未配置 `WECHAT_APP_ID`（如本地 mock）时自动退回原 URL 行为。
- 单元测试覆盖：type 2 载荷结构（appid/pagepath/url 兜底）、非法 appid 或 pagepath 回退 type 1、两群页面路径路由。

### 同批上线的界面改动（小程序，均不影响业务逻辑）

- 首页顶部头图：可选的照片背景 + 预模糊图层 + 白色淡化曲线；默认纯白，用户可在「编辑资料 → 首页背景」开启并自选图片（微信原生编辑裁剪，本地保存）。
- 二手页分类改为通栏等宽分段控件，内容区支持左右滑动切换分类；移除平台提示条与两条分隔细线。
- 搜索页工具栏改为 B 版分段控件（综合/最新）+ 方形筛选按钮。
- 发现/工具/发布/编辑资料等页面去除头部与区块分隔细线；「校园服务」页移除商家跳转提示与冗余副标题；工具频道移除介绍块与底部跳转提示。
- 编辑资料：新增「用微信头像」（chooseAvatar 官方通道）与微信昵称联想输入；新用户首次登录自动引导到编辑资料一次。
- 清理：`wx.saveFile/removeSavedFile` 改为 FileSystemManager；`getSystemInfoSync` 换 `getDeviceInfo`；`selectable` 换 `userSelect`；背景图压缩限宽 1080。

## 部署步骤（待执行）

1. **备份生产库**（沿用既有备份路径规范，如 `/var/backups/campus-circle/`）。
2. **执行迁移**：
   ```bash
   mysql <生产库> < backend/migrations/063_student_verification.sql
   ```
   迁移为幂等建表 + 约束替换；回滚脚本见上。
3. **部署后端**：构建并切换 release，重启 `campus-api`（沿用既有发布目录与软链规范）。
4. **发布管理后台静态**（本次包含新菜单「学生认证」）。
5. **小程序发版**：上传体验版→真机验收→提交审核（界面改动 + 学生验证页）。
6. **平台侧检查（人工）**：上线前确认小程序后台《用户隐私保护指引》覆盖相册/头像选择等接口；如账号已开启隐私检查，需补 `__usePrivacyCheck__` 与隐私授权回调（本次未包含，属独立上线门槛）。

## 验证清单（部署后执行并记录）

- `GET /healthz` 返回 `{"code":0,"message":"ok"}`；`/version` 返回新运行时版本与 commit。
- 未登录访问 `/api/v1/auth/student-verification` 与 `/api/v1/admin/student-verifications` 均返回 401。
- 后台「学生认证」菜单可见，列表/筛选/详情/驳回/解除操作可用（真实管理员会话）。
- 小程序真机：提交认证（照片裁剪）→ 后台通过 → 小程序显示已认证；申请取消 → 后台解除 → 小程序显示已解除并可重新提交。
- 图片裁剪为微信原生编辑能力，开发者工具模拟器不支持，必须在真机验收。

## 回滚方式

- 代码：后端与后台静态分别切回上一 release 并重启；小程序回滚需重新提交版本。
- 数据库：执行 `backend/migrations/rollback/063_student_verification.sql`（删除三张新表并恢复媒体用途约束）。学生认证为纯新增，回滚代码时也可暂不撤销表结构。

## 本地验收结果（2026-11-16，本地 campus_circle）

- 迁移 063 已在本地库实际执行：三张表创建、媒体用途约束包含 `student_verification`；执行前已备份（schema+data，57KB gz）。
- API 冒烟（本地后端 + mock 登录）通过：未登录用户/管理端接口均 401；新用户 mock 登录成功（默认昵称「校园用户」，与首登引导判定一致）；无记录查询 404；字段缺失 400；伪造素材提交返回 409 媒体冲突；无记录申请取消 404。
- 真实 MySQL 闭环验收 14/14 通过：提交→通过→过期版本拦截→防自审→已认证禁止重提→申请取消（重复申请幂等）→解除绑定→重新提交（照片资产替换）→驳回（原因落库）→审计事件序列完整。
- 新增 MySQL 集成测试 `backend/internal/module/studentverification/repository_mysql_test.go`（按仓库规范使用 TEST_MYSQL_DSN 临时库，门控跳过；本地 campus 账号无建库权限时自动 skip）。
- 管理后台「学生认证」页面与小程序真机流程未在本次本地验收中人工操作（见下方人工清单）。

## 本次验证结果（部署前，本机）

- Go：`studentverification`、`migrations`、`user`、`cmd/server` 测试通过；`go build ./...`、`go vet ./...` 通过。
- 已知存量问题（与本批改动无关，已用 git stash 复证）：`internal/module/media`、`media/miniappflow` 中针对本机 MySQL 8.4 的外部集成测试失败，属本地数据库环境问题。
- 前端：`apps/admin` 与 `apps/miniapp` typecheck、生产构建通过；契约脚本（边界/迁移/路由/业务闭环/UI 系统/资料中心/外观模块等）全部通过。

## 仍未验收

- 生产库迁移与部署未执行（本文档为准备稿）。
- 学生认证全流程及照片裁剪未在真机验收。
- 隐私保护指引与隐私授权回调未配置（上线前独立检查项）。
