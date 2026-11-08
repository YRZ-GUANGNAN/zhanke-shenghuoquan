-- Refuse destructive rollback once allocation or processing audit facts exist.
DELIMITER $$
DROP PROCEDURE IF EXISTS rollback_056_delivery_rider_withdrawal_allocations$$
CREATE PROCEDURE rollback_056_delivery_rider_withdrawal_allocations()
SQL SECURITY INVOKER
BEGIN
  DECLARE row_count BIGINT DEFAULT 0;
  IF EXISTS (SELECT 1 FROM information_schema.tables WHERE table_schema=DATABASE() AND table_name='delivery_rider_withdrawal_allocations') THEN
    SELECT COUNT(*) INTO row_count FROM delivery_rider_withdrawal_allocations;
    IF row_count > 0 THEN SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT = 'withdrawal allocation facts must be preserved'; END IF;
  END IF;
  IF EXISTS (SELECT 1 FROM information_schema.tables WHERE table_schema=DATABASE() AND table_name='delivery_rider_withdrawal_events') THEN
    SELECT COUNT(*) INTO row_count FROM delivery_rider_withdrawal_events;
    IF row_count > 0 THEN SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT = 'withdrawal processing audit must be preserved'; END IF;
  END IF;
  IF EXISTS (SELECT 1 FROM information_schema.tables WHERE table_schema=DATABASE() AND table_name='delivery_rider_withdrawal_claims') THEN
    SELECT COUNT(*) INTO row_count FROM delivery_rider_withdrawal_claims;
    IF row_count > 0 THEN SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT = 'withdrawal processing claims must be preserved'; END IF;
  END IF;
  IF EXISTS (SELECT 1 FROM information_schema.tables WHERE table_schema=DATABASE() AND table_name='delivery_rider_withdrawal_payment_references') THEN
    SELECT COUNT(*) INTO row_count FROM delivery_rider_withdrawal_payment_references;
    IF row_count > 0 THEN SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT = 'withdrawal payment references must be preserved'; END IF;
  END IF;
  DROP TABLE IF EXISTS delivery_rider_withdrawal_payment_references;
  DROP TABLE IF EXISTS delivery_rider_withdrawal_claims;
  DROP TABLE IF EXISTS delivery_rider_withdrawal_events;
  DROP TABLE IF EXISTS delivery_rider_withdrawal_allocations;
END$$
DELIMITER ;
CALL rollback_056_delivery_rider_withdrawal_allocations();
DROP PROCEDURE rollback_056_delivery_rider_withdrawal_allocations;
