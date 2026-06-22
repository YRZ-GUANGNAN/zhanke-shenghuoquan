-- P0-1: invalidate legacy role-snapshot tokens and remove mock administrative identities.
DELIMITER $$
CREATE PROCEDURE migrate_auth_hardening()
BEGIN
  IF NOT EXISTS (
    SELECT 1 FROM information_schema.columns
    WHERE table_schema = DATABASE() AND table_name = 'users' AND column_name = 'token_version'
  ) THEN
    ALTER TABLE users ADD COLUMN token_version BIGINT UNSIGNED NOT NULL DEFAULT 1 AFTER status;
  END IF;

  UPDATE users SET token_version = 1 WHERE token_version = 0;
  UPDATE users
    SET role = 'user', token_version = token_version + 1
    WHERE open_id LIKE 'mock_openid_%' AND role IN ('admin', 'superadmin');
END$$
CALL migrate_auth_hardening()$$
DROP PROCEDURE migrate_auth_hardening$$
DELIMITER ;
