# 小程序体验版白屏与工作台体积治理设计

## 背景

体验版在手机进入后白屏，且“我的”页面背景淡化、底部导航与钱包入口出现回退或崩坏。上一版上传为通过 2MB 包体限制，在构建产物外又使用了激进的 Terser 压缩（`unsafe`、`toplevel`、`drop_console`），微信 CLI 虽然上传成功，但没有真机运行证据。

## 目标

1. 让小程序以正常 Taro 构建产物运行，不再依赖上传前的激进二次压缩。
2. 移除旧的内置商家点餐/商家商品链路，保留外部微信商家目录与跳转能力。
3. 保留商家配送工作台、骑手配送工作台、钱包、私信、身份切换和订单接口预留。
4. 恢复“我的”页面背景淡化、底部导航和钱包入口的稳定视觉与路由契约。
5. 不修改后端接口、支付业务、数据库、学生端帖子/二手业务和外部商家合同。

## 范围

### 保留

- `pages/student-takeout/index`：只读取 `external_takeout_merchants`，跳转到外部微信商家小程序。
- `pages/merchant-home/index`、`pages/merchant-publish/index`、`pages/merchant-order-detail/index`、`pages/merchant-wallet/index`、`pages/merchant-messages/index`。
- `pages/delivery-rider/index`、`pages/rider-wallet/index`、`pages/rider-messages/index`、`pages/delivery-profile/index`、`pages/delivery-conversation/index`。
- 现有钱包余额、提现中、待打款队列和后台处理接口。

### 移出小程序编译入口

- 旧的内置点餐/商品业务页面（`takeout-*`、旧 `delivery-merchant`、旧 `takeout-order-detail` 等）及只服务于这些页面的演示入口。
- 不删除源码或后端模块；仅从 `app.config.ts` 和可达的前端静态入口中隔离，便于后续按资质重新接入。

## 实现策略

### 包体与运行时

- 使用 `npm run build:miniapp` 的正常 Taro 产物作为上传包。
- 禁止 `unsafe`、`toplevel`、`drop_console` 等上传前二次压缩。
- 通过移除不可达旧页面/演示链路降低包体；运行 `check-miniapp-package-size.cjs` 确认低于微信主包 2MB 限制。
- 首屏 `App`、会话恢复、外部商家目录和钱包页面不依赖实验性的 UX 组件；组件保留时提供基础样式/降级路径。

### UI 与导航稳定性

- 以当前工作树中最近稳定的“我的”背景淡化、`StudentBottomNav` 与钱包入口为基线，撤销只影响这些契约的实验性样式覆盖。
- 保留必要的触控、下拉刷新和页面返回；不把底部导航改成新的独立视觉系统。
- 钱包页面首次进入只执行原有加载，不要求点击“刷新资金记录”；加载失败显示可重试状态，不白屏。

## 错误处理

- 首屏运行时异常必须由 `RuntimeSessionGate`/`AppState` 呈现错误与重试，不允许空白容器。
- 外部商家接口失败只影响校园服务页，不能阻塞 App 启动和钱包路由。
- 钱包 API 或会话恢复失败显示可读错误和重试，保留当前账号隔离。

## 验收

1. `npm --workspace apps/miniapp run typecheck` 通过。
2. `npm run build:miniapp` 通过，且无上传前二次压缩步骤。
3. `node scripts/check-miniapp-package-size.cjs` 通过，主包小于 2MB。
4. 外部商家目录、角色工作台、钱包、私信和底部导航既有检查通过。
5. 微信开发者工具使用正常 `apps/miniapp/dist` 预览：首屏不白屏，进入“我的”、商家/骑手工作台和钱包均可见；本地预览通过后再决定是否上传体验版。

## 非目标

- 不在本次实现三端外卖自动接单、微信支付生产密钥配置或正式发布。
- 不删除后端旧模块，不清理数据库，不覆盖其他未提交修改。
