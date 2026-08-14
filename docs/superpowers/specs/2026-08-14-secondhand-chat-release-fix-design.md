# 二手私信发布阻塞修复设计

> 日期：2026-08-14
> 决策授权：用户已明确将方案和实现完全交由 Codex，并只做最终审查。

## 问题

当前二手私信页有两个会影响发布质量的问题：

1. 输入栏展示图片入口，但点击后仅提示“图片消息暂未开放”，这是可见但不可用的操作；
2. 消息列表底部仅预留 `150rpx`，不足以覆盖输入栏、内边距和 iPhone `safe-area-inset-bottom` 的总高度，最后一条消息可能被固定输入栏遮挡。

## 方案比较

### 方案 A：立即打通二手私信图片消息

需要新增独立的 `secondhand_im` 媒体用途、内容安全审核、会话参与者鉴权、临时图片令牌、解析接口、腾讯云 IM 自定义消息协议及双端真机验收。功能完整，但涉及后端、迁移、存储与生产配置，超出本次发布候选的最小安全修复范围。

### 方案 B：隐藏未开放图片入口，并修复底部安全区（采用）

二手私信在当前版本明确只提供文字消息，不展示死按钮；输入栏改为“文本输入 + 发送”的两列布局。消息区底部使用固定输入栏基础高度加 `env(safe-area-inset-bottom)` 的动态留白，键盘弹起时保持现有手动定位逻辑。该方案不改变后端协议、不制造假能力，且能立即消除两个发布阻塞。

### 方案 C：保留图片按钮但置灰

可以减少误触，但仍向用户展示不可完成的能力，也会继续挤占小屏输入空间，因此不采用。

## 组件与行为

- `SecondhandChatPage` 继续复用真实腾讯云 IM 文本消息链路；删除图片入口 JSX 和对应装饰样式。
- `.secondhand-chat-inputbar` 改为两列网格，保留输入框与发送按钮的现有状态、错误提示和键盘监听。
- `.secondhand-chat-scroll__inner` 的底部留白改为 `calc(184rpx + env(safe-area-inset-bottom))`；键盘出现后输入栏本身不再叠加安全区，现有 `bottom: keyboardHeight` 保持不变。
- 不增加任何本地消息、模拟图片、占位上传或绕过审核的行为。

## 错误与兼容边界

- 腾讯云 IM 登录、历史读取和文本发送错误保持现有可重试提示。
- iOS Home Indicator 由 CSS 安全区处理；Android 无安全区时使用 `184rpx` 基础留白。
- 图片消息列为后续独立能力，不在当前页面暴露入口，也不在本次发布报告中宣称支持。

## 验证

1. 扩充 `scripts/check-miniapp-no-fake-pages.cjs`：禁止二手私信再次出现“图片消息暂未开放”或旧媒体入口类名。
2. 扩充 `scripts/check-secondhand-messaging-ui.cjs`：要求两列输入栏和带安全区的消息底部留白。
3. 运行：

```powershell
npm run check:miniapp-no-fake-pages
npm run check:secondhand-messaging-ui
npm --workspace apps/miniapp run typecheck
npm run build:miniapp
git diff --check
```

4. 最终真机审查仍需在一台带 Home Indicator 的 iPhone 和一台 Android 上分别确认最后一条消息、键盘升降、发送按钮及退出重进历史。
