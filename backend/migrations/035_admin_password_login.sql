-- Independent administrator password credentials, web sessions and audit trail.
-- Forward-only: authentication audit facts are retained across application rollback.

DELIMITER $$
DROP PROCEDURE IF EXISTS migrate_admin_password_login$$
CREATE PROCEDURE migrate_admin_password_login()
SQL SECURITY INVOKER
BEGIN
  DECLARE lock_acquired INT DEFAULT 0;

  DECLARE EXIT HANDLER FOR SQLEXCEPTION
  BEGIN
    IF lock_acquired = 1 THEN
      DO RELEASE_LOCK('campus_circle:migration:035');
    END IF;
    RESIGNAL;
  END;

  SELECT GET_LOCK('campus_circle:migration:035', 10) INTO lock_acquired;
  IF lock_acquired IS NULL OR lock_acquired <> 1 THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT = 'migration 035 lock unavailable';
  END IF;

  IF NOT EXISTS (
    SELECT 1 FROM information_schema.tables
    WHERE table_schema=DATABASE() AND table_name='users'
  ) THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT = 'migration 035 users table missing';
  END IF;
  IF (SELECT COUNT(*) FROM information_schema.columns
      WHERE table_schema=DATABASE() AND table_name='users' AND (
        (column_name='id' AND column_type='bigint unsigned' AND is_nullable='NO') OR
        (column_name='role' AND data_type='varchar' AND is_nullable='NO') OR
        (column_name='status' AND data_type='varchar' AND is_nullable='NO')
      )) <> 3 THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT = 'migration 035 users schema mismatch';
  END IF;

  IF NOT EXISTS (
    SELECT 1 FROM information_schema.tables
    WHERE table_schema=DATABASE() AND table_name='admin_credentials'
  ) THEN
    CREATE TABLE admin_credentials (
      id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
      username VARCHAR(64) NOT NULL,
      user_id BIGINT UNSIGNED NOT NULL,
      password_hash VARCHAR(255) NOT NULL,
      failed_attempts SMALLINT UNSIGNED NOT NULL DEFAULT 0,
      locked_until DATETIME(3) NULL,
      password_changed_at DATETIME(3) NOT NULL,
      row_version BIGINT UNSIGNED NOT NULL DEFAULT 1,
      created_at DATETIME(3) NOT NULL,
      updated_at DATETIME(3) NOT NULL,
      PRIMARY KEY (id),
      UNIQUE KEY uq_admin_credentials_username (username),
      UNIQUE KEY uq_admin_credentials_user_id (user_id),
      CONSTRAINT fk_admin_credentials_user
        FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE RESTRICT
    ) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
  END IF;

  IF NOT EXISTS (
    SELECT 1 FROM information_schema.tables
    WHERE table_schema=DATABASE() AND table_name='admin_web_sessions'
  ) THEN
    CREATE TABLE admin_web_sessions (
      id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
      credential_id BIGINT UNSIGNED NOT NULL,
      user_id BIGINT UNSIGNED NOT NULL,
      token_hash BINARY(32) NOT NULL,
      csrf_hash BINARY(32) NOT NULL,
      idle_expires_at DATETIME(3) NOT NULL,
      absolute_expires_at DATETIME(3) NOT NULL,
      last_seen_at DATETIME(3) NOT NULL,
      revoked_at DATETIME(3) NULL,
      ip_hash BINARY(32) NULL,
      user_agent_hash BINARY(32) NULL,
      created_at DATETIME(3) NOT NULL,
      updated_at DATETIME(3) NOT NULL,
      PRIMARY KEY (id),
      UNIQUE KEY uq_admin_web_sessions_token_hash (token_hash),
      KEY idx_admin_web_sessions_credential_active (credential_id, revoked_at, absolute_expires_at),
      KEY idx_admin_web_sessions_user_active (user_id, revoked_at, absolute_expires_at),
      CONSTRAINT fk_admin_web_sessions_credential
        FOREIGN KEY (credential_id) REFERENCES admin_credentials(id) ON DELETE RESTRICT,
      CONSTRAINT fk_admin_web_sessions_user
        FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE RESTRICT
    ) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
  END IF;

  IF NOT EXISTS (
    SELECT 1 FROM information_schema.tables
    WHERE table_schema=DATABASE() AND table_name='admin_auth_audit_logs'
  ) THEN
    CREATE TABLE admin_auth_audit_logs (
      id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
      credential_id BIGINT UNSIGNED NULL,
      user_id BIGINT UNSIGNED NULL,
      username VARCHAR(64) NOT NULL,
      event_type VARCHAR(32) NOT NULL,
      outcome VARCHAR(16) NOT NULL,
      ip_hash BINARY(32) NULL,
      user_agent_hash BINARY(32) NULL,
      detail VARCHAR(255) NOT NULL DEFAULT '',
      created_at DATETIME(3) NOT NULL,
      PRIMARY KEY (id),
      KEY idx_admin_auth_audit_username_created (username, created_at, id),
      KEY idx_admin_auth_audit_user_created (user_id, created_at, id),
      CONSTRAINT chk_admin_auth_audit_event CHECK (
        event_type IN ('login','logout','password_changed','credential_bootstrap','credential_reset','account_locked','sessions_revoked')
      ),
      CONSTRAINT chk_admin_auth_audit_outcome CHECK (outcome IN ('success','failure','blocked'))
    ) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
  END IF;

  IF (SELECT COUNT(*) FROM information_schema.tables
      WHERE table_schema=DATABASE()
        AND table_name IN ('admin_credentials','admin_web_sessions','admin_auth_audit_logs')) <> 3 THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT = 'migration 035 authentication tables incomplete';
  END IF;

  DO RELEASE_LOCK('campus_circle:migration:035');
  SET lock_acquired = 0;
END$$
CALL migrate_admin_password_login()$$
DROP PROCEDURE migrate_admin_password_login$$
DELIMITER ;
