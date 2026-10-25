-- Remove only metadata added by migration 048.

ALTER TABLE delivery_refunds
  DROP INDEX idx_delivery_refund_out_trade_no,
  DROP INDEX idx_delivery_refund_verify,
  DROP COLUMN last_error_code,
  DROP COLUMN verify_attempt_count,
  DROP COLUMN verify_lease_until,
  DROP COLUMN next_verify_at,
  DROP COLUMN provider_status,
  DROP COLUMN out_trade_no;

ALTER TABLE delivery_payment_attempts
  DROP INDEX idx_delivery_payment_verify,
  DROP COLUMN last_error_code,
  DROP COLUMN verify_attempt_count,
  DROP COLUMN verify_lease_until,
  DROP COLUMN next_verify_at,
  DROP COLUMN provider_status,
  DROP COLUMN payer_openid;
