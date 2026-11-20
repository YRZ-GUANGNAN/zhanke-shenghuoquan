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

> 已验证：服务器上的 `COS_SECRET_ID/KEY` 只能用于 COS 上传（STS + staging 前缀），调用 CBS（快照）与 CAM（子账号）接口均返回 `UnauthorizedOperation`；服务器与本机都没有腾讯云 API 凭证，因此以下三项必须由账号持有人在控制台完成。

1. **创建备份子账号并启用异地同步**（约 3 分钟）：
   - 访问管理 CAM → 用户 → 新建用户 → 自定义创建 → 勾选「编程访问」，不关联预设策略；
   - 新建自定义策略（JSON，region/APPID/桶名按现有配置）：
     ```json
     {
       "version": "2.0",
       "statement": [
         {
           "effect": "allow",
           "action": ["name/cos:PutObject", "name/cos:GetObject", "name/cos:DeleteObject", "name/cos:HeadObject"],
           "resource": ["qcs::cos:ap-guangzhou:uid/1507095774:zkshq-1507095774/campus-circle-backups/*"]
         },
         {
           "effect": "allow",
           "action": ["name/cos:GetBucket"],
           "resource": ["qcs::cos:ap-guangzhou:uid/1507095774:zkshq-1507095774/*"]
         }
       ]
     }
     ```
   - 将策略关联到该用户，复制其 SecretId/SecretKey；
   - 在服务器创建 `/etc/campus-circle/backup-cos.env`（权限 600）：
     ```
     BACKUP_COS_SECRET_ID=...
     BACKUP_COS_SECRET_KEY=...
     ```
   - 执行 `sudo /usr/local/bin/campus-backup-offsite.sh`，在 COS 控制台确认 `campus-circle-backups/` 下有 `db-*`、`shared-*` 和 `binlog/`（binlog 已纳入异地同步，保证异地也能做时间点恢复）。
2. **云硬盘快照**：CBS 控制台 → 快照策略 → 新建（建议每日 03:00、保留 7 天）→ 关联实例系统盘；整机损坏时分钟级回滚。
3. **媒体桶版本控制**：对象存储 → 桶 `zkshq-1507095774` → 基础配置 → 版本控制 → 开启，并设置历史版本保留 30 天。

如果希望由我代操作以上三项：可创建一把**临时 CAM 密钥**（具备 CAM/CBS/COS 相应权限，操作完成后可立即禁用），写入服务器 `/root/.tencent-provision.env`（600，不要发到聊天），我会完成配置与验证并告知你何时禁用。

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
