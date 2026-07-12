# UI-21a 运行时身份门控验收报告

## 1. 范围

本次修复针对小程序静态壳在 Taro 运行时下无法稳定挂载页面的问题。

- 分支：`feat/ui21a-runtime-identity`
- 修复提交：`a91d3384 fix(UI-21a): 修复Taro页面运行时挂载`
- 关键原则：
  - `apps/miniapp/src/app.tsx` 始终返回 `props.children`；
  - 登录、身份读取、非法身份拦截等副作用下沉到 `RuntimeSessionGate`；
  - 学生、商家、骑手页面保留运行时身份守卫；
  - 不触碰 `feat/p0-identity-switch@c9417df8`；
  - 不改后端、接口、迁移或业务 API。

## 2. 变更文件

实际代码变更仅限以下文件：

- `apps/miniapp/src/app.tsx`
- `apps/miniapp/src/components/runtime-session-gate.tsx`
- `apps/miniapp/src/pages/home/index.tsx`
- `apps/miniapp/src/pages/profile/index.tsx`
- `apps/miniapp/src/pages/merchant-home/index.tsx`
- `apps/miniapp/src/pages/delivery-rider/index.tsx`

本次报告文件：

- `docs/UI-21A-RUNTIME-IDENTITY-ACCEPTANCE.md`

## 3. Red / Green 证据

### Red：修复前挂载失败

基线提交：`6515cb828d7dc56a712c05302a287e3bc6780ef9`

现象：页面根节点为空，只有 `<page></page>`，token 已存在但首屏不渲染。

证据保留在：

- `D:\codex-temp\ui21a-runtime\red-build-6515cb82.log`
- `D:\codex-temp\ui21a-runtime\red-mounted-assertion-6515cb82.log`
- `D:\codex-temp\ui21a-runtime\red-mounted-assertion-6515cb82.json`
- `D:\codex-temp\ui21a-runtime\red-mounted-assertion-6515cb82.png`

### Green：修复后可挂载

修复后通过微信开发者工具自动化连接验证：学生首页首屏可渲染，根节点非空。

证据保留在：

- `D:\codex-temp\ui21a-runtime\green-mounted-assertion.log`
- `D:\codex-temp\ui21a-runtime\green-mounted-assertion.json`
- `D:\codex-temp\ui21a-runtime\green-mounted-assertion.png`

自动化脚本位置：

- `D:\codex-temp\ui21a-automator\assert-mounted-green.js`

脚本使用 `miniprogram-automator.connect({ wsEndpoint: 'ws://127.0.0.1:9420' })` 连接本机微信开发者工具。

## 4. 本地检查

在 `C:\Users\33141\Documents\ui21a-runtime-identity\apps\miniapp` 执行：

```powershell
npm run typecheck
```

结果：PASS。

在 `C:\Users\33141\Documents\ui21a-runtime-identity` 仓库根目录执行：

```powershell
npm run build:miniapp
```

结果：PASS。

说明：`build:miniapp` 是仓库根目录脚本，不是 `apps/miniapp` 子包脚本。曾在子目录误执行一次并得到 `Missing script: build:miniapp`，随后已在正确目录重跑通过。

构建只出现既有 Sass 警告：

- `apps/miniapp/src/pages/publish-post/index.scss` 的 Sass `@import` deprecation warning。

该警告不是 UI-21a 新增错误。

## 5. 五场景自动化验收状态

2026-07-12 在主仓库 `master@a4aa474b` 上完成补充验收。为确保“冷启动”是真正的新进程启动，而不是同一 Zustand 实例内的 `reLaunch`，每个冷启动场景均通过微信开发者工具 CLI 关闭项目后重新启动，并使用独立自动化端口连接。

补充验收环境：

- 真实 MySQL 8.4：本机 `MySQL84`，`127.0.0.1:3306`；
- 真实 Meilisearch：`127.0.0.1:7700`；
- 真实后端：主仓库 `backend/cmd/server`，`http://127.0.0.1:8080/healthz` 返回成功；
- 真实微信开发者工具：AppID `wx27a4c1be22695ed6`；
- 自动化：`miniprogram-automator` + 微信开发者工具 CLI `--trust-project`；
- 临时验收夹具仅为测试用户补充 active rider 身份，结束后恢复原身份并删除夹具记录。

完整结果：

1. **无 token 真冷启动：PASS**
   页面根节点非空，token 成功写入；后端日志确认仅有一次 `POST /api/v1/auth/wechat-login`。
2. **merchant 真冷启动恢复：PASS**
   最终路由 `pages/merchant-home/index`；`.merchant-home-shop`、`.merchant-home-metrics`、`.merchant-home-shortcuts` 均为 1，订单卡为 2。
3. **rider 真冷启动恢复：PASS**
   最终路由 `pages/delivery-rider/index`；`.rider-grab-user`、`.rider-grab-wallet-summary`、`.rider-grab-tabs` 均为 1，订单卡为 3。
4. **身份切换重复确认：PASS**
   对同一确认按钮并发触发两次点击，后端日志仅出现一次 `POST /api/v1/auth/identity`，最终进入商家页。
5. **student 非法深链 fail-closed：PASS**
   分别深链进入商家页与骑手页；20 轮快速采样中全部受限业务选择器最大命中数均为 0，两个场景最终都回到 `pages/profile/index`。

机器可读证据：

- `D:\codex-temp\ui21a-current\evidence\ui21a-runtime-scenarios.json`
- `D:\codex-temp\ui21a-current\evidence\screens\01-no-token-cold-start.png`
- `D:\codex-temp\ui21a-current\evidence\screens\02-merchant-cold-start.png`
- `D:\codex-temp\ui21a-current\evidence\screens\03-rider-cold-start.png`
- `D:\codex-temp\ui21a-current\evidence\screens\04-double-confirm-merchant.png`
- `D:\codex-temp\ui21a-current\evidence\screens\05-illegal-merchant-deeplink.png`
- `D:\codex-temp\ui21a-current\evidence\screens\05-illegal-rider-deeplink.png`

补充验收脚本：

- `D:\codex-temp\ui21a-automator\runtime-scenarios-current.js`

原先的 websocket `Conditional` 已通过“每次冷启动独立 CLI 自动化端口 + 页面就绪重试”消除；本轮五场景可判定为全量 PASS。

## 6. 边界与副作用

本次没有修改：

- `backend/**`
- `backend/migrations/**`
- `apps/admin/**`
- `package.json`
- lockfile
- `feat/p0-identity-switch`

运行时守卫仍保留：

- 商家页未授权时不展示 `.merchant-home-shop`、`.merchant-home-metrics`、`.merchant-home-shortcuts`、`.merchant-order-card` 等业务内容；
- 骑手页未授权时不展示 `.rider-grab-user`、`.rider-grab-wallet-summary`、`.rider-grab-tabs`、`.rider-order-card` 等业务内容；
- `RuntimeSessionGate` 负责登录态与身份态，页面组件不再阻断 Taro 挂载根节点。

## 7. 残留

临时证据目录保留，供复核：

- `D:\codex-temp\ui21a-runtime`
- `D:\codex-temp\ui21a-automator`
- `D:\codex-temp\ui21a-current\evidence`

验收过程中创建的 rider 夹具已清理，测试用户身份已恢复；验收过程中启动的后端 / Meilisearch 进程在收尾阶段关闭。证据目录保留，不属于仓库残留。

## 8. 结论

UI-21a 的核心问题“运行时门控导致 Taro 页面空挂载”已经修复并通过 Red/Green 证据验证。

当前结论：

- 代码修复：PASS；
- `npm run typecheck`：PASS；
- `npm run build:miniapp`：PASS；
- Red/Green 挂载验收：PASS；
- 五场景全自动验收：PASS；
- 原 `Conditional`：已解除。
