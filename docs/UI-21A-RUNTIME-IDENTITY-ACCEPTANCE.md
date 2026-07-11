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

预期完整场景脚本：

- `D:\codex-temp\ui21a-automator\runtime-scenarios-connect.js`

设计覆盖：

1. 无 token 冷启动，只触发一次 `POST /api/v1/auth/wechat-login`，并正常渲染页面；
2. merchant 身份冷启动进入商家首页；
3. rider 身份冷启动进入骑手首页；
4. 快速重复确认身份切换只发送一次 `POST /api/v1/auth/identity`；
5. student 非法深链进入商家 / 骑手页时，不泄露业务内容并回到安全页面。

当前状态：未完整跑通，原因是本机微信开发者工具自动化 websocket 连接不稳定。

具体证据：

```text
Error: Failed connecting to ws://127.0.0.1:9420,
check if target project window is opened with automation enabled
```

直接启动微信开发者工具 CLI 自动化服务时，CLI 能启动服务：

```powershell
& 'D:\3314\微信web开发者工具\cli.bat' auto --project 'C:\Users\33141\Documents\ui21a-runtime-identity\apps\miniapp' --auto-port 9445 --trust-project
```

输出显示：

```text
√ IDE server has started, listening on http://127.0.0.1:9420
√ Using AppID: wx27a4c1be22695ed6
√ auto
```

但 `miniprogram-automator.connect` 仍无法稳定连接该 websocket。本项判定为本机 DevTools 自动化链路限制，不作为应用代码错误处理。

因此，本次可确认：

- Red/Green 页面挂载问题已被验证；
- `typecheck` 与 `build:miniapp` 均通过；
- 五场景完整自动化仍保留为手动 / 后续工具链修复后的补充验收项；
- 未冒充五场景全量 PASS。

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

验收过程中启动的后端 / Meilisearch 进程在收尾阶段关闭；若微信开发者工具窗口仍开着，视为 owner 本地交互窗口，不强制关闭。

## 8. 结论

UI-21a 的核心问题“运行时门控导致 Taro 页面空挂载”已经修复并通过 Red/Green 证据验证。

当前结论：

- 代码修复：PASS；
- `npm run typecheck`：PASS；
- `npm run build:miniapp`：PASS；
- Red/Green 挂载验收：PASS；
- 五场景全自动验收：受本机 DevTools websocket 限制，未完整通过，保留为 conditional。
