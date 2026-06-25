-- G-4: takeout payment attempts and idempotent provider notifications.
-- Domain G keeps logical references only; no cross-domain foreign keys.

CREATE TABLE takeout_payment_attempts (
  id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  payment_public_id CHAR(26) NOT NULL,
  order_public_id CHAR(26) NOT NULL,
  buyer_user_id BIGINT UNSIGNED NOT NULL,
  provider VARCHAR(16) NOT NULL,
  status VARCHAR(24) NOT NULL,
  amount_cents BIGINT UNSIGNED NOT NULL,
  currency VARCHAR(8) NOT NULL DEFAULT 'CNY',
  out_trade_no VARCHAR(64) NOT NULL,
  prepay_id VARCHAR(128) NULL,
  wechat_transaction_id VARCHAR(128) NULL,
  pay_no VARCHAR(64) NULL,
  mock_trace_id VARCHAR(64) NULL,
  row_version BIGINT UNSIGNED NOT NULL,
  created_at DATETIME(3) NOT NULL,
  updated_at DATETIME(3) NOT NULL,
  paid_at DATETIME(3) NULL,
  closed_at DATETIME(3) NULL,
  PRIMARY KEY (id),
  UNIQUE INDEX uq_takeout_payment_public_id (payment_public_id),
  UNIQUE INDEX uq_takeout_payment_out_trade_no (out_trade_no),
  UNIQUE INDEX uq_takeout_payment_wechat_transaction_id (wechat_transaction_id),
  INDEX idx_takeout_payment_order (order_public_id),
  INDEX idx_takeout_payment_buyer (buyer_user_id),
  INDEX idx_takeout_payment_status (status),
  CONSTRAINT chk_takeout_payment_provider CHECK (provider IN ('wechat','mock')),
  CONSTRAINT chk_takeout_payment_status CHECK (status IN ('created','prepay_issued','paid','failed','closed'))
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE takeout_payment_notifications (
  id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  notification_public_id CHAR(26) NOT NULL,
  provider VARCHAR(16) NOT NULL,
  out_trade_no VARCHAR(64) NOT NULL,
  wechat_transaction_id VARCHAR(128) NULL,
  body_sha256 CHAR(64) NOT NULL,
  status VARCHAR(24) NOT NULL,
  processed_at DATETIME(3) NULL,
  created_at DATETIME(3) NOT NULL,
  PRIMARY KEY (id),
  UNIQUE INDEX uq_takeout_payment_notification_public_id (notification_public_id),
  UNIQUE INDEX uq_takeout_payment_notification_body_sha256 (body_sha256),
  INDEX idx_takeout_payment_notification_out_trade_no (out_trade_no),
  INDEX idx_takeout_payment_notification_status (status),
  CONSTRAINT chk_takeout_payment_notification_provider CHECK (provider IN ('wechat','mock')),
  CONSTRAINT chk_takeout_payment_notification_status CHECK (status IN ('received','processed','ignored','failed'))
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
