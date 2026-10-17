# 模拟体验包实施计划

> 本任务按用户“全自动完成”授权在当前会话执行，不另外派代理。

**Goal:** 让用户在微信中查看头像、封面、帖子、商品和私信的真实组件效果。

**Architecture:** `TARO_APP_VISUAL_DEMO=true` 构建时使用 `NormalModuleReplacementPlugin` 精确替换 `services/taro`、`services/e-im`、`hooks/use-page-share`。普通构建不替换。示例仅在 `src/demo` 与其素材目录内。

**Tech Stack:** Taro 4、React、TypeScript、Webpack、官方微信 CLI。

- [x] 添加 `src/demo/data.ts`，定义 6 用户、24 帖子、18 商品及 6 人互相关联的数据。
- [x] `src/demo/media-sources.json` 记录来源和授权；脚本从 Wikimedia 的已核对文件拉取并压缩照片。
- [x] `src/demo/runtime.ts` 检查 develop/trial；`src/demo/taro.ts` 拦截请求、登录、存储、付款；拒绝未知业务请求，无后端回落。
- [x] `src/demo/im.ts` 本地演示会话、历史与临时发送；不实例化腾讯 SDK。
- [x] `src/demo/indicator.tsx` 和 `src/demo/share.ts` 提供持续演示标识与照片署名，禁止分享演示关系到生产。说明弹层使用 `RootPortal`，不受头像与吸顶栏的层叠上下文遮挡。
- [x] 修改 `config/index.ts`、`config/prod.ts`、`app.config.ts` 与 `components/capsule-page-header.tsx` 加入明确编译开关；资源 URL 适配兼容加载时空头像。普通构建无 demo，演示在版本名中标识。
- [x] 按最终反馈使用正常摄影头像和封面，保留 2 二次元、2 追星、2 日常用户兴趣，发帖归属一致；17 张素材逐张检查内容和许可。
- [x] `scripts/test-visual-demo.cjs` 验证分页、双向用户归属、未知请求拒绝、存储前缀和 release 拒绝；运行 TypeScript 和构建，核对文件树及包体。
- [x] 官方 CLI 已上传 `2026.10.17.4-demo`，备注“优化体验”，回执包体 1,512,984 字节。实现已独立提交，推送与预览二维码见发布记录。
- [ ] 微信公众平台“设为体验版”尚未核验；CLI 开发上传不等于此步骤，不送审、不发布。
