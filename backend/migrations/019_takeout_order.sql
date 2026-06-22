-- G-3: takeout order domain core. All identifiers outside these tables are
-- logical references. No foreign key couples G to A-F, Media, or other G tables.

CREATE TABLE IF NOT EXISTS takeout_fee_config (
  id BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY,
  config_scope VARCHAR(32) NOT NULL,
  rider_fee_cents BIGINT UNSIGNED NOT NULL,
  platform_cut_cents BIGINT UNSIGNED NOT NULL,
  row_version BIGINT UNSIGNED NOT NULL,
  updated_at DATETIME NOT NULL,
  UNIQUE INDEX uq_takeout_fee_config_scope (config_scope)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

INSERT INTO takeout_fee_config
  (config_scope, rider_fee_cents, platform_cut_cents, row_version, updated_at)
VALUES ('global', 100, 50, 1, UTC_TIMESTAMP())
ON DUPLICATE KEY UPDATE config_scope = VALUES(config_scope);

CREATE TABLE IF NOT EXISTS takeout_orders (
  id BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY,
  order_public_id CHAR(26) NOT NULL,
  buyer_user_id BIGINT UNSIGNED NOT NULL,
  merchant_public_id CHAR(26) NOT NULL,
  rider_user_id BIGINT UNSIGNED NULL,
  status VARCHAR(24) NOT NULL,
  items_amount_cents BIGINT UNSIGNED NOT NULL,
  delivery_fee_cents BIGINT UNSIGNED NOT NULL DEFAULT 0,
  total_amount_cents BIGINT UNSIGNED NOT NULL,
  contact_name VARCHAR(64) NOT NULL DEFAULT '',
  contact_phone VARCHAR(32) NOT NULL DEFAULT '',
  address_building VARCHAR(128) NOT NULL DEFAULT '',
  address_room VARCHAR(64) NOT NULL DEFAULT '',
  remark VARCHAR(512) NOT NULL DEFAULT '',
  pay_no VARCHAR(64) NULL,
  paid_at DATETIME NULL,
  accepted_at DATETIME NULL,
  preparing_at DATETIME NULL,
  ready_at DATETIME NULL,
  delivering_at DATETIME NULL,
  completed_at DATETIME NULL,
  cancelled_at DATETIME NULL,
  refunded_at DATETIME NULL,
  row_version BIGINT UNSIGNED NOT NULL,
  created_at DATETIME NOT NULL,
  updated_at DATETIME NOT NULL,
  CONSTRAINT chk_takeout_order_status CHECK (status IN (
    'pending_payment','paid','accepted','preparing','ready_for_pickup',
    'delivering','completed','cancelled','refunded'
  )),
  UNIQUE INDEX uq_takeout_order_public_id (order_public_id),
  INDEX idx_takeout_order_buyer (buyer_user_id),
  INDEX idx_takeout_order_merchant (merchant_public_id),
  INDEX idx_takeout_order_rider (rider_user_id),
  INDEX idx_takeout_order_status (status)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE IF NOT EXISTS takeout_order_items (
  id BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY,
  order_public_id CHAR(26) NOT NULL,
  dish_public_id CHAR(26) NOT NULL,
  dish_name_snapshot VARCHAR(128) NOT NULL,
  unit_price_cents_snapshot BIGINT UNSIGNED NOT NULL,
  quantity INT UNSIGNED NOT NULL,
  line_amount_cents BIGINT UNSIGNED NOT NULL,
  created_at DATETIME NOT NULL,
  INDEX idx_takeout_order_item_order (order_public_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE IF NOT EXISTS takeout_rider_earning_records (
  id BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY,
  earning_public_id CHAR(26) NOT NULL,
  order_public_id CHAR(26) NOT NULL,
  rider_user_id BIGINT UNSIGNED NOT NULL,
  rider_fee_cents BIGINT UNSIGNED NOT NULL,
  platform_cut_cents BIGINT UNSIGNED NOT NULL,
  settlement_status VARCHAR(24) NOT NULL,
  settled_at DATETIME NULL,
  row_version BIGINT UNSIGNED NOT NULL,
  created_at DATETIME NOT NULL,
  updated_at DATETIME NOT NULL,
  CONSTRAINT chk_takeout_earning_settlement_status CHECK (settlement_status IN ('pending_settlement','settled')),
  UNIQUE INDEX uq_takeout_earning_public_id (earning_public_id),
  UNIQUE INDEX uq_takeout_earning_order (order_public_id),
  INDEX idx_takeout_earning_settlement_status (settlement_status)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

