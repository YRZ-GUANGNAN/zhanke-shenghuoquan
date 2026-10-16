-- Delivery source metadata only. No platform connection or payment is enabled.
-- Additive and repeatable; retain these columns if application code is rolled back.
DELIMITER $$
DROP PROCEDURE IF EXISTS migrate_delivery_order_source$$
CREATE PROCEDURE migrate_delivery_order_source()
SQL SECURITY INVOKER
BEGIN
  DECLARE column_count INT DEFAULT 0;
  DECLARE signature_count INT DEFAULT 0;
  DECLARE lock_acquired INT DEFAULT 0;
  DECLARE EXIT HANDLER FOR SQLEXCEPTION
  BEGIN
    IF lock_acquired = 1 THEN
      DO RELEASE_LOCK('campus_circle:migration:045');
    END IF;
    RESIGNAL;
  END;
  SELECT GET_LOCK('campus_circle:migration:045', 10) INTO lock_acquired;
  IF lock_acquired IS NULL OR lock_acquired <> 1 THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT = 'migration 045 lock unavailable';
  END IF;
  SELECT COUNT(*) INTO column_count FROM information_schema.columns
    WHERE table_schema=DATABASE() AND table_name='delivery_orders'
      AND column_name IN ('source_platform','pickup_code','source_shop_id','source_order_id');
  IF column_count = 0 THEN
    ALTER TABLE delivery_orders
      ADD COLUMN source_platform VARCHAR(12) CHARACTER SET ascii COLLATE ascii_bin NOT NULL DEFAULT 'local' AFTER merchant_request_id,
      ADD COLUMN pickup_code VARCHAR(16) NOT NULL DEFAULT '' AFTER source_platform,
      ADD COLUMN source_shop_id VARCHAR(64) CHARACTER SET utf8mb4 COLLATE utf8mb4_0900_bin NULL AFTER pickup_code,
      ADD COLUMN source_order_id VARCHAR(128) CHARACTER SET utf8mb4 COLLATE utf8mb4_0900_bin NULL AFTER source_shop_id,
      ADD CONSTRAINT chk_delivery_order_source_platform CHECK (source_platform IN ('local','taobao','jd','meituan')),
      ADD CONSTRAINT chk_delivery_order_source_identity CHECK (
        (source_shop_id IS NULL AND source_order_id IS NULL) OR
        (source_platform <> 'local' AND source_shop_id IS NOT NULL AND source_order_id IS NOT NULL
          AND CHAR_LENGTH(TRIM(source_shop_id)) > 0 AND CHAR_LENGTH(TRIM(source_order_id)) > 0)
      ),
      ADD UNIQUE INDEX uq_delivery_order_source (merchant_user_id, source_platform, source_shop_id, source_order_id);
  ELSEIF column_count <> 4 THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT = 'migration 045 partial source schema';
  END IF;
  SELECT COUNT(*) INTO signature_count FROM information_schema.columns
    WHERE table_schema=DATABASE() AND table_name='delivery_orders' AND (
      (column_name='source_platform' AND column_type='varchar(12)' AND collation_name='ascii_bin' AND is_nullable='NO' AND column_default='local') OR
      (column_name='pickup_code' AND column_type='varchar(16)' AND is_nullable='NO' AND column_default='') OR
      (column_name='source_shop_id' AND column_type='varchar(64)' AND collation_name='utf8mb4_0900_bin' AND is_nullable='YES') OR
      (column_name='source_order_id' AND column_type='varchar(128)' AND collation_name='utf8mb4_0900_bin' AND is_nullable='YES')
    );
  IF signature_count <> 4 THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT = 'migration 045 source schema mismatch';
  END IF;
  SELECT COUNT(*) INTO signature_count FROM information_schema.statistics
    WHERE table_schema=DATABASE() AND table_name='delivery_orders' AND index_name='uq_delivery_order_source'
      AND non_unique=0 AND sub_part IS NULL AND (
        (seq_in_index=1 AND column_name='merchant_user_id') OR
        (seq_in_index=2 AND column_name='source_platform') OR
        (seq_in_index=3 AND column_name='source_shop_id') OR
        (seq_in_index=4 AND column_name='source_order_id')
      );
  IF signature_count <> 4 THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT = 'migration 045 source unique index mismatch';
  END IF;
  SELECT COUNT(*) INTO signature_count FROM information_schema.table_constraints
    WHERE table_schema=DATABASE() AND table_name='delivery_orders' AND constraint_type='CHECK' AND enforced='YES'
      AND constraint_name IN ('chk_delivery_order_source_platform','chk_delivery_order_source_identity');
  IF signature_count <> 2 THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT = 'migration 045 source checks missing';
  END IF;
  DO RELEASE_LOCK('campus_circle:migration:045');
END$$
CALL migrate_delivery_order_source()$$
DROP PROCEDURE migrate_delivery_order_source$$
DELIMITER ;
