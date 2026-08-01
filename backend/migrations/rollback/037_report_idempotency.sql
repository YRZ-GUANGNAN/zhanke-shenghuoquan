-- Remove report idempotency support introduced by migration 037.
DELIMITER $$
DROP PROCEDURE IF EXISTS rollback_report_idempotency$$
CREATE PROCEDURE rollback_report_idempotency()
SQL SECURITY INVOKER
BEGIN
  DECLARE lock_acquired INT DEFAULT 0;

  DECLARE EXIT HANDLER FOR SQLEXCEPTION
  BEGIN
    IF lock_acquired = 1 THEN
      DO RELEASE_LOCK('campus_circle:rollback:037');
    END IF;
    RESIGNAL;
  END;

  SELECT GET_LOCK('campus_circle:rollback:037', 10) INTO lock_acquired;
  IF lock_acquired IS NULL OR lock_acquired <> 1 THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT = 'rollback 037 lock unavailable';
  END IF;

  IF EXISTS (
    SELECT 1 FROM information_schema.statistics
    WHERE table_schema=DATABASE() AND table_name='reports'
      AND index_name='idx_reports_reporter_request'
  ) THEN
    ALTER TABLE reports DROP INDEX idx_reports_reporter_request;
  END IF;

  IF EXISTS (
    SELECT 1 FROM information_schema.columns
    WHERE table_schema=DATABASE() AND table_name='reports' AND column_name='client_request_id'
  ) THEN
    ALTER TABLE reports DROP COLUMN client_request_id;
  END IF;

  DO RELEASE_LOCK('campus_circle:rollback:037');
  SET lock_acquired = 0;
END$$
CALL rollback_report_idempotency()$$
DROP PROCEDURE rollback_report_idempotency$$
DELIMITER ;
