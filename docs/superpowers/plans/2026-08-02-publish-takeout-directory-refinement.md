# 发布入口与学生外卖目录定向优化 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 在不改变真实业务路由、数据合同和身份工作台的前提下，把发布入口与学生校外外卖目录整理为更紧凑、清晰、可交付的主流移动端界面。

**Architecture:** 发布入口仅重排现有两个真实发布路由；学生外卖目录继续以 `listExternalTakeoutMerchants()` 为唯一数据源，在前端派生真实数量与营业状态筛选，并保留原有小程序跳转、重复点击锁和失败反馈。所有新增状态都由真实接口结果推导，不引入模拟商家信息。

**Tech Stack:** Taro 4、React、TypeScript、SCSS、Node.js 静态契约检查。

---

## Task 1：建立视觉与业务边界契约

- [ ] 新建 `scripts/check-publish-takeout-ui.cjs`，校验发布路由、外卖真实数据源、状态筛选和禁止虚构字段。
- [ ] 在 `package.json` 增加 `check:publish-takeout-ui` 并接入 `verify`。
- [ ] 先运行检查并确认在旧实现上失败，证明契约能捕获目标差异。

## Task 2：重构发布入口

- [ ] 保留 `/pages/publish-post/index` 与 `/pages/publish-secondhand/index` 两条真实路由。
- [ ] 将大面积渐变横幅改为紧凑白底说明区，明确“帖子用于交流，二手用于交易”。
- [ ] 将两个入口统一为信息层级清晰的列表卡片，保留可点击反馈与无障碍语义。
- [ ] 删除装饰圆球、冗余提示条与不必要的色彩竞争。

## Task 3：重构学生外卖目录

- [ ] 新增 `MerchantFilter = 'all' | 'open' | 'resting'`，由接口结果派生真实数量。
- [ ] 增加“全部 / 营业中 / 休息中”筛选，并为筛选无结果提供诚实空状态。
- [ ] 将目录头部、提示条、商家卡片与主操作整理为紧凑层级。
- [ ] 保留 `navigateToMiniProgram`、重复点击锁、休息状态拦截与失败提示。
- [ ] 不添加评分、销量、配送时间、距离、起送价、优惠或虚构图片。

## Task 4：验证与候选包

- [ ] 运行新增契约、外卖目录、发布入口、胶囊布局、首页工具、UI 系统与 TypeScript 检查。
- [ ] 运行 `npm run verify` 与 `npm run build:miniapp`。
- [ ] 审查差异，仅提交本轮文件，不混入用户未跟踪文档。
- [ ] 创建带提交短哈希的独立候选目录与 zip，并核对文件哈希和压缩包 SHA-256。
- [ ] 不上传微信版本，把最终候选包交给用户做最后审查。

## Commit

```powershell
git add docs/superpowers/plans/2026-08-02-publish-takeout-directory-refinement.md
git commit -m "计划：实施发布入口与外卖目录优化"
```
