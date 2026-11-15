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
