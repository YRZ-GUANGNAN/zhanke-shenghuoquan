-- G-1: takeout merchant domain core. A users and P0-4 media assets are logical
-- references only; this table intentionally has no cross-domain foreign keys.

CREATE TABLE IF NOT EXISTS takeout_merchants (
  id BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY,
  merchant_public_id CHAR(26) NOT NULL,
  owner_user_id BIGINT UNSIGNED NOT NULL,
  name VARCHAR(128) NOT NULL,
  contact_phone VARCHAR(32) NOT NULL DEFAULT '',
  location VARCHAR(128) NOT NULL DEFAULT '',
  business_hours VARCHAR(64) NOT NULL DEFAULT '',
  logo_asset_public_id CHAR(26) NULL,
  status VARCHAR(24) NOT NULL,
  row_version BIGINT UNSIGNED NOT NULL,
  created_at DATETIME NOT NULL,
  updated_at DATETIME NOT NULL,
  CONSTRAINT chk_takeout_merchant_status CHECK (status IN ('pending_review','active','resting','rejected','suspended')),
  UNIQUE INDEX uq_takeout_merchant_public_id (merchant_public_id),
  INDEX idx_takeout_merchant_owner (owner_user_id),
  INDEX idx_takeout_merchant_status (status)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
