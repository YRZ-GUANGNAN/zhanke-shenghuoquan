# 2026-11-18 订单通知与企微卡片修复部署记录

本次发布修复两侧问题：改派订单通知全链路（此前从未发出），以及企微群卡片发送闸门引用不存在的列导致**所有订单群卡片静默丢失**（candidate.9 起）。

## 本次内容

- **改派通知打通**：`DeliveryOrderReassigned` 事件（状态不变、只换骑手）此前在收件人解析、企微 formatter、微信短语三处都被「事件必然伴随状态变化」的假设拒绝，通知从未发出。现在：改派 payload 携带起送地址并要求骑手真实变更（`appendReassignNotificationTx`）；解析器接受改派事件（From==To + 换骑手校验），改派卡以「订单已改派」为标题；过期判定为「又被改派 / 订单已取消或送达 → 跳过，订单仅推进状态 → 照常通知」；微信订阅短语补「已改派」。
- **企微群卡片闸门修复**：闸门 SQL 引用不存在的 `delivery_orders.deleted_at` 列，MySQL 1054 错误被 `return nil, nil` 吞掉，candidate.9（11-15）起所有订单群卡片实际未发送。改为 `status / row_version / IFNULL(rider_user_id,0)` 查询（已在生产库只读验证）；查询失败显式报错进入重试/死信（后台可见），不再静默；两个值班群都未启用时短路，避免连累站内信/订阅通道。
- **卡片层级**（京东/美团式）：标题=状态，副标题=平台+短单号，条目=完整单号/起点/终点/时间，金额用 emphasis 突出。
- **测试**：改派 payload（含电话/备注不入事件的隐私断言）、解析器、formatter、微信短语单测；新增 `user_notification_wecom_mysql_test.go`（TEST_MYSQL_DSN 门控，覆盖闸门真实 SQL 的发送/跳过/报错/双群关闭路径）。

## 部署记录（2026-11-18）

- 代码：commit `876ef05a51f1` 已推送 origin/main。无数据库迁移。
- **后端**：版本 `2026.11.18.candidate.5`，release `/opt/campus-circle/releases/20261118e-wecom-reassign-876ef05`，二进制 SHA256 `12c883e8f288ec3722f57ebfeb2b21b190163ed3f9177dee06f1986383e2aba4`；软链原子切换（`ln -sfn` + `mv -T`）后 `campus-api` 重启 active。
- **线上验证**：`/healthz` ok；`/version` 返回 candidate.5 / commit `876ef05a51f1`。
- **群卡片验证**：商家群 2 条（骑手已接单、订单已改派）、骑手群 2 条（订单已改派、配送中），全部 `errcode=0`；卡片为按新结构的模拟负载（appid+pagepath 跳小程序，桌面端回退站点 URL）。
- **回滚目标（上一版）**：后端 `20261118d-update-log-f38568f`（candidate.4）。

## 已知事项

- candidate.9 至 candidate.4 期间产生的订单，商家群/骑手群**没有收到过任何卡片**（闸门 1054 被静默吞掉）；期间的群内缺卡属此原因，不是漏配置。
- 模拟测试卡片不经过后端事件链路；真实发卡（含改派）需下次真实订单/改派时验证。
