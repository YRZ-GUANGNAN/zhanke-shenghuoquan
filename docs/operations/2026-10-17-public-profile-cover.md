# 公开主页封面配套后端发布

用户确认安装当前个人主页和二手展示更新并上传微信与 GitHub。线上版本核验为 `2026.10.16.4 / 302660924ac5`；现有 `users.cover` 已保存封面，但公开 `Profile` 投影尚未返回该字段。

本次复用已完成且源码干净的后端发布工作区，创建 `codex/public-profile-api-20261017`，基于线上代码只补充 `Profile.Cover` 和对应用户、社区回归。未引入开发分支中的外部收件表或审核策略更新。无需迁移数据库，不改变身份和功能开关。

发布脚本 `deploy/native/release-public-profile.py` 默认只读预检。明确校验原版本、原二进制、新产物 SHA256、私有记录目录及磁盘余量；在发布锁中复核基线，安装到新的版本目录再原子切换，失败恢复原版本并等待就绪。旧 `/opt/campus-circle/releases/20261016-delivery-api` 完整保留作为回滚备份。发布前后比对功能策略、分享设置、运行环境和受保护接口状态，不写数据库。

版本：`2026.10.17.1`。目标：`/opt/campus-circle/releases/20261017-public-profile`。回执：`/var/backups/campus-circle/20261017-public-profile-result.json`。

验证：用户封面与公开主页定向 Go 测试、隔离生产数据库凭据后的全部 Go 测试、Linux amd64 构建均通过。

2026-10-17 14:49:42（Asia/Shanghai）部署成功。源码提交 `092ddc3521259f2fc238d73a07c99ca94a120c0e`，二进制 SHA256 `641c456b6ad49cdeb26306098b96ce8b4a0d9a23f2a8cab52514c27328d8b530`。公网 `/healthz` 和 `/version` 返回 200，主机 `/readyz` 为 ready，服务 active/running、NRestarts=0、ExecMainStatus=0。

发布前后功能策略、分享设置、运行环境摘要一致。公开主页匿名响应保持 403，商家/骑手订单和管理分享接口保持 401。旧二进制 SHA256 复核一致；未执行数据库迁移、未写测试数据、未触发回滚。

微信登录后的真实封面显示仍需设备验收，匿名权限检查不能替代此验收。小程序开发版本上传和正式发布分开记录。
