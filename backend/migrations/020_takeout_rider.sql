-- G-5: rider crowdsourcing and fulfillment domain core. User and order IDs
-- are logical references only; no foreign key couples domain tables.

CREATE TABLE IF NOT EXISTS takeout_riders (
  id BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY,
  rider_user_id BIGINT UNSIGNED NOT NULL,
  status VARCHAR(24) NOT NULL,
  real_name VARCHAR(64) NOT NULL DEFAULT '',
  phone VARCHAR(32) NOT NULL DEFAULT '',
  row_version BIGINT UNSIGNED NOT NULL,
  created_at DATETIME NOT NULL,
  updated_at DATETIME NOT NULL,
  CONSTRAINT chk_takeout_rider_status CHECK (status IN ('active','resting','suspended')),
  UNIQUE INDEX uq_takeout_rider_user (rider_user_id),
  INDEX idx_takeout_rider_status (status)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE IF NOT EXISTS takeout_fulfillments (
  id BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY,
  fulfillment_public_id CHAR(26) NOT NULL,
  order_public_id CHAR(26) NOT NULL,
  rider_user_id BIGINT UNSIGNED NOT NULL,
  status VARCHAR(24) NOT NULL,
  assigned_at DATETIME NULL,
  to_store_at DATETIME NULL,
  picked_at DATETIME NULL,
  to_user_at DATETIME NULL,
  delivered_at DATETIME NULL,
  row_version BIGINT UNSIGNED NOT NULL,
  created_at DATETIME NOT NULL,
  updated_at DATETIME NOT NULL,
  CONSTRAINT chk_takeout_fulfillment_status CHECK (status IN ('ASSIGNED','TO_STORE','PICKED','TO_USER','DELIVERED')),
  UNIQUE INDEX uq_takeout_fulfillment_public_id (fulfillment_public_id),
  UNIQUE INDEX uq_takeout_fulfillment_order (order_public_id),
  INDEX idx_takeout_fulfillment_rider (rider_user_id),
  INDEX idx_takeout_fulfillment_status (status),
  INDEX idx_takeout_fulfillment_rider_status (rider_user_id, status)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE IF NOT EXISTS takeout_rider_config (
  id BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY,
  config_scope VARCHAR(32) NOT NULL,
  max_in_flight_per_rider INT UNSIGNED NOT NULL,
  row_version BIGINT UNSIGNED NOT NULL,
  updated_at DATETIME NOT NULL,
  UNIQUE INDEX uq_takeout_rider_config_scope (config_scope)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

INSERT INTO takeout_rider_config
  (config_scope, max_in_flight_per_rider, row_version, updated_at)
VALUES ('global', 1000, 1, UTC_TIMESTAMP())
ON DUPLICATE KEY UPDATE config_scope = VALUES(config_scope);

