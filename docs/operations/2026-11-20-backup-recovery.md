# 2026-11-20 生产备份与恢复方案落地

## 本次落地内容

生产机已安装每日备份、每周恢复演练与日志轮转（root 权限）：

- `/usr/local/bin/campus-backup.sh`：每日 02:37
  - MySQL 全量：`mysqldump --single-transaction --quick --no-tablespaces --default-character-set=utf8mb4 --source-data=2`，gzip，权限 600；记录 binlog 位点，支持时间点恢复。
  - 本地上传资源：`/opt/campus-circle/shared/`（导航图标、分享封面）打包。
  - 关键配置：`/etc/campus-circle/campus-api.env`、systemd unit、nginx 站点、备份 cron 打包，仅保留本机（权限 600），不上传。
  - 本地保留 14 天，成功后写 `LAST_SUCCESS`。
- `/usr/local/bin/campus-backup-offsite.sh`：每周日 02:45 把最近一份数据库全量与 shared 包同步到 COS 私有桶（服务端加密，COS 侧保留 90 天）。**需要专用备份密钥**（见待办）；未配置时自动跳过，不影响本地备份。
- `/usr/local/bin/campus-restore-verify.sh`：每周日 03:20 把最近一份 dump 恢复到临时库、核对表数量与用户数后自动删除临时库。
- `/etc/cron.d/campus-backup`、`/etc/logrotate.d/campus-backup`（日志保留 8 周）。

当前 binlog 已开启（`log_bin=ON`，保留 30 天），配合每日全量可做分钟级时间点恢复。

## 首次执行验证（2026-11-20）

- 备份产物：`db-20261120T110722Z.sql.gz`（69K）、`shared-…tar.gz`（363K）、`config-…tar.gz`（2.8K），权限均为 600。
- 恢复演练：临时库恢复 **115 张表 = 当前 115 张表**，`users=47`，演练后临时库已删除。
- 异地同步：按设计跳过（`/etc/campus-circle/backup-cos.env` 未配置）。
- cron 服务 active，定时任务已注册。

## 恢复目标

| 场景 | 方案 | RPO | RTO |
|---|---|---|---|
| 误删/误操作 | binlog 时间点恢复 | 分钟级 | ~30 分钟 |
| 数据库损坏 | 每日全量 + binlog | ≤ 24 小时 | 30–60 分钟 |
| 整机损坏 | 云硬盘快照（建议开启） | ≤ 24 小时 | 分钟级 |
| 发版出错 | release 软链回滚 | 0 | 分钟级 |

## 待办（需要控制台/人工）

1. **异地备份密钥**：主线 COS 密钥只能申请 STS，无法直接写对象。请在腾讯云创建一个专用子账号，仅授权 `campus-circle-backups/*` 的 `PutObject / GetObject / DeleteObject / ListBucket`，然后写入
   `/etc/campus-circle/backup-cos.env`（权限 600）：
   ```
   BACKUP_COS_SECRET_ID=...
   BACKUP_COS_SECRET_KEY=...
   ```
   随后执行 `sudo /usr/local/bin/campus-backup-offsite.sh` 并在 COS 控制台确认对象存在。
2. **云硬盘快照**：控制台为系统盘开启每日快照、保留 7 天（整机损坏时的最快恢复手段）。
3. **媒体桶版本控制**：为媒体 COS 桶开启版本控制，防止误删对象。

## 常用运维命令

```bash
sudo /usr/local/bin/campus-backup.sh          # 手动备份
sudo /usr/local/bin/campus-backup-offsite.sh  # 手动异地同步
sudo /usr/local/bin/campus-restore-verify.sh  # 手动恢复演练
sudo tail -50 /var/log/campus-backup.log      # 查看备份日志
sudo ls -lh /var/backups/campus-circle/       # 查看备份文件
cat /var/backups/campus-circle/LAST_SUCCESS   # 最近一次成功时间
```

恢复示例（整库恢复到新库）：

```bash
sudo mysql --defaults-file=/etc/mysql/debian.cnf -e "CREATE DATABASE campus_restore CHARACTER SET utf8mb4"
sudo gunzip -c /var/backups/campus-circle/db-<STAMP>.sql.gz | sudo mysql --defaults-file=/etc/mysql/debian.cnf --default-character-set=utf8mb4 campus_restore
```
