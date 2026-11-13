# 湛科生活圈 Claude 交接说明

更新日期：2026-11-13
工作分支：`codex/experience-ui-wallet-20261101`
交接基线：`0a2c7ba`，最终代码以本分支 HEAD 为准。

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

微信公众平台当前存在的“湛科生活圈”是个人主体、未认证的普通公众号，不是企业服务号；其功能设置中没有网页授权域名，也不具备本项目所需的模板消息能力。

项目决策：不再把微信服务号放入上线路径，也不执行 migration 057 作为发布前置条件。服务号相关代码继续保留为默认关闭的可选能力，但用户端和管理端不再显示未启用的公众号入口。

当前消息方案为：

1. 小程序内通知中心作为可靠底账。
2. 小程序订阅消息按用户授权次数发送。
3. 企业微信群用于商家、骑手值班提醒。
4. 后台记录聚合发送量、失败和重试状态。

企业微信管理端当前尚未登录组织账号，因此正式群机器人还未配置。

## Claude 下一步

### 1. 企业微信正式机器人

1. 登录企业微信管理端。
2. 分别创建商家值班群和骑手值班群。
3. 在群内添加机器人，取得完整 webhook。
4. webhook 只写入生产环境：
   - `WECOM_MERCHANT_WEBHOOK_URL`
   - `WECOM_RIDER_WEBHOOK_URL`

### 2. 安全写入生产环境

不要把密钥放入命令行历史、仓库文件或工具输出。建议交互式登录服务器后使用受控编辑器写入 `/etc/campus-circle/campus-api.env`，并保持文件仅服务用户可读。

需要增加的配置名称：

```text
ENABLE_WECHAT_OFFICIAL_NOTIFICATIONS=false
WECOM_MERCHANT_WEBHOOK_URL
WECOM_RIDER_WEBHOOK_URL
```

### 3. 数据库与发布顺序

1. 先备份生产数据库并核对当前 migration 状态。
2. 服务号关闭时不需要执行 migration 057。
3. 发布后端二进制和管理后台静态文件。
4. 重启 `campus-api`。
5. 检查 `/healthz`、`/readyz`、`/version`。
6. 打开管理后台“消息通知”，确认小程序内通知、小程序订阅和值班群状态。
7. 分别向商家群、骑手群发送后台测试消息。
8. 创建一笔测试配送订单，验证商家和骑手收到站内、订阅或群提醒。
9. 真实外发通过后，再评估是否把 `NOTIFICATION_AUDIENCE` 切到 `all`。

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
- 不再要求普通个人公众号承担服务号 OAuth 或模板消息。

## 建议的首轮检查

```powershell
git status --short --branch
npm.cmd run verify
npm.cmd --workspace apps/admin run build
```

如果 Claude 继续上线工作，不应重新引入服务号依赖；优先恢复生产后端、配置企业微信群、发布最新代码并用真实订单验收。
