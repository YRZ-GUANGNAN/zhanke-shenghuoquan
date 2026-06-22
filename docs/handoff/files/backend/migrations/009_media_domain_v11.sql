-- P0-4 Module 2: Media Domain v1.1 data model.
-- This migration is additive over the legacy 007/008 media schema. It does
-- not create foreign keys to users, posts, products, or any other A/B/C table.

DELIMITER $$
DROP PROCEDURE IF EXISTS migrate_media_domain_v11$$
CREATE PROCEDURE migrate_media_domain_v11()
BEGIN
  IF NOT EXISTS (SELECT 1 FROM information_schema.columns WHERE table_schema = DATABASE() AND table_name = 'media_assets' AND column_name = 'asset_public_id') THEN
    ALTER TABLE media_assets ADD COLUMN asset_public_id CHAR(26) NULL AFTER id;
  END IF;
  IF NOT EXISTS (SELECT 1 FROM information_schema.columns WHERE table_schema = DATABASE() AND table_name = 'media_assets' AND column_name = 'owner_id') THEN
    ALTER TABLE media_assets ADD COLUMN owner_id BIGINT UNSIGNED NULL AFTER asset_public_id;
  END IF;
  IF NOT EXISTS (SELECT 1 FROM information_schema.columns WHERE table_schema = DATABASE() AND table_name = 'media_assets' AND column_name = 'storage_stage') THEN
    ALTER TABLE media_assets ADD COLUMN storage_stage VARCHAR(16) NOT NULL DEFAULT 'staging' AFTER purpose;
  END IF;
  IF NOT EXISTS (SELECT 1 FROM information_schema.columns WHERE table_schema = DATABASE() AND table_name = 'media_assets' AND column_name = 'review_status') THEN
    ALTER TABLE media_assets ADD COLUMN review_status VARCHAR(16) NOT NULL DEFAULT 'pending' AFTER storage_stage;
  END IF;
  IF NOT EXISTS (SELECT 1 FROM information_schema.columns WHERE table_schema = DATABASE() AND table_name = 'media_assets' AND column_name = 'technical_status') THEN
    ALTER TABLE media_assets ADD COLUMN technical_status VARCHAR(24) NOT NULL DEFAULT 'pending_upload' AFTER review_status;
  END IF;
  IF NOT EXISTS (SELECT 1 FROM information_schema.columns WHERE table_schema = DATABASE() AND table_name = 'media_assets' AND column_name = 'trusted_sha256') THEN
    ALTER TABLE media_assets ADD COLUMN trusted_sha256 CHAR(64) NULL AFTER technical_status;
  END IF;
  IF NOT EXISTS (SELECT 1 FROM information_schema.columns WHERE table_schema = DATABASE() AND table_name = 'media_assets' AND column_name = 'byte_size') THEN
    ALTER TABLE media_assets ADD COLUMN byte_size BIGINT UNSIGNED NOT NULL DEFAULT 0 AFTER verified_mime;
  END IF;
  IF NOT EXISTS (SELECT 1 FROM information_schema.columns WHERE table_schema = DATABASE() AND table_name = 'media_assets' AND column_name = 'staging_key') THEN
    ALTER TABLE media_assets ADD COLUMN staging_key VARCHAR(512) NULL AFTER height;
  END IF;
  IF NOT EXISTS (SELECT 1 FROM information_schema.columns WHERE table_schema = DATABASE() AND table_name = 'media_assets' AND column_name = 'staging_version_id') THEN
    ALTER TABLE media_assets ADD COLUMN staging_version_id VARCHAR(128) NULL AFTER staging_key;
  END IF;
  IF NOT EXISTS (SELECT 1 FROM information_schema.columns WHERE table_schema = DATABASE() AND table_name = 'media_assets' AND column_name = 'final_key') THEN
    ALTER TABLE media_assets ADD COLUMN final_key VARCHAR(512) NULL AFTER staging_version_id;
  END IF;
  IF NOT EXISTS (SELECT 1 FROM information_schema.columns WHERE table_schema = DATABASE() AND table_name = 'media_assets' AND column_name = 'final_version_id') THEN
    ALTER TABLE media_assets ADD COLUMN final_version_id VARCHAR(128) NULL AFTER final_key;
  END IF;
  IF NOT EXISTS (SELECT 1 FROM information_schema.columns WHERE table_schema = DATABASE() AND table_name = 'media_assets' AND column_name = 'applied_decision_version') THEN
    ALTER TABLE media_assets ADD COLUMN applied_decision_version BIGINT UNSIGNED NOT NULL DEFAULT 0 AFTER review_task_id;
  END IF;
  IF NOT EXISTS (SELECT 1 FROM information_schema.columns WHERE table_schema = DATABASE() AND table_name = 'media_assets' AND column_name = 'projected_decision') THEN
    ALTER TABLE media_assets ADD COLUMN projected_decision VARCHAR(16) NOT NULL DEFAULT 'pending' AFTER applied_decision_version;
  END IF;
  IF NOT EXISTS (SELECT 1 FROM information_schema.columns WHERE table_schema = DATABASE() AND table_name = 'media_assets' AND column_name = 'projected_decided_at') THEN
    ALTER TABLE media_assets ADD COLUMN projected_decided_at DATETIME(3) NULL AFTER projected_decision;
  END IF;
  IF NOT EXISTS (SELECT 1 FROM information_schema.columns WHERE table_schema = DATABASE() AND table_name = 'media_assets' AND column_name = 'active_ref_count') THEN
    ALTER TABLE media_assets ADD COLUMN active_ref_count BIGINT UNSIGNED NOT NULL DEFAULT 0 AFTER projected_decided_at;
  END IF;
  IF NOT EXISTS (SELECT 1 FROM information_schema.columns WHERE table_schema = DATABASE() AND table_name = 'media_assets' AND column_name = 'review_deadline_at') THEN
    ALTER TABLE media_assets ADD COLUMN review_deadline_at DATETIME(3) NULL AFTER upload_expires_at;
  END IF;
  IF NOT EXISTS (SELECT 1 FROM information_schema.columns WHERE table_schema = DATABASE() AND table_name = 'media_assets' AND column_name = 'deleted_at') THEN
    ALTER TABLE media_assets ADD COLUMN deleted_at DATETIME(3) NULL AFTER storage_deleted_at;
  END IF;
  IF NOT EXISTS (SELECT 1 FROM information_schema.columns WHERE table_schema = DATABASE() AND table_name = 'media_assets' AND column_name = 'row_version') THEN
    ALTER TABLE media_assets ADD COLUMN row_version BIGINT UNSIGNED NOT NULL DEFAULT 1 AFTER deleted_at;
  END IF;

  ALTER TABLE media_assets MODIFY COLUMN review_task_id BIGINT UNSIGNED NULL DEFAULT NULL;

  UPDATE media_assets
    SET asset_public_id = UPPER(public_id)
    WHERE (asset_public_id IS NULL OR asset_public_id = '')
      AND CHAR_LENGTH(public_id) = 26
      AND UPPER(public_id) REGEXP '^[0-7][0-9A-HJKMNP-TV-Z]{25}$';
  IF EXISTS (SELECT 1 FROM media_assets WHERE asset_public_id IS NULL OR asset_public_id = '') THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT = 'media_assets requires an audited ULID backfill before Module 2 migration';
  END IF;
  UPDATE media_assets SET owner_id = owner_user_id WHERE owner_id IS NULL AND owner_user_id IS NOT NULL;
  UPDATE media_assets SET purpose = 'post' WHERE purpose = 'post_image' AND asset_public_id <> '';
  UPDATE media_assets SET purpose = 'product' WHERE purpose = 'product_image' AND asset_public_id <> '';
  UPDATE media_assets SET byte_size = GREATEST(verified_size, 0) WHERE byte_size = 0 AND verified_size > 0;
  UPDATE media_assets SET technical_status = status WHERE status IS NOT NULL AND status <> '';
  UPDATE media_assets SET review_task_id = NULL WHERE review_task_id = 0;

  ALTER TABLE media_assets
    MODIFY COLUMN asset_public_id CHAR(26) NOT NULL,
    MODIFY COLUMN owner_id BIGINT UNSIGNED NOT NULL,
    MODIFY COLUMN public_id CHAR(32) NULL,
    MODIFY COLUMN owner_user_id BIGINT UNSIGNED NULL,
    MODIFY COLUMN object_key VARCHAR(512) NULL,
    MODIFY COLUMN declared_mime VARCHAR(64) NOT NULL DEFAULT '',
    MODIFY COLUMN declared_size BIGINT NOT NULL DEFAULT 0,
    MODIFY COLUMN status VARCHAR(24) NOT NULL DEFAULT 'pending_upload',
    MODIFY COLUMN review_task_id BIGINT UNSIGNED NULL DEFAULT NULL,
    MODIFY COLUMN upload_expires_at DATETIME(3) NULL,
    MODIFY COLUMN review_available_at DATETIME(3) NULL;

  IF NOT EXISTS (SELECT 1 FROM information_schema.statistics WHERE table_schema = DATABASE() AND table_name = 'media_assets' AND index_name = 'uq_media_asset_public_id_v11') THEN
    ALTER TABLE media_assets ADD UNIQUE INDEX uq_media_asset_public_id_v11 (asset_public_id);
  END IF;
  IF NOT EXISTS (SELECT 1 FROM information_schema.statistics WHERE table_schema = DATABASE() AND table_name = 'media_assets' AND index_name = 'uq_media_owner_idempotency_v11') THEN
    ALTER TABLE media_assets ADD UNIQUE INDEX uq_media_owner_idempotency_v11 (owner_id, idempotency_key);
  END IF;
  IF NOT EXISTS (SELECT 1 FROM information_schema.statistics WHERE table_schema = DATABASE() AND table_name = 'media_assets' AND index_name = 'uq_media_staging_key_v11') THEN
    ALTER TABLE media_assets ADD UNIQUE INDEX uq_media_staging_key_v11 (staging_key);
  END IF;
  IF NOT EXISTS (SELECT 1 FROM information_schema.statistics WHERE table_schema = DATABASE() AND table_name = 'media_assets' AND index_name = 'uq_media_final_key_v11') THEN
    ALTER TABLE media_assets ADD UNIQUE INDEX uq_media_final_key_v11 (final_key);
  END IF;
  IF NOT EXISTS (SELECT 1 FROM information_schema.statistics WHERE table_schema = DATABASE() AND table_name = 'media_assets' AND index_name = 'uq_media_review_task_v11') THEN
    ALTER TABLE media_assets ADD UNIQUE INDEX uq_media_review_task_v11 (review_task_id);
  END IF;
  IF NOT EXISTS (SELECT 1 FROM information_schema.statistics WHERE table_schema = DATABASE() AND table_name = 'media_assets' AND index_name = 'idx_media_technical_upload_v11') THEN
    ALTER TABLE media_assets ADD INDEX idx_media_technical_upload_v11 (technical_status, upload_expires_at, id);
  END IF;
  IF NOT EXISTS (SELECT 1 FROM information_schema.statistics WHERE table_schema = DATABASE() AND table_name = 'media_assets' AND index_name = 'idx_media_review_deadline_v11') THEN
    ALTER TABLE media_assets ADD INDEX idx_media_review_deadline_v11 (technical_status, review_deadline_at, id);
  END IF;
  IF NOT EXISTS (SELECT 1 FROM information_schema.statistics WHERE table_schema = DATABASE() AND table_name = 'media_assets' AND index_name = 'idx_media_delete_status_v11') THEN
    ALTER TABLE media_assets ADD INDEX idx_media_delete_status_v11 (technical_status, delete_requested_at, id);
  END IF;
END$$
CALL migrate_media_domain_v11()$$
DROP PROCEDURE migrate_media_domain_v11$$
DELIMITER ;

CREATE TABLE IF NOT EXISTS media_asset_references (
  id BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY,
  asset_id BIGINT UNSIGNED NOT NULL,
  reference_type VARCHAR(16) NOT NULL,
  reference_id VARCHAR(64) NOT NULL,
  slot SMALLINT UNSIGNED NOT NULL,
  status VARCHAR(16) NOT NULL DEFAULT 'pending',
  parent_visibility VARCHAR(16) NOT NULL DEFAULT 'hidden',
  aggregate_version BIGINT UNSIGNED NOT NULL DEFAULT 0,
  operation_id CHAR(26) NOT NULL,
  revocation_fence CHAR(26) NULL,
  fence_expires_at DATETIME(3) NULL,
  row_version BIGINT UNSIGNED NOT NULL DEFAULT 1,
  created_at DATETIME(3) NOT NULL,
  updated_at DATETIME(3) NOT NULL,
  CONSTRAINT fk_media_reference_asset FOREIGN KEY (asset_id) REFERENCES media_assets(id) ON DELETE RESTRICT ON UPDATE RESTRICT,
  CONSTRAINT chk_media_reference_type CHECK (reference_type IN ('post','product','avatar')),
  CONSTRAINT chk_media_reference_status CHECK (status IN ('pending','active','revoking','revoked')),
  CONSTRAINT chk_media_parent_visibility CHECK (parent_visibility IN ('hidden','visible')),
  UNIQUE INDEX uq_media_reference_slot (reference_type, reference_id, slot),
  UNIQUE INDEX uq_media_reference_asset_slot (asset_id, reference_type, reference_id, slot),
  INDEX idx_media_reference_asset_status (asset_id, status),
  INDEX idx_media_reference_parent_status (reference_type, reference_id, status),
  INDEX idx_media_reference_operation (operation_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE IF NOT EXISTS media_upload_tickets (
  ticket_id CHAR(26) PRIMARY KEY,
  asset_id BIGINT UNSIGNED NOT NULL,
  owner_id BIGINT UNSIGNED NOT NULL,
  expected_size BIGINT UNSIGNED NOT NULL,
  expected_md5 CHAR(24) NOT NULL,
  expected_content_type VARCHAR(64) NOT NULL,
  status VARCHAR(24) NOT NULL DEFAULT 'created',
  credential_issue_count TINYINT UNSIGNED NOT NULL DEFAULT 0,
  expires_at DATETIME(3) NOT NULL,
  row_version BIGINT UNSIGNED NOT NULL DEFAULT 1,
  created_at DATETIME(3) NOT NULL,
  updated_at DATETIME(3) NOT NULL,
  CONSTRAINT fk_media_ticket_asset FOREIGN KEY (asset_id) REFERENCES media_assets(id) ON DELETE RESTRICT ON UPDATE RESTRICT,
  CONSTRAINT chk_media_ticket_status CHECK (status IN ('created','authorization_issued','uploaded','consumed','expired','cancelled')),
  CONSTRAINT chk_media_ticket_credential_count CHECK (credential_issue_count <= 1),
  UNIQUE INDEX uq_media_ticket_asset (asset_id),
  INDEX idx_media_ticket_owner_status (owner_id, status, expires_at),
  INDEX idx_media_ticket_expiry (status, expires_at)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE IF NOT EXISTS media_quota_reservations (
  reservation_id CHAR(26) PRIMARY KEY,
  owner_id BIGINT UNSIGNED NOT NULL,
  bytes_reserved BIGINT UNSIGNED NOT NULL,
  count_reserved INT UNSIGNED NOT NULL,
  status VARCHAR(16) NOT NULL DEFAULT 'active',
  ticket_id CHAR(26) NOT NULL,
  row_version BIGINT UNSIGNED NOT NULL DEFAULT 1,
  created_at DATETIME(3) NOT NULL,
  updated_at DATETIME(3) NOT NULL,
  CONSTRAINT fk_media_quota_ticket FOREIGN KEY (ticket_id) REFERENCES media_upload_tickets(ticket_id) ON DELETE RESTRICT ON UPDATE RESTRICT,
  CONSTRAINT chk_media_quota_status CHECK (status IN ('active','committed','released')),
  UNIQUE INDEX uq_media_quota_ticket (ticket_id),
  INDEX idx_media_quota_owner_status (owner_id, status)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE IF NOT EXISTS media_outbox (
  event_id CHAR(26) PRIMARY KEY,
  aggregate_id CHAR(26) NOT NULL,
  aggregate_version BIGINT UNSIGNED NOT NULL,
  operation VARCHAR(48) NOT NULL,
  payload JSON NOT NULL,
  processed_at DATETIME(3) NULL,
  row_version BIGINT UNSIGNED NOT NULL DEFAULT 1,
  created_at DATETIME(3) NOT NULL,
  updated_at DATETIME(3) NOT NULL,
  UNIQUE INDEX uq_media_outbox_aggregate_operation (aggregate_id, aggregate_version, operation),
  INDEX idx_media_outbox_pending (processed_at, created_at, event_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
