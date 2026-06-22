-- G-2: takeout catalog domain core. Merchant/category/media identifiers are
-- logical references only. No foreign key crosses or couples domain tables.

CREATE TABLE IF NOT EXISTS takeout_dish_categories (
  id BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY,
  category_public_id CHAR(26) NOT NULL,
  merchant_public_id CHAR(26) NOT NULL,
  name VARCHAR(64) NOT NULL,
  sort_order INT NOT NULL DEFAULT 0,
  row_version BIGINT UNSIGNED NOT NULL,
  created_at DATETIME NOT NULL,
  updated_at DATETIME NOT NULL,
  UNIQUE INDEX uq_takeout_category_public_id (category_public_id),
  INDEX idx_takeout_category_merchant (merchant_public_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE IF NOT EXISTS takeout_dishes (
  id BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY,
  dish_public_id CHAR(26) NOT NULL,
  merchant_public_id CHAR(26) NOT NULL,
  category_public_id CHAR(26) NULL,
  name VARCHAR(128) NOT NULL,
  description VARCHAR(512) NOT NULL DEFAULT '',
  price_cents BIGINT UNSIGNED NOT NULL,
  image_asset_public_id CHAR(26) NULL,
  status VARCHAR(24) NOT NULL,
  sort_order INT NOT NULL DEFAULT 0,
  row_version BIGINT UNSIGNED NOT NULL,
  created_at DATETIME NOT NULL,
  updated_at DATETIME NOT NULL,
  CONSTRAINT chk_takeout_dish_status CHECK (status IN ('draft','on_shelf','sold_out','off_shelf','removed')),
  UNIQUE INDEX uq_takeout_dish_public_id (dish_public_id),
  INDEX idx_takeout_dish_merchant (merchant_public_id),
  INDEX idx_takeout_dish_status (status)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
