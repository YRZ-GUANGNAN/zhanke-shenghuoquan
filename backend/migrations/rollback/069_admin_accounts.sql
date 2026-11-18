-- 回滚 069：恢复“管理员账号必须绑定 users 行”的旧模型。
-- 注意：新模型下创建的管理员账号没有 user_id，无法在旧模型中表示，将随回滚删除；
-- 旧模型创建的历史账号（user_id 非空）保留原绑定与密码哈希。

-- ---------- 1. 审计事件必须收敛回 058 的集合 ----------
SET @new_events := (SELECT COUNT(*) FROM admin_auth_audit_logs
  WHERE event_type IN ('account_created','account_updated','account_disabled','account_enabled','admin_role_changed','password_reset'));
SET @ddl := IF(@new_events > 0,
  'SIGNAL SQLSTATE ''45000'' SET MESSAGE_TEXT = ''rollback 069: new admin audit events exist; export or delete them first''',
  'SELECT 1');
PREPARE rollback_stmt FROM @ddl; EXECUTE rollback_stmt; DEALLOCATE PREPARE rollback_stmt;

SET @chk_exists := (SELECT COUNT(*) FROM information_schema.TABLE_CONSTRAINTS
  WHERE CONSTRAINT_SCHEMA = DATABASE() AND TABLE_NAME = 'admin_auth_audit_logs' AND CONSTRAINT_NAME = 'chk_admin_auth_audit_event');
SET @ddl := IF(@chk_exists = 1,
  'ALTER TABLE admin_auth_audit_logs DROP CHECK chk_admin_auth_audit_event',
  'SELECT 1');
PREPARE rollback_stmt FROM @ddl; EXECUTE rollback_stmt; DEALLOCATE PREPARE rollback_stmt;

ALTER TABLE admin_auth_audit_logs ADD CONSTRAINT chk_admin_auth_audit_event CHECK (
  event_type IN (
    'login','logout','password_changed','credential_bootstrap','credential_reset',
    'credential_set','credential_revoke','account_locked','sessions_revoked'
  )
);

SET @col_exists := (SELECT COUNT(*) FROM information_schema.COLUMNS
  WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'admin_auth_audit_logs' AND COLUMN_NAME = 'actor_credential_id');
SET @ddl := IF(@col_exists = 1,
  'ALTER TABLE admin_auth_audit_logs DROP COLUMN actor_credential_id',
  'SELECT 1');
PREPARE rollback_stmt FROM @ddl; EXECUTE rollback_stmt; DEALLOCATE PREPARE rollback_stmt;

-- ---------- 2. 操作人列回滚 ----------
SET @col_exists := (SELECT COUNT(*) FROM information_schema.COLUMNS
  WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'review_tasks' AND COLUMN_NAME = 'operator_admin_id');
SET @ddl := IF(@col_exists = 1, 'ALTER TABLE review_tasks DROP COLUMN operator_admin_id', 'SELECT 1');
PREPARE rollback_stmt FROM @ddl; EXECUTE rollback_stmt; DEALLOCATE PREPARE rollback_stmt;

SET @col_exists := (SELECT COUNT(*) FROM information_schema.COLUMNS
  WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'user_feedbacks' AND COLUMN_NAME = 'handled_by_admin_id');
SET @ddl := IF(@col_exists = 1, 'ALTER TABLE user_feedbacks DROP COLUMN handled_by_admin_id', 'SELECT 1');
PREPARE rollback_stmt FROM @ddl; EXECUTE rollback_stmt; DEALLOCATE PREPARE rollback_stmt;

SET @col_exists := (SELECT COUNT(*) FROM information_schema.COLUMNS
  WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'feature_policy_configs' AND COLUMN_NAME = 'updated_by_admin_id');
SET @ddl := IF(@col_exists = 1, 'ALTER TABLE feature_policy_configs DROP COLUMN updated_by_admin_id', 'SELECT 1');
PREPARE rollback_stmt FROM @ddl; EXECUTE rollback_stmt; DEALLOCATE PREPARE rollback_stmt;

SET @col_exists := (SELECT COUNT(*) FROM information_schema.COLUMNS
  WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'feature_policy_audit_logs' AND COLUMN_NAME = 'actor_admin_id');
SET @ddl := IF(@col_exists = 1, 'ALTER TABLE feature_policy_audit_logs DROP COLUMN actor_admin_id', 'SELECT 1');
PREPARE rollback_stmt FROM @ddl; EXECUTE rollback_stmt; DEALLOCATE PREPARE rollback_stmt;

SET @col_exists := (SELECT COUNT(*) FROM information_schema.COLUMNS
  WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'share_settings' AND COLUMN_NAME = 'updated_by_admin_id');
SET @ddl := IF(@col_exists = 1, 'ALTER TABLE share_settings DROP COLUMN updated_by_admin_id', 'SELECT 1');
PREPARE rollback_stmt FROM @ddl; EXECUTE rollback_stmt; DEALLOCATE PREPARE rollback_stmt;

SET @col_exists := (SELECT COUNT(*) FROM information_schema.COLUMNS
  WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'share_settings_audit_logs' AND COLUMN_NAME = 'actor_admin_id');
SET @ddl := IF(@col_exists = 1, 'ALTER TABLE share_settings_audit_logs DROP COLUMN actor_admin_id', 'SELECT 1');
PREPARE rollback_stmt FROM @ddl; EXECUTE rollback_stmt; DEALLOCATE PREPARE rollback_stmt;

SET @col_exists := (SELECT COUNT(*) FROM information_schema.COLUMNS
  WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'notification_channel_settings' AND COLUMN_NAME = 'updated_by_admin_id');
SET @ddl := IF(@col_exists = 1, 'ALTER TABLE notification_channel_settings DROP COLUMN updated_by_admin_id', 'SELECT 1');
PREPARE rollback_stmt FROM @ddl; EXECUTE rollback_stmt; DEALLOCATE PREPARE rollback_stmt;

SET @col_exists := (SELECT COUNT(*) FROM information_schema.COLUMNS
  WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'delivery_order_admin_actions' AND COLUMN_NAME = 'actor_admin_id');
SET @ddl := IF(@col_exists = 1, 'ALTER TABLE delivery_order_admin_actions DROP COLUMN actor_admin_id', 'SELECT 1');
PREPARE rollback_stmt FROM @ddl; EXECUTE rollback_stmt; DEALLOCATE PREPARE rollback_stmt;

SET @col_exists := (SELECT COUNT(*) FROM information_schema.COLUMNS
  WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'delivery_rider_admin_actions' AND COLUMN_NAME = 'actor_admin_id');
SET @ddl := IF(@col_exists = 1, 'ALTER TABLE delivery_rider_admin_actions DROP COLUMN actor_admin_id', 'SELECT 1');
PREPARE rollback_stmt FROM @ddl; EXECUTE rollback_stmt; DEALLOCATE PREPARE rollback_stmt;

SET @col_exists := (SELECT COUNT(*) FROM information_schema.COLUMNS
  WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'delivery_rider_payout_records' AND COLUMN_NAME = 'operator_admin_id');
SET @ddl := IF(@col_exists = 1, 'ALTER TABLE delivery_rider_payout_records DROP COLUMN operator_admin_id', 'SELECT 1');
PREPARE rollback_stmt FROM @ddl; EXECUTE rollback_stmt; DEALLOCATE PREPARE rollback_stmt;

SET @col_exists := (SELECT COUNT(*) FROM information_schema.COLUMNS
  WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'delivery_rider_payout_events' AND COLUMN_NAME = 'operator_admin_id');
SET @ddl := IF(@col_exists = 1, 'ALTER TABLE delivery_rider_payout_events DROP COLUMN operator_admin_id', 'SELECT 1');
PREPARE rollback_stmt FROM @ddl; EXECUTE rollback_stmt; DEALLOCATE PREPARE rollback_stmt;

SET @col_exists := (SELECT COUNT(*) FROM information_schema.COLUMNS
  WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'delivery_rider_withdrawal_events' AND COLUMN_NAME = 'operator_admin_id');
SET @ddl := IF(@col_exists = 1, 'ALTER TABLE delivery_rider_withdrawal_events DROP COLUMN operator_admin_id', 'SELECT 1');
PREPARE rollback_stmt FROM @ddl; EXECUTE rollback_stmt; DEALLOCATE PREPARE rollback_stmt;

SET @col_exists := (SELECT COUNT(*) FROM information_schema.COLUMNS
  WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'delivery_rider_withdrawal_claims' AND COLUMN_NAME = 'operator_admin_id');
SET @ddl := IF(@col_exists = 1, 'ALTER TABLE delivery_rider_withdrawal_claims DROP COLUMN operator_admin_id', 'SELECT 1');
PREPARE rollback_stmt FROM @ddl; EXECUTE rollback_stmt; DEALLOCATE PREPARE rollback_stmt;

SET @col_exists := (SELECT COUNT(*) FROM information_schema.COLUMNS
  WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'identity_applications' AND COLUMN_NAME = 'reviewer_admin_id');
SET @ddl := IF(@col_exists = 1, 'ALTER TABLE identity_applications DROP COLUMN reviewer_admin_id', 'SELECT 1');
PREPARE rollback_stmt FROM @ddl; EXECUTE rollback_stmt; DEALLOCATE PREPARE rollback_stmt;

SET @col_exists := (SELECT COUNT(*) FROM information_schema.COLUMNS
  WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'identity_application_events' AND COLUMN_NAME = 'actor_admin_id');
SET @ddl := IF(@col_exists = 1, 'ALTER TABLE identity_application_events DROP COLUMN actor_admin_id', 'SELECT 1');
PREPARE rollback_stmt FROM @ddl; EXECUTE rollback_stmt; DEALLOCATE PREPARE rollback_stmt;

SET @col_exists := (SELECT COUNT(*) FROM information_schema.COLUMNS
  WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'student_verifications' AND COLUMN_NAME = 'reviewer_admin_id');
SET @ddl := IF(@col_exists = 1, 'ALTER TABLE student_verifications DROP COLUMN reviewer_admin_id', 'SELECT 1');
PREPARE rollback_stmt FROM @ddl; EXECUTE rollback_stmt; DEALLOCATE PREPARE rollback_stmt;

SET @col_exists := (SELECT COUNT(*) FROM information_schema.COLUMNS
  WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'student_verification_events' AND COLUMN_NAME = 'actor_admin_id');
SET @ddl := IF(@col_exists = 1, 'ALTER TABLE student_verification_events DROP COLUMN actor_admin_id', 'SELECT 1');
PREPARE rollback_stmt FROM @ddl; EXECUTE rollback_stmt; DEALLOCATE PREPARE rollback_stmt;

-- 独立管理员回复在旧模型中无 user_id，可保留 is_maintainer 语义但必须补一个占位用户不可行，直接删除这类回复。
DELETE FROM feature_suggestion_comments WHERE user_id IS NULL;

SET @col_exists := (SELECT COUNT(*) FROM information_schema.COLUMNS
  WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'feature_suggestion_comments' AND COLUMN_NAME = 'admin_account_id');
SET @ddl := IF(@col_exists = 1, 'ALTER TABLE feature_suggestion_comments DROP COLUMN admin_account_id', 'SELECT 1');
PREPARE rollback_stmt FROM @ddl; EXECUTE rollback_stmt; DEALLOCATE PREPARE rollback_stmt;

SET @nullable := (SELECT is_nullable FROM information_schema.COLUMNS
  WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'feature_suggestion_comments' AND COLUMN_NAME = 'user_id');
SET @ddl := IF(@nullable = 'YES',
  'ALTER TABLE feature_suggestion_comments MODIFY COLUMN user_id BIGINT UNSIGNED NOT NULL',
  'SELECT 1');
PREPARE rollback_stmt FROM @ddl; EXECUTE rollback_stmt; DEALLOCATE PREPARE rollback_stmt;

-- ---------- 3. 会话与账号恢复旧约束 ----------
DELETE FROM admin_web_sessions WHERE user_id IS NULL;

SET @nullable := (SELECT is_nullable FROM information_schema.COLUMNS
  WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'admin_web_sessions' AND COLUMN_NAME = 'user_id');
SET @ddl := IF(@nullable = 'YES',
  'ALTER TABLE admin_web_sessions MODIFY COLUMN user_id BIGINT UNSIGNED NOT NULL',
  'SELECT 1');
PREPARE rollback_stmt FROM @ddl; EXECUTE rollback_stmt; DEALLOCATE PREPARE rollback_stmt;

-- 新模型中创建的独立账号（无 user_id）在旧模型无法表示，随回滚删除。
DELETE FROM admin_credentials WHERE user_id IS NULL;

SET @nullable := (SELECT is_nullable FROM information_schema.COLUMNS
  WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'admin_credentials' AND COLUMN_NAME = 'user_id');
SET @ddl := IF(@nullable = 'YES',
  'ALTER TABLE admin_credentials MODIFY COLUMN user_id BIGINT UNSIGNED NOT NULL',
  'SELECT 1');
PREPARE rollback_stmt FROM @ddl; EXECUTE rollback_stmt; DEALLOCATE PREPARE rollback_stmt;

SET @chk_exists := (SELECT COUNT(*) FROM information_schema.TABLE_CONSTRAINTS
  WHERE CONSTRAINT_SCHEMA = DATABASE() AND TABLE_NAME = 'admin_credentials' AND CONSTRAINT_NAME = 'chk_admin_credentials_status');
SET @ddl := IF(@chk_exists = 1,
  'ALTER TABLE admin_credentials DROP CHECK chk_admin_credentials_status',
  'SELECT 1');
PREPARE rollback_stmt FROM @ddl; EXECUTE rollback_stmt; DEALLOCATE PREPARE rollback_stmt;

SET @chk_exists := (SELECT COUNT(*) FROM information_schema.TABLE_CONSTRAINTS
  WHERE CONSTRAINT_SCHEMA = DATABASE() AND TABLE_NAME = 'admin_credentials' AND CONSTRAINT_NAME = 'chk_admin_credentials_role');
SET @ddl := IF(@chk_exists = 1,
  'ALTER TABLE admin_credentials DROP CHECK chk_admin_credentials_role',
  'SELECT 1');
PREPARE rollback_stmt FROM @ddl; EXECUTE rollback_stmt; DEALLOCATE PREPARE rollback_stmt;

SET @idx_exists := (SELECT COUNT(*) FROM information_schema.STATISTICS
  WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'admin_credentials' AND INDEX_NAME = 'uq_admin_credentials_legacy_user');
SET @ddl := IF(@idx_exists = 1,
  'ALTER TABLE admin_credentials DROP INDEX uq_admin_credentials_legacy_user',
  'SELECT 1');
PREPARE rollback_stmt FROM @ddl; EXECUTE rollback_stmt; DEALLOCATE PREPARE rollback_stmt;

SET @col_exists := (SELECT COUNT(*) FROM information_schema.COLUMNS
  WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'admin_credentials' AND COLUMN_NAME = 'legacy_user_id');
SET @ddl := IF(@col_exists = 1, 'ALTER TABLE admin_credentials DROP COLUMN legacy_user_id', 'SELECT 1');
PREPARE rollback_stmt FROM @ddl; EXECUTE rollback_stmt; DEALLOCATE PREPARE rollback_stmt;

SET @col_exists := (SELECT COUNT(*) FROM information_schema.COLUMNS
  WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'admin_credentials' AND COLUMN_NAME = 'display_name');
SET @ddl := IF(@col_exists = 1, 'ALTER TABLE admin_credentials DROP COLUMN display_name', 'SELECT 1');
PREPARE rollback_stmt FROM @ddl; EXECUTE rollback_stmt; DEALLOCATE PREPARE rollback_stmt;

SET @col_exists := (SELECT COUNT(*) FROM information_schema.COLUMNS
  WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'admin_credentials' AND COLUMN_NAME = 'status');
SET @ddl := IF(@col_exists = 1, 'ALTER TABLE admin_credentials DROP COLUMN status', 'SELECT 1');
PREPARE rollback_stmt FROM @ddl; EXECUTE rollback_stmt; DEALLOCATE PREPARE rollback_stmt;

SET @col_exists := (SELECT COUNT(*) FROM information_schema.COLUMNS
  WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'admin_credentials' AND COLUMN_NAME = 'role');
SET @ddl := IF(@col_exists = 1, 'ALTER TABLE admin_credentials DROP COLUMN role', 'SELECT 1');
PREPARE rollback_stmt FROM @ddl; EXECUTE rollback_stmt; DEALLOCATE PREPARE rollback_stmt;
