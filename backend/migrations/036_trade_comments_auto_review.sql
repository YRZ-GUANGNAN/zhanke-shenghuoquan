-- Public second-hand product comments, notification coordinates and review
-- decision audit. Reapplying the migration is guarded and idempotent.

DELIMITER $$
DROP PROCEDURE IF EXISTS migrate_trade_comments_auto_review$$
CREATE PROCEDURE migrate_trade_comments_auto_review()
SQL SECURITY INVOKER
BEGIN
  DECLARE lock_acquired INT DEFAULT 0;
  DECLARE notification_index_columns TEXT DEFAULT NULL;
  DECLARE notification_index_non_unique INT DEFAULT NULL;

  DECLARE EXIT HANDLER FOR SQLEXCEPTION
  BEGIN
    IF lock_acquired = 1 THEN
      DO RELEASE_LOCK('campus_circle:migration:036');
    END IF;
    RESIGNAL;
  END;

  SELECT GET_LOCK('campus_circle:migration:036', 10) INTO lock_acquired;
  IF lock_acquired IS NULL OR lock_acquired <> 1 THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT = 'migration 036 lock unavailable';
  END IF;

  IF (SELECT COUNT(*) FROM information_schema.tables
      WHERE table_schema=DATABASE()
        AND table_name IN ('products','notifications','review_tasks')) <> 3 THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT = 'migration 036 base tables missing';
  END IF;
  IF (SELECT COUNT(*) FROM information_schema.columns
      WHERE table_schema=DATABASE() AND (
        (table_name='products' AND column_name='id' AND column_type='bigint unsigned' AND is_nullable='NO') OR
        (table_name='products' AND column_name='seller_id' AND column_type='bigint unsigned' AND is_nullable='NO') OR
        (table_name='notifications' AND column_name='recipient_id' AND column_type='bigint unsigned' AND is_nullable='NO') OR
        (table_name='notifications' AND column_name='actor_id' AND column_type='bigint unsigned' AND is_nullable='NO') OR
        (table_name='notifications' AND column_name='type' AND data_type='varchar' AND is_nullable='NO') OR
        (table_name='notifications' AND column_name='post_id' AND column_type='bigint unsigned' AND is_nullable='NO') OR
        (table_name='notifications' AND column_name='comment_id' AND column_type='bigint unsigned' AND is_nullable='NO') OR
        (table_name='review_tasks' AND column_name='id' AND column_type='bigint unsigned' AND is_nullable='NO') OR
        (table_name='review_tasks' AND column_name='object_type' AND data_type='varchar' AND is_nullable='NO') OR
        (table_name='review_tasks' AND column_name='status' AND data_type='varchar' AND is_nullable='NO')
      )) <> 10 THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT = 'migration 036 base schema mismatch';
  END IF;

  IF NOT EXISTS (
    SELECT 1 FROM information_schema.tables
    WHERE table_schema=DATABASE() AND table_name='product_comments'
  ) THEN
    CREATE TABLE product_comments (
      id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
      product_id BIGINT UNSIGNED NOT NULL,
      user_id BIGINT UNSIGNED NOT NULL,
      parent_comment_id BIGINT UNSIGNED NOT NULL DEFAULT 0,
      reply_to_user_id BIGINT UNSIGNED NOT NULL DEFAULT 0,
      content VARCHAR(500) NOT NULL,
      review_status VARCHAR(24) NOT NULL DEFAULT 'submitted',
      client_request_id CHAR(26) NOT NULL,
      request_hash BINARY(32) NOT NULL,
      created_at DATETIME(3) NOT NULL,
      updated_at DATETIME(3) NOT NULL,
      deleted_at DATETIME(3) NULL,
      PRIMARY KEY (id),
      UNIQUE INDEX uq_product_comments_user_request (user_id, client_request_id),
      INDEX idx_product_comments_public (product_id, parent_comment_id, review_status, created_at, id),
      INDEX idx_product_comment_replies (parent_comment_id, review_status, created_at, id),
      INDEX idx_product_comments_user_recent (user_id, created_at, id)
    ) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
  END IF;

  IF (SELECT COUNT(*) FROM information_schema.columns
      WHERE table_schema=DATABASE() AND table_name='product_comments' AND (
        (column_name='id' AND column_type='bigint unsigned' AND is_nullable='NO') OR
        (column_name='product_id' AND column_type='bigint unsigned' AND is_nullable='NO') OR
        (column_name='user_id' AND column_type='bigint unsigned' AND is_nullable='NO') OR
        (column_name='parent_comment_id' AND column_type='bigint unsigned' AND is_nullable='NO') OR
        (column_name='reply_to_user_id' AND column_type='bigint unsigned' AND is_nullable='NO') OR
        (column_name='content' AND data_type='varchar' AND character_maximum_length=500 AND is_nullable='NO') OR
        (column_name='review_status' AND data_type='varchar' AND character_maximum_length=24 AND is_nullable='NO') OR
        (column_name='client_request_id' AND data_type='char' AND character_maximum_length=26 AND is_nullable='NO') OR
        (column_name='request_hash' AND data_type='binary' AND character_maximum_length=32 AND is_nullable='NO') OR
        (column_name='created_at' AND data_type='datetime' AND is_nullable='NO') OR
        (column_name='updated_at' AND data_type='datetime' AND is_nullable='NO') OR
        (column_name='deleted_at' AND data_type='datetime' AND is_nullable='YES')
      )) <> 12 THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT = 'migration 036 product_comments schema mismatch';
  END IF;

  IF NOT EXISTS (
    SELECT 1 FROM information_schema.columns
    WHERE table_schema=DATABASE() AND table_name='notifications' AND column_name='product_id'
  ) THEN
    ALTER TABLE notifications
      ADD COLUMN product_id BIGINT UNSIGNED NOT NULL DEFAULT 0 AFTER comment_id;
  END IF;
  IF NOT EXISTS (
    SELECT 1 FROM information_schema.columns
    WHERE table_schema=DATABASE() AND table_name='notifications' AND column_name='product_comment_id'
  ) THEN
    ALTER TABLE notifications
      ADD COLUMN product_comment_id BIGINT UNSIGNED NOT NULL DEFAULT 0 AFTER product_id;
  END IF;
  IF (SELECT COUNT(*) FROM information_schema.columns
      WHERE table_schema=DATABASE() AND table_name='notifications' AND (
        (column_name='product_id' AND column_type='bigint unsigned' AND is_nullable='NO') OR
        (column_name='product_comment_id' AND column_type='bigint unsigned' AND is_nullable='NO')
      )) <> 2 THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT = 'migration 036 notification coordinate schema mismatch';
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
      ADD UNIQUE INDEX idx_notification_event (
        recipient_id, actor_id, type, post_id, comment_id, product_id, product_comment_id
      );
  ELSEIF notification_index_columns='recipient_id,actor_id,type,post_id,comment_id' AND
         notification_index_non_unique=0 THEN
    ALTER TABLE notifications
      DROP INDEX idx_notification_event,
      ADD UNIQUE INDEX idx_notification_event (
        recipient_id, actor_id, type, post_id, comment_id, product_id, product_comment_id
      );
  ELSEIF notification_index_columns<>'recipient_id,actor_id,type,post_id,comment_id,product_id,product_comment_id' OR
         notification_index_non_unique<>0 THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT = 'migration 036 notification event index mismatch';
  END IF;

  IF NOT EXISTS (
    SELECT 1 FROM information_schema.columns
    WHERE table_schema=DATABASE() AND table_name='review_tasks' AND column_name='decision_source'
  ) THEN
    ALTER TABLE review_tasks
      ADD COLUMN decision_source VARCHAR(24) NOT NULL DEFAULT 'manual' AFTER operator_id;
  END IF;
  IF NOT EXISTS (
    SELECT 1 FROM information_schema.columns
    WHERE table_schema=DATABASE() AND table_name='review_tasks'
      AND column_name='decision_source' AND data_type='varchar'
      AND character_maximum_length=24 AND is_nullable='NO'
      AND column_default='manual'
  ) THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT = 'migration 036 decision_source schema mismatch';
  END IF;
  IF EXISTS (
    SELECT 1 FROM review_tasks
    WHERE decision_source NOT IN ('manual','automatic','manual_override')
  ) THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT = 'migration 036 unsupported decision source';
  END IF;
  IF NOT EXISTS (
    SELECT 1 FROM information_schema.table_constraints
    WHERE constraint_schema=DATABASE() AND table_name='review_tasks'
      AND constraint_name='chk_review_tasks_decision_source'
      AND constraint_type='CHECK'
  ) THEN
    ALTER TABLE review_tasks
      ADD CONSTRAINT chk_review_tasks_decision_source
      CHECK (decision_source IN ('manual','automatic','manual_override'));
  END IF;

  DO RELEASE_LOCK('campus_circle:migration:036');
  SET lock_acquired = 0;
END$$
CALL migrate_trade_comments_auto_review()$$
DROP PROCEDURE migrate_trade_comments_auto_review$$
DELIMITER ;
