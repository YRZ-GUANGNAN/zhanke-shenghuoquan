#!/usr/bin/env bash
# 每日备份：MySQL 全量 + 本地上传资源 + 关键配置（仅本机保留）。
# 数据库 dump 记录 binlog 位点，配合 binlog 可做时间点恢复。
set -euo pipefail
umask 077

BACKUP_DIR=/var/backups/campus-circle
LOG=/var/log/campus-backup.log
STAMP=$(date -u +%Y%m%dT%H%M%SZ)
mkdir -p "$BACKUP_DIR"
exec >>"$LOG" 2>&1

echo "[$(date -Is)] daily backup start $STAMP"

# 1) MySQL 全量（不锁表；--source-data=2 记录 binlog 位点）
tmp="$BACKUP_DIR/db-$STAMP.sql.gz.tmp"
mysqldump --defaults-file=/etc/mysql/debian.cnf \
  --single-transaction --quick --no-tablespaces \
  --default-character-set=utf8mb4 --source-data=2 \
  campus_circle | gzip > "$tmp"
gzip -t "$tmp"
mv "$tmp" "$BACKUP_DIR/db-$STAMP.sql.gz"
chmod 600 "$BACKUP_DIR/db-$STAMP.sql.gz"

# 2) 本地上传资源（导航图标、分享封面）
tmp="$BACKUP_DIR/shared-$STAMP.tar.gz.tmp"
tar -czf "$tmp" -C /opt/campus-circle shared
mv "$tmp" "$BACKUP_DIR/shared-$STAMP.tar.gz"
chmod 600 "$BACKUP_DIR/shared-$STAMP.tar.gz"

# 3) 关键配置（含密钥，只保留在本机，权限 600，不上传 COS）
tmp="$BACKUP_DIR/config-$STAMP.tar.gz.tmp"
tar -czf "$tmp" --ignore-failed-read -C / \
  etc/campus-circle/campus-api.env \
  etc/systemd/system/campus-api.service \
  etc/nginx/sites-enabled \
  etc/cron.d/campus-backup
mv "$tmp" "$BACKUP_DIR/config-$STAMP.tar.gz"
chmod 600 "$BACKUP_DIR/config-$STAMP.tar.gz"

# 4) 本地保留 14 天
find "$BACKUP_DIR" -maxdepth 1 -type f \( -name 'db-*.sql.gz' -o -name 'shared-*.tar.gz' -o -name 'config-*.tar.gz' \) -mtime +14 -delete

echo "[$(date -Is)] daily backup done: $(ls -1 "$BACKUP_DIR"/db-*.sql.gz | tail -1)"
date -Is > "$BACKUP_DIR/LAST_SUCCESS"
