-- D-2: delivery dispatch, idempotent merchant commands and reliable rider
-- notification outbox. All identifiers outside Delivery are logical only.

ALTER TABLE delivery_orders
  ADD COLUMN merchant_request_id CHAR(26) NULL AFTER merchant_user_id,
  ADD UNIQUE INDEX uq_delivery_order_merchant_request (merchant_user_id, merchant_request_id);

CREATE TABLE delivery_outbox (
  id BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY,
  event_public_id CHAR(26) NOT NULL,
  aggregate_public_id CHAR(26) NOT NULL,
  aggregate_version BIGINT UNSIGNED NOT NULL,
  event_type VARCHAR(48) NOT NULL,
  payload JSON NOT NULL,
  status VARCHAR(24) NOT NULL,
  attempt_count INT UNSIGNED NOT NULL DEFAULT 0,
  next_attempt_at DATETIME(3) NULL,
  lease_token CHAR(26) NULL,
  lease_expires_at DATETIME(3) NULL,
  last_error_code VARCHAR(64) NOT NULL DEFAULT '',
  last_error VARCHAR(512) NOT NULL DEFAULT '',
  processed_at DATETIME(3) NULL,
  row_version BIGINT UNSIGNED NOT NULL,
  created_at DATETIME(3) NOT NULL,
  updated_at DATETIME(3) NOT NULL,
  CONSTRAINT chk_delivery_outbox_status CHECK (
    status IN ('pending','processing','retry_wait','succeeded','skipped','deadletter')
  ),
  UNIQUE INDEX uq_delivery_outbox_event_public_id (event_public_id),
  UNIQUE INDEX uq_delivery_outbox_aggregate_event (
    aggregate_public_id, aggregate_version, event_type
  ),
  INDEX idx_delivery_outbox_claim (
    status, next_attempt_at, lease_expires_at, id
  )
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
