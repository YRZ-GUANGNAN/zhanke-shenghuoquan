# UI-21b-2b 社区写入闭环修复报告

## Development

本次在 `feat/ui21b2b1-community-writes` 上完成社区写入闭环的小程序前端修复，范围限定为 `apps/miniapp/src/**`，未修改后端/API/迁移。

改动内容：

- `apps/miniapp/src/services/b-community.ts`
  - 修复并整理社区 service，恢复清晰的中文文案和类型定义。
  - 补齐社区发帖、详情、评论、点赞、收藏、关注、通知等调用封装。
  - `CreateCommunityPostInput.topic` 设为可选，未传时默认 `湛科日常`，兼容当前静态发帖 UI。
  - `listCommunityNotifications()` 返回 `{ items, unread }`，保持通知页既有调用形态。

- `apps/miniapp/src/pages/publish-post/index.tsx`
  - 接通真实发帖写入路径：表单内容、话题选择、图片上传后生成的 `imageAssetIds` 会进入 `createCommunityPost`。
  - 保留当前静态 UI 壳子，不引入新架构、不改后端契约。
  - 发帖成功后给出成功提示并回到首页。

- `apps/miniapp/src/pages/post-detail/index.tsx`
  - 接通帖子详情写操作：点赞、收藏、关注、发表评论。
  - 交互采用前端乐观更新 + 失败回滚/提示，避免静态按钮“点了没写入”。
  - 保留当前 UI 风格与页面结构，不引入新依赖。

## Acceptance

### 只读/边界检查

- `git diff -- backend`：无输出，确认后端未改。
- `git diff --stat`：仅 3 个小程序前端文件变更。
- `rg "%E|�|undefined\\.call|TODO_UI21|console\\.log"` 针对本次 3 个文件：无命中，确认没有明显乱码、百分号编码残留或调试输出。
- `Test-Path apps/miniapp/node_modules`：`False`，确认未留下临时依赖目录/链接残留。

### Typecheck / Build 取证

标准验证在当前 isolated worktree 内受依赖状态阻塞：

- `npm --prefix apps/miniapp run typecheck`
  - 失败原因：`'tsc' is not recognized...`
  - 判定：当前 worktree 未安装/链接 miniapp 本地依赖，不是本次源码变更直接错误。

- `npm --prefix apps/miniapp run build:weapp`
  - 失败原因：`'taro' is not recognized...`
  - 判定：当前 worktree 未安装/链接 miniapp 本地依赖，不是本次源码变更直接错误。

补充使用主仓库根 `node_modules/.bin` 做取证：

- `tsc -p apps/miniapp/tsconfig.json --noEmit`
  - 失败集中在 Taro 全局类型、`@tarojs/components`、`zustand`、`@tencentcloud/chat` 等依赖解析，以及既有 `apps/miniapp/src/pages/home/index.tsx(226,48)` 老错误。
  - 本次任务文件未出现新增业务类型错误；仅被同类 Taro 模块/全局类型解析问题扫到。

- `taro build --type weapp`
  - Taro 启动到 `Taro v4.2.0` 后失败：找不到插件依赖 `@tarojs/plugin-platform-weapp`，项目路径指向当前 isolated worktree。
  - 判定：仍为 isolated worktree 依赖解析问题。

因此本次验收结论为：源码修复已完成，边界符合要求；完整 typecheck/build 需要在依赖完整的小程序工作区重新执行。

## Boundary

- 未修改 `backend/**`。
- 未修改迁移、路由、后端 handler 或 API 契约。
- 未新增依赖。
- 未进入外卖/配送/后台管理等其它模块。
- 本次只补社区 B 域小程序前端写入闭环。

## Residue

- 未启动后端、未创建测试数据。
- 未留下临时 `node_modules`、junction 或测试进程。
- 提交前工作区仅包含本任务前端文件与本报告。

