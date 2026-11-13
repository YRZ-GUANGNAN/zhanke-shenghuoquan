# 2026-11-13 管理后台与用户绑定生产发布记录

本记录只描述本次实际部署与验证结果，不代表真机或真实交易验收。

## 发布身份

| 项 | 值 |
| --- | --- |
| 运行时版本 | `2026.11.13.candidate.1` |
| 运行时 commit | `285e62884626e821548098fe50b00cb83a977ad9` |
| 后端 release | `/opt/campus-circle/releases/20261113-admin-ops-285e628` |
| 后台静态 release | `/var/www/campus-admin/releases/admin-20261113-285e628` |
| 后端二进制 SHA256 | `2347da609d48f25ee70bc0015e2c31fe64b74330ad49589901fb22e667ecaff2` |
| 上一后端 release | `20261112-rider-wallet-1b919e5` |
| 上一后台 release | `admin-20261113-services-3ad3b3a` |

## 本次内容

- 后台多管理员：管理员与超级管理员均可登录，超级管理员可分配、重置、移除登录账号。
- 消息通知：新增发送记录列表与后台投递任务运行状态。
- 意见反馈：支持标记处理状态并填写处理说明。
- 手机号绑定：冲突提示、转移绑定、解绑，后台可查看掩码号码并解绑。
- 小程序新增「反馈与举报记录」页；登录页改版。
- 清理旧二手（`market`、`product-detail`、`publish`）与旧私信（`chat`、`conversations`）五个未注册页面。

## 数据库迁移

部署前已执行：

- `057_notification_official_account.sql`
- `058_admin_credential_events.sql`
- `059_user_feedback_handling.sql`

迁移前备份：`/var/backups/campus-circle/pre-migration-20261113-211025.sql.gz`（61K）。

同一窗口内完成了生产数据库密码轮换，`campus-api` 重启后通过验证；旧密码已失效。

## 验证结果

- `https://zhanduiwang.top/healthz` 返回 `{"code":0,"message":"ok"}`。
- `/version` 返回 `2026.11.13.candidate.1` 与上述 commit。
- 新增接口 `GET /api/v1/auth/feedbacks`、`GET /api/v1/reports/mine` 返回 `401`（已注册、需登录），不再是 `404`。
- 管理后台首页、`/assets/index-CtBK5UhO.js`、`/assets/index-B42LJ-HB.css` 均返回 `200`，与本次构建产物一致。
- `campus-api` 服务状态 `active`。

## 回滚方式

将两个 `current` 软链接原子切回上一 release 并重启 `campus-api`。`057`–`059` 均为增量迁移，回退二进制不需要撤销表结构；处理类字段与审计记录必须保留。

## 仍未验收

- 未登录真实管理员账号在线上验证多管理员与反馈处理界面。
- 服务号绑定仍缺已认证服务号与 `WECHAT_OFFICIAL_*` 配置，绑定链路未启用。
- 微信订阅/服务号真实送达、真机页面与支付退款仍未验收。
