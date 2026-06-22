-- Roll back P0-4 Module 2 to the 008 media schema while preserving rows that
-- were created through the v1.1 model.

DROP TABLE IF EXISTS media_outbox;
DROP TABLE IF EXISTS media_quota_reservations;
DROP TABLE IF EXISTS media_upload_tickets;
DROP TABLE IF EXISTS media_asset_references;

DROP TRIGGER IF EXISTS media_assets_v11_before_update;
DROP TRIGGER IF EXISTS media_assets_v11_before_insert;

UPDATE media_assets SET public_id = asset_public_id WHERE public_id IS NULL OR public_id = '';
UPDATE media_assets SET owner_user_id = owner_id WHERE owner_user_id IS NULL;
UPDATE media_assets SET object_key = COALESCE(staging_key, final_key, CONCAT('rollback/', asset_public_id)) WHERE object_key IS NULL OR object_key = '';
UPDATE media_assets SET declared_mime = COALESCE(NULLIF(verified_mime, ''), 'application/octet-stream') WHERE declared_mime = '';
UPDATE media_assets SET declared_size = byte_size WHERE declared_size = 0 AND byte_size > 0;
UPDATE media_assets SET status = technical_status WHERE technical_status IS NOT NULL AND technical_status <> '';
UPDATE media_assets SET idempotency_key = asset_public_id WHERE idempotency_key = '';
UPDATE media_assets SET upload_expires_at = DATE_ADD(COALESCE(created_at, UTC_TIMESTAMP(3)), INTERVAL 30 MINUTE) WHERE upload_expires_at IS NULL;
UPDATE media_assets SET review_available_at = COALESCE(created_at, UTC_TIMESTAMP(3)) WHERE review_available_at IS NULL;
UPDATE media_assets SET purpose = 'post_image' WHERE purpose = 'post';
UPDATE media_assets SET purpose = 'product_image' WHERE purpose = 'product';

ALTER TABLE media_assets
  DROP INDEX idx_media_delete_status_v11,
  DROP INDEX idx_media_review_deadline_v11,
  DROP INDEX idx_media_technical_upload_v11,
  DROP INDEX uq_media_final_key_v11,
  DROP INDEX uq_media_review_task_v11,
  DROP INDEX uq_media_staging_key_v11,
  DROP INDEX uq_media_owner_idempotency_v11,
  DROP INDEX uq_media_asset_public_id_v11;

UPDATE media_assets SET review_task_id = 0 WHERE review_task_id IS NULL;

ALTER TABLE media_assets
  MODIFY COLUMN public_id CHAR(32) NOT NULL,
  MODIFY COLUMN owner_user_id BIGINT UNSIGNED NOT NULL,
  MODIFY COLUMN object_key VARCHAR(512) NOT NULL,
  MODIFY COLUMN review_task_id BIGINT UNSIGNED NOT NULL DEFAULT 0,
  MODIFY COLUMN upload_expires_at DATETIME(3) NOT NULL,
  MODIFY COLUMN review_available_at DATETIME(3) NOT NULL,
  DROP COLUMN row_version,
  DROP COLUMN deleted_at,
  DROP COLUMN review_deadline_at,
  DROP COLUMN active_ref_count,
  DROP COLUMN projected_decided_at,
  DROP COLUMN projected_decision,
  DROP COLUMN applied_decision_version,
  DROP COLUMN final_version_id,
  DROP COLUMN final_key,
  DROP COLUMN staging_version_id,
  DROP COLUMN staging_key,
  DROP COLUMN byte_size,
  DROP COLUMN trusted_sha256,
  DROP COLUMN technical_status,
  DROP COLUMN review_status,
  DROP COLUMN storage_stage,
  DROP COLUMN owner_id,
  DROP COLUMN asset_public_id;
