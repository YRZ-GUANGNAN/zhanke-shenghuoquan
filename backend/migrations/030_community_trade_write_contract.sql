-- UI-21b-2a: retry-safe Community/Trade writes, viewer-state indexes,
-- and domain-owned creation outboxes. Cross-domain identifiers are logical;
-- this migration deliberately creates no foreign keys.

DELIMITER $$
DROP PROCEDURE IF EXISTS migrate_ui21b2a_write_contract$$
CREATE PROCEDURE migrate_ui21b2a_write_contract()
BEGIN
  IF NOT EXISTS (
    SELECT 1 FROM information_schema.columns
    WHERE table_schema = DATABASE()
      AND table_name = 'posts'
      AND column_name = 'client_request_id'
  ) THEN
    ALTER TABLE posts
      ADD COLUMN client_request_id CHAR(26) NULL AFTER user_id;
  END IF;

  IF NOT EXISTS (
    SELECT 1 FROM information_schema.columns
    WHERE table_schema = DATABASE()
      AND table_name = 'posts'
      AND column_name = 'create_request_hash'
  ) THEN
    ALTER TABLE posts
      ADD COLUMN create_request_hash BINARY(32) NULL AFTER client_request_id;
  END IF;

  IF NOT EXISTS (
    SELECT 1 FROM information_schema.statistics
    WHERE table_schema = DATABASE()
      AND table_name = 'posts'
      AND index_name = 'uq_posts_user_client_request'
  ) THEN
    ALTER TABLE posts
      ADD UNIQUE INDEX uq_posts_user_client_request (user_id, client_request_id);
  END IF;

  IF NOT EXISTS (
    SELECT 1 FROM information_schema.table_constraints
    WHERE table_schema = DATABASE()
      AND table_name = 'posts'
      AND constraint_name = 'chk_posts_create_request_pair'
  ) THEN
    ALTER TABLE posts
      ADD CONSTRAINT chk_posts_create_request_pair CHECK (
        (client_request_id IS NULL AND create_request_hash IS NULL)
        OR
        (client_request_id IS NOT NULL AND create_request_hash IS NOT NULL)
      );
  END IF;

  IF NOT EXISTS (
    SELECT 1 FROM information_schema.columns
    WHERE table_schema = DATABASE()
      AND table_name = 'comments'
      AND column_name = 'client_request_id'
  ) THEN
    ALTER TABLE comments
      ADD COLUMN client_request_id CHAR(26) NULL AFTER user_id;
  END IF;

  IF NOT EXISTS (
    SELECT 1 FROM information_schema.columns
    WHERE table_schema = DATABASE()
      AND table_name = 'comments'
      AND column_name = 'create_request_hash'
  ) THEN
    ALTER TABLE comments
      ADD COLUMN create_request_hash BINARY(32) NULL AFTER client_request_id;
  END IF;

  IF NOT EXISTS (
    SELECT 1 FROM information_schema.statistics
    WHERE table_schema = DATABASE()
      AND table_name = 'comments'
      AND index_name = 'uq_comments_user_client_request'
  ) THEN
    ALTER TABLE comments
      ADD UNIQUE INDEX uq_comments_user_client_request (user_id, client_request_id);
  END IF;

  IF NOT EXISTS (
    SELECT 1 FROM information_schema.table_constraints
    WHERE table_schema = DATABASE()
      AND table_name = 'comments'
      AND constraint_name = 'chk_comments_create_request_pair'
  ) THEN
    ALTER TABLE comments
      ADD CONSTRAINT chk_comments_create_request_pair CHECK (
        (client_request_id IS NULL AND create_request_hash IS NULL)
        OR
        (client_request_id IS NOT NULL AND create_request_hash IS NOT NULL)
      );
  END IF;

  IF NOT EXISTS (
    SELECT 1 FROM information_schema.columns
    WHERE table_schema = DATABASE()
      AND table_name = 'products'
      AND column_name = 'client_request_id'
  ) THEN
    ALTER TABLE products
      ADD COLUMN client_request_id CHAR(26) NULL AFTER seller_id;
  END IF;

  IF NOT EXISTS (
    SELECT 1 FROM information_schema.columns
    WHERE table_schema = DATABASE()
      AND table_name = 'products'
      AND column_name = 'create_request_hash'
  ) THEN
    ALTER TABLE products
      ADD COLUMN create_request_hash BINARY(32) NULL AFTER client_request_id;
  END IF;

  IF NOT EXISTS (
    SELECT 1 FROM information_schema.statistics
    WHERE table_schema = DATABASE()
      AND table_name = 'products'
      AND index_name = 'uq_products_seller_client_request'
  ) THEN
    ALTER TABLE products
      ADD UNIQUE INDEX uq_products_seller_client_request (seller_id, client_request_id);
  END IF;

  IF NOT EXISTS (
    SELECT 1 FROM information_schema.table_constraints
    WHERE table_schema = DATABASE()
      AND table_name = 'products'
      AND constraint_name = 'chk_products_create_request_pair'
  ) THEN
    ALTER TABLE products
      ADD CONSTRAINT chk_products_create_request_pair CHECK (
        (client_request_id IS NULL AND create_request_hash IS NULL)
        OR
        (client_request_id IS NOT NULL AND create_request_hash IS NOT NULL)
      );
  END IF;

  IF NOT EXISTS (
    SELECT 1 FROM information_schema.statistics
    WHERE table_schema = DATABASE()
      AND table_name = 'post_likes'
      AND index_name = 'idx_post_likes_user_post'
  ) THEN
    ALTER TABLE post_likes
      ADD INDEX idx_post_likes_user_post (user_id, post_id);
  END IF;

  IF NOT EXISTS (
    SELECT 1 FROM information_schema.statistics
    WHERE table_schema = DATABASE()
      AND table_name = 'post_favorites'
      AND index_name = 'idx_post_favorites_user_post'
  ) THEN
    ALTER TABLE post_favorites
      ADD INDEX idx_post_favorites_user_post (user_id, post_id);
  END IF;

  IF NOT EXISTS (
    SELECT 1 FROM information_schema.statistics
    WHERE table_schema = DATABASE()
      AND table_name = 'product_favorites'
      AND index_name = 'idx_product_favorites_user_product'
  ) THEN
    ALTER TABLE product_favorites
      ADD INDEX idx_product_favorites_user_product (user_id, product_id);
  END IF;
END$$
CALL migrate_ui21b2a_write_contract()$$
DROP PROCEDURE migrate_ui21b2a_write_contract$$
DELIMITER ;

CREATE TABLE IF NOT EXISTS community_write_outbox (
  id BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY,
  event_public_id CHAR(26) NOT NULL,
  aggregate_type VARCHAR(32) NOT NULL,
  aggregate_id BIGINT UNSIGNED NOT NULL,
  owner_user_id BIGINT UNSIGNED NOT NULL,
  event_type VARCHAR(48) NOT NULL,
  reservation_token VARCHAR(128) NULL,
  payload JSON NOT NULL,
  status VARCHAR(24) NOT NULL,
  attempt_count INT UNSIGNED NOT NULL DEFAULT 0,
  available_at DATETIME(3) NOT NULL,
  lease_token CHAR(26) NULL,
  lease_expires_at DATETIME(3) NULL,
  processed_at DATETIME(3) NULL,
  failed_at DATETIME(3) NULL,
  last_error VARCHAR(512) NOT NULL DEFAULT '',
  row_version BIGINT UNSIGNED NOT NULL,
  created_at DATETIME(3) NOT NULL,
  updated_at DATETIME(3) NOT NULL,
  CONSTRAINT chk_community_write_outbox_status CHECK (
    status IN ('pending','processing','retry_wait','succeeded','deadletter')
  ),
  UNIQUE INDEX uq_community_write_outbox_event_public_id (event_public_id),
  UNIQUE INDEX uq_community_write_outbox_aggregate_event (
    aggregate_type, aggregate_id, event_type
  ),
  INDEX idx_community_write_outbox_owner (owner_user_id, created_at, id),
  INDEX idx_community_write_outbox_available (status, available_at, id),
  INDEX idx_community_write_outbox_lease (status, lease_expires_at, id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE IF NOT EXISTS trade_write_outbox (
  id BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY,
  event_public_id CHAR(26) NOT NULL,
  aggregate_type VARCHAR(32) NOT NULL,
  aggregate_id BIGINT UNSIGNED NOT NULL,
  owner_user_id BIGINT UNSIGNED NOT NULL,
  event_type VARCHAR(48) NOT NULL,
  reservation_token VARCHAR(128) NULL,
  payload JSON NOT NULL,
  status VARCHAR(24) NOT NULL,
  attempt_count INT UNSIGNED NOT NULL DEFAULT 0,
  available_at DATETIME(3) NOT NULL,
  lease_token CHAR(26) NULL,
  lease_expires_at DATETIME(3) NULL,
  processed_at DATETIME(3) NULL,
  failed_at DATETIME(3) NULL,
  last_error VARCHAR(512) NOT NULL DEFAULT '',
  row_version BIGINT UNSIGNED NOT NULL,
  created_at DATETIME(3) NOT NULL,
  updated_at DATETIME(3) NOT NULL,
  CONSTRAINT chk_trade_write_outbox_status CHECK (
    status IN ('pending','processing','retry_wait','succeeded','deadletter')
  ),
  UNIQUE INDEX uq_trade_write_outbox_event_public_id (event_public_id),
  UNIQUE INDEX uq_trade_write_outbox_aggregate_event (
    aggregate_type, aggregate_id, event_type
  ),
  INDEX idx_trade_write_outbox_owner (owner_user_id, created_at, id),
  INDEX idx_trade_write_outbox_available (status, available_at, id),
  INDEX idx_trade_write_outbox_lease (status, lease_expires_at, id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

DELIMITER $$
DROP PROCEDURE IF EXISTS migrate_ui21b2a_outbox_indexes$$
CREATE PROCEDURE migrate_ui21b2a_outbox_indexes()
BEGIN
  IF EXISTS (
    SELECT 1 FROM information_schema.statistics
    WHERE table_schema = DATABASE()
      AND table_name = 'community_write_outbox'
      AND index_name = 'idx_community_write_outbox_claim'
  ) THEN
    ALTER TABLE community_write_outbox
      DROP INDEX idx_community_write_outbox_claim;
  END IF;
  IF NOT EXISTS (
    SELECT 1 FROM information_schema.statistics
    WHERE table_schema = DATABASE()
      AND table_name = 'community_write_outbox'
      AND index_name = 'idx_community_write_outbox_available'
  ) THEN
    ALTER TABLE community_write_outbox
      ADD INDEX idx_community_write_outbox_available (status, available_at, id);
  END IF;
  IF NOT EXISTS (
    SELECT 1 FROM information_schema.statistics
    WHERE table_schema = DATABASE()
      AND table_name = 'community_write_outbox'
      AND index_name = 'idx_community_write_outbox_lease'
  ) THEN
    ALTER TABLE community_write_outbox
      ADD INDEX idx_community_write_outbox_lease (status, lease_expires_at, id);
  END IF;

  IF EXISTS (
    SELECT 1 FROM information_schema.statistics
    WHERE table_schema = DATABASE()
      AND table_name = 'trade_write_outbox'
      AND index_name = 'idx_trade_write_outbox_claim'
  ) THEN
    ALTER TABLE trade_write_outbox
      DROP INDEX idx_trade_write_outbox_claim;
  END IF;
  IF NOT EXISTS (
    SELECT 1 FROM information_schema.statistics
    WHERE table_schema = DATABASE()
      AND table_name = 'trade_write_outbox'
      AND index_name = 'idx_trade_write_outbox_available'
  ) THEN
    ALTER TABLE trade_write_outbox
      ADD INDEX idx_trade_write_outbox_available (status, available_at, id);
  END IF;
  IF NOT EXISTS (
    SELECT 1 FROM information_schema.statistics
    WHERE table_schema = DATABASE()
      AND table_name = 'trade_write_outbox'
      AND index_name = 'idx_trade_write_outbox_lease'
  ) THEN
    ALTER TABLE trade_write_outbox
      ADD INDEX idx_trade_write_outbox_lease (status, lease_expires_at, id);
  END IF;
END$$
CALL migrate_ui21b2a_outbox_indexes()$$
DROP PROCEDURE migrate_ui21b2a_outbox_indexes$$
DELIMITER ;
