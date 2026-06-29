-- D-1: delivery order domain skeleton.
-- User and rider identifiers are logical references only. The delivery domain
-- is independent from the frozen takeout domain and has no cross-domain FK.

CREATE TABLE IF NOT EXISTS delivery_orders (
  id BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY,
  delivery_order_public_id CHAR(26) NOT NULL,
  merchant_user_id BIGINT UNSIGNED NOT NULL,
  rider_user_id BIGINT UNSIGNED NULL,
  pickup_contact_name VARCHAR(64) NOT NULL DEFAULT '',
  pickup_contact_phone VARCHAR(32) NOT NULL DEFAULT '',
  pickup_address VARCHAR(256) NOT NULL DEFAULT '',
  recipient_name VARCHAR(64) NOT NULL DEFAULT '',
  recipient_phone VARCHAR(32) NOT NULL DEFAULT '',
  delivery_address VARCHAR(256) NOT NULL DEFAULT '',
  remark VARCHAR(512) NOT NULL DEFAULT '',
  delivery_fee_cents BIGINT UNSIGNED NOT NULL,
  rider_fee_cents BIGINT UNSIGNED NOT NULL,
  platform_cut_cents BIGINT UNSIGNED NOT NULL,
  status VARCHAR(24) NOT NULL,
  ready_for_claim_at DATETIME NULL,
  claimed_at DATETIME NULL,
  to_pickup_at DATETIME NULL,
  picked_up_at DATETIME NULL,
  delivering_at DATETIME NULL,
  delivered_at DATETIME NULL,
  payment_failed_at DATETIME NULL,
  cancelled_at DATETIME NULL,
  row_version BIGINT UNSIGNED NOT NULL,
  created_at DATETIME NOT NULL,
  updated_at DATETIME NOT NULL,
  CONSTRAINT chk_delivery_order_status CHECK (status IN (
    'pending_fee_payment','payment_failed','ready_for_claim','claimed',
    'to_pickup','picked_up','delivering','delivered','cancelled'
  )),
  CONSTRAINT chk_delivery_order_fee_snapshot CHECK (
    delivery_fee_cents = rider_fee_cents + platform_cut_cents
  ),
  UNIQUE INDEX uq_delivery_order_public_id (delivery_order_public_id),
  INDEX idx_delivery_order_merchant_created (merchant_user_id, created_at, id),
  INDEX idx_delivery_order_status_created (status, created_at, id),
  INDEX idx_delivery_order_rider_status (rider_user_id, status)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
