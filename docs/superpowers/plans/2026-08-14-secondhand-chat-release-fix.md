# 二手私信发布阻塞修复实施计划

> 日期：2026-08-14
> 设计依据：`docs/superpowers/specs/2026-08-14-secondhand-chat-release-fix-design.md`

## 目标

在不改变真实腾讯云 IM 文字消息协议、不新增假功能的前提下，移除二手私信中不可用的图片入口，修复 iPhone Home Indicator 场景下最后一条消息被固定输入栏遮挡的问题，并把这两个要求固化为自动化契约。

## 任务 1：先扩充失败契约

**修改文件：**

- `scripts/check-miniapp-no-fake-pages.cjs`
- `scripts/check-secondhand-messaging-ui.cjs`

**要求：**

- 禁止 `secondhand-chat` 再出现“图片消息暂未开放”文案或旧媒体入口类名；
- 要求输入栏使用文字输入与发送按钮的两列布局；
- 要求消息列表底部包含 `env(safe-area-inset-bottom)`，且基础预留不低于固定输入栏实际高度。

先运行契约，确认它能捕获当前缺陷，再进入实现。

## 任务 2：收口二手私信界面

**修改文件：**

- `apps/miniapp/src/pages/secondhand-chat/index.tsx`
- `apps/miniapp/src/pages/secondhand-chat/index.scss`

**要求：**

- 删除不可用的图片入口 JSX 和对应样式；
- 输入栏改成 `1fr 92rpx` 两列；
- 消息区底部预留改成 `calc(184rpx + env(safe-area-inset-bottom))`；
- 保留现有文字 IM 登录、历史消息、发送、重试和键盘定位逻辑。

## 任务 3：加固后端安全测试入口

**修改文件：**

- `scripts/test-backend-safe.cjs`

**要求：**

- 继续清空所有 MySQL DSN；
- 同时清空测试中可能触发真实 COS、腾讯云 IM、微信或端到端外部动作的开关和凭据；
- 使用 `go test -count=1 ./...`，避免 Go 测试缓存制造假通过；
- 不打印任何环境变量值。

## 任务 4：分层复验

```powershell
npm run check:miniapp-no-fake-pages
npm run check:secondhand-messaging-ui
npm --workspace apps/miniapp run typecheck
npm run test:backend
npm run verify
npm run build:miniapp
npm run build:admin
rg -n -i "127\.0\.0\.1|localhost|dev-api|mock-api" apps/miniapp/dist apps/admin/dist
git diff --check
```

## 任务 5：交付边界

- 自动验证和生产构建必须全部通过；
- 不上传微信版本、不部署、不写共享数据库；
- iOS/Android 真机的最后一条消息、键盘升降、退出重进历史仍列为最终人工验收项。
