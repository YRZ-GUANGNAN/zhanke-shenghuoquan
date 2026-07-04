# Miniapp Claude Design Rebuild Spec

## Goal

按用户选择的 B 方案，先对 `apps/miniapp` 做全量前端重构：以交接包中的“湛科生活圈”小程序为准，复用 Claude Design 的温暖、克制、层级化表达，但使用项目交接文档定义的清爽校园蓝设计系统，不改后端接口和路由边界。

## Scope

- 学生端：`home`、`food`、`market`、`publish`、`profile`、`post-detail`、`product-detail`、`my-content`。
- 配送端：`delivery-rider`、`delivery-merchant`、`delivery-wallet`、`delivery-conversation`。
- 消息辅助页：`conversations`、`chat`、`notifications`、`media-history`。
- 全局：`app.scss` 与 `components.tsx` 的视觉 token、基础组件、底部导航、头像、空状态、图片占位。

## Non-goals

- 不修改后端/API/服务层协议。
- 不新增不存在的骑手地图、距离、在线状态、评分、服务类型等能力。
- 不暴露“腾讯云IM”“后端”“CAS”“Media 链路”等技术词。
- 后台 `apps/admin` 作为第二阶段，不在本轮阻塞小程序交付。

## Visual System

- 主色：`#2E7DD4`，深蓝 `#2168B8`，浅蓝底 `#E6F1FB`。
- 功能色：橙 `#FF7A45`，绿 `#1D9E75`，粉 `#FF6B8A`。
- 背景：`#F7F8FA` / `#F2F5F9`，白色卡片。
- 字体：`PingFang SC` 优先，标题和数字使用 500-600，避免超粗字重。
- 组件：14-18px 圆角、轻阴影、分段 tab、浅底 badge、底部五栏导航，中间发布按钮突出。

## UX Requirements

- 首页首屏必须体现“湛科生活圈 / 湛江科技学院”，并呈现关注/发现/工具、搜索、工具入口、小红书式内容流。
- 外卖页只做合作商家聚合和跳转表达，不表现为平台内自营点单。
- 二手页强调学生私下交易和平台展示，商品详情保留联系卖家。
- 个人页保留身份切换，并清楚进入骑手/商家工作台。
- 骑手抢单大厅展示钱包、今日收入、订单状态 tab；抢单前隐藏完整地址/电话，备注可见。
- 商家工作台展示余额、今日单量、配送中、快捷入口、订单 tab；发布配送单不让商家手填配送费。
- 钱包与会话页使用用户可理解的中文，不出现底层技术描述。

## Verification

- `npm --workspace apps/miniapp run typecheck`
- `npm --workspace apps/miniapp run build:weapp`
- 扫描 `apps/miniapp/src` 与 `apps/miniapp/dist` 是否仍有明显乱码片段。
