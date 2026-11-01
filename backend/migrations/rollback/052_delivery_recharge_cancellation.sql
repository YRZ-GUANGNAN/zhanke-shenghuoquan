-- Roll back only after all requested cancellations have reached a terminal state.
DELIMITER $$
DROP PROCEDURE IF EXISTS rollback_052_delivery_recharge_cancellation$$
CREATE PROCEDURE rollback_052_delivery_recharge_cancellation()
SQL SECURITY INVOKER
BEGIN
  IF EXISTS (SELECT 1 FROM delivery_payment_attempts
             WHERE cancel_requested_at IS NOT NULL AND status IN ('created', 'prepay_issued')) THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT = 'pending recharge cancellation exists';
  END IF;
  ALTER TABLE delivery_payment_attempts DROP COLUMN cancel_requested_at;
END$$
CALL rollback_052_delivery_recharge_cancellation()$$
DROP PROCEDURE rollback_052_delivery_recharge_cancellation$$
DELIMITER ;
