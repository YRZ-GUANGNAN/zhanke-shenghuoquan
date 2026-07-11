# UI-21a 运行时登录与身份地基 Residue Report

## 仓库残留

- 源 worktree：最终提交后要求 `git status` clean。
- 主仓库 `master`：保持 `18c5b7f4`，未合并、未改文件。
- 临时 D 盘验证克隆：验收完成后删除。
- 验收进程：本模块使用 `go test`、npm 构建和 `httptest`，命令结束即退出；未启动常驻后端、watcher 或 dev server。
- MySQL：UI-21a 测试数据使用固定 fence scoped 清理；迁移最终保持 apply 状态，符合交付基线。
- 凭证：未复制、打印或提交 `backend/.env`。

## 保留项

- `feat/p0-identity-switch@c9417df8` 按授权作为历史参考保留，不属于残留。
- `D:\claude-cache\Claude-3p\vm_bundles` 是为 C 盘整理保留的旧客户端缓存，原路径有目录联接；不是项目验收临时产物。

## 上线前待办

- owner 在微信开发者工具/真机完成 Acceptance Report 中 5 条 Conditional 肉眼验收。
- 生产环境仍需有效微信 AppID/AppSecret、HTTPS 合法域名与现有生产配置；本模块未放宽任何生产安全门。
- 验收通过后由 owner 在主仓库执行串行 `--ff-only` 合并并打 UI-21a baseline tag；本开发任务不代做合并/tag。
