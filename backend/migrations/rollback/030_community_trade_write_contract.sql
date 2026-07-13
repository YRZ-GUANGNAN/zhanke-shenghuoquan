-- Roll back UI-21b-2a Community/Trade write-contract storage only.

DROP TABLE IF EXISTS trade_write_outbox;
DROP TABLE IF EXISTS community_write_outbox;

DELIMITER $$
DROP PROCEDURE IF EXISTS rollback_ui21b2a_write_contract$$
CREATE PROCEDURE rollback_ui21b2a_write_contract()
BEGIN
  IF EXISTS (
    SELECT 1 FROM information_schema.statistics
    WHERE table_schema = DATABASE()
      AND table_name = 'product_favorites'
      AND index_name = 'idx_product_favorites_user_product'
  ) THEN
    ALTER TABLE product_favorites
      DROP INDEX idx_product_favorites_user_product;
  END IF;

  IF EXISTS (
    SELECT 1 FROM information_schema.statistics
    WHERE table_schema = DATABASE()
      AND table_name = 'post_favorites'
      AND index_name = 'idx_post_favorites_user_post'
  ) THEN
    ALTER TABLE post_favorites
      DROP INDEX idx_post_favorites_user_post;
  END IF;

  IF EXISTS (
    SELECT 1 FROM information_schema.statistics
    WHERE table_schema = DATABASE()
      AND table_name = 'post_likes'
      AND index_name = 'idx_post_likes_user_post'
  ) THEN
    ALTER TABLE post_likes
      DROP INDEX idx_post_likes_user_post;
  END IF;

  IF EXISTS (
    SELECT 1 FROM information_schema.table_constraints
    WHERE table_schema = DATABASE()
      AND table_name = 'products'
      AND constraint_name = 'chk_products_create_request_pair'
  ) THEN
    ALTER TABLE products
      DROP CHECK chk_products_create_request_pair;
  END IF;

  IF EXISTS (
    SELECT 1 FROM information_schema.statistics
    WHERE table_schema = DATABASE()
      AND table_name = 'products'
      AND index_name = 'uq_products_seller_client_request'
  ) THEN
    ALTER TABLE products
      DROP INDEX uq_products_seller_client_request;
  END IF;

  IF EXISTS (
    SELECT 1 FROM information_schema.columns
    WHERE table_schema = DATABASE()
      AND table_name = 'products'
      AND column_name = 'create_request_hash'
  ) THEN
    ALTER TABLE products
      DROP COLUMN create_request_hash;
  END IF;

  IF EXISTS (
    SELECT 1 FROM information_schema.columns
    WHERE table_schema = DATABASE()
      AND table_name = 'products'
      AND column_name = 'client_request_id'
  ) THEN
    ALTER TABLE products
      DROP COLUMN client_request_id;
  END IF;

  IF EXISTS (
    SELECT 1 FROM information_schema.table_constraints
    WHERE table_schema = DATABASE()
      AND table_name = 'comments'
      AND constraint_name = 'chk_comments_create_request_pair'
  ) THEN
    ALTER TABLE comments
      DROP CHECK chk_comments_create_request_pair;
  END IF;

  IF EXISTS (
    SELECT 1 FROM information_schema.statistics
    WHERE table_schema = DATABASE()
      AND table_name = 'comments'
      AND index_name = 'uq_comments_user_client_request'
  ) THEN
    ALTER TABLE comments
      DROP INDEX uq_comments_user_client_request;
  END IF;

  IF EXISTS (
    SELECT 1 FROM information_schema.columns
    WHERE table_schema = DATABASE()
      AND table_name = 'comments'
      AND column_name = 'create_request_hash'
  ) THEN
    ALTER TABLE comments
      DROP COLUMN create_request_hash;
  END IF;

  IF EXISTS (
    SELECT 1 FROM information_schema.columns
    WHERE table_schema = DATABASE()
      AND table_name = 'comments'
      AND column_name = 'client_request_id'
  ) THEN
    ALTER TABLE comments
      DROP COLUMN client_request_id;
  END IF;

  IF EXISTS (
    SELECT 1 FROM information_schema.table_constraints
    WHERE table_schema = DATABASE()
      AND table_name = 'posts'
      AND constraint_name = 'chk_posts_create_request_pair'
  ) THEN
    ALTER TABLE posts
      DROP CHECK chk_posts_create_request_pair;
  END IF;

  IF EXISTS (
    SELECT 1 FROM information_schema.statistics
    WHERE table_schema = DATABASE()
      AND table_name = 'posts'
      AND index_name = 'uq_posts_user_client_request'
  ) THEN
    ALTER TABLE posts
      DROP INDEX uq_posts_user_client_request;
  END IF;

  IF EXISTS (
    SELECT 1 FROM information_schema.columns
    WHERE table_schema = DATABASE()
      AND table_name = 'posts'
      AND column_name = 'create_request_hash'
  ) THEN
    ALTER TABLE posts
      DROP COLUMN create_request_hash;
  END IF;

  IF EXISTS (
    SELECT 1 FROM information_schema.columns
    WHERE table_schema = DATABASE()
      AND table_name = 'posts'
      AND column_name = 'client_request_id'
  ) THEN
    ALTER TABLE posts
      DROP COLUMN client_request_id;
  END IF;
END$$
CALL rollback_ui21b2a_write_contract()$$
DROP PROCEDURE rollback_ui21b2a_write_contract$$
DELIMITER ;
