# 2026-11-15 管理后台整体切换为原型界面与派单能力发布记录

本记录只描述本次实际部署与验证结果，不代表真机或真实交易验收。

## 发布身份

| 项 | 值 |
| --- | --- |
| 运行时版本 | `2026.11.15.candidate.1` |
| 运行时 commit | `93b8f4f9ad1e7f22b7adb368ba5bf9902dee675a` |
| 后端 release | `/opt/campus-circle/releases/20261115-console-93b8f4f` |
| 后台静态 release | `/var/www/campus-admin/releases/admin-20261115b-605b879`（首版 admin-20261115-93b8f4f） |
| 后端二进制 SHA256 | `19f89bbefb87d928d1fc54a1b4aa49e5ace243a3b549a94a36fba1851afdc522` |
| 上一后端 release | `20261114-admin-ops-b71fc54` |
| 上一后台 release | `admin-20261114-b71fc54` |

## 本次内容

- 管理后台整体切换为原型界面：登录守卫、账号面板、按功能开关过滤的导航、真实待办徽章。
- 七个视图组全部接入真实接口：经营总览、配送与结算（含改派/取消/骑手派单）、审核与内容、用户与权限（含认证改判与后台登录凭据）、消息通知（渠道/发送记录/死信重投）、设置（导航/功能开关/分享设置）、旧版外卖总览。
- 管理端新增接口：订单改派/取消（CAS + 审计）、骑手列表/在途订单/状态管理、举报详情、认证终态改判、审核通过依据、通知人工重投。
- 后端不具备的能力（消息模板、Webhook 渠道管理、素材审核写操作）在界面中按真实能力降级或移除。

## 数据库迁移

- 执行 `060_delivery_admin_actions.sql`（生产库）：
  - `delivery_orders` 新增 `cancel_reason` 列；
  - 新建 `delivery_order_admin_actions` 审计表。
- 执行前已将 `delivery_orders` 表结构备份到服务器 `/tmp/delivery_orders_before_060.sql`。
- 回滚脚本：`backend/migrations/rollback/060_delivery_admin_actions.sql`。

## 验证结果

- `https://zhanduiwang.top/healthz` 返回 `{"code":0,"message":"ok"}`。
- `/version` 返回 `2026.11.15.candidate.1` 与上述 commit。
- 管理后台首页与 `assets/index-BsN2u2iO.js`、`assets/index-D8njK-3z.css` 均返回 `200`（首版）。
- 修复 antd `visible` 弃用警告后重新构建并切换静态 release `admin-20261115b-605b879`，首页引用 `assets/index-CrPb4vFk.js`，返回 `200`；后端无变化。
- `GET /api/v1/admin/delivery/riders` 未登录返回 `401`，确认新接口已随后端上线。
- `campus-api` 重启后状态 `active`。
- 本机网络访问 `zhandui.top` SSL 连接失败，未能从本机直接验收后台页面；以上后台静态验证是在服务器本机带 Host 头完成的。登录后的人工验收仍需在浏览器完成。

## 回滚方式

后端与后台静态分别切回上一 release 软链接并重启 `campus-api`。数据库迁移 060 为纯新增（列默认空串、新表），回滚代码时无需撤销；如需完全回退可执行 060 的回滚脚本。

## 仍未验收

- 未用真实管理员账号在浏览器验证登录后的新界面与各视图数据。
- 未在真机验证小程序端行为（本次未改动小程序与主要业务逻辑）。

## 补充发布：2026.11.15.candidate.2（同日）

| 项 | 值 |
| --- | --- |
| 运行时版本 | `2026.11.15.candidate.2` |
| 运行时 commit | `21b1993f7d9a0a3de3b0774fd1e59f62204768b3` |
| 后端 release | `/opt/campus-circle/releases/20261115b-console-21b1993` |
| 后台静态 release | `/var/www/campus-admin/releases/admin-20261115c-21b1993` |
| 后端二进制 SHA256 | `3425a3dd3cac13cc39401b1db188ee0ae50e491c48c8cfb9240dcef80f3bc531` |

本次内容：
- 列表整行可点击打开详情（订单/社区/商品/举报/认证/反馈/骑手/资金各表），行内操作列阻止冒泡。
- 全站图片接入真实媒体地址：帖子/商品/认证证明/反馈附图使用业务数据里的真实链接；新增管理端素材预览网关 `GET /api/v1/admin/media/assets/:assetId/content`（302 到 COS 签名地址，仅限已定稿资产），素材中心卡片与详情显示真实图片，未定稿标注「未定稿」。
- 移除渐变假图；图片加载失败显示明确占位。

数据库：无新增迁移。验证：`/version` 返回上述版本与 commit；后台首页与 `assets/index-CUtcpiJW.js`、`assets/index-CUTAtt3m.css` 返回 `200`；管理端素材预览接口未登录返回 `401`；生产 42 个媒体资产中 38 个已定稿可在后台预览，4 个未定稿显示「未定稿」。

## 补充发布：素材网格紧凑布局（同日，静态）

- 后台静态 release 更新为 `/var/www/campus-admin/releases/admin-20261115d-9ce745e`，commit `9ce745e`。
- 素材中心网格改为自适应多列（大屏 5–8 列、1100px 以下两列），图片预览区高度 96px，一屏可核对更多素材。
- 后端无变化；首页引用 `assets/index-CeoTGRLQ.js`、`assets/index-sYRXa0GN.css`，返回 `200`。

## 补充发布：2026.11.15.candidate.3（同日，分享封面拖拽上传）

| 项 | 值 |
| --- | --- |
| 运行时版本 | `2026.11.15.candidate.3` |
| 运行时 commit | `6b18a0c0b5fcb4ec431610797abb6e3be992411a` |
| 后端 release | `/opt/campus-circle/releases/20261115c-console-6b18a0c` |
| 后台静态 release | `/var/www/campus-admin/releases/admin-20261115e-6b18a0c` |
| 后端二进制 SHA256 | `bf2d69ca5baf739b9eb5b612c7481deb7ef2a0a027be3b50b891b337cf31e6f0` |

本次内容：
- 分享封面支持拖拽上传：`POST /api/v1/admin/share-settings/cover`（multipart，JPG/PNG ≤5MB、尺寸 ≤4096，内容哈希命名），公开读取 `GET /api/v1/share-covers/:name`（长缓存不可变）。
- 封面文件存放在独立持久目录 `/opt/campus-circle/shared/share-covers`（campus 可写），不随 release 切换丢失；`SHARE_COVER_DIR` 已写入 `/etc/campus-circle/campus-api.env`（原文件备份为 `campus-api.env.bak-20261115`）。
- 分享设置页新增拖拽上传区，上传成功自动回填封面地址，保存设置后生效；手工粘贴地址的方式保留。
- 数据库：无新增迁移。

验证：`/version` 返回上述版本；后台首页引用 `assets/index-DgfpbYQD.js`；上传接口未登录返回 `401`；封面对外读取路由存在（不存在文件返回 `404`）。真实上传与保存效果需登录后台人工验收。

### candidate.3 部署修正（同日，systemd）

首次上线后线上拖拽上传返回「保存封面图失败」。原因：`campus-api.service` 启用了 `ProtectSystem=strict`，整个文件系统对服务只读，`/opt/campus-circle/shared` 未在白名单内导致写盘被拒。修复：新增 drop-in `/etc/systemd/system/campus-api.service.d/readwrite-shared.conf`（`ReadWritePaths=/opt/campus-circle/shared`），`daemon-reload` 并重启服务；`/proc/<pid>/mountinfo` 确认该目录在服务沙箱内为可写挂载。

## 补充发布：品牌 Logo 更换（同日，静态）

- 后台静态 release 更新为 `/var/www/campus-admin/releases/admin-20261115f-d24affc`，commit `d24affc`。
- 「湛」字标记（侧栏、移动端抽屉、登录页、会话校验、分享预览品牌位）替换为「湛科万事通」圆形 logo（用户提供原图处理为 256px 圆形透明 PNG，`/images/brand-logo.png`）。
- 验证：首页引用 `assets/index-CS-xp9dh.js`、`assets/index-CGpf7_sM.css`，logo 图片返回 `200 image/png`。

## 补充发布：2026.11.15.candidate.4（同日，值班群渠道后台填写）

| 项 | 值 |
| --- | --- |
| 运行时版本 | `2026.11.15.candidate.4` |
| 运行时 commit | `06065b42c7fee53b9a00a64351efdc218db41fb8` |
| 后端 release | `/opt/campus-circle/releases/20261115d-console-06065b4` |
| 后台静态 release | `/var/www/campus-admin/releases/admin-20261115g-06065b4` |
| 后端二进制 SHA256 | `5ad1ccdf1836c728590882c908c936f33a80c73c0b6be6a3fe65797dedafe492` |

本次内容：
- 新增迁移 `061_notification_channel_settings.sql`：单行表保存商家/骑手值班群 Webhook；`updated_by` 为空表示从未保存，继续以环境变量为准。
- 新增 `PUT /api/v1/admin/delivery/notification-channels/wecom`：按字段更新（未填写的群保持不变）、行版本 CAS、企微 Webhook 严格校验、记录操作人；保存后立即作用于用户通知与配送通知两个发送器，无需重启。
- 企微群通道改为始终初始化：环境变量留空时也能通过后台启用；消息格式化动态读取当前配置。
- 消息通知页新增「渠道配置」表单，已配置群显示密钥尾号；overview 返回脱敏提示与来源。

数据库：迁移 061 已在生产执行（1 行、row_version=1、updated_by 为空，即 env 兜底）。验证：`/version` 返回上述版本；首页引用 `assets/index-Ub77jhDY.js`；渠道配置接口未登录返回 `401`。真实保存与群消息送达需登录后台并配置真实机器人后人工验收。

## 补充发布：2026.11.15.candidate.5（同日，订单通知模板卡片）

| 项 | 值 |
| --- | --- |
| 运行时版本 | `2026.11.15.candidate.5` |
| 运行时 commit | `02b7f5b831a3d73130f11fedab64bb942aa75167` |
| 后端 release | `/opt/campus-circle/releases/20261115e-console-02b7f5b` |
| 后端二进制 SHA256 | `081d63995c7277e8fad483d561ab69323cf980b0627f17cbdee78be9753d3f80` |

本次内容：
- 订单通知从 markdown 文本改为企业微信模板卡片（template_card / text_notice）：来源标注平台（京东订单/淘宝订单/美团订单/私人独立订单）、大号金额（商家群显示配送费、骑手群显示赏金）、单号/状态/时间条目、底部点击跳转。
- 订单事件负载新增来源平台与骑手报酬字段；**保持不含地址、电话、备注**的隐私最小化边界（delivery 包测试继续强制）。
- 新增卡片结构单元测试；`npm run verify` 25 项回归通过。
- 端到端验证：模拟 4 条订单（京东/美团/淘宝/私人）触发 8 条企微卡片投递，全部 accepted；卡片快照含平台标注与金额。

数据库：无新增迁移。说明：群机器人 webhook 仅支持 text_notice 卡片，底部按钮为固定「详情」样式（自定义「接单」按钮需企业微信应用回调，不在群机器人能力内）；跳转地址暂为站点首页，待小程序接单页 scheme 就绪后替换。
