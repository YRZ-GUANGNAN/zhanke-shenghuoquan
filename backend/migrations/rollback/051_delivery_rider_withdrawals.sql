-- Refuse rollback while withdrawal facts exist, so financial history cannot
-- be silently removed.

DELIMITER $$
DROP PROCEDURE IF EXISTS rollback_delivery_rider_withdrawals$$
CREATE PROCEDURE rollback_delivery_rider_withdrawals()
SQL SECURITY INVOKER
BEGIN
  DECLARE row_count BIGINT DEFAULT 0;
  SELECT COUNT(*) INTO row_count FROM delivery_rider_withdrawals;
  IF row_count <> 0 THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT = 'migration 051 rollback blocked: rider withdrawals are not empty';
  END IF;
  DROP TABLE IF EXISTS delivery_rider_withdrawals;
END$$
CALL rollback_delivery_rider_withdrawals()$$
DROP PROCEDURE rollback_delivery_rider_withdrawals$$
DELIMITER ;
