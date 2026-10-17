# 公开主页封面配套后端发布

用户确认安装当前个人主页和二手展示更新并上传微信与 GitHub。线上版本核验为 `2026.10.16.4 / 302660924ac5`；现有 `users.cover` 已保存封面，但公开 `Profile` 投影尚未返回该字段。

本次复用已完成且源码干净的后端发布工作区，创建 `codex/public-profile-api-20261017`，基于线上代码只补充 `Profile.Cover` 和对应用户、社区回归。未引入开发分支中的外部收件表或审核策略更新。无需迁移数据库，不改变身份和功能开关。

发布脚本 `deploy/native/release-public-profile.py` 默认只读预检。明确校验原版本、原二进制、新产物 SHA256、私有记录目录及磁盘余量；在发布锁中复核基线，安装到新的版本目录再原子切换，失败恢复原版本并等待就绪。旧 `/opt/campus-circle/releases/20261016-delivery-api` 完整保留作为回滚备份。发布前后比对功能策略、分享设置、运行环境和受保护接口状态，不写数据库。

版本：`2026.10.17.1`。目标：`/opt/campus-circle/releases/20261017-public-profile`。回执：`/var/backups/campus-circle/20261017-public-profile-result.json`。

验证：用户封面与公开主页定向 Go 测试通过；Linux amd64 构建和线上健康核验在发布阶段执行。微信登录后的真实封面显示仍需设备验收，匿名权限检查不能替代此验收。
