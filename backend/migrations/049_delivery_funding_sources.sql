-- Delivery wallet funding provenance.
-- This migration intentionally creates no rows: historical balances remain unknown
-- until a transaction explicitly records their unverified source.

CREATE TABLE delivery_funding_lots (
  id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  funding_lot_public_id CHAR(26) NOT NULL,
  wallet_public_id CHAR(26) NOT NULL,
  merchant_user_id BIGINT UNSIGNED NOT NULL,
  source VARCHAR(32) NOT NULL,
  verification_status VARCHAR(24) NOT NULL,
  payment_public_id CHAR(26) NULL,
  total_amount_cents BIGINT UNSIGNED NOT NULL,
  allocated_amount_cents BIGINT UNSIGNED NOT NULL DEFAULT 0,
  refunded_amount_cents BIGINT UNSIGNED NOT NULL DEFAULT 0,
  currency VARCHAR(8) NOT NULL DEFAULT 'CNY',
  row_version BIGINT UNSIGNED NOT NULL,
  created_at DATETIME(3) NOT NULL,
  updated_at DATETIME(3) NOT NULL,
  PRIMARY KEY (id),
  UNIQUE INDEX uq_delivery_funding_lot_public_id (funding_lot_public_id),
  UNIQUE INDEX uq_delivery_funding_lot_payment (payment_public_id),
  INDEX idx_delivery_funding_lot_wallet_fifo (wallet_public_id, created_at, id),
  INDEX idx_delivery_funding_lot_merchant_verification (merchant_user_id, verification_status, created_at, id),
  CONSTRAINT chk_delivery_funding_lot_source CHECK (
    (source='wechat_recharge' AND verification_status='verified' AND payment_public_id IS NOT NULL)
    OR
    (source='mock_recharge' AND verification_status='unverified' AND payment_public_id IS NOT NULL)
    OR
    (source='historical_unknown' AND verification_status='unverified' AND payment_public_id IS NULL)
  ),
  CONSTRAINT chk_delivery_funding_lot_amounts CHECK (
    total_amount_cents > 0
    AND refunded_amount_cents <= allocated_amount_cents
    AND allocated_amount_cents-refunded_amount_cents <= total_amount_cents
  ),
  CONSTRAINT chk_delivery_funding_lot_currency CHECK (currency='CNY')
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE delivery_funding_allocations (
  id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  funding_allocation_public_id CHAR(26) NOT NULL,
  settlement_public_id CHAR(26) NOT NULL,
  funding_lot_public_id CHAR(26) NOT NULL,
  merchant_user_id BIGINT UNSIGNED NOT NULL,
  amount_cents BIGINT UNSIGNED NOT NULL,
  refunded_amount_cents BIGINT UNSIGNED NOT NULL DEFAULT 0,
  row_version BIGINT UNSIGNED NOT NULL,
  created_at DATETIME(3) NOT NULL,
  refunded_at DATETIME(3) NULL,
  PRIMARY KEY (id),
  UNIQUE INDEX uq_delivery_funding_allocation_public_id (funding_allocation_public_id),
  UNIQUE INDEX uq_delivery_funding_allocation_settlement_lot (settlement_public_id, funding_lot_public_id),
  INDEX idx_delivery_funding_allocation_lot (funding_lot_public_id, id),
  INDEX idx_delivery_funding_allocation_merchant (merchant_user_id, created_at, id),
  CONSTRAINT chk_delivery_funding_allocation_amounts CHECK (
    amount_cents > 0
    AND refunded_amount_cents <= amount_cents
    AND (
      (refunded_amount_cents=0 AND refunded_at IS NULL)
      OR
      (refunded_amount_cents>0 AND refunded_at IS NOT NULL)
    )
  )
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
