-- Durable rider payout queue, reservation and audit facts.
-- This migration intentionally creates no rows and does not change rider balances.

CREATE TABLE delivery_rider_payout_queue (
  id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  payout_queue_public_id CHAR(26) NOT NULL,
  earning_public_id CHAR(26) NOT NULL,
  delivery_order_public_id CHAR(26) NOT NULL,
  rider_user_id BIGINT UNSIGNED NOT NULL,
  amount_cents BIGINT UNSIGNED NOT NULL,
  currency VARCHAR(8) NOT NULL DEFAULT 'CNY',
  status VARCHAR(16) NOT NULL,
  reason_code VARCHAR(64) NOT NULL DEFAULT '',
  source_policy_version VARCHAR(32) NOT NULL,
  source_evidence_hash CHAR(64) NOT NULL DEFAULT '',
  row_version BIGINT UNSIGNED NOT NULL,
  cancelled_at DATETIME(3) NULL,
  created_at DATETIME(3) NOT NULL,
  updated_at DATETIME(3) NOT NULL,
  PRIMARY KEY (id),
  UNIQUE INDEX uq_delivery_rider_payout_queue_public_id (payout_queue_public_id),
  UNIQUE INDEX uq_delivery_rider_payout_queue_earning (earning_public_id),
  INDEX idx_delivery_rider_payout_queue_rider_created (rider_user_id, created_at, id),
  INDEX idx_delivery_rider_payout_queue_status_created (status, created_at, id),
  CONSTRAINT chk_delivery_rider_payout_queue_amount CHECK (amount_cents > 0),
  CONSTRAINT chk_delivery_rider_payout_queue_currency CHECK (currency = 'CNY'),
  CONSTRAINT chk_delivery_rider_payout_queue_status CHECK (status IN ('blocked', 'queued', 'cancelled')),
  CONSTRAINT chk_delivery_rider_payout_queue_cancelled CHECK (
    (status = 'cancelled' AND cancelled_at IS NOT NULL)
    OR (status IN ('blocked', 'queued') AND cancelled_at IS NULL)
  )
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE delivery_rider_payout_reservations (
  id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  reservation_public_id CHAR(26) NOT NULL,
  payout_queue_public_id CHAR(26) NOT NULL,
  earning_public_id CHAR(26) NOT NULL,
  wallet_public_id CHAR(26) NOT NULL,
  rider_user_id BIGINT UNSIGNED NOT NULL,
  amount_cents BIGINT UNSIGNED NOT NULL,
  status VARCHAR(16) NOT NULL,
  row_version BIGINT UNSIGNED NOT NULL,
  reserved_at DATETIME(3) NOT NULL,
  released_at DATETIME(3) NULL,
  PRIMARY KEY (id),
  UNIQUE INDEX uq_delivery_rider_payout_reservation_public_id (reservation_public_id),
  UNIQUE INDEX uq_delivery_rider_payout_reservation_queue (payout_queue_public_id),
  UNIQUE INDEX uq_delivery_rider_payout_reservation_earning (earning_public_id),
  INDEX idx_delivery_rider_payout_reservation_rider_status (rider_user_id, status, id),
  CONSTRAINT chk_delivery_rider_payout_reservation_amount CHECK (amount_cents > 0),
  CONSTRAINT chk_delivery_rider_payout_reservation_status CHECK (status IN ('active', 'released')),
  CONSTRAINT chk_delivery_rider_payout_reservation_release CHECK (
    (status = 'active' AND released_at IS NULL)
    OR (status = 'released' AND released_at IS NOT NULL)
  )
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE delivery_rider_payout_events (
  id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  event_public_id CHAR(26) NOT NULL,
  payout_queue_public_id CHAR(26) NOT NULL,
  event_type VARCHAR(24) NOT NULL,
  actor_type VARCHAR(16) NOT NULL,
  operator_user_id BIGINT UNSIGNED NULL,
  reason_code VARCHAR(64) NOT NULL DEFAULT '',
  reason VARCHAR(256) NOT NULL DEFAULT '',
  idempotency_key CHAR(26) NULL,
  expected_row_version BIGINT UNSIGNED NULL,
  created_at DATETIME(3) NOT NULL,
  PRIMARY KEY (id),
  UNIQUE INDEX uq_delivery_rider_payout_event_public_id (event_public_id),
  UNIQUE INDEX uq_delivery_rider_payout_event_idempotency (idempotency_key),
  INDEX idx_delivery_rider_payout_event_queue_created (payout_queue_public_id, created_at, id),
  CONSTRAINT chk_delivery_rider_payout_event_type CHECK (
    event_type IN ('blocked', 'queued', 'cancelled', 'source_invalidated')
  ),
  CONSTRAINT chk_delivery_rider_payout_event_actor CHECK (
    (actor_type = 'system' AND operator_user_id IS NULL)
    OR (actor_type = 'admin' AND operator_user_id IS NOT NULL)
  )
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
