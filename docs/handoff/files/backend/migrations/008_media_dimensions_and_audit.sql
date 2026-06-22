-- P0-4 acceptance hardening: server-verified dimensions and append-only media audit.
DELIMITER $$
DROP PROCEDURE IF EXISTS migrate_media_dimensions$$
CREATE PROCEDURE migrate_media_dimensions()
BEGIN
  IF NOT EXISTS (
    SELECT 1 FROM information_schema.columns
    WHERE table_schema = DATABASE() AND table_name = 'media_assets' AND column_name = 'width'
  ) THEN
    ALTER TABLE media_assets ADD COLUMN width BIGINT NOT NULL DEFAULT 0 AFTER verified_size;
  END IF;

  IF NOT EXISTS (
    SELECT 1 FROM information_schema.columns
    WHERE table_schema = DATABASE() AND table_name = 'media_assets' AND column_name = 'height'
  ) THEN
    ALTER TABLE media_assets ADD COLUMN height BIGINT NOT NULL DEFAULT 0 AFTER width;
  END IF;
END$$
CALL migrate_media_dimensions()$$
DROP PROCEDURE migrate_media_dimensions$$
DELIMITER ;

CREATE TABLE IF NOT EXISTS media_asset_audits (
  id BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY,
  asset_id BIGINT UNSIGNED NOT NULL,
  from_status VARCHAR(24) NOT NULL,
  to_status VARCHAR(24) NOT NULL,
  action VARCHAR(48) NOT NULL,
  actor_type VARCHAR(24) NOT NULL,
  actor_id BIGINT UNSIGNED NOT NULL DEFAULT 0,
  detail VARCHAR(240) NOT NULL DEFAULT '',
  created_at DATETIME(3) NOT NULL,
  INDEX idx_media_audit_asset (asset_id, created_at),
  INDEX idx_media_audit_action (action, created_at)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
