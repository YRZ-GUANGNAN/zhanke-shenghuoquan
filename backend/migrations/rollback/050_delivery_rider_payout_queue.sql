-- Refuse destructive rollback while any durable payout fact exists.

DELIMITER $$
DROP PROCEDURE IF EXISTS rollback_delivery_rider_payout_queue$$
CREATE PROCEDURE rollback_delivery_rider_payout_queue()
SQL SECURITY INVOKER
BEGIN
  DECLARE row_count BIGINT DEFAULT 0;

  SELECT COUNT(*) INTO row_count FROM delivery_rider_payout_events;
  IF row_count <> 0 THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT = 'migration 050 rollback blocked: payout events are not empty';
  END IF;

  SELECT COUNT(*) INTO row_count FROM delivery_rider_payout_reservations;
  IF row_count <> 0 THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT = 'migration 050 rollback blocked: payout reservations are not empty';
  END IF;

  SELECT COUNT(*) INTO row_count FROM delivery_rider_payout_queue;
  IF row_count <> 0 THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT = 'migration 050 rollback blocked: payout queue is not empty';
  END IF;

  DROP TABLE IF EXISTS delivery_rider_payout_events;
  DROP TABLE IF EXISTS delivery_rider_payout_reservations;
  DROP TABLE IF EXISTS delivery_rider_payout_queue;
END$$
CALL rollback_delivery_rider_payout_queue()$$
DROP PROCEDURE rollback_delivery_rider_payout_queue$$
DELIMITER ;
