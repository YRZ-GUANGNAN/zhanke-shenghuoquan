-- 后台管理员账号独立化（与小程序 users 解耦）。
-- 1) admin_credentials 自带 role/status/display_name；user_id 改为可空（仅保留历史绑定，新账号为 NULL）；
-- 2) admin_web_sessions.user_id 改为可空（新会话只写 credential_id）；
-- 3) admin_auth_audit_logs 增加操作者列并扩展事件集合；
-- 4) 各后台操作人列新增 *_admin_id 并回填历史值（旧列保留为 legacy，新代码只写新列）。
-- 幂等：所有 DDL 通过 information_schema 守卫，可重放。

-- ---------- 1. admin_credentials：独立账号字段 ----------
SET @col_exists := (SELECT COUNT(*) FROM information_schema.COLUMNS
  WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'admin_credentials' AND COLUMN_NAME = 'role');
SET @ddl := IF(@col_exists = 0,
  'ALTER TABLE admin_credentials ADD COLUMN role VARCHAR(24) NOT NULL DEFAULT ''admin'' AFTER username',
  'SELECT 1');
PREPARE migrate_stmt FROM @ddl; EXECUTE migrate_stmt; DEALLOCATE PREPARE migrate_stmt;

SET @col_exists := (SELECT COUNT(*) FROM information_schema.COLUMNS
  WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'admin_credentials' AND COLUMN_NAME = 'status');
SET @ddl := IF(@col_exists = 0,
  'ALTER TABLE admin_credentials ADD COLUMN status VARCHAR(24) NOT NULL DEFAULT ''active'' AFTER role',
  'SELECT 1');
PREPARE migrate_stmt FROM @ddl; EXECUTE migrate_stmt; DEALLOCATE PREPARE migrate_stmt;

SET @col_exists := (SELECT COUNT(*) FROM information_schema.COLUMNS
  WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'admin_credentials' AND COLUMN_NAME = 'display_name');
SET @ddl := IF(@col_exists = 0,
  'ALTER TABLE admin_credentials ADD COLUMN display_name VARCHAR(64) NOT NULL DEFAULT '''' AFTER status',
  'SELECT 1');
PREPARE migrate_stmt FROM @ddl; EXECUTE migrate_stmt; DEALLOCATE PREPARE migrate_stmt;

SET @col_exists := (SELECT COUNT(*) FROM information_schema.COLUMNS
  WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'admin_credentials' AND COLUMN_NAME = 'legacy_user_id');
SET @ddl := IF(@col_exists = 0,
  'ALTER TABLE admin_credentials ADD COLUMN legacy_user_id BIGINT UNSIGNED NULL AFTER display_name',
  'SELECT 1');
PREPARE migrate_stmt FROM @ddl; EXECUTE migrate_stmt; DEALLOCATE PREPARE migrate_stmt;

SET @nullable := (SELECT is_nullable FROM information_schema.COLUMNS
  WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'admin_credentials' AND COLUMN_NAME = 'user_id');
SET @ddl := IF(@nullable = 'NO',
  'ALTER TABLE admin_credentials MODIFY COLUMN user_id BIGINT UNSIGNED NULL',
  'SELECT 1');
PREPARE migrate_stmt FROM @ddl; EXECUTE migrate_stmt; DEALLOCATE PREPARE migrate_stmt;

SET @idx_exists := (SELECT COUNT(*) FROM information_schema.STATISTICS
  WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'admin_credentials' AND INDEX_NAME = 'uq_admin_credentials_legacy_user');
SET @ddl := IF(@idx_exists = 0,
  'ALTER TABLE admin_credentials ADD UNIQUE KEY uq_admin_credentials_legacy_user (legacy_user_id)',
  'SELECT 1');
PREPARE migrate_stmt FROM @ddl; EXECUTE migrate_stmt; DEALLOCATE PREPARE migrate_stmt;

SET @chk_exists := (SELECT COUNT(*) FROM information_schema.TABLE_CONSTRAINTS
  WHERE CONSTRAINT_SCHEMA = DATABASE() AND TABLE_NAME = 'admin_credentials' AND CONSTRAINT_NAME = 'chk_admin_credentials_role');
SET @ddl := IF(@chk_exists = 0,
  'ALTER TABLE admin_credentials ADD CONSTRAINT chk_admin_credentials_role CHECK (role IN (''admin'',''superadmin''))',
  'SELECT 1');
PREPARE migrate_stmt FROM @ddl; EXECUTE migrate_stmt; DEALLOCATE PREPARE migrate_stmt;

SET @chk_exists := (SELECT COUNT(*) FROM information_schema.TABLE_CONSTRAINTS
  WHERE CONSTRAINT_SCHEMA = DATABASE() AND TABLE_NAME = 'admin_credentials' AND CONSTRAINT_NAME = 'chk_admin_credentials_status');
SET @ddl := IF(@chk_exists = 0,
  'ALTER TABLE admin_credentials ADD CONSTRAINT chk_admin_credentials_status CHECK (status IN (''active'',''disabled''))',
  'SELECT 1');
PREPARE migrate_stmt FROM @ddl; EXECUTE migrate_stmt; DEALLOCATE PREPARE migrate_stmt;

-- ---------- 2. admin_web_sessions：新会话不再写 user_id ----------
SET @nullable := (SELECT is_nullable FROM information_schema.COLUMNS
  WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'admin_web_sessions' AND COLUMN_NAME = 'user_id');
SET @ddl := IF(@nullable = 'NO',
  'ALTER TABLE admin_web_sessions MODIFY COLUMN user_id BIGINT UNSIGNED NULL',
  'SELECT 1');
PREPARE migrate_stmt FROM @ddl; EXECUTE migrate_stmt; DEALLOCATE PREPARE migrate_stmt;

-- ---------- 3. admin_auth_audit_logs：操作者列 + 事件集合 ----------
SET @col_exists := (SELECT COUNT(*) FROM information_schema.COLUMNS
  WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'admin_auth_audit_logs' AND COLUMN_NAME = 'actor_credential_id');
SET @ddl := IF(@col_exists = 0,
  'ALTER TABLE admin_auth_audit_logs ADD COLUMN actor_credential_id BIGINT UNSIGNED NULL AFTER credential_id',
  'SELECT 1');
PREPARE migrate_stmt FROM @ddl; EXECUTE migrate_stmt; DEALLOCATE PREPARE migrate_stmt;

SET @chk_exists := (SELECT COUNT(*) FROM information_schema.TABLE_CONSTRAINTS
  WHERE CONSTRAINT_SCHEMA = DATABASE() AND TABLE_NAME = 'admin_auth_audit_logs' AND CONSTRAINT_NAME = 'chk_admin_auth_audit_event');
SET @ddl := IF(@chk_exists = 1,
  'ALTER TABLE admin_auth_audit_logs DROP CHECK chk_admin_auth_audit_event',
  'SELECT 1');
PREPARE migrate_stmt FROM @ddl; EXECUTE migrate_stmt; DEALLOCATE PREPARE migrate_stmt;

ALTER TABLE admin_auth_audit_logs ADD CONSTRAINT chk_admin_auth_audit_event CHECK (
  event_type IN (
    'login','logout','password_changed','credential_bootstrap','credential_reset',
    'credential_set','credential_revoke','account_locked','sessions_revoked',
    'account_created','account_updated','account_disabled','account_enabled',
    'admin_role_changed','password_reset'
  )
);

-- ---------- 4. 历史回填：绑定用户 → 独立账号 ----------
UPDATE admin_credentials c JOIN users u ON u.id = c.user_id
SET c.legacy_user_id = u.id,
    c.role = CASE WHEN u.role IN ('admin','superadmin') THEN u.role ELSE 'admin' END,
    c.status = CASE WHEN u.status = 'active' AND u.role IN ('admin','superadmin') THEN 'active' ELSE 'disabled' END,
    c.display_name = IF(c.display_name = '', COALESCE(NULLIF(u.nickname, ''), c.username), c.display_name)
WHERE c.user_id IS NOT NULL AND c.legacy_user_id IS NULL;

-- 存在后台账号但没有任何启用中的超级管理员时中止迁移（避免把运维锁在门外）。
SET @cred_total := (SELECT COUNT(*) FROM admin_credentials);
SET @active_super := (SELECT COUNT(*) FROM admin_credentials WHERE role = 'superadmin' AND status = 'active');
SET @ddl := IF(@cred_total > 0 AND @active_super = 0,
  'SIGNAL SQLSTATE ''45000'' SET MESSAGE_TEXT = ''migration 069: credentials exist but no active superadmin''',
  'SELECT 1');
PREPARE migrate_stmt FROM @ddl; EXECUTE migrate_stmt; DEALLOCATE PREPARE migrate_stmt;

-- ---------- 5. 各后台操作人列（新列可空；历史值按 legacy_user_id 回填） ----------
SET @col_exists := (SELECT COUNT(*) FROM information_schema.COLUMNS
  WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'review_tasks' AND COLUMN_NAME = 'operator_admin_id');
SET @ddl := IF(@col_exists = 0,
  'ALTER TABLE review_tasks ADD COLUMN operator_admin_id BIGINT UNSIGNED NULL AFTER operator_id',
  'SELECT 1');
PREPARE migrate_stmt FROM @ddl; EXECUTE migrate_stmt; DEALLOCATE PREPARE migrate_stmt;

SET @col_exists := (SELECT COUNT(*) FROM information_schema.COLUMNS
  WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'user_feedbacks' AND COLUMN_NAME = 'handled_by_admin_id');
SET @ddl := IF(@col_exists = 0,
  'ALTER TABLE user_feedbacks ADD COLUMN handled_by_admin_id BIGINT UNSIGNED NULL AFTER handled_by',
  'SELECT 1');
PREPARE migrate_stmt FROM @ddl; EXECUTE migrate_stmt; DEALLOCATE PREPARE migrate_stmt;

SET @col_exists := (SELECT COUNT(*) FROM information_schema.COLUMNS
  WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'feature_policy_configs' AND COLUMN_NAME = 'updated_by_admin_id');
SET @ddl := IF(@col_exists = 0,
  'ALTER TABLE feature_policy_configs ADD COLUMN updated_by_admin_id BIGINT UNSIGNED NULL AFTER updated_by',
  'SELECT 1');
PREPARE migrate_stmt FROM @ddl; EXECUTE migrate_stmt; DEALLOCATE PREPARE migrate_stmt;

SET @col_exists := (SELECT COUNT(*) FROM information_schema.COLUMNS
  WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'feature_policy_audit_logs' AND COLUMN_NAME = 'actor_admin_id');
SET @ddl := IF(@col_exists = 0,
  'ALTER TABLE feature_policy_audit_logs ADD COLUMN actor_admin_id BIGINT UNSIGNED NULL AFTER actor_id',
  'SELECT 1');
PREPARE migrate_stmt FROM @ddl; EXECUTE migrate_stmt; DEALLOCATE PREPARE migrate_stmt;

SET @col_exists := (SELECT COUNT(*) FROM information_schema.COLUMNS
  WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'share_settings' AND COLUMN_NAME = 'updated_by_admin_id');
SET @ddl := IF(@col_exists = 0,
  'ALTER TABLE share_settings ADD COLUMN updated_by_admin_id BIGINT UNSIGNED NULL AFTER updated_by',
  'SELECT 1');
PREPARE migrate_stmt FROM @ddl; EXECUTE migrate_stmt; DEALLOCATE PREPARE migrate_stmt;

SET @col_exists := (SELECT COUNT(*) FROM information_schema.COLUMNS
  WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'share_settings_audit_logs' AND COLUMN_NAME = 'actor_admin_id');
SET @ddl := IF(@col_exists = 0,
  'ALTER TABLE share_settings_audit_logs ADD COLUMN actor_admin_id BIGINT UNSIGNED NULL AFTER actor_id',
  'SELECT 1');
PREPARE migrate_stmt FROM @ddl; EXECUTE migrate_stmt; DEALLOCATE PREPARE migrate_stmt;

SET @col_exists := (SELECT COUNT(*) FROM information_schema.COLUMNS
  WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'notification_channel_settings' AND COLUMN_NAME = 'updated_by_admin_id');
SET @ddl := IF(@col_exists = 0,
  'ALTER TABLE notification_channel_settings ADD COLUMN updated_by_admin_id BIGINT UNSIGNED NULL AFTER updated_by',
  'SELECT 1');
PREPARE migrate_stmt FROM @ddl; EXECUTE migrate_stmt; DEALLOCATE PREPARE migrate_stmt;

SET @col_exists := (SELECT COUNT(*) FROM information_schema.COLUMNS
  WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'delivery_order_admin_actions' AND COLUMN_NAME = 'actor_admin_id');
SET @ddl := IF(@col_exists = 0,
  'ALTER TABLE delivery_order_admin_actions ADD COLUMN actor_admin_id BIGINT UNSIGNED NULL AFTER actor_user_id',
  'SELECT 1');
PREPARE migrate_stmt FROM @ddl; EXECUTE migrate_stmt; DEALLOCATE PREPARE migrate_stmt;

SET @col_exists := (SELECT COUNT(*) FROM information_schema.COLUMNS
  WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'delivery_rider_admin_actions' AND COLUMN_NAME = 'actor_admin_id');
SET @ddl := IF(@col_exists = 0,
  'ALTER TABLE delivery_rider_admin_actions ADD COLUMN actor_admin_id BIGINT UNSIGNED NULL AFTER actor_user_id',
  'SELECT 1');
PREPARE migrate_stmt FROM @ddl; EXECUTE migrate_stmt; DEALLOCATE PREPARE migrate_stmt;

SET @col_exists := (SELECT COUNT(*) FROM information_schema.COLUMNS
  WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'delivery_rider_payout_records' AND COLUMN_NAME = 'operator_admin_id');
SET @ddl := IF(@col_exists = 0,
  'ALTER TABLE delivery_rider_payout_records ADD COLUMN operator_admin_id BIGINT UNSIGNED NULL AFTER operator_user_id',
  'SELECT 1');
PREPARE migrate_stmt FROM @ddl; EXECUTE migrate_stmt; DEALLOCATE PREPARE migrate_stmt;

SET @col_exists := (SELECT COUNT(*) FROM information_schema.COLUMNS
  WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'delivery_rider_payout_events' AND COLUMN_NAME = 'operator_admin_id');
SET @ddl := IF(@col_exists = 0,
  'ALTER TABLE delivery_rider_payout_events ADD COLUMN operator_admin_id BIGINT UNSIGNED NULL AFTER operator_user_id',
  'SELECT 1');
PREPARE migrate_stmt FROM @ddl; EXECUTE migrate_stmt; DEALLOCATE PREPARE migrate_stmt;

SET @col_exists := (SELECT COUNT(*) FROM information_schema.COLUMNS
  WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'delivery_rider_withdrawal_events' AND COLUMN_NAME = 'operator_admin_id');
SET @ddl := IF(@col_exists = 0,
  'ALTER TABLE delivery_rider_withdrawal_events ADD COLUMN operator_admin_id BIGINT UNSIGNED NULL AFTER operator_user_id',
  'SELECT 1');
PREPARE migrate_stmt FROM @ddl; EXECUTE migrate_stmt; DEALLOCATE PREPARE migrate_stmt;

SET @col_exists := (SELECT COUNT(*) FROM information_schema.COLUMNS
  WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'delivery_rider_withdrawal_claims' AND COLUMN_NAME = 'operator_admin_id');
SET @ddl := IF(@col_exists = 0,
  'ALTER TABLE delivery_rider_withdrawal_claims ADD COLUMN operator_admin_id BIGINT UNSIGNED NULL AFTER operator_user_id',
  'SELECT 1');
PREPARE migrate_stmt FROM @ddl; EXECUTE migrate_stmt; DEALLOCATE PREPARE migrate_stmt;

SET @col_exists := (SELECT COUNT(*) FROM information_schema.COLUMNS
  WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'identity_applications' AND COLUMN_NAME = 'reviewer_admin_id');
SET @ddl := IF(@col_exists = 0,
  'ALTER TABLE identity_applications ADD COLUMN reviewer_admin_id BIGINT UNSIGNED NULL AFTER reviewer_user_id',
  'SELECT 1');
PREPARE migrate_stmt FROM @ddl; EXECUTE migrate_stmt; DEALLOCATE PREPARE migrate_stmt;

SET @col_exists := (SELECT COUNT(*) FROM information_schema.COLUMNS
  WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'identity_application_events' AND COLUMN_NAME = 'actor_admin_id');
SET @ddl := IF(@col_exists = 0,
  'ALTER TABLE identity_application_events ADD COLUMN actor_admin_id BIGINT UNSIGNED NULL AFTER actor_user_id',
  'SELECT 1');
PREPARE migrate_stmt FROM @ddl; EXECUTE migrate_stmt; DEALLOCATE PREPARE migrate_stmt;

SET @col_exists := (SELECT COUNT(*) FROM information_schema.COLUMNS
  WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'student_verifications' AND COLUMN_NAME = 'reviewer_admin_id');
SET @ddl := IF(@col_exists = 0,
  'ALTER TABLE student_verifications ADD COLUMN reviewer_admin_id BIGINT UNSIGNED NULL AFTER reviewer_user_id',
  'SELECT 1');
PREPARE migrate_stmt FROM @ddl; EXECUTE migrate_stmt; DEALLOCATE PREPARE migrate_stmt;

SET @col_exists := (SELECT COUNT(*) FROM information_schema.COLUMNS
  WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'student_verification_events' AND COLUMN_NAME = 'actor_admin_id');
SET @ddl := IF(@col_exists = 0,
  'ALTER TABLE student_verification_events ADD COLUMN actor_admin_id BIGINT UNSIGNED NULL AFTER actor_user_id',
  'SELECT 1');
PREPARE migrate_stmt FROM @ddl; EXECUTE migrate_stmt; DEALLOCATE PREPARE migrate_stmt;

SET @col_exists := (SELECT COUNT(*) FROM information_schema.COLUMNS
  WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'feature_suggestion_comments' AND COLUMN_NAME = 'admin_account_id');
SET @ddl := IF(@col_exists = 0,
  'ALTER TABLE feature_suggestion_comments ADD COLUMN admin_account_id BIGINT UNSIGNED NULL AFTER user_id',
  'SELECT 1');
PREPARE migrate_stmt FROM @ddl; EXECUTE migrate_stmt; DEALLOCATE PREPARE migrate_stmt;

SET @nullable := (SELECT is_nullable FROM information_schema.COLUMNS
  WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'feature_suggestion_comments' AND COLUMN_NAME = 'user_id');
SET @ddl := IF(@nullable = 'NO',
  'ALTER TABLE feature_suggestion_comments MODIFY COLUMN user_id BIGINT UNSIGNED NULL',
  'SELECT 1');
PREPARE migrate_stmt FROM @ddl; EXECUTE migrate_stmt; DEALLOCATE PREPARE migrate_stmt;

-- ---------- 6. 历史值回填（按 legacy_user_id 映射到管理员账号 id） ----------
UPDATE review_tasks t JOIN admin_credentials c ON c.legacy_user_id = t.operator_id
SET t.operator_admin_id = c.id WHERE t.operator_admin_id IS NULL AND t.operator_id > 0;

UPDATE user_feedbacks f JOIN admin_credentials c ON c.legacy_user_id = f.handled_by
SET f.handled_by_admin_id = c.id WHERE f.handled_by_admin_id IS NULL AND f.handled_by > 0;

UPDATE feature_policy_configs p JOIN admin_credentials c ON c.legacy_user_id = p.updated_by
SET p.updated_by_admin_id = c.id WHERE p.updated_by_admin_id IS NULL AND p.updated_by > 0;

UPDATE feature_policy_audit_logs a JOIN admin_credentials c ON c.legacy_user_id = a.actor_id
SET a.actor_admin_id = c.id WHERE a.actor_admin_id IS NULL AND a.actor_id > 0;

UPDATE share_settings s JOIN admin_credentials c ON c.legacy_user_id = s.updated_by
SET s.updated_by_admin_id = c.id WHERE s.updated_by_admin_id IS NULL AND s.updated_by > 0;

UPDATE share_settings_audit_logs a JOIN admin_credentials c ON c.legacy_user_id = a.actor_id
SET a.actor_admin_id = c.id WHERE a.actor_admin_id IS NULL AND a.actor_id > 0;

UPDATE notification_channel_settings n JOIN admin_credentials c ON c.legacy_user_id = n.updated_by
SET n.updated_by_admin_id = c.id WHERE n.updated_by_admin_id IS NULL AND n.updated_by IS NOT NULL;

UPDATE delivery_order_admin_actions a JOIN admin_credentials c ON c.legacy_user_id = a.actor_user_id
SET a.actor_admin_id = c.id WHERE a.actor_admin_id IS NULL AND a.actor_user_id > 0;

UPDATE delivery_rider_admin_actions a JOIN admin_credentials c ON c.legacy_user_id = a.actor_user_id
SET a.actor_admin_id = c.id WHERE a.actor_admin_id IS NULL AND a.actor_user_id > 0;

UPDATE delivery_rider_payout_records r JOIN admin_credentials c ON c.legacy_user_id = r.operator_user_id
SET r.operator_admin_id = c.id WHERE r.operator_admin_id IS NULL AND r.operator_user_id > 0;

UPDATE delivery_rider_payout_events e JOIN admin_credentials c ON c.legacy_user_id = e.operator_user_id
SET e.operator_admin_id = c.id WHERE e.operator_admin_id IS NULL AND e.operator_user_id > 0;

UPDATE delivery_rider_withdrawal_events e JOIN admin_credentials c ON c.legacy_user_id = e.operator_user_id
SET e.operator_admin_id = c.id WHERE e.operator_admin_id IS NULL AND e.operator_user_id > 0;

UPDATE delivery_rider_withdrawal_claims cl JOIN admin_credentials c ON c.legacy_user_id = cl.operator_user_id
SET cl.operator_admin_id = c.id WHERE cl.operator_admin_id IS NULL AND cl.operator_user_id > 0;

UPDATE identity_applications a JOIN admin_credentials c ON c.legacy_user_id = a.reviewer_user_id
SET a.reviewer_admin_id = c.id WHERE a.reviewer_admin_id IS NULL AND a.reviewer_user_id IS NOT NULL;

-- 事件表混存申请人与审核人：只回填审核结论事件，申请人事件保持 legacy 语义。
UPDATE identity_application_events e JOIN admin_credentials c ON c.legacy_user_id = e.actor_user_id
SET e.actor_admin_id = c.id
WHERE e.actor_admin_id IS NULL AND e.event_type IN ('approved','rejected');

UPDATE student_verifications v JOIN admin_credentials c ON c.legacy_user_id = v.reviewer_user_id
SET v.reviewer_admin_id = c.id WHERE v.reviewer_admin_id IS NULL AND v.reviewer_user_id IS NOT NULL;

UPDATE student_verification_events e JOIN admin_credentials c ON c.legacy_user_id = e.actor_user_id
SET e.actor_admin_id = c.id
WHERE e.actor_admin_id IS NULL AND e.event_type IN ('approved','rejected');

UPDATE feature_suggestion_comments fc JOIN admin_credentials c ON c.legacy_user_id = fc.user_id
SET fc.admin_account_id = c.id WHERE fc.admin_account_id IS NULL AND fc.is_maintainer = 1;
