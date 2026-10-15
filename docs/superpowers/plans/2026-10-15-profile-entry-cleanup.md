# 个人封面入口合并与设置图标修正

**Goal:** 按用户确认的方向，将封面操作统一放在编辑资料页，并修正偏心的设置齿轮。

**Architecture:** 主页删除重复入口，保留现有编辑资料路由、导航锁及 `ProfileCoverEditor` 的独立保存流程。只调整表现，不修改上传、审核、权限、接口或原项目。

**Tech Stack:** Taro / React / SCSS / SVG；独立 HTML 预览。

## 已确认范围

- 主页通过现有“编辑资料”（以及已有头像/资料点击区）进入编辑页；不再显示“设置封面/更换封面”。
- 编辑页内现有 `ProfileCoverEditor` 保留选择、保存、取消更换、恢复默认、失败重试、账号隔离。
- 设置图标保持 `#2E7DD4` 和 `48 × 48` 画布，内外轮廓统一中心 `(24,24)`、描边 `3.2`，不改 `AppIcon` 接口或设置路由。
- 本轮同步预览上述两处；前轮预览的其余完整排版不视为已落地。
- 原有项目配置与发布产物不纳入本次修改，不执行上线或上传。

## 实施顺序

- [x] `apps/miniapp/src/pages/profile/index.tsx`：工具栏仅保留标题，删除独立封面入口，保留 `openProfileEditor` 及编辑按钮。

```tsx
<View className='student-profile-hero__toolbar'>
  <Text className='student-profile-hero__title'>我的</Text>
</View>
```

- [x] `apps/miniapp/src/pages/profile/index.scss`：移除 `.student-profile-cover__edit` 的样式、按压样式和 reduced-motion 选择器；其余布局不变。
- [x] `apps/miniapp/src/assets/icons/action-settings.svg`：重建同心、八向对称的齿轮轮廓，根圆半径 14、齿顶半径 17、内孔半径 5.5，描边 3.2。
- [x] 可视化目录保留旧预览，生成 `profile-clarity-v2.html`；删除顶部封面按钮和对应 DOM 更新，编辑资料中保留背景区域；同步修正后的 SVG。
- [x] 运行现有 `check-profile-presentation.cjs`、`check-profile-cover.cjs`、`check-profile-center-ui.cjs`，确认编辑导航及封面保存行为保留。
- [x] 编译小程序并检查预览 320/390/430/1080 宽度、图标资源、编辑按钮 44px、无顶部封面入口、设置图标实际显示。
- [x] 打开更新后的预览，报告本地完成和尚未上传的边界。

## 验证结果

三项现有检查通过；旧测试强制要求主页封面按钮，已按用户新要求更新，同时保留并通过防重复导航、封面上传完成确认、保存失败重试及账号隔离断言。小程序构建成功，构建仍有 vendors.js 731 KiB 的体积建议警告。

四种宽度预览无横向溢出、无缺图；浏览器已实际查看“账户与支持”中的新齿轮。此次未上传微信开发版、未提审、未发布；仍待用户真机查看。

## 参考文档复核

最新参考资料库 `C:\Users\33141\Documents\Codex\帖子软件排版与逻辑` 的 `docs/02-页面状态机与复刻规格.md` 明确提供 P17「自己的主页」和 P19「资料编辑」页面状态，以及资料编辑的原始值/未保存值/错误边界；第 8.1 节要求按 `(viewerAccountId, profileUserId)` 隔离个人页状态，并在编辑后统一更新头像昵称缓存。此次实现继续使用原有账号作用域、保存后刷新 `currentUser` 和独立封面事务，没有引入文档中“未知”的吸顶、折叠或第三方功能。
