# 私信模块体验优化实施计划

## 目标

在隔离分支内优化首页私信会话列表和二手交易单聊页的状态层级、返回顺序、时间分组、输入与按压反馈；保持现有接口、权限、路由和消息能力不变。

## 文件范围

- `apps/miniapp/src/pages/home/index.tsx`：会话列表真实字段映射、错误/空态、触控反馈和返回刷新。
- `apps/miniapp/src/pages/home/index.scss`：会话行、状态块和减少动效样式。
- `apps/miniapp/src/pages/secondhand-chat/index.tsx`：消息日期分组、输入/返回处理、发送失败反馈。
- `apps/miniapp/src/pages/secondhand-chat/index.scss`：消息分组、单聊状态、输入区和微动效。
- `docs/superpowers/specs/2026-10-16-private-messaging-polish-design.md`：设计边界和自审记录。

## 实施步骤

1. 先补充不依赖后端扩展字段的显示模型和纯函数，确保消息 ID、时间、路由参数不变。
2. 更新会话列表的结构与样式，明确加载、空态、错误重试和真实商品归属。
3. 更新单聊的消息日期分组、发送状态、返回时键盘收起和输入区反馈；不新增媒体能力。
4. 增加/更新私信专项静态检查，覆盖真实字段、稳定消息键、失败保留草稿、会话账号隔离和减少动效。
5. 执行 TypeScript、专项检查、小程序构建、`git diff --check`，保留未跟踪 `artifacts/` 不动。

## 验证命令

```text
npm --workspace apps/miniapp run typecheck
node scripts/check-miniapp-back-navigation.cjs
npm run build:miniapp
git diff --check
```

真机验收仍需人工确认：键盘收起后返回、不同机型安全区、实时消息到达、真实会话权限和滚动位置。
