# 湛科生活圈 Claude 交接说明

更新日期：2026-11-13
工作分支：`codex/experience-ui-wallet-20261101`
交接基线：`0a2c7ba`，最终代码以本分支 HEAD 为准。

## 2026-11-13 后续：管理后台可用性改造

本节记录在上文之后完成、且已通过 `npm run verify` 的改动。

### 多管理员支持

- 后台不再是单一固定账号：`admin_credentials` 允许任意**管理员或超级管理员**登录，用户名可自定义。
- 超级管理员可在「用户与权限 → 后台登录账号」为管理员分配、重置、移除登录账号；普通管理员只能看到自己的页面，管理接口返回 403。
- 新增迁移 `058_admin_credential_events.sql`（扩展审计事件类型），回滚脚本在 `migrations/rollback/`。
- 相关接口：`GET/PUT/DELETE /api/v1/admin-auth/credentials`，均需管理员会话与 CSRF。

### 消息通知补全

- 后台「消息通知」新增**发送记录**面板，可按状态筛选并翻页，展示通道、接收人、结果、重试次数和失败原因。
- 新接口：`GET /api/v1/admin/delivery/notification-channels/deliveries`，只接受白名单状态值，不接受任意查询条件。

### 后台界面与文案

- 审核中心修复：类型标签重复渲染、`takeout_merchant` 等类型显示空白、时间显示为原始 ISO、风险字段直接暴露英文技术串。
- 全站字号上调（原 10–11px 中文偏小），按钮与表格行高同步调整。
- 管理后台文案统一改为运营视角，移除「服务端迁移」「账本核验」「显式开关」「数据版本」等技术表述。
- 素材中心补齐 `unavailable` 等状态映射；服务入口把 `legacy-*` 占位值显示为「未配置」。
- 登录页用户名改为可编辑，并提示密码长度要求。
- 侧边栏与登录页视觉重做。

### 微信与手机号绑定

- 微信登录（openid）本身已完整，无需改动。
- 手机号绑定补全：号码被其他账号占用时返回 409 与明确文案；用户确认归属后可把绑定转移到当前账号（`takeOver`）；新增解绑接口。
- 后台新增手机号列（掩码）与解绑操作，供客服处理号码被误绑的情况。
- 接口：`POST /auth/me/phone`（body 增加可选 `takeOver`）、`DELETE /auth/me/phone`、`DELETE /admin/users/:id/phone`。
- 小程序设置页：已绑定时显示掩码与「解绑」，冲突时弹出转移确认。
- 手机号不作为独立登录方式；界面文案已改为说明其真实用途。

### 用户反馈处理

- 后台「意见反馈」支持标记为待处理 / 处理中 / 已解决 / 不处理，并填写处理说明。
- 迁移 `059_user_feedback_handling.sql` 新增 `admin_note`、`handled_by`、`handled_at` 与 `(status, created_at)` 索引，回滚脚本在 `migrations/rollback/`。
- 接口：`PATCH /admin/feedbacks/:id`，只允许管理员与超级管理员调用。
- 已知边界：MySQL 在写入值与现值相同时返回 `RowsAffected=0`，仓储层已改为再查一次存在性，避免把幂等更新误判为「记录不存在」。

### 本地开发环境

- 本地库已补齐 038–058 迁移；`meilisearch.exe` 与后端 8080 可本地直接启动。
- 管理员本地密码需自行用 `backend/cmd/admin-credential` 或 `adminauth` 的 argon2id 规则设置，密码长度 14–72。
- 开启 `ENABLE_USER_NOTIFICATIONS=true` 时，`MYSQL_DSN` 必须带固定偏移时区（`time_zone=%27%2B08%3A00%27`），否则启动即拒绝。本地 `.env` 已按此配置。
- 服务号绑定路由在通知启用后注册；`ENABLE_WECHAT_OFFICIAL_NOTIFICATIONS=false` 时 `/notification/official-account/status` 返回「未开通」。


## 项目目标

本项目是已上线的湛科生活圈微信小程序，包含 Taro 小程序、Go 后端和 React 管理后台。本轮目标是：

1. 将管理后台整理成经营与运营工作台，并显示真实配送营收。
2. 统一后台业务用语、导航分组、状态表达和视觉层级。
3. 补齐稳定订单通知：小程序内通知、小程序订阅、微信服务号、企业微信值班群。
4. 保持订单、钱包、支付、退款、提现、媒体、审核等既有业务逻辑不变。
5. 为 Claude 留下可验证、可继续部署的完整状态，不伪造凭证或外部送达。

## 已完成

### 管理后台

- 新增独立“经营总览”，直接读取现有真实接口：
  - 累计平台配送收入
  - 今日平台收入
  - 今日配送成交额
  - 今日订单和今日完成单
  - 商家钱包、骑手钱包、待结骑手报酬、已登记结款
- “今日”统一按北京时间 00:00 计算。
- 导航重排为“工作台、业务运营、内容与用户、平台设置”。
- 业务名称改为“经营总览、配送与结算、消息通知、素材中心、商家骑手认证”等直观用词。
- 新增独立“消息通知”页面，展示：
  - 小程序通知中心
  - 小程序订单提醒
  - 微信服务号提醒
  - 企业微信值班群提醒
  - 发送开关、受众、24 小时发送量、排队量、异常量
  - 服务号绑定与关注状态
  - 商家、骑手值班群安全测试
- 后台不接收、不展示 AppSecret、OpenID 或机器人 webhook。
- 媒体、配送、退款、提现等页面清理了 `pending`、`deadletter`、`GC`、`Row Version` 等技术展示词。

关键文件：

- `apps/admin/src/App.tsx`
- `apps/admin/src/NotificationConsole.tsx`
- `apps/admin/src/admin-navigation.ts`
- `apps/admin/src/styles.css`
- `backend/internal/module/admin/notificationconsole/`
- `backend/internal/module/admin/deliveryconsole/`

### 稳定订单通知

- 小程序订阅消息继续按一次性额度累积。
- 新增微信服务号绑定、OAuth 回调、OpenID 存储、关注状态和模板消息发送。
- 新增企业微信商家群与骑手群机器人投递、重试、失败状态和后台测试接口。
- 订单通知发送前会复核当前订单事实，过期或已不符合资格的订单不会误发。
- 私信正文、手机号、地址和敏感凭证不会进入企业微信群消息。
- 通知核心模型与 HTTP/provider 适配器已按仓库边界规则拆分。

关键文件：

- `backend/internal/module/notification/official_account.go`
- `backend/internal/module/notification/official_sender.go`
- `backend/internal/module/notification/official_worker.go`
- `backend/internal/module/notification/wecom_sender.go`
- `backend/internal/module/notification/wecom_worker.go`
- `backend/cmd/server/user_notification_wecom.go`
- `apps/miniapp/src/pages/settings/index.tsx`
- `apps/miniapp/src/pages/official-account-bind/`
- `backend/migrations/057_notification_official_account.sql`

## 已验证

以下命令在本分支已通过：

```powershell
npm.cmd run verify
npm.cmd --workspace apps/admin run build
npm.cmd run typecheck
go test ./...
node scripts/check-notification-contract.cjs
git diff --check
```

`npm run verify` 包含 33 个小程序路由、后台合同、通知合同、边界检查、媒体检查、钱包/支付回归、Go 后端全量测试和上线回归。测试通过不等同于生产环境或真机验收。

管理后台已使用本地模拟接口做过浏览器视觉检查，模拟数据没有写入仓库，也没有冒充真实营收或真实消息送达。

## 外部现状（2026-11-13 实际核对）

生产服务器：`admin@zhanduiwang.top`
生产环境文件：`/etc/campus-circle/campus-api.env`

以下环境键目前均为空：

```text
ENABLE_WECHAT_OFFICIAL_NOTIFICATIONS
WECHAT_OFFICIAL_APP_ID
WECHAT_OFFICIAL_APP_SECRET
WECHAT_OFFICIAL_OAUTH_REDIRECT_URL
WECHAT_OFFICIAL_ORDER_TEMPLATE_ID
WECHAT_OFFICIAL_ORDER_TEMPLATE_FIELDS
WECOM_MERCHANT_WEBHOOK_URL
WECOM_RIDER_WEBHOOK_URL
```

微信公众平台当前登录账号下只看到小程序账号和小程序/小游戏测试号，没有可用服务号。企业微信管理端当前停留在扫码登录页，尚无可配置的组织登录态。

因此当前不能在本地诚实完成正式服务号外发和正式企业微信群送达。

## Claude 下一步

### 1. 微信服务号正式凭证

需要一个已认证并登录的服务号，且最好与小程序绑定在同一微信开放平台主体：

1. 获取服务号 AppID。
2. 在服务号后台生成或确认 AppSecret，值只写入服务器环境文件，不进入 Git、日志、聊天或截图。
3. 配置 OAuth 回调：
   `https://zhanduiwang.top/api/v1/notification/official-account/callback`
4. 在服务号后台配置对应网页授权域名。
5. 在小程序后台“业务域名”加入 `https://zhanduiwang.top`，否则 `web-view` 绑定页无法打开。
6. 创建并审核订单模板，记录模板 ID。
7. 按模板实际字段构造审核过的 JSON 映射，例如：

```json
{"character_string1":"orderId","thing2":"status","time3":"time"}
```

实际字段必须与微信后台模板一致，不能猜测。

### 2. 企业微信正式机器人

1. 登录企业微信管理端。
2. 分别创建商家值班群和骑手值班群。
3. 在群内添加机器人，取得完整 webhook。
4. webhook 只写入生产环境：
   - `WECOM_MERCHANT_WEBHOOK_URL`
   - `WECOM_RIDER_WEBHOOK_URL`

### 3. 安全写入生产环境

不要把密钥放入命令行历史、仓库文件或工具输出。建议交互式登录服务器后使用受控编辑器写入 `/etc/campus-circle/campus-api.env`，并保持文件仅服务用户可读。

需要增加的配置名称：

```text
ENABLE_WECHAT_OFFICIAL_NOTIFICATIONS=true
WECHAT_OFFICIAL_APP_ID
WECHAT_OFFICIAL_APP_SECRET
WECHAT_OFFICIAL_API_BASE=https://api.weixin.qq.com
WECHAT_OFFICIAL_OAUTH_REDIRECT_URL=https://zhanduiwang.top/api/v1/notification/official-account/callback
WECHAT_OFFICIAL_ORDER_TEMPLATE_ID
WECHAT_OFFICIAL_ORDER_TEMPLATE_FIELDS
WECOM_MERCHANT_WEBHOOK_URL
WECOM_RIDER_WEBHOOK_URL
```

### 4. 数据库与发布顺序

1. 先备份生产数据库并核对当前 migration 状态。
2. 在维护窗口执行 `backend/migrations/057_notification_official_account.sql`。
3. 发布后端二进制和管理后台静态文件。
4. 重启 `campus-api`。
5. 检查 `/healthz`、`/readyz`、`/version`。
6. 打开管理后台“消息通知”，确认四类通道与配置状态。
7. 用测试账号绑定服务号。
8. 分别向商家群、骑手群发送后台测试消息。
9. 创建一笔测试配送订单，验证商家和骑手收到站内、服务号或群提醒。
10. 真实外发通过后，再评估是否把 `NOTIFICATION_AUDIENCE` 切到 `all`。

发布细节参考：

- `docs/superpowers/plans/2026-09-08-production-release.md`
- `docs/ADMIN-PASSWORD-LOGIN-RUNBOOK.md`
- `docs/plans/2026-11-13-stable-order-notification.md`

## 必须保留的安全边界

- 不提交 `.env`、AppSecret、私钥、OpenID、机器人 webhook。
- 不把真实密钥回显到终端、截图或 PR 描述。
- 不伪造微信接口成功、群消息送达、订单或后台验收。
- 不在没有数据库备份和 migration 核对的情况下部署。
- 不把“代码测试通过”写成“生产通知已送达”。
- 服务号 OAuth 和模板字段必须按微信后台实际审核结果配置。

## 建议的首轮检查

```powershell
git status --short --branch
npm.cmd run verify
npm.cmd --workspace apps/admin run build
```

如果 Claude 只处理生产凭证配置，不应改动当前已通过验证的业务代码；优先完成外部账号、模板、域名和环境变量配置，再做真实订单验收。
