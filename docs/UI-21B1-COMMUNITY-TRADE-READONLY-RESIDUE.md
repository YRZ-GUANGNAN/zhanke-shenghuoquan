# UI-21b-1 社区与二手只读真实数据接线 Residue Report

## 数据清理

验收夹具使用独立 `UI21B1_` 前缀。最终清理结果：

```json
{
  "cleaned": true,
  "residue": {
    "comments": 0,
    "posts": 0,
    "products": 0,
    "users": 0
  }
}
```

夹具未生成 scoped trade outbox，`outboxCount=0`，未留下对应 Meili 文档或待消费事件。

## 进程与端口

验收过程中启动了本模块自有的：

- `cmd/server` 验收进程，监听 `18081`；
- Meilisearch 验收进程，监听 `7700`。

清理后只读检查结果：

- `18081` 无监听；
- `7700` 无监听；
- 无进程路径位于 `D:\codex-temp\ui21b1\*`；
- 验收没有触碰既有 `8080` 服务。

## 配置与凭证

- 未修改或提交 `backend/.env`；
- 未输出 MySQL、COS、微信或 Meili secret；
- 未修改小程序 token；
- 目标读取端点为公开 GET，因此无需注入或恢复验收 token；
- `apps/miniapp/project.config.json` 无差异。

## 磁盘恢复与依赖

首次 Go 构建在写入夹具前因 C 盘空间不足失败。随后将 `GOCACHE`、`GOTMPDIR`、npm 临时目录移到 D 盘，重新执行真实验收并通过。

磁盘处理遵循可再生和可验证原则：

1. 临时移动依赖时发现 npm workspace 链接会被跟随，立即停止并按文件数与字节数核对后原样恢复 `apps/admin`、`apps/miniapp` 依赖。
2. 删除临时 Junction，执行 `npm ci` 恢复工作树本地普通 `node_modules`。
3. 恢复后重新执行 typecheck、build、verify，全部 PASS。
4. 门禁完成后删除工作树和主仓库中未跟踪、可通过 `npm ci` 重建的根 `node_modules`，未删除任何源码或配置。
5. 删除 8 个已核实且无安装进程占用的 Visual Studio 安装临时目录、Maven `.m2\repository` 缓存，以及 Go 构建/测试缓存。
6. 删除 D 盘本模块的临时 workspace 依赖、Meili 数据、Go/npm 缓存和临时运行文件。

当前工作树没有 Junction；下次需要运行 npm 门禁时，在对应工作树执行 `npm ci` 即可重建依赖。

## 临时文件

以下运行时临时项已删除：

- 验收后端可执行文件；
- fixture Go 源文件；
- acceptance PowerShell 脚本；
- PID 文件；
- Meili 临时数据目录；
- D 盘临时依赖与缓存目录；
- 本模块验收 JSON/log（关键结论已经逐项抄录到正式报告）。

## Git 状态

源代码报告落盘前：

- 实施工作树无未提交源代码改动；
- `git diff --check master...HEAD` 通过；
- 仅 10 个授权前端文件相对 master 有代码差异；
- 独立审计未发现 P1/P2/P3；
- 受保护的 `feat/p0-identity-switch` 工作树未触碰；
- `master` 保持 `6979762d`，未合并、未打标签。

报告提交后还需再次确认工作树干净且分支可从 master 快进合并。

## 剩余条件项

以下不是运行时残留，但仍是 owner 验收待办：

- 微信开发者工具页面截图；
- 双源降级、网络恢复、快速分类切换的手工交互；
- 真实空分类/空评论页面；
- approved 图片的 `302 → COS → <Image>` 完整显示链路；
- 写控件 DevTools Network 零写请求证明。

在这些证据补齐前，验收结论保持 `CONDITIONAL PASS`。
