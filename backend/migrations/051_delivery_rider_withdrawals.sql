-- Rider wallet withdrawal requests. Applying a request reserves funds by
-- debiting delivery_rider_wallets; rejection credits them back atomically.

CREATE TABLE delivery_rider_withdrawals (
  id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  withdraw_public_id CHAR(26) NOT NULL,
  rider_user_id BIGINT UNSIGNED NOT NULL,
  wallet_public_id CHAR(26) NOT NULL,
  amount_cents BIGINT UNSIGNED NOT NULL,
  currency VARCHAR(8) NOT NULL DEFAULT 'CNY',
  status VARCHAR(16) NOT NULL,
  reason VARCHAR(256) NOT NULL DEFAULT '',
  idempotency_key CHAR(26) NOT NULL,
  row_version BIGINT UNSIGNED NOT NULL,
  processed_by BIGINT UNSIGNED NULL,
  process_key CHAR(26) NULL,
  process_version BIGINT UNSIGNED NULL,
  payment_reference VARCHAR(128) NOT NULL DEFAULT '',
  processed_at DATETIME(3) NULL,
  created_at DATETIME(3) NOT NULL,
  updated_at DATETIME(3) NOT NULL,
  PRIMARY KEY (id),
  UNIQUE INDEX uq_delivery_rider_withdrawal_public_id (withdraw_public_id),
  UNIQUE INDEX uq_delivery_rider_withdrawal_idempotency (idempotency_key),
  UNIQUE INDEX uq_delivery_rider_withdrawal_process_key (process_key),
  INDEX idx_delivery_rider_withdrawal_rider_status_created (rider_user_id, status, created_at, id),
  INDEX idx_delivery_rider_withdrawal_status_created (status, created_at, id),
  INDEX idx_delivery_rider_withdrawal_wallet (wallet_public_id),
  CONSTRAINT chk_delivery_rider_withdrawal_amount CHECK (amount_cents > 0),
  CONSTRAINT chk_delivery_rider_withdrawal_currency CHECK (currency = 'CNY'),
  CONSTRAINT chk_delivery_rider_withdrawal_status CHECK (status IN ('processing', 'paid', 'rejected')),
  CONSTRAINT chk_delivery_rider_withdrawal_processed CHECK (
    (status = 'processing' AND processed_by IS NULL AND processed_at IS NULL AND process_key IS NULL AND process_version IS NULL)
    OR (status IN ('paid', 'rejected') AND processed_by IS NOT NULL AND processed_by > 0
      AND processed_at IS NOT NULL AND process_key IS NOT NULL
      AND process_version IS NOT NULL AND process_version > 0)
  ),
  CONSTRAINT chk_delivery_rider_withdrawal_payment CHECK (status <> 'paid' OR CHAR_LENGTH(TRIM(payment_reference)) > 0),
  CONSTRAINT chk_delivery_rider_withdrawal_rejected_reason CHECK (
    (status = 'rejected' AND CHAR_LENGTH(TRIM(reason)) > 0)
    OR (status IN ('processing', 'paid'))
  )
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
