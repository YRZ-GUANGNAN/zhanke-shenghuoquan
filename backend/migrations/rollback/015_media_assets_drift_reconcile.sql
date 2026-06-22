-- Roll back migration 015 to the exact media_assets definitions observed on
-- the real database before reconciliation. Values are backfilled first so the
-- rollback remains safe after v1.1 rows have been created.

DELIMITER $$
DROP PROCEDURE IF EXISTS rollback_media_assets_drift_v15$$
CREATE PROCEDURE rollback_media_assets_drift_v15()
BEGIN
  IF NOT EXISTS (
    SELECT 1 FROM information_schema.tables
    WHERE table_schema = DATABASE() AND table_name = 'media_assets'
  ) THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT = 'media_assets is required before rollback 015';
  END IF;

  UPDATE media_assets
    SET public_id = asset_public_id
    WHERE public_id IS NULL OR public_id = '';
  UPDATE media_assets
    SET owner_user_id = owner_id
    WHERE owner_user_id IS NULL;
  UPDATE media_assets
    SET object_key = COALESCE(NULLIF(staging_key, ''), NULLIF(final_key, ''), CONCAT('rollback/', asset_public_id))
    WHERE object_key IS NULL OR object_key = '';
  UPDATE media_assets
    SET upload_expires_at = DATE_ADD(COALESCE(created_at, UTC_TIMESTAMP(3)), INTERVAL 30 MINUTE)
    WHERE upload_expires_at IS NULL;
  UPDATE media_assets
    SET review_available_at = COALESCE(created_at, UTC_TIMESTAMP(3))
    WHERE review_available_at IS NULL;

  IF EXISTS (
    SELECT 1 FROM information_schema.statistics
    WHERE table_schema = DATABASE()
      AND table_name = 'media_assets'
      AND index_name = 'uq_media_review_task_v11'
  ) THEN
    ALTER TABLE media_assets DROP INDEX uq_media_review_task_v11;
  END IF;

  ALTER TABLE media_assets
    MODIFY COLUMN public_id VARCHAR(32) NOT NULL,
    MODIFY COLUMN owner_user_id BIGINT UNSIGNED NOT NULL,
    MODIFY COLUMN object_key VARCHAR(512) NOT NULL,
    MODIFY COLUMN declared_mime VARCHAR(64) NOT NULL,
    MODIFY COLUMN declared_size BIGINT NOT NULL,
    MODIFY COLUMN status VARCHAR(24) NOT NULL,
    MODIFY COLUMN reference_type VARCHAR(24) NULL DEFAULT NULL,
    MODIFY COLUMN reference_id BIGINT UNSIGNED NULL DEFAULT NULL,
    MODIFY COLUMN reference_sort BIGINT NULL DEFAULT NULL,
    MODIFY COLUMN reservation_token VARCHAR(32) NULL DEFAULT NULL,
    MODIFY COLUMN upload_expires_at DATETIME(3) NOT NULL,
    MODIFY COLUMN review_available_at DATETIME(3) NOT NULL;
END$$
CALL rollback_media_assets_drift_v15()$$
DROP PROCEDURE rollback_media_assets_drift_v15$$
DELIMITER ;
