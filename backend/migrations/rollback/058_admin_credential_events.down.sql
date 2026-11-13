-- Refuse rollback while audit rows rely on the multi-administrator event types.
DELIMITER $$
DROP PROCEDURE IF EXISTS rollback_058_admin_credential_events$$
CREATE PROCEDURE rollback_058_admin_credential_events()
SQL SECURITY INVOKER
BEGIN
  DECLARE row_count BIGINT DEFAULT 0;
  SELECT COUNT(*) INTO row_count FROM admin_auth_audit_logs
    WHERE event_type IN ('credential_set','credential_revoke');
  IF row_count > 0 THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT = 'administrator credential audit rows must be preserved';
  END IF;
  ALTER TABLE admin_auth_audit_logs DROP CHECK chk_admin_auth_audit_event;
  ALTER TABLE admin_auth_audit_logs ADD CONSTRAINT chk_admin_auth_audit_event CHECK (
    event_type IN (
      'login','logout','password_changed','credential_bootstrap','credential_reset',
      'account_locked','sessions_revoked'
    )
  );
END$$
DELIMITER ;
CALL rollback_058_admin_credential_events();
DROP PROCEDURE rollback_058_admin_credential_events;
