-- P0-3: Meilisearch outbox, transaction hot-path indexes, and real Tencent C2C identifiers.
CREATE TABLE IF NOT EXISTS trade_search_outbox (
  id BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY,
  product_id BIGINT UNSIGNED NOT NULL,
  operation VARCHAR(16) NOT NULL,
  attempts BIGINT NOT NULL DEFAULT 0,
  available_at DATETIME(3) NOT NULL,
  processed_at DATETIME(3) NULL,
  last_error VARCHAR(512) NOT NULL DEFAULT '',
  created_at DATETIME(3) NULL,
  updated_at DATETIME(3) NULL,
  INDEX idx_trade_search_outbox_product (product_id),
  INDEX idx_trade_search_outbox_pending (processed_at, available_at, id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

DELIMITER $$
DROP PROCEDURE IF EXISTS migrate_transaction_closure$$
CREATE PROCEDURE migrate_transaction_closure()
BEGIN
  IF EXISTS (
    SELECT 1 FROM information_schema.statistics
    WHERE table_schema = DATABASE() AND table_name = 'im_conversations'
      AND index_name = 'conversation_id' AND non_unique = 0
  ) THEN
    ALTER TABLE im_conversations DROP INDEX conversation_id;
  END IF;

  IF EXISTS (
    SELECT 1 FROM information_schema.tables
    WHERE table_schema = DATABASE() AND table_name = 'im_conversations'
  ) AND NOT EXISTS (
    SELECT 1 FROM information_schema.statistics
    WHERE table_schema = DATABASE() AND table_name = 'im_conversations'
      AND index_name = 'idx_conversations_conversation_id'
  ) THEN
    ALTER TABLE im_conversations ADD INDEX idx_conversations_conversation_id (conversation_id);
  END IF;

  IF EXISTS (
    SELECT 1 FROM information_schema.tables
    WHERE table_schema = DATABASE() AND table_name = 'im_conversations'
  ) AND NOT EXISTS (
    SELECT 1 FROM information_schema.statistics
    WHERE table_schema = DATABASE() AND table_name = 'im_conversations'
      AND index_name = 'idx_conversations_buyer_updated'
  ) THEN
    ALTER TABLE im_conversations ADD INDEX idx_conversations_buyer_updated (buyer_id, updated_at, id);
  END IF;

  IF EXISTS (
    SELECT 1 FROM information_schema.tables
    WHERE table_schema = DATABASE() AND table_name = 'im_conversations'
  ) AND NOT EXISTS (
    SELECT 1 FROM information_schema.statistics
    WHERE table_schema = DATABASE() AND table_name = 'im_conversations'
      AND index_name = 'idx_conversations_seller_updated'
  ) THEN
    ALTER TABLE im_conversations ADD INDEX idx_conversations_seller_updated (seller_id, updated_at, id);
  END IF;

  IF EXISTS (
    SELECT 1 FROM information_schema.tables
    WHERE table_schema = DATABASE() AND table_name = 'products'
  ) AND EXISTS (
    SELECT 1 FROM information_schema.columns
    WHERE table_schema = DATABASE() AND table_name = 'products'
      AND column_name = 'review_status'
  ) AND NOT EXISTS (
    SELECT 1 FROM information_schema.statistics
    WHERE table_schema = DATABASE() AND table_name = 'products'
      AND index_name = 'idx_products_public_status_created'
  ) THEN
    ALTER TABLE products ADD INDEX idx_products_public_status_created (review_status, status, deleted_at, created_at, id);
  END IF;

  IF EXISTS (
    SELECT 1 FROM information_schema.tables
    WHERE table_schema = DATABASE() AND table_name = 'product_favorites'
  ) AND NOT EXISTS (
    SELECT 1 FROM information_schema.statistics
    WHERE table_schema = DATABASE() AND table_name = 'product_favorites'
      AND index_name = 'idx_product_favorites_user'
  ) THEN
    ALTER TABLE product_favorites ADD INDEX idx_product_favorites_user (user_id, created_at);
  END IF;

  IF EXISTS (
    SELECT 1 FROM information_schema.tables
    WHERE table_schema = DATABASE() AND table_name = 'im_user_mappings'
  ) THEN
    UPDATE im_user_mappings SET im_user_id = CONCAT('campus_', user_id)
    WHERE im_user_id <> CONCAT('campus_', user_id);
  END IF;
END$$
CALL migrate_transaction_closure()$$
DROP PROCEDURE migrate_transaction_closure$$
DELIMITER ;

INSERT INTO trade_search_outbox(product_id, operation, attempts, available_at, created_at, updated_at)
SELECT id,
       CASE WHEN review_status = 'approved' AND deleted_at IS NULL THEN 'upsert' ELSE 'delete' END,
       0, NOW(3), NOW(3), NOW(3)
FROM products;
