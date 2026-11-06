-- Delivery-domain rider duty state and one global, operator-adjustable capacity.
-- No historical orders or balances are changed by this additive migration.
CREATE TABLE IF NOT EXISTS delivery_rider_availability (
  id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  rider_user_id BIGINT UNSIGNED NOT NULL,
  state VARCHAR(16) CHARACTER SET ascii COLLATE ascii_bin NOT NULL DEFAULT 'off_duty',
  scope VARCHAR(16) CHARACTER SET ascii COLLATE ascii_bin NOT NULL DEFAULT 'campus',
  eligibility_revision VARCHAR(128) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  row_version BIGINT UNSIGNED NOT NULL DEFAULT 1,
  created_at DATETIME(3) NOT NULL,
  updated_at DATETIME(3) NOT NULL,
  PRIMARY KEY (id),
  UNIQUE INDEX uq_delivery_rider_availability_rider (rider_user_id),
  INDEX idx_delivery_rider_availability_state (state, scope, rider_user_id),
  CONSTRAINT chk_delivery_rider_availability_state CHECK (state IN ('off_duty', 'on_duty')),
  CONSTRAINT chk_delivery_rider_availability_scope CHECK (scope = 'campus'),
  CONSTRAINT chk_delivery_rider_availability_revision CHECK (CHAR_LENGTH(eligibility_revision) BETWEEN 1 AND 128),
  CONSTRAINT chk_delivery_rider_availability_identity CHECK (rider_user_id > 0 AND row_version > 0)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE IF NOT EXISTS delivery_rider_broadcast_config (
  id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  config_scope VARCHAR(16) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  max_in_flight_per_rider INT UNSIGNED NOT NULL DEFAULT 3,
  row_version BIGINT UNSIGNED NOT NULL DEFAULT 1,
  created_at DATETIME(3) NOT NULL,
  updated_at DATETIME(3) NOT NULL,
  PRIMARY KEY (id),
  UNIQUE INDEX uq_delivery_rider_broadcast_config_scope (config_scope),
  CONSTRAINT chk_delivery_rider_broadcast_config_scope CHECK (config_scope = 'global'),
  CONSTRAINT chk_delivery_rider_broadcast_config_limit CHECK (max_in_flight_per_rider BETWEEN 1 AND 100),
  CONSTRAINT chk_delivery_rider_broadcast_config_version CHECK (row_version > 0)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

INSERT INTO delivery_rider_broadcast_config
  (config_scope, max_in_flight_per_rider, row_version, created_at, updated_at)
VALUES ('global', 3, 1, UTC_TIMESTAMP(3), UTC_TIMESTAMP(3))
ON DUPLICATE KEY UPDATE config_scope = VALUES(config_scope);
