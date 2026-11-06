-- Refuse destructive rollback once duty or capacity facts exist.
DELIMITER $$
DROP PROCEDURE IF EXISTS rollback_055_delivery_rider_availability$$
CREATE PROCEDURE rollback_055_delivery_rider_availability()
SQL SECURITY INVOKER
BEGIN
  DECLARE row_count BIGINT DEFAULT 0;
  IF EXISTS (SELECT 1 FROM information_schema.tables WHERE table_schema=DATABASE() AND table_name='delivery_rider_availability') THEN
    SELECT COUNT(*) INTO row_count FROM delivery_rider_availability;
    IF row_count > 0 THEN SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT = 'rider availability facts must be preserved'; END IF;
  END IF;
  IF EXISTS (SELECT 1 FROM information_schema.tables WHERE table_schema=DATABASE() AND table_name='delivery_rider_broadcast_config') THEN
    SELECT COUNT(*) INTO row_count FROM delivery_rider_broadcast_config
      WHERE config_scope <> 'global' OR max_in_flight_per_rider <> 3 OR row_version <> 1 OR updated_at <> created_at;
    IF row_count > 0 THEN SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT = 'rider broadcast configuration changed'; END IF;
    SELECT COUNT(*) INTO row_count FROM delivery_rider_broadcast_config;
    IF row_count > 1 THEN SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT = 'rider broadcast configuration changed'; END IF;
  END IF;
  DROP TABLE IF EXISTS delivery_rider_availability;
  DROP TABLE IF EXISTS delivery_rider_broadcast_config;
END$$
DELIMITER ;
CALL rollback_055_delivery_rider_availability();
DROP PROCEDURE rollback_055_delivery_rider_availability;
