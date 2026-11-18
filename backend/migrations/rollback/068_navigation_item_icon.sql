-- 回滚 068：移除导航条目自定义图标列。
SET @col_exists := (SELECT COUNT(*) FROM information_schema.COLUMNS
  WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'navigation_items' AND COLUMN_NAME = 'icon_path');
SET @ddl := IF(@col_exists = 1,
  'ALTER TABLE navigation_items DROP COLUMN icon_path',
  'SELECT 1');
PREPARE rollback_stmt FROM @ddl;
EXECUTE rollback_stmt;
DEALLOCATE PREPARE rollback_stmt;
