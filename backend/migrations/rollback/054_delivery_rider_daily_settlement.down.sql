-- Refuse rollback while any earning fact exists. The forward migration is
-- irreversible with live money rows; archive them before removing columns.
CREATE TEMPORARY TABLE rider_daily_settlement_rollback_guard (
  valid TINYINT NOT NULL CHECK (valid = 1)
);
INSERT INTO rider_daily_settlement_rollback_guard (valid)
SELECT CASE WHEN (SELECT COUNT(*) FROM delivery_rider_earning_records) = 0 THEN 1 ELSE 0 END;
DROP TEMPORARY TABLE rider_daily_settlement_rollback_guard;

ALTER TABLE delivery_rider_earning_records
  DROP INDEX idx_delivery_rider_earning_due,
  DROP COLUMN settled_at,
  DROP COLUMN settle_at;
