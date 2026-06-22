-- P0-4 media_assets schema drift reconciliation.
--
-- 009 is the frozen contract. The legacy GORM model was still registered for
-- AutoMigrate and later changed 13 media_assets column/index definitions back
-- toward the 007 shape. This migration is deliberately limited to those 13
-- differences. It does not touch A/B/C domain tables or migrations 001-014.

DELIMITER $$
DROP PROCEDURE IF EXISTS reconcile_media_assets_drift_v15$$
CREATE PROCEDURE reconcile_media_assets_drift_v15()
BEGIN
  IF NOT EXISTS (
    SELECT 1 FROM information_schema.tables
    WHERE table_schema = DATABASE() AND table_name = 'media_assets'
  ) THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT = 'media_assets is required before migration 015';
  END IF;

  -- The frozen 007/009 contract requires these values to be non-null. Repair
  -- any rows produced while the legacy AutoMigrate definition was active.
  UPDATE media_assets SET reference_type = '' WHERE reference_type IS NULL;
  UPDATE media_assets SET reference_id = 0 WHERE reference_id IS NULL;
  UPDATE media_assets SET reference_sort = 0 WHERE reference_sort IS NULL;
  UPDATE media_assets SET reservation_token = '' WHERE reservation_token IS NULL;

  IF EXISTS (
    SELECT review_task_id
    FROM media_assets
    WHERE review_task_id IS NOT NULL
    GROUP BY review_task_id
    HAVING COUNT(*) > 1
  ) THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT = 'duplicate review_task_id blocks migration 015';
  END IF;

  ALTER TABLE media_assets
    MODIFY COLUMN public_id CHAR(32) NULL DEFAULT NULL,
    MODIFY COLUMN owner_user_id BIGINT UNSIGNED NULL DEFAULT NULL,
    MODIFY COLUMN object_key VARCHAR(512) NULL DEFAULT NULL,
    MODIFY COLUMN declared_mime VARCHAR(64) NOT NULL DEFAULT '',
    MODIFY COLUMN declared_size BIGINT NOT NULL DEFAULT 0,
    MODIFY COLUMN status VARCHAR(24) NOT NULL DEFAULT 'pending_upload',
    MODIFY COLUMN reference_type VARCHAR(24) NOT NULL DEFAULT '',
    MODIFY COLUMN reference_id BIGINT UNSIGNED NOT NULL DEFAULT 0,
    MODIFY COLUMN reference_sort BIGINT NOT NULL DEFAULT 0,
    MODIFY COLUMN reservation_token CHAR(32) NOT NULL DEFAULT '',
    MODIFY COLUMN upload_expires_at DATETIME(3) NULL DEFAULT NULL,
    MODIFY COLUMN review_available_at DATETIME(3) NULL DEFAULT NULL;

  IF NOT EXISTS (
    SELECT 1 FROM information_schema.statistics
    WHERE table_schema = DATABASE()
      AND table_name = 'media_assets'
      AND index_name = 'uq_media_review_task_v11'
  ) THEN
    ALTER TABLE media_assets ADD UNIQUE INDEX uq_media_review_task_v11 (review_task_id);
  END IF;
END$$
CALL reconcile_media_assets_drift_v15()$$
DROP PROCEDURE reconcile_media_assets_drift_v15$$
DELIMITER ;
