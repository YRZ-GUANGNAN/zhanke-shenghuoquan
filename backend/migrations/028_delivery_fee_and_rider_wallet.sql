-- D-6a: delivery fee configuration, rider wallet and rider earning settlement.
-- All user/order identifiers are logical references. No cross-domain foreign keys.

CREATE TABLE delivery_fee_configs (
  id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  config_scope VARCHAR(32) NOT NULL,
  delivery_fee_cents BIGINT UNSIGNED NOT NULL,
  platform_cut_cents BIGINT UNSIGNED NOT NULL,
  status VARCHAR(16) NOT NULL,
  row_version BIGINT UNSIGNED NOT NULL,
  created_at DATETIME(3) NOT NULL,
  updated_at DATETIME(3) NOT NULL,
  PRIMARY KEY (id),
  UNIQUE INDEX uq_delivery_fee_config_scope (config_scope),
  INDEX idx_delivery_fee_config_status (status),
  CONSTRAINT chk_delivery_fee_config_status CHECK (status IN ('active','disabled')),
  CONSTRAINT chk_delivery_fee_config_amount CHECK (delivery_fee_cents > 0 AND platform_cut_cents < delivery_fee_cents)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

INSERT INTO delivery_fee_configs
  (config_scope, delivery_fee_cents, platform_cut_cents, status, row_version, created_at, updated_at)
VALUES
  ('global', 150, 50, 'active', 1, UTC_TIMESTAMP(3), UTC_TIMESTAMP(3));

CREATE TABLE delivery_merchant_fee_overrides (
  id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  merchant_user_id BIGINT UNSIGNED NOT NULL,
  delivery_fee_cents BIGINT UNSIGNED NOT NULL,
  platform_cut_cents BIGINT UNSIGNED NOT NULL,
  status VARCHAR(16) NOT NULL,
  row_version BIGINT UNSIGNED NOT NULL,
  created_at DATETIME(3) NOT NULL,
  updated_at DATETIME(3) NOT NULL,
  PRIMARY KEY (id),
  UNIQUE INDEX uq_delivery_merchant_fee_override (merchant_user_id),
  INDEX idx_delivery_merchant_fee_status (status),
  CONSTRAINT chk_delivery_merchant_fee_status CHECK (status IN ('active','disabled')),
  CONSTRAINT chk_delivery_merchant_fee_amount CHECK (delivery_fee_cents > 0 AND platform_cut_cents < delivery_fee_cents)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE delivery_rider_wallets (
  id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  wallet_public_id CHAR(26) NOT NULL,
  rider_user_id BIGINT UNSIGNED NOT NULL,
  available_balance_cents BIGINT UNSIGNED NOT NULL DEFAULT 0,
  row_version BIGINT UNSIGNED NOT NULL,
  created_at DATETIME(3) NOT NULL,
  updated_at DATETIME(3) NOT NULL,
  PRIMARY KEY (id),
  UNIQUE INDEX uq_delivery_rider_wallet_public_id (wallet_public_id),
  UNIQUE INDEX uq_delivery_rider_wallet_rider (rider_user_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE delivery_rider_earning_records (
  id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  earning_public_id CHAR(26) NOT NULL,
  delivery_order_public_id CHAR(26) NOT NULL,
  rider_user_id BIGINT UNSIGNED NOT NULL,
  delivery_fee_cents BIGINT UNSIGNED NOT NULL,
  platform_cut_cents BIGINT UNSIGNED NOT NULL,
  rider_fee_cents BIGINT UNSIGNED NOT NULL,
  settlement_status VARCHAR(24) NOT NULL,
  paid_offline_at DATETIME(3) NULL,
  row_version BIGINT UNSIGNED NOT NULL,
  created_at DATETIME(3) NOT NULL,
  updated_at DATETIME(3) NOT NULL,
  PRIMARY KEY (id),
  UNIQUE INDEX uq_delivery_rider_earning_public_id (earning_public_id),
  UNIQUE INDEX uq_delivery_rider_earning_order (delivery_order_public_id),
  INDEX idx_delivery_rider_earning_rider_status (rider_user_id, settlement_status, id),
  INDEX idx_delivery_rider_earning_status (settlement_status),
  CONSTRAINT chk_delivery_rider_earning_status CHECK (settlement_status IN ('pending','settled','paid_offline')),
  CONSTRAINT chk_delivery_rider_earning_amount CHECK (
    delivery_fee_cents > 0 AND
    rider_fee_cents > 0 AND
    platform_cut_cents < delivery_fee_cents AND
    delivery_fee_cents = rider_fee_cents + platform_cut_cents
  )
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE delivery_rider_payout_records (
  id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  payout_public_id CHAR(26) NOT NULL,
  earning_public_id CHAR(26) NOT NULL,
  delivery_order_public_id CHAR(26) NOT NULL,
  rider_user_id BIGINT UNSIGNED NOT NULL,
  operator_user_id BIGINT UNSIGNED NOT NULL,
  amount_cents BIGINT UNSIGNED NOT NULL,
  idempotency_key CHAR(26) NOT NULL,
  note VARCHAR(256) NOT NULL DEFAULT '',
  paid_offline_at DATETIME(3) NOT NULL,
  row_version BIGINT UNSIGNED NOT NULL,
  created_at DATETIME(3) NOT NULL,
  updated_at DATETIME(3) NOT NULL,
  PRIMARY KEY (id),
  UNIQUE INDEX uq_delivery_rider_payout_public_id (payout_public_id),
  UNIQUE INDEX uq_delivery_rider_payout_earning (earning_public_id),
  UNIQUE INDEX uq_delivery_rider_payout_order (delivery_order_public_id),
  UNIQUE INDEX uq_delivery_rider_payout_idempotency (idempotency_key),
  INDEX idx_delivery_rider_payout_rider_created (rider_user_id, created_at, id),
  CONSTRAINT chk_delivery_rider_payout_amount CHECK (amount_cents > 0)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE delivery_rider_ledger (
  id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  ledger_public_id CHAR(26) NOT NULL,
  wallet_public_id CHAR(26) NOT NULL,
  rider_user_id BIGINT UNSIGNED NOT NULL,
  direction VARCHAR(16) NOT NULL,
  entry_type VARCHAR(24) NOT NULL,
  amount_cents BIGINT UNSIGNED NOT NULL,
  balance_before_cents BIGINT UNSIGNED NOT NULL,
  balance_after_cents BIGINT UNSIGNED NOT NULL,
  reference_type VARCHAR(32) NOT NULL,
  reference_public_id CHAR(26) NOT NULL,
  idempotency_key CHAR(26) NOT NULL,
  created_at DATETIME(3) NOT NULL,
  PRIMARY KEY (id),
  UNIQUE INDEX uq_delivery_rider_ledger_public_id (ledger_public_id),
  UNIQUE INDEX uq_delivery_rider_ledger_idempotency (idempotency_key),
  INDEX idx_delivery_rider_ledger_wallet_created (wallet_public_id, created_at, id),
  INDEX idx_delivery_rider_ledger_rider_created (rider_user_id, created_at, id),
  INDEX idx_delivery_rider_ledger_reference (reference_type, reference_public_id),
  CONSTRAINT chk_delivery_rider_ledger_direction CHECK (direction IN ('credit','debit')),
  CONSTRAINT chk_delivery_rider_ledger_entry_type CHECK (entry_type IN ('income','adjustment','payout')),
  CONSTRAINT chk_delivery_rider_ledger_balance CHECK (
    (direction='credit' AND balance_after_cents=balance_before_cents+amount_cents)
    OR
    (direction='debit' AND balance_before_cents=balance_after_cents+amount_cents)
  )
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
