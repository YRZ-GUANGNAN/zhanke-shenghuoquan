# UI-21b-1 社区与二手只读真实数据接线 Acceptance Report

## 验收结论

**CONDITIONAL PASS**

自动门禁、真实 MySQL、真实 `cmd/server`、真实 HTTP 列表/详情/分页/404 验收均通过。微信开发者工具页面级点击、Network 零写请求、真实空态及真实图片 `302 → COS → <Image>` 链路没有形成完整人工证据，因此不得报告为全量 PASS。

验收日期：2026-07-12

真实后端端口：`127.0.0.1:18081`

## 自动门禁

| 项目 | 命令 | 结果 |
| --- | --- | --- |
| Boundary | `npm run check:boundaries` | PASS |
| 静态路由 | `npm run check:miniapp-static-routes` | PASS，25 routes |
| Miniapp typecheck | `npm --workspace apps/miniapp run typecheck` | PASS |
| Miniapp build | `npm run build:miniapp` | PASS |
| 全仓 verify | `npm run verify` | PASS |
| Diff check | `git diff --check master...HEAD` | PASS |

上述门禁在执行 `npm ci`、恢复本地真实依赖目录后完整重跑。构建仅出现既有 `publish-post/index.scss` Sass `@import` 弃用警告，不属于 UI-21b-1 新增错误。门禁通过后，为处理 C 盘空间不足，未跟踪且可再生的 `node_modules` 被删除；这不改变上述已留存的门禁结果。

真实性静态检查确认：

- 四个目标页面不再包含目标功能演示数组或固定 demo ID；
- 四页无直接 `Taro.request`；
- 数据读取只经过 `b-community.ts` / `c-trade.ts`；
- `catch` 不注入演示数据；
- 源代码改动范围仅为 10 个授权前端文件。

实施计划中的 `$allowed` 示例对页面目录分支缺少 `.*`，会把合法页面文件误报为越界。本次只在验收命令中修正为：

```text
apps/miniapp/src/pages/(home|post-detail|student-secondhand|secondhand-detail)/.*
```

仓库边界脚本未修改。

## 真实 MySQL 与 HTTP

真实环境摘要：

```json
{
  "status": "PASS",
  "mysql": "real",
  "backend": "cmd/server",
  "port": 18081,
  "fixturePosts": 25,
  "fixtureProducts": 25,
  "fixtureComments": 25
}
```

验收夹具：

- user ID：`371`
- post IDs：`18`–`42`
- product IDs：`21`–`45`
- comment IDs：`2`–`26`
- 评论所属帖子：`18`
- scoped outbox：`0`

真实 HTTP 断言：

| 项目 | 结果 |
| --- | --- |
| 社区列表→详情 ID | `1` → `1` |
| 二手列表→详情 ID | `45` → `45` |
| Community total | `28` |
| Trade total | `29` |
| Comments total | `25` |
| 跨页重复数 | `0` |
| 不存在帖子 | HTTP 404 |
| 不存在商品 | HTTP 404 |

真实调用覆盖：

- `GET /api/v1/community/posts`
- `GET /api/v1/community/posts/:id`
- `GET /api/v1/community/posts/:id/comments`
- `GET /api/v1/trade/products`
- `GET /api/v1/trade/products/:id`

结果证明：列表 ID 可读取对应详情；社区、二手和评论均能跨页读取；两页合并后无重复；合法但不存在的帖子、商品均 fail-closed 返回 404；金额仍由后端整数分返回，前端只在服务层转换为元。

## 条件验收项

微信开发者工具已成功导入并打开精确的实施工作树，但临时后端触发 Windows 防火墙安全提示。遵循安全边界，本次没有代替 owner 操作系统安全授权，因此以下项目没有完整人工截图或 DevTools Network 证据，维持 **CONDITIONAL**：

1. 首页真实 Community/Trade 双源卡片的微信开发者工具渲染。
2. 首页一源失败、另一源成功时的局部降级和点击重试。
3. 两源同时失败时的整页错误态与后端恢复后重试。
4. 帖子、二手详情 404 页面文案和点击重试。
5. 评论第二页失败后保留首屏评论并重试。
6. 快速切换多个二手分类时旧响应不覆盖新分类。
7. 真实空分类/空评论的页面空态。
8. 写控件在 DevTools Network 中确认零 POST/PUT/PATCH/DELETE。
9. 真机/微信开发者工具视觉几何与冻结稿逐项比对。

代码审查和自动门禁已覆盖对应并发保护、去重、错误分支及无直接写请求，但不能替代人工 UI 证据。

## 图片链路

本次验收夹具未制造图片，现有真实列表也未提供可用于验收的 approved 图片行，因此未完成：

```text
images[0].url → content 302 → COS 短期地址 → <Image>
```

该项维持 **CONDITIONAL**。不得制造公开图片 URL，也不得把“无图占位代码存在”报告为真实图片链路 PASS。

## 最终判定

- 自动门禁：PASS
- 真实 MySQL + 真实后端 + 真实 HTTP：PASS
- 分页、去重、列表→详情、404：PASS
- 微信开发者工具人工 UI：CONDITIONAL
- 真实图片显示链路：CONDITIONAL
- 真实空态：CONDITIONAL

owner 在合并前仍需完成上述人工场景；在证据补齐前，本模块只报告 `CONDITIONAL PASS`。
