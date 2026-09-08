# 小程序 AppID 切换设计

## 目标

将“湛科生活圈”当前小程序身份从 `wx0dacef976daa8e55` 切换为 `wx065d8f3ce4a056ee`，使微信开发者工具工程配置与后端部署模板保持一致。

## 范围

- 更新 `apps/miniapp/project.config.json`，作为微信开发者工具实际工程 AppID。
- 更新 `apps/miniapp/project.private.config.example.json`，避免新建本地私有配置时回退到旧 AppID。
- 更新根目录与 `backend` 的 `.env.example` 中 `WECHAT_APP_ID`，使部署模板匹配小程序工程。
- 不修改外部商家导航的 `appId` 数据；那是被跳转的小程序身份，不是本项目身份。
- 不修改历史验收报告和历史实施计划中的旧 AppID，它们记录当时事实。
- 不修改后端测试中的样例 AppID；这些测试验证配置读取，不代表部署值。
- 不创建或猜测 `WECHAT_APP_SECRET`。真实微信登录必须在部署环境中注入与新 AppID 配套的 Secret。

## 验证

1. 四个当前配置入口均精确等于 `wx065d8f3ce4a056ee`。
2. 两个 JSON 配置可正常解析。
3. MiniApp TypeScript 类型检查与生产构建通过。
4. Git 暂存仅包含本次配置、规格和计划文件，不包含用户已有的 MiniApp 脏改动或 `artifacts/`。

## 风险边界

AppID 改变后，微信侧 OpenID 命名空间会随应用身份变化；若这是不同微信小程序主体/应用，旧 AppID 下的 OpenID 不能直接当作新 AppID 用户身份复用。代码切换不等于微信公众平台迁移，服务器域名、业务域名、隐私协议、类目、体验成员和新 AppSecret 仍需在微信公众平台配置。
