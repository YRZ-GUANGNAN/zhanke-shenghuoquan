-- P0-4: controlled media assets for avatars, community posts and products.
CREATE TABLE IF NOT EXISTS media_assets (
  id BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY,
  public_id CHAR(32) NOT NULL,
  owner_user_id BIGINT UNSIGNED NOT NULL,
  purpose VARCHAR(32) NOT NULL,
  object_key VARCHAR(512) NOT NULL,
  declared_mime VARCHAR(64) NOT NULL,
  verified_mime VARCHAR(64) NOT NULL DEFAULT '',
  declared_size BIGINT NOT NULL,
  verified_size BIGINT NOT NULL DEFAULT 0,
  status VARCHAR(24) NOT NULL,
  review_task_id BIGINT UNSIGNED NOT NULL DEFAULT 0,
  wechat_open_id VARCHAR(96) NOT NULL DEFAULT '',
  review_trace_id VARCHAR(128) NULL,
  review_suggest VARCHAR(24) NOT NULL DEFAULT '',
  review_label BIGINT NOT NULL DEFAULT 0,
  reference_type VARCHAR(24) NOT NULL DEFAULT '',
  reference_id BIGINT UNSIGNED NOT NULL DEFAULT 0,
  reference_sort BIGINT NOT NULL DEFAULT 0,
  reservation_token CHAR(32) NOT NULL DEFAULT '',
  reservation_expires_at DATETIME(3) NULL,
  idempotency_key VARCHAR(96) NOT NULL,
  upload_expires_at DATETIME(3) NOT NULL,
  review_attempts BIGINT NOT NULL DEFAULT 0,
  review_available_at DATETIME(3) NOT NULL,
  last_error VARCHAR(512) NOT NULL DEFAULT '',
  delete_requested_at DATETIME(3) NULL,
  storage_deleted_at DATETIME(3) NULL,
  created_at DATETIME(3) NULL,
  updated_at DATETIME(3) NULL,
  UNIQUE INDEX idx_media_public_id (public_id),
  UNIQUE INDEX idx_media_object_key (object_key),
  UNIQUE INDEX idx_media_owner_idempotency (owner_user_id, idempotency_key),
  UNIQUE INDEX idx_media_review_trace (review_trace_id),
  INDEX idx_media_owner_created (owner_user_id, created_at),
  INDEX idx_media_review_pending (status, review_available_at, id),
  INDEX idx_media_delete_pending (delete_requested_at, storage_deleted_at, id),
  INDEX idx_media_reference (reference_type, reference_id, reference_sort),
  INDEX idx_media_review_task (review_task_id),
  INDEX idx_media_upload_expires (upload_expires_at),
  INDEX idx_media_reservation (reservation_token)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

DELIMITER $$
DROP PROCEDURE IF EXISTS migrate_media_pipeline$$
CREATE PROCEDURE migrate_media_pipeline()
BEGIN
  IF NOT EXISTS (
    SELECT 1 FROM information_schema.columns
    WHERE table_schema = DATABASE() AND table_name = 'post_images' AND column_name = 'asset_id'
  ) THEN
    ALTER TABLE post_images ADD COLUMN asset_id BIGINT UNSIGNED NULL AFTER post_id,
      ADD INDEX idx_post_images_asset_id (asset_id);
  END IF;

  IF NOT EXISTS (
    SELECT 1 FROM information_schema.columns
    WHERE table_schema = DATABASE() AND table_name = 'product_images' AND column_name = 'asset_id'
  ) THEN
    ALTER TABLE product_images ADD COLUMN asset_id BIGINT UNSIGNED NULL AFTER product_id,
      ADD INDEX idx_product_images_asset_id (asset_id);
  END IF;

  IF NOT EXISTS (
    SELECT 1 FROM information_schema.columns
    WHERE table_schema = DATABASE() AND table_name = 'users' AND column_name = 'avatar_asset_id'
  ) THEN
    ALTER TABLE users ADD COLUMN avatar_asset_id BIGINT UNSIGNED NULL AFTER avatar,
      ADD INDEX idx_users_avatar_asset_id (avatar_asset_id);
  END IF;
END$$
CALL migrate_media_pipeline()$$
DROP PROCEDURE migrate_media_pipeline$$
DELIMITER ;
