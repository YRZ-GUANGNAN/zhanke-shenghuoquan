-- Make report creation replay-safe without changing historical rows.
DELIMITER $$
DROP PROCEDURE IF EXISTS migrate_report_idempotency$$
CREATE PROCEDURE migrate_report_idempotency()
SQL SECURITY INVOKER
BEGIN
  DECLARE lock_acquired INT DEFAULT 0;
  DECLARE index_columns TEXT DEFAULT NULL;
  DECLARE index_non_unique INT DEFAULT NULL;

  DECLARE EXIT HANDLER FOR SQLEXCEPTION
  BEGIN
    IF lock_acquired = 1 THEN
      DO RELEASE_LOCK('campus_circle:migration:037');
    END IF;
    RESIGNAL;
  END;

  SELECT GET_LOCK('campus_circle:migration:037', 10) INTO lock_acquired;
  IF lock_acquired IS NULL OR lock_acquired <> 1 THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT = 'migration 037 lock unavailable';
  END IF;

  IF NOT EXISTS (
    SELECT 1 FROM information_schema.tables
    WHERE table_schema=DATABASE() AND table_name='reports'
  ) THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT = 'migration 037 reports table missing';
  END IF;

  IF NOT EXISTS (
    SELECT 1 FROM information_schema.columns
    WHERE table_schema=DATABASE() AND table_name='reports' AND column_name='client_request_id'
  ) THEN
    ALTER TABLE reports ADD COLUMN client_request_id VARCHAR(96) NULL AFTER reporter_id;
  ELSEIF NOT EXISTS (
    SELECT 1 FROM information_schema.columns
    WHERE table_schema=DATABASE() AND table_name='reports' AND column_name='client_request_id'
      AND data_type='varchar' AND character_maximum_length=96 AND is_nullable='YES'
  ) THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT = 'migration 037 client_request_id schema mismatch';
  END IF;

  SELECT GROUP_CONCAT(column_name ORDER BY seq_in_index SEPARATOR ','), MIN(non_unique)
    INTO index_columns, index_non_unique
  FROM information_schema.statistics
  WHERE table_schema=DATABASE() AND table_name='reports'
    AND index_name='idx_reports_reporter_request';

  IF index_columns IS NULL THEN
    ALTER TABLE reports
      ADD UNIQUE INDEX idx_reports_reporter_request (reporter_id, client_request_id);
  ELSEIF index_columns<>'reporter_id,client_request_id' OR index_non_unique<>0 THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT = 'migration 037 report idempotency index mismatch';
  END IF;

  DO RELEASE_LOCK('campus_circle:migration:037');
  SET lock_acquired = 0;
END$$
CALL migrate_report_idempotency()$$
DROP PROCEDURE migrate_report_idempotency$$
DELIMITER ;
