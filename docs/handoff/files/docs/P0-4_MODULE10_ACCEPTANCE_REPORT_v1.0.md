# P0-4 Module ⑩ Acceptance Report v1.0

日期：2026-06-21  
验收对象：MiniApp Upload Flow  
最终结论：**CONDITIONAL PASS**

## 1. 验收矩阵

| 验收项 | 结果 | 证据/说明 |
|---|---|---|
| Create Upload Ticket | PASS | 通过冻结 UploadTicketService 创建，票据与 owner/asset 一对一绑定 |
| STS Authorization | PASS | 真实腾讯云 GetFederationToken，精确 staging key 单 PUT |
| COS PUT | PASS | 真实 COS，签名覆盖 Length/MD5/Type/Host/forbid-overwrite |
| Upload Completion | PASS | 使用冻结 CompletionService，固定版本 HEAD/GET/Copy |
| Review Projection | PASS | 通过冻结 ReviewProjectionService 建立投影 |
| Consumer Processing | PASS | Outbox → Inbox → Projection，重复事件只消费一次 |
| Final Asset State | PASS | 服务端返回事实状态，前端不自行推导 |
| 重复 Complete | PASS | Final、Outbox、Inbox 均未重复产生 |
| 权限与身份 | PASS | 路由鉴权；owner 隔离；无 Token 401；跨用户不可见 |
| 上传失败/过期恢复 | PASS | 服务端返回 new_upload/complete_upload/poll 等恢复动作 |
| 上传历史与状态展示 | PASS | MiniApp 页面、类型检查与构建通过 |
| 真实 MySQL 8.4 | PASS | MySQL 8.4.0 外部集成测试通过，测试后残留 0 |
| 真实腾讯云 STS/COS | PASS | 无 Mock 外部集成测试通过，COS 版本和删除标记残留 0 |
| 微信开发者工具载入 | PASS | Stable v2.01.2510290，SDK 3.15.2，模拟器页面可运行 |
| 微信开发者工具真实 UI 全链路 | **BLOCKED** | IDE AppID 与后端 AppID 不一致，尚无原生选图到最终状态的同会话证据 |
| Boundary Check | PASS | 无新迁移、无领域模型、无跨域写入、无直接 COS/数据库绕行 |
| `go test ./... -count=1` | PASS | 破坏性迁移套件以 `GOFLAGS=-p=1` 串行执行 |
| `npm run verify` | PASS | Boundary、Admin Typecheck、MiniApp Typecheck、Backend Tests 全通过 |
| MiniApp Build | PASS | Taro production build 成功 |

## 2. 真实云闭环结果

无 Mock 外部测试实际执行：

1. 创建真实 MySQL Asset、Ticket 与 Reservation；
2. 真实腾讯云 STS 签发精确 staging key 授权；
3. 使用签名 URL 单 PUT 到真实 COS；
4. 使用冻结 CompletionService 完成固定版本 HEAD、GET、Copy；
5. 固化 Final Version；
6. 建立 Review Projection；
7. 写入并消费 Outbox，生成唯一 Inbox；
8. 重复 Complete 返回同一 Asset，不重复 Copy 或消费；
9. 跨 owner 查询被拒绝；
10. 删除全部 COS 版本/删除标记并清理数据库，残留为 0。

该结果证明后台、数据库和腾讯云链路是真实闭环，但不能替代微信开发者工具内原生 `chooseMedia/compressImage/request` 的最终验收。

## 3. 微信开发者工具验收

已验证：

- 真实微信开发者工具 Stable v2.01.2510290；
- 官方 CLI 自动化端口可连接；
- 模拟器型号 iPhone 12/13 Pro；
- SDKVersion 3.15.2；
- 当前页 `pages/home/index`；
- `wx.login` 在工具内返回真实 code；
- MiniApp Type Check 与生产构建通过。

未通过项：工具会话实际启用了测试 AppID `wx961b****f25b`，后端微信配置为 `wx27a****95ed6`。因此登录 code 无法在同一 AppID 下换取后端身份，未形成“真实登录 → 原生选图 → COS PUT → Complete → 状态展示”的完整工具端证据。测试未使用 `mockWxMethod`、Fake COS 或 Fake Completion。

## 4. 权限与边界

- API 统一使用现有 `AuthRequired + UnmutedRequired`；
- ownerID 仅取服务端认证上下文，不接受客户端传入；
- 查询和完成操作均进行 owner 约束；
- 响应 DTO 不包含 SecretId、SecretKey、SessionToken、staging/final key、versionId 或数据库内部字段；
- 小程序不依赖 COS SDK，不持有长期凭证；
- 未新增迁移 015，未改迁移 009～014；
- 未修改 Module ②～⑨冻结生产契约；
- 发帖、头像场景当前仅媒体上传和只读状态接线，不写 Community/User/Trade 表。

## 5. 自动化与残留

| 项目 | 结果 |
|---|---|
| Media miniappflow tests | PASS |
| Real MySQL 8.4 test | PASS |
| Real STS/COS E2E | PASS |
| Frozen Module ④ real COS sanity regression | PASS |
| Full Go regression | PASS |
| Admin Typecheck | PASS |
| MiniApp Typecheck | PASS |
| Boundary Check | PASS |
| MiniApp Build | PASS |
| DB fixture residue | 0 |
| COS staging/final versions and delete markers | 0 |
| New migration | 0 |

冻结迁移测试会在多个 Go package 中对同一验收库执行 rollback/up。并行 package 执行会互相拆表，因此全量验收使用 `GOFLAGS=-p=1` 串行运行；这是现有验收测试隔离约束，不是生产链路降级。

## 6. 剩余风险

### 高风险（验收门禁）

1. 微信开发者工具真实用户端到端尚无完整证据。需要将 IDE 切换为后端同一 AppID，再完成一张图片的真实原生上传和状态刷新。关闭前不得宣告 Module ⑩ 正式 PASS。

### 中风险

1. 业务引用绑定尚未实施。当前是本阶段刻意保持的只读接线；应在 P0-4 整体业务联调获得单独授权后，通过既有引用/Outbox 边界完成，不能在小程序端直接写 image id。
2. 破坏性迁移验收测试共享数据库，CI 必须串行执行或为各 package 分配独立数据库。

### 低风险

1. MiniApp production bundle 中 vendors.js 约 731 KiB，构建仅告警，后续上线准备可评估分包；不影响本模块正确性。

## 7. 最终结论

Module ⑩ 的代码、真实 MySQL、真实 STS/COS、权限、边界、幂等、全量回归和零残留均已通过；微信开发者工具真实 UI 全链路仍缺同 AppID 下的一次最终验证。因此本报告结论为 **CONDITIONAL PASS**，暂不冻结为正式 PASS。

在补齐该证据前，停止开发，不进入 P0-4 整体联调后续工作，不进入 P0-5。
