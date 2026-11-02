-- Stop delivery and all money writers before applying this one-time migration.
-- Legacy pending earnings were ALREADY credited at delivery. Require their
-- exact income ledger and wallet before doing any DDL; never credit them again.
-- All DATETIME values are UTC; new earnings use next Asia/Shanghai midnight.
CREATE TEMPORARY TABLE rider_daily_settlement_migration_guard (
  valid TINYINT NOT NULL CHECK (valid = 1)
);

INSERT INTO rider_daily_settlement_migration_guard (valid)
SELECT CASE WHEN NOT EXISTS (
  SELECT 1 FROM delivery_rider_earning_records e
  WHERE (SELECT COUNT(*) FROM delivery_rider_ledger l
    JOIN delivery_rider_wallets w ON w.wallet_public_id = l.wallet_public_id
    WHERE l.idempotency_key = e.delivery_order_public_id
      AND l.reference_type = 'delivery_order'
      AND l.reference_public_id = e.delivery_order_public_id
      AND l.rider_user_id = e.rider_user_id AND w.rider_user_id = e.rider_user_id
      AND l.direction = 'credit' AND l.entry_type = 'income'
      AND l.amount_cents = e.rider_fee_cents
      AND l.balance_after_cents = l.balance_before_cents + l.amount_cents) <> 1
) THEN 1 ELSE 0 END;

DROP TEMPORARY TABLE rider_daily_settlement_migration_guard;

ALTER TABLE delivery_rider_earning_records
  ADD COLUMN settle_at DATETIME(3) NULL AFTER settlement_status,
  ADD COLUMN settled_at DATETIME(3) NULL AFTER settle_at;

-- Preserve historical balances, ledger keys, timestamps and paid_offline facts.
-- The ledger records the real old credit time, not a fabricated T+1 timestamp.
UPDATE delivery_rider_earning_records e
JOIN delivery_rider_ledger l ON l.idempotency_key = e.delivery_order_public_id
SET e.settle_at = l.created_at, e.settled_at = l.created_at,
    e.row_version = e.row_version + 1,
    e.settlement_status = CASE WHEN e.settlement_status = 'pending' THEN 'settled' ELSE e.settlement_status END
WHERE e.settle_at IS NULL;

ALTER TABLE delivery_rider_earning_records
  MODIFY COLUMN settle_at DATETIME(3) NOT NULL,
  ADD INDEX idx_delivery_rider_earning_due (settlement_status, settle_at, id);
