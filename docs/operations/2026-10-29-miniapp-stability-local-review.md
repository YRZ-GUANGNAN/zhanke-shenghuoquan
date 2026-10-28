# 小程序稳定性修复：本地验收与发布前缺口

日期：2026-10-29。分支：`codex/native-theme-motion-20261025`。

本记录区分本地代码、模拟器、线上接口和微信发布。不执行微信上传、GitHub push、后端部署、迁移或真实付款。

## 已核实的修复

- 服务入口不再导入 `ServiceMerchantCatalog`，移除 `merchantId` 内置目录分支；保留 `external_takeout_merchants`、筛选、搜索及 `navigateToMiniProgram`。
- 封面渐变使用 `--profile-cover-rgb` 通道，不再生成无效的 `rgba(var(--cl), ...)`；保留原渐变停靠点及 `-1PX` 接缝处理。
- 底栏保持现有布局与图标，增加圆角、72滚动阈值、停止600ms恢复、反向立即恢复、减弱动效降级。
- 复审发现全局滚动广播会让离屏频道/隐藏页收起当前底栏。已用 `$taroPath`、滚动源和生命周期代际隔离；隐藏、重新显示、卸载和缺少路由时安全恢复。
- 底栏新测试执行真实 React hook 与生命周期，不再仅用正则匹配。旧实现在“离屏频道 enabled=false”场景失败，修复后通过。
- 采用正常 Taro 构建，仅配置 `ascii_only:false`，不进行上传前 `unsafe/toplevel/drop_console` 二次压缩。
- 包体检查现在拒绝缺失必要文件的空产物，不能以空目录冒充包体通过。

## 修改边界

稳定性白名单为下列21项；另外仅暂存独立候选重新生成的暗色样式头部（30→31页），保留当前工作区钱包色彩变更。

```text
apps/miniapp/config/prod.ts
apps/miniapp/src/components/delivery-workspace-nav.scss
apps/miniapp/src/components/delivery-workspace-nav.tsx
apps/miniapp/src/components/refresh-scroll-view.tsx
apps/miniapp/src/components/student-bottom-nav.scss
apps/miniapp/src/components/student-bottom-nav.tsx
apps/miniapp/src/components/surface-enhancements.scss
apps/miniapp/src/pages/delivery-profile/index.tsx
apps/miniapp/src/pages/merchant-messages/content.tsx
apps/miniapp/src/pages/student-takeout/index.tsx
apps/miniapp/src/styles/profile-cover-surface.scss
apps/miniapp/src/styles/system-dark-theme.scss
apps/miniapp/src/utils/bottom-nav-motion.ts
scripts/check-bottom-nav-motion.cjs
scripts/check-profile-cover-theme.cjs
scripts/check-external-takeout-directory.cjs
scripts/check-miniapp-package-size.cjs
scripts/check-refresh-feedback.cjs
scripts/check-sheet-dismiss.cjs
scripts/check-tactile-navigation.cjs
scripts/test-delivery-workspace.cjs
```

其中 `surface-enhancements.scss` 只是说明稳定降级的注释变化；action sheet、登录状态等部分恢复结果已经等同原HEAD，不能重复计为新增实现。

以下前序改动完整保留，不混入稳定性提交：钱包/订单支付、后台资金处理、Go后端与迁移、钱包计划、预览和旧上传回执。`check-miniapp-business-closure.cjs`、`check-role-workbench-ui.cjs`、`test-delivery-stage-workbench.cjs` 当前差异属于前序钱包功能。`system-dark-colors.generated.scss` 同时包含路由头更新与钱包样式变化，不能整体误归类。

审查起点：`git status --short` 为68行；展开未跟踪目录后为95个文件（40个已跟踪修改、55个未跟踪文件），并非68个源码文件。暂存区为空。混合工作区可以按白名单拆分，不是必须让用户重新选择的技术阻塞。

## 当前工作区定向验收

下列24条命令均以 `exit 0` 结束。逐项日志和起止时间保存于忽略目录 `.codex-tmp/stability-20261029/`。

```text
npm --workspace apps/miniapp run typecheck
node scripts/check-external-takeout-directory.cjs
node scripts/check-bottom-nav-motion.cjs
node scripts/check-profile-cover-theme.cjs
node scripts/check-refresh-feedback.cjs
node scripts/check-sheet-dismiss.cjs
node scripts/check-miniapp-ui-system.cjs
node scripts/check-miniapp-business-closure.cjs
node scripts/check-miniapp-static-routes.cjs
node scripts/check-role-workbench-ui.cjs
node scripts/check-home-real-tools-contract.cjs
node scripts/check-private-messaging.cjs
node scripts/test-private-messaging.cjs
node scripts/test-wallet-pages.cjs
node scripts/test-wallet-recovery.cjs
node scripts/test-manual-delivery-payment.cjs
node scripts/test-delivery-workspace.cjs
node scripts/test-delivery-stage-workbench.cjs
node scripts/test-delivery-publish-feedback.cjs
node scripts/check-tactile-navigation.cjs
node scripts/check-miniapp-dark-theme.cjs
npm run build:miniapp
node scripts/check-miniapp-package-size.cjs
git diff --check
```

工作区产物：140文件，2,065,035 bytes，距离2 MiB限制剩32,117 bytes。31个路由保留两端钱包、工作台、个人页、服务页；必要的JS/JSON/WXML/WXSS均存在，旧内置点餐路由未编译。

- 校验清单：`.codex-tmp/stability-20261029/validation.json`。
- 源文件清单SHA-256：`00e8e1a6c7fdd4f34184621faa7a697ee92e21dbf847f72cb70c368e28bffe7f`。
- 产物清单SHA-256：`73281cf88396b9da99c3cfc91331655afb478c32b8c6b181f1b41e8c1eaae346`。
- 冻结副本：`.codex-tmp/stability-20261029/candidate/`，逐文件核对相同。
- 此包来自整个当前工作区，包含前序钱包UI，不能称为“仅稳定性提交构建”，也不是上传回执。
- 冻结副本首次复制因遗漏 `recursive:true` 未成功；随后单独验证所有源码及产物散列未改变后复制成功，未重新覆盖用户源码。

## 仅稳定性修复的独立候选

为消除混合工作区歧义，另用 `git archive 3d83eaa` 解档并仅覆盖以上21个文件，在副本内重新生成主题。未建立新Git工作树、未移动/覆盖原有钱包/后台工作；仅复用已安装依赖。

- 根目录：`.codex-tmp/stability-only-20261029/`。
- 暗色生成结果：31页、119条颜色声明。相对原HEAD仅注释中的30→31变化，未混入工作区的114条钱包色彩变更。
- 候选 `typecheck`、`build:miniapp`、包体、暗色主题、外部目录、封面、底栏真实hook、刷新、角色、私信、配送工作区检查均 `exit 0`；主代理复核包体/主题/路由业务检查亦通过。
- 包体：140文件，2,042,445 bytes，剩54,707 bytes。
- 包聚合SHA-256：`DD6594F31275A106F9DCBE6D8B235F356646113D17B24F8C9E4A3623E5D0A89A`。
- 生成样式SHA-256：`841CCC39B56A6BE21E07A6C56C00A988595797CE7D5C244C5F9D97DE4B28F16C`。
- 清单：`.codex-tmp/stability-only-20261029-package-manifest.json`；构建、类型和检查日志为同前缀的 `.log` 文件。
- 这个候选仍包含已提交基线中的钱包，不包含其后未提交的新提现/充值恢复UI。它是独立稳定性候选，不是全部功能整包。
- 上述模拟器逐页观察针对当前完整工作区产物；不能声称独立候选也已逐页或真机验证。

## 全仓回归不是全部通过

`npm run verify` 返回1，停在 `preverify` 的旧路由断言。为查明其它问题，另行执行后续命令，未把分段执行冒充完整通过。

| 检查 | 结果与范围 |
| --- | --- |
| `check-reference-layout.cjs` | FAIL：固定30页，当前31页；新增骑手钱包路由来自既有 `7f098be`。骑手钱包SCSS也缺少该检查要求的语义mixin接入，需要钱包布局任务复审，而非只改计数。 |
| `check-boundaries.cjs` | FAIL：既有 `backend/internal/module/delivery/rider_wallet_query.go` 包含数据库访问，但文件名不符合repository约束。本轮不改后端或放宽检查。 |
| `check-admin-ui-system.cjs` | FAIL：前序未提交 `DeliveryConsole.tsx` 使用 `window.prompt`。本轮不代改后台资金流程。 |
| 其余preverify命令 | 分段执行PASS，包括封面、手势、内容展示、弹层、刷新、搜索与媒体策略。 |
| 其余verify命令 | 分段执行PASS，包括角色、身份申请、私信、布局、工具、部署契约和凭据卫生。 |
| `npm run typecheck` | 小程序与后台均PASS。 |
| `npm run test:backend` | 安全脚本清空MySQL测试DSN后PASS；不代表数据库集成迁移已验收，不触碰共享数据库。 |

## 微信开发者工具实际观察

使用现有官方开发者工具打开 `apps/miniapp` 的正常 `dist`，AppID保持 `wx065d8f3ce4a056ee`，API保持 `https://zhanduiwang.top/api/v1`，不注入模拟数据或绕过身份守卫。

| 页面/动作 | 直接观察 |
| --- | --- |
| 首页 | 发现/二手/工具、真实空态及圆角底栏可见。 |
| 学生“我的” | 真实头像/封面加载后，底部淡化衔接可见；原“我的内容”保留。 |
| 骑手工作台/资料 | 接单大厅、今日统计占位、资料页与收入入口可见；统计请求404。 |
| 骑手钱包 | 蓝色余额卡与资金区块可见，接口404后显示错误和重试，不显示假金额、不白屏。 |
| 商家工作台/资料 | 订单管理与底栏、资料页及钱包入口可见。 |
| 商家钱包 | 支付能力读取404时显示“暂不可用”和“重试检查”，未开启充值扣费。 |
| 校园服务 | 只显示合作商家目录，真实空态“暂无可用合作商家”，没有内置商品目录。 |
| 返回首页 | 可返回首页，身份最终恢复学生；未创建订单或资金申请。 |

本次观察到的错误是HTTP404，未观察到此前的模块 `undefined.call` 崩溃；不能据此断定手机白屏根因全部解决。构建期间Taro重新生成dist，IDE短暂提示app.json不存在，构建完成后重新加载恢复，这不属于上传包缺文件。

截图仅保存在本地忽略目录，包含用户资料，不提交Git：`profile-fade-runtime.png`、`rider-wallet-runtime.png`、`merchant-wallet-runtime.png`、`external-directory-runtime.png`。

## 明确未完成及下一步边界

1. **线上接口未打通**：模拟器请求 `/delivery/capabilities`、`/delivery/rider/stats/today` 和骑手钱包相关接口返回404。本地后端源码虽包含相应路由，但当前线上部署/反向代理/开关版本需单独核实。普通HTTP健康/版本查询在本环境超时，未获取线上版本，不声称服务器健康通过。
2. **全仓三个失败项**需要在各自任务范围修复；不删断言，不放宽业务边界。
3. **真实资金**：没有验证微信充值/退款/提现到账，没有执行打款，不把内部待处理状态当成到账。
4. **设备验收**：本次是Windows模拟器，不是iOS/Android手机；背景、玻璃降级、长列表连续滑动的真机验收仍需后续体验包。
5. **外部发布**：本轮未上传微信、未覆盖体验版、未提交审核、未正式发布、未push GitHub。旧 `wechat-upload-2026.10.26.1.json` 不对应当前产物，不能复用为成功证明。
6. **后续上传前**：选定包含哪些已验收功能的源码树，再生成主题、构建、核对包体/源散列；确认微信登录，使用官方CLI上传，并分别记录上传回执、体验版设置与真机结果。

## 对先前状态说明的更正

此前“回归全部通过、模拟器0红色错误”只代表当时定向脚本与首页观察，不能推广到全仓或钱包实际请求。本轮已补足上面的反例与明确缺口。本地准备完成不等于钱包业务、体验版或生产版本已完成。
