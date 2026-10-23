# 私信头像与聊天页实施计划

**Goal:** 双向真实头像贯通列表和聊天，改善文本聊天的排版与输入体验。

**Architecture:** IM HTTP 层在原授权结果上投影公开对方资料；小程序共享头像与参与者解析，保持原 Tencent IM 收发链路。网页预览仅替换服务依赖，渲染真实页面组件。

**Tech Stack:** Go/Gin、React/Taro、TypeScript、SCSS；本地隔离浏览器验证。

## 执行与检查

- [x] `backend/internal/module/im/conversation_view.go`：批量取得授权对方资料，投影 `id/nickname/avatar`；`handler.go` 包装 list/detail 响应；`handler_test.go` 增加双方、越权、缺失、失败测试。
- [x] `apps/miniapp/src/services/c-trade.ts`：附加可选 peer；`utils/conversation-presentation.ts`：校验当前参与者后取对方；`components/chat-avatar.tsx/.scss`：真实图片、首字及加载失败回退。
- [x] `pages/home/index.tsx/.scss`：列表改为共享头像与昵称。`pages/secondhand-chat/index.tsx/.scss`：标题与两边气泡头像、商品条、flex 输入布局、同步发送锁与请求生命周期检查。
- [x] `scripts/test-private-messaging.cjs`：执行真实页面处理函数，验证重复提交、失败保留草稿、卸载/账号切换隔离；加入现有 `check:private-messaging` 验证入口。
- [x] 在独立可视化目录扩展真实聊天预览，验证双方头像/缺图、三种屏宽、键盘、空态/错误及列表到聊天导航，保存截图。
- [x] 执行 `npm run test:backend`、miniapp typecheck、私信/回归/胶囊/键盘/边界检查及微信构建。审查 diff，仅保存本次文件，不触及原项目或已有 artifacts，不上传/部署。边界检查的既有失败见下。

实际发送接口不在浏览器预览调用，双设备真机与线上新字段验证保持待发布后的验收项。

## 2026-10-17 验证记录

- 隔离后端测试通过。新用例验证双方 peer 仅投影三个公开字段、列表一次批量取资料、缺失/异常回退、越权不触发资料查询。
- 小程序类型、私信行为、回归契约、胶囊布局、键盘补偿、返回和 30 条静态路由检查通过。
- 微信构建通过，仍有既有 `vendors.js 731 KiB` 体积建议。
- `check:boundaries` 未通过：`backend/internal/module/delivery/external_intake.go` 在仓储层外使用数据库。确认该文件与检查脚本都和基线 `d5f883f` 一致，本轮没有改配送文件或放宽检查。
- 本地网页渲染实际 TSX/SCSS，13 个聊天画面/交互场景通过；另外检查买卖双方会话列表和点击进入聊天。实屏发现并修复窄屏主页入口换行及快速图片失败后首字消失问题。
- 预览 `http://127.0.0.1:5197/?view=chat`。可视化目录 `market-polish-20261017` 下保留 `chat-verification.json`、`chat-390.png`、`chat-seller-390.png`、`chat-keyboard-390.png`、`private-messages-390.png` 等证据。
- 测试资料的头像字段为空，展示真实无图回退；不生成图片、不将商品图片当头像。图片 URL 解析、真实图片分支与失败回退由组件行为测试验证。线上图片域名与双设备收发尚未验收。
- 上线需要同时更新后端与小程序；无需数据库迁移。当前后端字段向后兼容，本轮只在隔离分支保存，没有部署、上传或送审。
