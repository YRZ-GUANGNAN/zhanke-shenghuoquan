-- Domain A settings, blocks, feedback and account-closure tombstones.
-- Forward-only: historical business and audit facts are intentionally retained.

ALTER TABLE users
  ADD COLUMN phone VARCHAR(32) NULL AFTER open_id,
  ADD COLUMN closed_at DATETIME(3) NULL AFTER token_version,
  ADD UNIQUE KEY uq_users_phone (phone),
  ADD KEY idx_users_status_closed (status, closed_at);

CREATE TABLE user_settings (
  id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  user_id BIGINT UNSIGNED NOT NULL,
  order_notice BOOLEAN NOT NULL DEFAULT TRUE,
  social_notice BOOLEAN NOT NULL DEFAULT TRUE,
  quiet_hours BOOLEAN NOT NULL DEFAULT FALSE,
  personalized BOOLEAN NOT NULL DEFAULT TRUE,
  created_at DATETIME(3) NOT NULL,
  updated_at DATETIME(3) NOT NULL,
  PRIMARY KEY (id),
  UNIQUE KEY uq_user_settings_user (user_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE user_blocks (
  user_id BIGINT UNSIGNED NOT NULL,
  blocked_user_id BIGINT UNSIGNED NOT NULL,
  created_at DATETIME(3) NOT NULL,
  PRIMARY KEY (user_id, blocked_user_id),
  KEY idx_user_blocks_blocked (blocked_user_id, created_at),
  CONSTRAINT chk_user_blocks_not_self CHECK (user_id <> blocked_user_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE user_feedbacks (
  id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  feedback_public_id CHAR(26) NOT NULL,
  user_id BIGINT UNSIGNED NOT NULL,
  category VARCHAR(24) NOT NULL,
  content VARCHAR(1000) NOT NULL,
  status VARCHAR(24) NOT NULL DEFAULT 'open',
  created_at DATETIME(3) NOT NULL,
  updated_at DATETIME(3) NOT NULL,
  PRIMARY KEY (id),
  UNIQUE KEY uq_user_feedback_public_id (feedback_public_id),
  KEY idx_user_feedback_user_created (user_id, created_at, id),
  KEY idx_user_feedback_status_created (status, created_at, id),
  CONSTRAINT chk_user_feedback_category CHECK (category IN ('bug','suggestion','report','other')),
  CONSTRAINT chk_user_feedback_status CHECK (status IN ('open','processing','resolved','closed'))
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE user_feedback_images (
  id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  feedback_id BIGINT UNSIGNED NOT NULL,
  asset_public_id CHAR(26) NOT NULL,
  slot SMALLINT UNSIGNED NOT NULL,
  created_at DATETIME(3) NOT NULL,
  PRIMARY KEY (id),
  UNIQUE KEY uq_user_feedback_image_slot (feedback_id, slot),
  UNIQUE KEY uq_user_feedback_image_asset (asset_public_id),
  KEY idx_user_feedback_images_feedback (feedback_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

ALTER TABLE media_asset_references
  ADD CONSTRAINT chk_media_reference_type_feedback_guard
  CHECK (reference_type IN ('post','product','avatar','dish','merchant_logo','delivery_proof','delivery_im','feedback'));

ALTER TABLE media_asset_references
  DROP CHECK chk_media_reference_type;

ALTER TABLE media_asset_references
  ADD CONSTRAINT chk_media_reference_type
  CHECK (reference_type IN ('post','product','avatar','dish','merchant_logo','delivery_proof','delivery_im','feedback'));

ALTER TABLE media_asset_references
  DROP CHECK chk_media_reference_type_feedback_guard;
