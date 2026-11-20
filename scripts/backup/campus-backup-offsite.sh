#!/usr/bin/env bash
# 每周异地同步：把最近一份数据库全量与上传资源复制到 COS 私有桶（服务端加密）。
# 需要专用的备份子账号密钥（主线 CMS 密钥只能申请 STS，无法直接写对象）：
#   /etc/campus-circle/backup-cos.env
#   BACKUP_COS_SECRET_ID=...
#   BACKUP_COS_SECRET_KEY=...
#   BACKUP_COS_BUCKET=...   （可选，默认沿用 COS_BUCKET）
#   BACKUP_COS_REGION=...   （可选，默认沿用 COS_REGION）
# 该文件不存在时仅记录日志并退出 0，不影响本地备份。
set -euo pipefail
umask 077

ENV_FILE=/etc/campus-circle/campus-api.env
CRED_FILE=/etc/campus-circle/backup-cos.env
BACKUP_DIR=/var/backups/campus-circle
LOG=/var/log/campus-backup.log
exec >>"$LOG" 2>&1

get_env() { grep -E "^$1=" "$ENV_FILE" | head -1 | cut -d= -f2- | tr -d '"'; }
get_cred() { grep -E "^$1=" "$CRED_FILE" | head -1 | cut -d= -f2- | tr -d '"'; }

if [ ! -r "$CRED_FILE" ]; then
  echo "[$(date -Is)] offsite disabled: $CRED_FILE not configured"
  exit 0
fi

db_file=$(ls -1t "$BACKUP_DIR"/db-*.sql.gz 2>/dev/null | head -1)
shared_file=$(ls -1t "$BACKUP_DIR"/shared-*.tar.gz 2>/dev/null | head -1)
if [ -z "$db_file" ] || [ -z "$shared_file" ]; then
  echo "[$(date -Is)] offsite skip: no local backup found"
  exit 1
fi

access_key="$(get_cred BACKUP_COS_SECRET_ID)"
secret_key="$(get_cred BACKUP_COS_SECRET_KEY)"
bucket="$(get_cred BACKUP_COS_BUCKET)"
region="$(get_cred BACKUP_COS_REGION)"
[ -n "$bucket" ] || bucket="$(get_env COS_BUCKET)"
[ -n "$region" ] || region="$(get_env COS_REGION)"

export RCLONE_CONFIG_COS_TYPE=s3
export RCLONE_CONFIG_COS_PROVIDER=TencentCOS
export RCLONE_CONFIG_COS_ACCESS_KEY_ID="$access_key"
export RCLONE_CONFIG_COS_SECRET_ACCESS_KEY="$secret_key"
export RCLONE_CONFIG_COS_ENDPOINT="https://cos.$region.myqcloud.com"
export RCLONE_CONFIG_COS_REGION="$region"
export RCLONE_CONFIG_COS_ACL=private

prefix=campus-circle-backups
echo "[$(date -Is)] offsite sync start: $(basename "$db_file") $(basename "$shared_file")"
for file in "$db_file" "$shared_file"; do
  rclone copyto "$file" "cos:$bucket/$prefix/$(basename "$file")" \
    --header-upload "x-cos-server-side-encryption: AES256" \
    --retries 3 --low-level-retries 5 --timeout 120s
done

# binlog 一并异地保存：没有 binlog，异地 dump 只能恢复到备份时点。
datadir=$(mysql --defaults-file=/etc/mysql/debian.cnf -N -B -e "SHOW VARIABLES LIKE 'datadir'" | awk '{print $2}')
if [ -n "$datadir" ] && [ -d "$datadir" ]; then
  rclone copy "$datadir" "cos:$bucket/$prefix/binlog" \
    --include "mysql-bin.*" --max-age 8d \
    --header-upload "x-cos-server-side-encryption: AES256" \
    --retries 3 --low-level-retries 5 --timeout 120s || true
fi

# COS 侧保留 90 天，避免无限增长
rclone delete "cos:$bucket/$prefix" --min-age 90d >/dev/null 2>&1 || true

date -Is > "$BACKUP_DIR/LAST_OFFSITE_SUCCESS"
echo "[$(date -Is)] offsite sync done"
