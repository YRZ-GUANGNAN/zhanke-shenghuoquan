# 2026-11-14 管理后台与配送分组生产发布记录

本记录只描述本次实际部署与验证结果，不代表真机或真实交易验收。

## 发布身份

| 项 | 值 |
| --- | --- |
| 运行时版本 | `2026.11.14.candidate.1` |
| 运行时 commit | `b71fc5438422161165be514d3d09b3d4d7e9d2ac` |
| 后端 release | `/opt/campus-circle/releases/20261114-admin-ops-b71fc54` |
| 后台静态 release | `/var/www/campus-admin/releases/admin-20261114-b71fc54` |
| 后端二进制 SHA256 | `29564e30d32f17675853af6b6f6af37f33464ebc07c97698573a532c4cb92760` |
| 上一后端 release | `20261113-admin-ops-285e628` |
| 上一后台 release | `admin-20261113-285e628` |

## 本次内容

- 配送与结算：11 个平铺标签收敛为「订单与概览 / 派单管理 / 资金与结算 / 费率配置」四组，组内二级切换。
- 经营总览：待办入口接入真实队列数量（审核、认证、反馈）。
- 意见反馈：后台列表支持按状态筛选，接口新增 `status` 参数。
- 手机号绑定：补充幂等写入与已注销账号占用号码的处理（不改变现有接口形态）。
- 新增本地原型目录 `apps/admin/src/preview`，**不参与生产构建**。

## 数据库迁移

本次无新增迁移；生产库已包含 `057`–`059`。

## 验证结果

- `https://zhanduiwang.top/healthz` 返回 `{"code":0,"message":"ok"}`。
- `/version` 返回 `2026.11.14.candidate.1` 与上述 commit。
- 管理后台首页与 `assets/index-D05CaaSS.js`、`assets/index-TdMxsDf_.css` 均返回 `200`。
- `GET /api/v1/admin/feedbacks?status=open` 返回 `401`（已注册、需管理员会话），确认筛选参数已上线。
- `campus-api` 重启后状态 `active`。

## 回滚方式

把 `/opt/campus-circle/current` 与 `/var/www/campus-admin/current` 两个软链接切回上一 release 并重启 `campus-api`。本次无数据库变更，回滚不涉及数据。

## 仍未验收

- 未用真实管理员账号登录线上验证配送分组与待办计数。
- 原型目录仅供设计评审，未接后端接口，不属于本次发布范围。
