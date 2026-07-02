-- D-6a rollback. Rider money facts must be explicitly archived/cleared first.
-- The guard below deliberately fails when rider wallet/earning/payout facts exist.

CREATE TEMPORARY TABLE IF NOT EXISTS d6a_delivery_rollback_guard (
  must_be_empty TINYINT NOT NULL,
  CONSTRAINT chk_d6a_delivery_rollback_guard CHECK (must_be_empty = 1)
);

INSERT INTO d6a_delivery_rollback_guard (must_be_empty)
SELECT CASE WHEN (
  SELECT COUNT(*) FROM delivery_rider_ledger
) + (
  SELECT COUNT(*) FROM delivery_rider_payout_records
) + (
  SELECT COUNT(*) FROM delivery_rider_earning_records
) + (
  SELECT COUNT(*) FROM delivery_rider_wallets
) = 0 THEN 1 ELSE 0 END;

DROP TEMPORARY TABLE IF EXISTS d6a_delivery_rollback_guard;

DROP TABLE IF EXISTS delivery_rider_ledger;
DROP TABLE IF EXISTS delivery_rider_payout_records;
DROP TABLE IF EXISTS delivery_rider_earning_records;
DROP TABLE IF EXISTS delivery_rider_wallets;
DROP TABLE IF EXISTS delivery_merchant_fee_overrides;
DROP TABLE IF EXISTS delivery_fee_configs;
