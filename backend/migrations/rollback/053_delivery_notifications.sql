-- Notification events, authorizations and receipts are durable facts. Refuse
-- destructive rollback after any row exists; roll application code back first.
DELIMITER $$
DROP PROCEDURE IF EXISTS rollback_053_delivery_notifications$$
CREATE PROCEDURE rollback_053_delivery_notifications()
SQL SECURITY INVOKER
BEGIN
  DECLARE row_count BIGINT DEFAULT 0;
  IF EXISTS (SELECT 1 FROM information_schema.tables WHERE table_schema=DATABASE() AND table_name='im_notification_receipts') THEN
    SELECT COUNT(*) INTO row_count FROM im_notification_receipts;
    IF row_count > 0 THEN SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT = 'notification receipts must be preserved'; END IF;
  END IF;
  IF EXISTS (SELECT 1 FROM information_schema.tables WHERE table_schema=DATABASE() AND table_name='notification_subscriptions') THEN
    SELECT COUNT(*) INTO row_count FROM notification_subscriptions;
    IF row_count > 0 THEN SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT = 'notification authorizations must be preserved'; END IF;
  END IF;
  IF EXISTS (SELECT 1 FROM information_schema.tables WHERE table_schema=DATABASE() AND table_name='delivery_notification_deliveries') THEN
    SELECT COUNT(*) INTO row_count FROM delivery_notification_deliveries;
    IF row_count > 0 THEN SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT = 'notification deliveries must be preserved'; END IF;
  END IF;
  IF EXISTS (SELECT 1 FROM information_schema.tables WHERE table_schema=DATABASE() AND table_name='delivery_notification_inbox') THEN
    SELECT COUNT(*) INTO row_count FROM delivery_notification_inbox;
    IF row_count > 0 THEN SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT = 'notification inbox must be preserved'; END IF;
  END IF;
  IF EXISTS (SELECT 1 FROM information_schema.tables WHERE table_schema=DATABASE() AND table_name='delivery_notification_events') THEN
    SELECT COUNT(*) INTO row_count FROM delivery_notification_events;
    IF row_count > 0 THEN SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT = 'notification events must be preserved'; END IF;
  END IF;
  DROP TABLE IF EXISTS im_notification_receipts;
  DROP TABLE IF EXISTS notification_subscriptions;
  DROP TABLE IF EXISTS delivery_notification_deliveries;
  DROP TABLE IF EXISTS delivery_notification_inbox;
  DROP TABLE IF EXISTS delivery_notification_events;
END$$
CALL rollback_053_delivery_notifications()$$
DROP PROCEDURE rollback_053_delivery_notifications$$
DELIMITER ;
