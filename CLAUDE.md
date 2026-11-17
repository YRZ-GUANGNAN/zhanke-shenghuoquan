# 项目约定

## 浏览器支持策略

**Browser Support:** Default to Baseline Widely available features without fallbacks. Allow Newly Available features only when feature-detected with graceful degradation, and only adopt custom fallback code that adds <= 20 lines and no external dependencies.

- 管理后台（apps/admin）面向现代 Chromium/Edge/Firefox/Safari；新特性须按上述策略渐进增强。
- 参考 [GoogleChrome/modern-web-guidance](https://github.com/GoogleChrome/modern-web-guidance)：涉及 HTML/CSS/客户端 JS 的改动前先 `npx -y modern-web-guidance@latest search "<query>"` 检索最佳实践。
