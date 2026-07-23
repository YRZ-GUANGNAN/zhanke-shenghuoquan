-- External takeout merchant directory. This migration is intentionally
-- forward-only: the schema and operator-entered merchant data are retained
-- when an application release is rolled back.

DELIMITER $$
DROP PROCEDURE IF EXISTS migrate_external_takeout_merchant_directory$$
CREATE PROCEDURE migrate_external_takeout_merchant_directory()
SQL SECURITY INVOKER
BEGIN
  DECLARE schema_object_count INT DEFAULT 0;
  DECLARE same_name_count INT DEFAULT 0;
  DECLARE system_category_count INT DEFAULT 0;
  DECLARE duplicate_target_count INT DEFAULT 0;
  DECLARE invalid_legacy_item_count INT DEFAULT 0;
  DECLARE lock_acquired INT DEFAULT 0;
  DECLARE signature_ok INT DEFAULT 0;

  DECLARE EXIT HANDLER FOR SQLEXCEPTION
  BEGIN
    IF lock_acquired = 1 THEN
      DO RELEASE_LOCK('campus_circle:migration:031');
    END IF;
    RESIGNAL;
  END;

  SELECT GET_LOCK('campus_circle:migration:031', 10) INTO lock_acquired;
  IF lock_acquired IS NULL OR lock_acquired <> 1 THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT = 'migration 031 lock unavailable';
  END IF;

  IF (SELECT COUNT(*) FROM information_schema.tables
      WHERE table_schema=DATABASE() AND table_name IN ('navigation_categories','navigation_items')) <> 2 THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT = 'migration 031 base tables missing';
  END IF;

  SELECT COUNT(*) INTO signature_ok
  FROM information_schema.columns
  WHERE table_schema=DATABASE() AND table_name='navigation_categories' AND (
    (column_name='id' AND column_type='bigint unsigned' AND is_nullable='NO' AND column_key='PRI' AND extra LIKE '%auto_increment%') OR
    (column_name='name' AND data_type='varchar' AND character_maximum_length=64 AND is_nullable='NO') OR
    (column_name='sort' AND column_type='bigint' AND is_nullable='NO') OR
    (column_name='enabled' AND column_type='tinyint(1)' AND is_nullable='NO') OR
    (column_name='created_at' AND data_type='datetime' AND datetime_precision=3 AND is_nullable='YES') OR
    (column_name='updated_at' AND data_type='datetime' AND datetime_precision=3 AND is_nullable='YES')
  );
  IF signature_ok <> 6 THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT = 'migration 031 base schema mismatch';
  END IF;

  SELECT COUNT(*) INTO signature_ok
  FROM information_schema.columns
  WHERE table_schema=DATABASE() AND table_name='navigation_items' AND (
    (column_name='id' AND column_type='bigint unsigned' AND is_nullable='NO' AND column_key='PRI' AND extra LIKE '%auto_increment%') OR
    (column_name='category_id' AND column_type='bigint unsigned' AND is_nullable='NO') OR
    (column_name='title' AND data_type='varchar' AND character_maximum_length=64 AND is_nullable='NO') OR
    (column_name='subtitle' AND data_type='varchar' AND character_maximum_length=120 AND is_nullable='NO') OR
    (column_name='icon_text' AND data_type='varchar' AND character_maximum_length=12 AND is_nullable='NO') OR
    (column_name='app_id' AND data_type='varchar' AND character_maximum_length=80 AND is_nullable='NO') OR
    (column_name='path' AND data_type='varchar' AND character_maximum_length=240 AND is_nullable='NO') OR
    (column_name='color' AND data_type='varchar' AND character_maximum_length=24 AND is_nullable='NO') OR
    (column_name='sort' AND column_type='bigint' AND is_nullable='NO') OR
    (column_name='enabled' AND column_type='tinyint(1)' AND is_nullable='NO') OR
    (column_name='created_at' AND data_type='datetime' AND datetime_precision=3 AND is_nullable='YES') OR
    (column_name='updated_at' AND data_type='datetime' AND datetime_precision=3 AND is_nullable='YES')
  );
  IF signature_ok <> 12 THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT = 'migration 031 base schema mismatch';
  END IF;

  SELECT
    (SELECT COUNT(*) FROM information_schema.columns
      WHERE table_schema=DATABASE() AND table_name='navigation_categories' AND column_name='system_key') +
    (SELECT COUNT(DISTINCT index_name) FROM information_schema.statistics
      WHERE table_schema=DATABASE() AND table_name='navigation_categories' AND index_name='uq_navigation_categories_system_key') +
    (SELECT COUNT(*) FROM information_schema.columns
      WHERE table_schema=DATABASE() AND table_name='navigation_items' AND column_name='operating_status') +
    (SELECT COUNT(*) FROM information_schema.table_constraints
      WHERE table_schema=DATABASE() AND table_name='navigation_items' AND constraint_name='chk_navigation_items_operating_status') +
    (SELECT COUNT(DISTINCT index_name) FROM information_schema.statistics
      WHERE table_schema=DATABASE() AND table_name='navigation_items' AND index_name='uq_navigation_items_category_target')
  INTO schema_object_count;

  IF schema_object_count BETWEEN 1 AND 4 THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT = 'migration 031 partial schema';
  END IF;

  SELECT COUNT(*) INTO same_name_count
  FROM navigation_categories WHERE name='外卖商家';
  SELECT COUNT(*) INTO duplicate_target_count FROM (
    SELECT category_id, app_id, path
    FROM navigation_items
    GROUP BY category_id, app_id, path
    HAVING COUNT(*) > 1
  ) AS duplicate_targets;

  IF same_name_count > 1 THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT = 'migration 031 ambiguous category';
  END IF;
  IF duplicate_target_count > 0 THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT = 'migration 031 duplicate navigation target';
  END IF;
  IF schema_object_count = 0 AND same_name_count = 1 THEN
    SELECT COUNT(*) INTO invalid_legacy_item_count
    FROM navigation_items AS item
    JOIN navigation_categories AS category ON category.id=item.category_id
    WHERE category.name='外卖商家' AND (
      CHAR_LENGTH(item.title) NOT BETWEEN 1 AND 64 OR
      REGEXP_LIKE(item.title,'^[[:space:]]|[[:space:]]$','c') OR item.title REGEXP '[[:cntrl:]]' OR
      CHAR_LENGTH(item.subtitle) > 120 OR
      REGEXP_LIKE(item.subtitle,'^[[:space:]]|[[:space:]]$','c') OR item.subtitle REGEXP '[[:cntrl:]]' OR
      CHAR_LENGTH(item.icon_text) NOT BETWEEN 1 AND 12 OR
      REGEXP_LIKE(item.icon_text,'^[[:space:]]|[[:space:]]$','c') OR item.icon_text REGEXP '[[:cntrl:]]' OR
      NOT REGEXP_LIKE(item.app_id,'^wx[A-Za-z0-9]{16}$','c') OR
      CHAR_LENGTH(item.path) NOT BETWEEN 1 AND 240 OR
      REGEXP_LIKE(item.path,'^[[:space:]]|[[:space:]]$','c') OR
      LEFT(item.path,1)='/' OR LOCATE('//',item.path)>0 OR LOCATE('://',item.path)>0 OR
      LOCATE(CHAR(92),item.path)>0 OR LOCATE('..',item.path)>0 OR
      LOCATE('#',item.path)>0 OR LOCATE('%',item.path)>0 OR
      LOCATE('?',item.path)>0 OR item.path REGEXP '[[:cntrl:]]' OR
      NOT REGEXP_LIKE(item.color,'^#[0-9A-Fa-f]{6}$','c') OR
      item.sort < 0 OR item.sort > 10000
    );
    IF invalid_legacy_item_count > 0 THEN
      SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT = 'migration 031 unsafe legacy category item';
    END IF;
  END IF;

  IF schema_object_count = 0 AND EXISTS (
    SELECT 1 FROM (
      SELECT index_name,
             MAX(non_unique) AS non_unique,
             GROUP_CONCAT(column_name ORDER BY seq_in_index SEPARATOR ',') AS columns_in_order,
             SUM(sub_part IS NOT NULL) AS prefix_parts
      FROM information_schema.statistics
      WHERE table_schema=DATABASE() AND table_name='navigation_items'
      GROUP BY index_name
    ) AS existing_indexes
    WHERE non_unique=0
      AND columns_in_order='category_id,app_id,path'
      AND prefix_parts=0
  ) THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT = 'migration 031 conflicting target index';
  END IF;
  IF schema_object_count = 0 AND EXISTS (
    SELECT 1 FROM information_schema.table_constraints
    WHERE constraint_schema=DATABASE()
      AND constraint_name='chk_navigation_items_operating_status'
      AND (table_name<>'navigation_items' OR constraint_type<>'CHECK')
  ) THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT = 'migration 031 conflicting check name';
  END IF;

  IF schema_object_count = 0 THEN
    ALTER TABLE navigation_categories
      ADD COLUMN system_key VARCHAR(64) CHARACTER SET utf8mb4 COLLATE utf8mb4_0900_bin NULL AFTER name,
      ADD UNIQUE INDEX uq_navigation_categories_system_key (system_key);
    ALTER TABLE navigation_items
      ADD COLUMN operating_status VARCHAR(16) CHARACTER SET utf8mb4 COLLATE utf8mb4_0900_bin NOT NULL DEFAULT 'open' AFTER enabled,
      ADD CONSTRAINT chk_navigation_items_operating_status CHECK (operating_status IN ('open','resting')),
      ADD UNIQUE INDEX uq_navigation_items_category_target (category_id, app_id, path);

    IF same_name_count = 1 THEN
      UPDATE navigation_categories
      SET system_key='external_takeout_merchants'
      WHERE name='外卖商家';
    ELSE
      INSERT INTO navigation_categories(name,system_key,sort,enabled,created_at,updated_at)
      VALUES('外卖商家','external_takeout_merchants',20,TRUE,NOW(3),NOW(3));
    END IF;
  END IF;

  SELECT COUNT(*) INTO signature_ok
  FROM information_schema.columns
  WHERE table_schema=DATABASE() AND table_name='navigation_categories'
    AND column_name='system_key' AND data_type='varchar'
    AND character_maximum_length=64 AND is_nullable='YES'
    AND character_set_name='utf8mb4' AND collation_name='utf8mb4_0900_bin';
  IF signature_ok <> 1 THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT = 'migration 031 signature mismatch';
  END IF;

  SELECT COUNT(*) INTO signature_ok FROM (
    SELECT index_name,
           MAX(non_unique) AS non_unique,
           GROUP_CONCAT(column_name ORDER BY seq_in_index SEPARATOR ',') AS columns_in_order,
           SUM(sub_part IS NOT NULL) AS prefix_parts
    FROM information_schema.statistics
    WHERE table_schema=DATABASE() AND table_name='navigation_categories'
      AND index_name='uq_navigation_categories_system_key'
    GROUP BY index_name
    HAVING non_unique=0 AND columns_in_order='system_key' AND prefix_parts=0
  ) AS category_key_signature;
  IF signature_ok <> 1 THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT = 'migration 031 signature mismatch';
  END IF;

  SELECT COUNT(*) INTO signature_ok
  FROM information_schema.columns
  WHERE table_schema=DATABASE() AND table_name='navigation_items'
    AND column_name='operating_status' AND data_type='varchar'
    AND character_maximum_length=16 AND is_nullable='NO'
    AND character_set_name='utf8mb4' AND collation_name='utf8mb4_0900_bin'
    AND HEX(column_default)=HEX('open');
  IF signature_ok <> 1 THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT = 'migration 031 signature mismatch';
  END IF;

  SELECT COUNT(*) INTO signature_ok
  FROM information_schema.table_constraints AS table_constraint
  JOIN information_schema.check_constraints AS check_constraint
    ON check_constraint.constraint_schema=table_constraint.constraint_schema
   AND check_constraint.constraint_name=table_constraint.constraint_name
  WHERE table_constraint.constraint_schema=DATABASE()
    AND table_constraint.table_name='navigation_items'
    AND table_constraint.constraint_name='chk_navigation_items_operating_status'
    AND table_constraint.constraint_type='CHECK'
    AND table_constraint.enforced='YES'
    AND REPLACE(
      LOWER(REPLACE(REPLACE(REPLACE(REPLACE(REPLACE(check_constraint.check_clause,'`',''),' ',''),'(',''),')',''),'_utf8mb4','')),
      CONCAT(CHAR(92),CHAR(39)),
      CHAR(39)
    )
      = CONCAT('operating_statusin',CHAR(39),'open',CHAR(39),',',CHAR(39),'resting',CHAR(39))
    AND LOCATE(
      HEX(CONCAT(CHAR(39),'open',CHAR(39))),
      HEX(REPLACE(check_constraint.check_clause,CONCAT(CHAR(92),CHAR(39)),CHAR(39)))
    ) > 0
    AND LOCATE(
      HEX(CONCAT(CHAR(39),'resting',CHAR(39))),
      HEX(REPLACE(check_constraint.check_clause,CONCAT(CHAR(92),CHAR(39)),CHAR(39)))
    ) > 0;
  IF signature_ok <> 1 THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT = 'migration 031 signature mismatch';
  END IF;

  SELECT COUNT(*) INTO signature_ok FROM (
    SELECT index_name,
           MAX(non_unique) AS non_unique,
           GROUP_CONCAT(column_name ORDER BY seq_in_index SEPARATOR ',') AS columns_in_order,
           SUM(sub_part IS NOT NULL) AS prefix_parts
    FROM information_schema.statistics
    WHERE table_schema=DATABASE() AND table_name='navigation_items'
      AND index_name='uq_navigation_items_category_target'
    GROUP BY index_name
    HAVING non_unique=0 AND columns_in_order='category_id,app_id,path' AND prefix_parts=0
  ) AS target_signature;
  IF signature_ok <> 1 THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT = 'migration 031 signature mismatch';
  END IF;

  SELECT COUNT(*) INTO system_category_count
  FROM navigation_categories
  WHERE system_key='external_takeout_merchants' AND name='外卖商家';
  IF system_category_count <> 1 OR
     (SELECT COUNT(*) FROM navigation_categories WHERE name='外卖商家') <> 1 THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT = 'migration 031 system category mismatch';
  END IF;

  DO RELEASE_LOCK('campus_circle:migration:031');
  SET lock_acquired = 0;
END$$
CALL migrate_external_takeout_merchant_directory()$$
DROP PROCEDURE migrate_external_takeout_merchant_directory$$
DELIMITER ;
