-- Refuse rollback once handling records exist; they are operational evidence.
DELIMITER $$
DROP PROCEDURE IF EXISTS rollback_059_user_feedback_handling$$
CREATE PROCEDURE rollback_059_user_feedback_handling()
SQL SECURITY INVOKER
BEGIN
  DECLARE handled BIGINT DEFAULT 0;
  SELECT COUNT(*) INTO handled FROM user_feedbacks WHERE handled_by IS NOT NULL;
  IF handled > 0 THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT = 'handled feedback records must be preserved';
  END IF;
  ALTER TABLE user_feedbacks
    DROP KEY idx_user_feedbacks_status_created,
    DROP COLUMN handled_at,
    DROP COLUMN handled_by,
    DROP COLUMN admin_note;
END$$
DELIMITER ;
CALL rollback_059_user_feedback_handling();
DROP PROCEDURE rollback_059_user_feedback_handling;
