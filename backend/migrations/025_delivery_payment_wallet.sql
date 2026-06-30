-- D-3: delivery merchant wallet, payments, settlements and refunds.
-- All user/order identifiers are logical references. No cross-domain foreign keys.

CREATE TABLE delivery_merchant_wallets (
  id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  wallet_public_id CHAR(26) NOT NULL,
  merchant_user_id BIGINT UNSIGNED NOT NULL,
  available_balance_cents BIGINT UNSIGNED NOT NULL DEFAULT 0,
  row_version BIGINT UNSIGNED NOT NULL,
  created_at DATETIME(3) NOT NULL,
  updated_at DATETIME(3) NOT NULL,
  PRIMARY KEY (id),
  UNIQUE INDEX uq_delivery_wallet_public_id (wallet_public_id),
  UNIQUE INDEX uq_delivery_wallet_merchant (merchant_user_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE delivery_wallet_ledger (
  id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  ledger_public_id CHAR(26) NOT NULL,
  wallet_public_id CHAR(26) NOT NULL,
  merchant_user_id BIGINT UNSIGNED NOT NULL,
  direction VARCHAR(16) NOT NULL,
  entry_type VARCHAR(24) NOT NULL,
  amount_cents BIGINT UNSIGNED NOT NULL,
  balance_before_cents BIGINT UNSIGNED NOT NULL,
  balance_after_cents BIGINT UNSIGNED NOT NULL,
  reference_type VARCHAR(24) NOT NULL,
  reference_public_id CHAR(26) NOT NULL,
  idempotency_key CHAR(26) NOT NULL,
  created_at DATETIME(3) NOT NULL,
  PRIMARY KEY (id),
  UNIQUE INDEX uq_delivery_ledger_public_id (ledger_public_id),
  UNIQUE INDEX uq_delivery_ledger_idempotency (idempotency_key),
  INDEX idx_delivery_ledger_wallet_created (wallet_public_id, created_at, id),
  INDEX idx_delivery_ledger_merchant_created (merchant_user_id, created_at, id),
  INDEX idx_delivery_ledger_reference (reference_type, reference_public_id),
  CONSTRAINT chk_delivery_ledger_direction CHECK (direction IN ('credit','debit')),
  CONSTRAINT chk_delivery_ledger_entry_type CHECK (entry_type IN ('recharge','order_fee','refund')),
  CONSTRAINT chk_delivery_ledger_balance CHECK (
    (direction='credit' AND balance_after_cents=balance_before_cents+amount_cents)
    OR
    (direction='debit' AND balance_before_cents=balance_after_cents+amount_cents)
  )
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE delivery_payment_attempts (
  id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  payment_public_id CHAR(26) NOT NULL,
  merchant_user_id BIGINT UNSIGNED NOT NULL,
  purpose VARCHAR(24) NOT NULL,
  delivery_order_public_id CHAR(26) NULL,
  provider VARCHAR(16) NOT NULL,
  status VARCHAR(24) NOT NULL,
  amount_cents BIGINT UNSIGNED NOT NULL,
  currency VARCHAR(8) NOT NULL DEFAULT 'CNY',
  client_request_id CHAR(26) NOT NULL,
  out_trade_no VARCHAR(64) NOT NULL,
  prepay_id VARCHAR(128) NULL,
  wechat_transaction_id VARCHAR(128) NULL,
  mock_trace_id VARCHAR(64) NULL,
  row_version BIGINT UNSIGNED NOT NULL,
  created_at DATETIME(3) NOT NULL,
  updated_at DATETIME(3) NOT NULL,
  paid_at DATETIME(3) NULL,
  closed_at DATETIME(3) NULL,
  PRIMARY KEY (id),
  UNIQUE INDEX uq_delivery_payment_public_id (payment_public_id),
  UNIQUE INDEX uq_delivery_payment_out_trade_no (out_trade_no),
  UNIQUE INDEX uq_delivery_payment_wechat_transaction (wechat_transaction_id),
  UNIQUE INDEX uq_delivery_payment_client_request (merchant_user_id, purpose, client_request_id),
  INDEX idx_delivery_payment_order (delivery_order_public_id),
  INDEX idx_delivery_payment_merchant_created (merchant_user_id, created_at, id),
  INDEX idx_delivery_payment_status (status),
  CONSTRAINT chk_delivery_payment_purpose CHECK (purpose IN ('order_fee','wallet_recharge')),
  CONSTRAINT chk_delivery_payment_provider CHECK (provider IN ('wechat','mock')),
  CONSTRAINT chk_delivery_payment_status CHECK (status IN ('created','prepay_issued','paid','failed','closed')),
  CONSTRAINT chk_delivery_payment_reference CHECK (
    (purpose='order_fee' AND delivery_order_public_id IS NOT NULL)
    OR
    (purpose='wallet_recharge' AND delivery_order_public_id IS NULL)
  )
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE delivery_payment_notifications (
  id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  notification_public_id CHAR(26) NOT NULL,
  out_trade_no VARCHAR(64) NOT NULL,
  wechat_transaction_id VARCHAR(128) NOT NULL,
  body_sha256 CHAR(64) NOT NULL,
  status VARCHAR(24) NOT NULL,
  processed_at DATETIME(3) NULL,
  created_at DATETIME(3) NOT NULL,
  PRIMARY KEY (id),
  UNIQUE INDEX uq_delivery_notification_public_id (notification_public_id),
  UNIQUE INDEX uq_delivery_notification_transaction (wechat_transaction_id),
  UNIQUE INDEX uq_delivery_notification_body_sha256 (body_sha256),
  INDEX idx_delivery_notification_out_trade_no (out_trade_no),
  INDEX idx_delivery_notification_status (status),
  CONSTRAINT chk_delivery_notification_status CHECK (status IN ('received','processed','ignored','failed'))
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE delivery_fee_settlements (
  id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  settlement_public_id CHAR(26) NOT NULL,
  delivery_order_public_id CHAR(26) NOT NULL,
  merchant_user_id BIGINT UNSIGNED NOT NULL,
  source VARCHAR(16) NOT NULL,
  amount_cents BIGINT UNSIGNED NOT NULL,
  payment_public_id CHAR(26) NULL,
  ledger_public_id CHAR(26) NULL,
  status VARCHAR(24) NOT NULL,
  paid_at DATETIME(3) NOT NULL,
  refunded_at DATETIME(3) NULL,
  row_version BIGINT UNSIGNED NOT NULL,
  PRIMARY KEY (id),
  UNIQUE INDEX uq_delivery_settlement_public_id (settlement_public_id),
  UNIQUE INDEX uq_delivery_settlement_order (delivery_order_public_id),
  UNIQUE INDEX uq_delivery_settlement_payment (payment_public_id),
  UNIQUE INDEX uq_delivery_settlement_ledger (ledger_public_id),
  INDEX idx_delivery_settlement_merchant (merchant_user_id),
  INDEX idx_delivery_settlement_status (status),
  CONSTRAINT chk_delivery_settlement_source CHECK (source IN ('wallet','wechat','mock')),
  CONSTRAINT chk_delivery_settlement_status CHECK (status IN ('paid','refund_pending','refunded'))
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE delivery_refunds (
  id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  refund_public_id CHAR(26) NOT NULL,
  settlement_public_id CHAR(26) NOT NULL,
  delivery_order_public_id CHAR(26) NOT NULL,
  merchant_user_id BIGINT UNSIGNED NOT NULL,
  source VARCHAR(16) NOT NULL,
  amount_cents BIGINT UNSIGNED NOT NULL,
  status VARCHAR(24) NOT NULL,
  idempotency_key CHAR(26) NOT NULL,
  out_refund_no VARCHAR(64) NOT NULL,
  wechat_refund_id VARCHAR(128) NULL,
  failure_reason VARCHAR(512) NOT NULL DEFAULT '',
  row_version BIGINT UNSIGNED NOT NULL,
  created_at DATETIME(3) NOT NULL,
  updated_at DATETIME(3) NOT NULL,
  succeeded_at DATETIME(3) NULL,
  PRIMARY KEY (id),
  UNIQUE INDEX uq_delivery_refund_public_id (refund_public_id),
  UNIQUE INDEX uq_delivery_refund_settlement (settlement_public_id),
  UNIQUE INDEX uq_delivery_refund_idempotency (idempotency_key),
  UNIQUE INDEX uq_delivery_refund_out_refund_no (out_refund_no),
  UNIQUE INDEX uq_delivery_refund_wechat_id (wechat_refund_id),
  INDEX idx_delivery_refund_order (delivery_order_public_id),
  INDEX idx_delivery_refund_status (status),
  CONSTRAINT chk_delivery_refund_source CHECK (source IN ('wallet','wechat','mock')),
  CONSTRAINT chk_delivery_refund_status CHECK (status IN ('pending','succeeded','failed','refund_pending'))
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
