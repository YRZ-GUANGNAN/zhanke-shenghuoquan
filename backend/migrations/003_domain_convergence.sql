-- Idempotent convergence for databases created by the pre-domain prototype.
-- New installations already match 001_init.sql; this procedure only acts when legacy columns/tables exist.
DELIMITER $$
DROP PROCEDURE IF EXISTS converge_legacy_schema$$
CREATE PROCEDURE converge_legacy_schema()
BEGIN
  IF NOT EXISTS (SELECT 1 FROM information_schema.columns WHERE table_schema=DATABASE() AND table_name='posts' AND column_name='review_status') THEN
    ALTER TABLE posts ADD COLUMN review_status VARCHAR(24) NOT NULL DEFAULT 'submitted' AFTER content;
  END IF;
  IF NOT EXISTS (SELECT 1 FROM information_schema.columns WHERE table_schema=DATABASE() AND table_name='posts' AND column_name='favorite_count') THEN
    ALTER TABLE posts ADD COLUMN favorite_count BIGINT NOT NULL DEFAULT 0 AFTER comment_count;
  END IF;
  IF EXISTS (SELECT 1 FROM information_schema.columns WHERE table_schema=DATABASE() AND table_name='posts' AND column_name='status') THEN
    UPDATE posts SET review_status=CASE status WHEN 'pending' THEN 'submitted' WHEN 'approved' THEN 'approved' WHEN 'rejected' THEN 'rejected' ELSE 'submitted' END;
    ALTER TABLE posts DROP COLUMN status;
  END IF;
  IF EXISTS (SELECT 1 FROM information_schema.columns WHERE table_schema=DATABASE() AND table_name='posts' AND column_name='images') THEN
    INSERT IGNORE INTO post_images(post_id,url,sort,created_at)
      SELECT p.id,j.url,j.ord-1,p.created_at FROM posts p
      JOIN JSON_TABLE(COALESCE(p.images,JSON_ARRAY()), '$[*]' COLUMNS(ord FOR ORDINALITY,url VARCHAR(512) PATH '$')) j;
    ALTER TABLE posts DROP COLUMN images;
  END IF;

  IF NOT EXISTS (SELECT 1 FROM information_schema.columns WHERE table_schema=DATABASE() AND table_name='comments' AND column_name='review_status') THEN
    ALTER TABLE comments ADD COLUMN review_status VARCHAR(24) NOT NULL DEFAULT 'submitted' AFTER content;
  END IF;
  IF EXISTS (SELECT 1 FROM information_schema.columns WHERE table_schema=DATABASE() AND table_name='comments' AND column_name='status') THEN
    UPDATE comments SET review_status=CASE status WHEN 'pending' THEN 'submitted' WHEN 'approved' THEN 'approved' WHEN 'rejected' THEN 'rejected' ELSE 'submitted' END;
    ALTER TABLE comments DROP COLUMN status;
  END IF;

  IF NOT EXISTS (SELECT 1 FROM information_schema.columns WHERE table_schema=DATABASE() AND table_name='products' AND column_name='review_status') THEN
    ALTER TABLE products ADD COLUMN review_status VARCHAR(24) NOT NULL DEFAULT 'submitted' AFTER status;
  END IF;
  IF NOT EXISTS (SELECT 1 FROM information_schema.columns WHERE table_schema=DATABASE() AND table_name='products' AND column_name='favorite_count') THEN
    ALTER TABLE products ADD COLUMN favorite_count BIGINT NOT NULL DEFAULT 0 AFTER review_status;
  END IF;
  UPDATE products SET review_status=CASE status WHEN 'pending' THEN 'submitted' WHEN 'on_sale' THEN 'approved' WHEN 'sold' THEN 'approved' WHEN 'off' THEN 'rejected' ELSE review_status END,
    status=CASE status WHEN 'pending' THEN 'selling' WHEN 'on_sale' THEN 'selling' WHEN 'sold' THEN 'sold' WHEN 'off' THEN 'selling' ELSE status END;
  IF EXISTS (SELECT 1 FROM information_schema.columns WHERE table_schema=DATABASE() AND table_name='products' AND column_name='images') THEN
    INSERT IGNORE INTO product_images(product_id,url,sort,created_at)
      SELECT p.id,j.url,j.ord-1,p.created_at FROM products p
      JOIN JSON_TABLE(COALESCE(p.images,JSON_ARRAY()), '$[*]' COLUMNS(ord FOR ORDINALITY,url VARCHAR(512) PATH '$')) j;
    ALTER TABLE products DROP COLUMN images;
  END IF;

  IF EXISTS (SELECT 1 FROM information_schema.tables WHERE table_schema=DATABASE() AND table_name='tool_links') THEN
    INSERT INTO navigation_categories(id,name,sort,enabled,created_at,updated_at) VALUES(1,'校园服务',10,TRUE,NOW(),NOW())
      ON DUPLICATE KEY UPDATE name=VALUES(name),updated_at=NOW();
    INSERT IGNORE INTO navigation_items(id,category_id,title,subtitle,icon_text,app_id,path,color,sort,enabled,created_at,updated_at)
      SELECT id,1,title,subtitle,icon_text,IF(app_id='','legacy-missing-appid',app_id),IF(path='','/',path),color,sort,enabled,created_at,updated_at FROM tool_links;
    DROP TABLE tool_links;
  END IF;

  IF EXISTS (SELECT 1 FROM information_schema.tables WHERE table_schema=DATABASE() AND table_name='audit_logs') THEN
    INSERT IGNORE INTO review_tasks(object_type,object_id,submitter_id,risk,status,reason,operator_id,created_at,updated_at)
      SELECT object,object_id,0,risk,CASE status WHEN 'pending' THEN 'submitted' WHEN 'approved' THEN 'approved' WHEN 'rejected' THEN 'rejected' ELSE 'submitted' END,'',0,created_at,updated_at FROM audit_logs;
    DROP TABLE audit_logs;
  END IF;

  DROP TABLE IF EXISTS orders;
END$$
CALL converge_legacy_schema()$$
DROP PROCEDURE converge_legacy_schema$$
DELIMITER ;
