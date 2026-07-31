-- Roll back product-comment storage and automatic-review audit fields only.
-- Product-comment notification events are new 036 data and must be removed
-- before the legacy notification uniqueness can be restored.

DELIMITER $$
DROP PROCEDURE IF EXISTS rollback_trade_comments_auto_review$$
CREATE PROCEDURE rollback_trade_comments_auto_review()
SQL SECURITY INVOKER
BEGIN
  DECLARE lock_acquired INT DEFAULT 0;
  DECLARE notification_index_columns TEXT DEFAULT NULL;
  DECLARE notification_index_non_unique INT DEFAULT NULL;

  DECLARE EXIT HANDLER FOR SQLEXCEPTION
  BEGIN
    IF lock_acquired = 1 THEN
      DO RELEASE_LOCK('campus_circle:rollback:036');
    END IF;
    RESIGNAL;
  END;

  SELECT GET_LOCK('campus_circle:rollback:036', 10) INTO lock_acquired;
  IF lock_acquired IS NULL OR lock_acquired <> 1 THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT = 'rollback 036 lock unavailable';
  END IF;

  IF EXISTS (
    SELECT 1 FROM information_schema.columns
    WHERE table_schema=DATABASE() AND table_name='notifications' AND column_name='product_id'
  ) THEN
    DELETE FROM notifications WHERE product_id <> 0;
  END IF;
  IF EXISTS (
    SELECT 1 FROM information_schema.columns
    WHERE table_schema=DATABASE() AND table_name='notifications' AND column_name='product_comment_id'
  ) THEN
    DELETE FROM notifications WHERE product_comment_id <> 0;
  END IF;

  SELECT GROUP_CONCAT(column_name ORDER BY seq_in_index SEPARATOR ','),
         MAX(non_unique)
    INTO notification_index_columns, notification_index_non_unique
  FROM information_schema.statistics
  WHERE table_schema=DATABASE()
    AND table_name='notifications'
    AND index_name='idx_notification_event';

  IF notification_index_columns IS NULL THEN
    ALTER TABLE notifications
      ADD UNIQUE INDEX idx_notification_event (recipient_id, actor_id, type, post_id, comment_id);
  ELSEIF notification_index_columns='recipient_id,actor_id,type,post_id,comment_id,product_id,product_comment_id' AND
         notification_index_non_unique=0 THEN
    ALTER TABLE notifications
      DROP INDEX idx_notification_event,
      ADD UNIQUE INDEX idx_notification_event (recipient_id, actor_id, type, post_id, comment_id);
  ELSEIF notification_index_columns<>'recipient_id,actor_id,type,post_id,comment_id' OR
         notification_index_non_unique<>0 THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT = 'rollback 036 notification event index mismatch';
  END IF;

  IF EXISTS (
    SELECT 1 FROM information_schema.columns
    WHERE table_schema=DATABASE() AND table_name='notifications' AND column_name='product_comment_id'
  ) THEN
    ALTER TABLE notifications
      DROP COLUMN product_comment_id;
  END IF;
  IF EXISTS (
    SELECT 1 FROM information_schema.columns
    WHERE table_schema=DATABASE() AND table_name='notifications' AND column_name='product_id'
  ) THEN
    ALTER TABLE notifications
      DROP COLUMN product_id;
  END IF;

  DROP TABLE IF EXISTS product_comments;

  IF EXISTS (
    SELECT 1 FROM information_schema.table_constraints
    WHERE constraint_schema=DATABASE() AND table_name='review_tasks'
      AND constraint_name='chk_review_tasks_decision_source'
      AND constraint_type='CHECK'
  ) THEN
    ALTER TABLE review_tasks
      DROP CHECK chk_review_tasks_decision_source;
  END IF;
  IF EXISTS (
    SELECT 1 FROM information_schema.columns
    WHERE table_schema=DATABASE() AND table_name='review_tasks' AND column_name='decision_source'
  ) THEN
    ALTER TABLE review_tasks
      DROP COLUMN decision_source;
  END IF;

  DO RELEASE_LOCK('campus_circle:rollback:036');
  SET lock_acquired = 0;
END$$
CALL rollback_trade_comments_auto_review()$$
DROP PROCEDURE rollback_trade_comments_auto_review$$
DELIMITER ;
