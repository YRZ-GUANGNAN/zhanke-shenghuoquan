-- 导航条目自定义图标：icon_path 保存内容寻址的图标文件名（由上传接口维护）。
SET @col_exists := (SELECT COUNT(*) FROM information_schema.COLUMNS
  WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'navigation_items' AND COLUMN_NAME = 'icon_path');
SET @ddl := IF(@col_exists = 0,
  'ALTER TABLE navigation_items ADD COLUMN icon_path VARCHAR(255) NOT NULL DEFAULT '''' AFTER color',
  'SELECT 1');
PREPARE migrate_stmt FROM @ddl;
EXECUTE migrate_stmt;
DEALLOCATE PREPARE migrate_stmt;
