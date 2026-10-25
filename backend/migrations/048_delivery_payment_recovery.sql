-- Delivery payment recovery metadata.
-- Historical rows remain unverified: nullable payer/source references are never backfilled.

ALTER TABLE delivery_payment_attempts
  ADD COLUMN payer_openid VARCHAR(128) NULL AFTER prepay_id,
  ADD COLUMN provider_status VARCHAR(32) NOT NULL DEFAULT '' AFTER mock_trace_id,
  ADD COLUMN next_verify_at DATETIME(3) NULL AFTER provider_status,
  ADD COLUMN verify_lease_until DATETIME(3) NULL AFTER next_verify_at,
  ADD COLUMN verify_attempt_count INT UNSIGNED NOT NULL DEFAULT 0 AFTER verify_lease_until,
  ADD COLUMN last_error_code VARCHAR(64) NOT NULL DEFAULT '' AFTER verify_attempt_count,
  ADD INDEX idx_delivery_payment_verify (status, next_verify_at, verify_lease_until);

ALTER TABLE delivery_refunds
  ADD COLUMN out_trade_no VARCHAR(64) NULL AFTER out_refund_no,
  ADD COLUMN provider_status VARCHAR(32) NOT NULL DEFAULT '' AFTER failure_reason,
  ADD COLUMN next_verify_at DATETIME(3) NULL AFTER provider_status,
  ADD COLUMN verify_lease_until DATETIME(3) NULL AFTER next_verify_at,
  ADD COLUMN verify_attempt_count INT UNSIGNED NOT NULL DEFAULT 0 AFTER verify_lease_until,
  ADD COLUMN last_error_code VARCHAR(64) NOT NULL DEFAULT '' AFTER verify_attempt_count,
  ADD INDEX idx_delivery_refund_verify (status, next_verify_at, verify_lease_until),
  ADD INDEX idx_delivery_refund_out_trade_no (out_trade_no);
