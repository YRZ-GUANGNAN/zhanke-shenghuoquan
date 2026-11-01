-- Persist merchant cancellation intent independently of provider observations.
-- Do not infer cancellation for historical payments.
ALTER TABLE delivery_payment_attempts
  ADD COLUMN cancel_requested_at DATETIME(3) NULL AFTER closed_at;
