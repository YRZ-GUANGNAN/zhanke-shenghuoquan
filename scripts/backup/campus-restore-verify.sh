#!/usr/bin/env bash
# 每周恢复演练：把最近一份数据库备份恢复到临时库，核对表数量后删除。
# 任何一步失败都会以非零退出码结束，便于监控发现。
set -euo pipefail
umask 077

BACKUP_DIR=/var/backups/campus-circle
LOG=/var/log/campus-backup.log
exec >>"$LOG" 2>&1

latest=$(ls -1t "$BACKUP_DIR"/db-*.sql.gz 2>/dev/null | head -1)
if [ -z "$latest" ]; then
  echo "[$(date -Is)] restore verify failed: no backup file"
  exit 1
fi

tmpdb="campus_restore_verify_$(date -u +%Y%m%d%H%M%S)"
mysql_root() { mysql --defaults-file=/etc/mysql/debian.cnf --default-character-set=utf8mb4 "$@"; }
drop_tmp() { mysql_root -e "DROP DATABASE IF EXISTS \`$tmpdb\`" >/dev/null 2>&1 || true; }
trap drop_tmp EXIT

echo "[$(date -Is)] restore verify start: $(basename "$latest")"
mysql_root -e "CREATE DATABASE \`$tmpdb\` CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci"
gunzip -c "$latest" | mysql_root "$tmpdb"

restored_tables=$(mysql_root -N -B -e "SELECT COUNT(*) FROM information_schema.tables WHERE table_schema='$tmpdb'")
live_tables=$(mysql_root -N -B -e "SELECT COUNT(*) FROM information_schema.tables WHERE table_schema='campus_circle'")
users=$(mysql_root -N -B -e "SELECT COUNT(*) FROM \`$tmpdb\`.users")

if [ "$restored_tables" -lt "$live_tables" ]; then
  echo "[$(date -Is)] restore verify FAILED: restored=$restored_tables live=$live_tables"
  exit 1
fi
echo "[$(date -Is)] restore verify ok: tables=$restored_tables live=$live_tables users=$users"
