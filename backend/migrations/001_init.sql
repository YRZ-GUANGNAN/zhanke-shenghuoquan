-- Domain A: users and authorization data.
CREATE TABLE IF NOT EXISTS users (
  id BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY,
  open_id VARCHAR(96) NOT NULL UNIQUE,
  nickname VARCHAR(64) NOT NULL DEFAULT '', avatar VARCHAR(512) NOT NULL DEFAULT '',
  school VARCHAR(80) NOT NULL DEFAULT '', campus VARCHAR(80) NOT NULL DEFAULT '',
  role VARCHAR(24) NOT NULL DEFAULT 'user', status VARCHAR(24) NOT NULL DEFAULT 'active', token_version BIGINT UNSIGNED NOT NULL DEFAULT 1,
  created_at DATETIME(3) NULL, updated_at DATETIME(3) NULL, deleted_at DATETIME(3) NULL,
  INDEX idx_users_deleted_at (deleted_at)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

-- Domain B: community data. Images and interactions are normalized.
CREATE TABLE IF NOT EXISTS posts (
  id BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY, user_id BIGINT UNSIGNED NOT NULL,
  topic VARCHAR(64) NOT NULL DEFAULT '', title VARCHAR(120) NOT NULL DEFAULT '', content TEXT,
  review_status VARCHAR(24) NOT NULL DEFAULT 'submitted', pinned BOOLEAN NOT NULL DEFAULT FALSE,
  liked_count BIGINT NOT NULL DEFAULT 0, comment_count BIGINT NOT NULL DEFAULT 0,
  favorite_count BIGINT NOT NULL DEFAULT 0, created_at DATETIME(3) NULL, updated_at DATETIME(3) NULL,
  deleted_at DATETIME(3) NULL, INDEX idx_posts_user_id (user_id), INDEX idx_posts_topic (topic),
  INDEX idx_posts_review_status (review_status), INDEX idx_posts_deleted_at (deleted_at)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
CREATE TABLE IF NOT EXISTS post_images (
  id BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY, post_id BIGINT UNSIGNED NOT NULL,
  url VARCHAR(512) NOT NULL DEFAULT '', sort BIGINT NOT NULL DEFAULT 0, created_at DATETIME(3) NULL,
  UNIQUE INDEX idx_post_image_sort (post_id, sort)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
CREATE TABLE IF NOT EXISTS comments (
  id BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY, post_id BIGINT UNSIGNED NOT NULL, user_id BIGINT UNSIGNED NOT NULL,
  content TEXT, review_status VARCHAR(24) NOT NULL DEFAULT 'submitted', created_at DATETIME(3) NULL, updated_at DATETIME(3) NULL,
  INDEX idx_comments_post_id (post_id), INDEX idx_comments_user_id (user_id), INDEX idx_comments_review_status (review_status)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
CREATE TABLE IF NOT EXISTS post_likes (post_id BIGINT UNSIGNED NOT NULL, user_id BIGINT UNSIGNED NOT NULL, created_at DATETIME(3) NULL, PRIMARY KEY (post_id, user_id)) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
CREATE TABLE IF NOT EXISTS post_favorites (post_id BIGINT UNSIGNED NOT NULL, user_id BIGINT UNSIGNED NOT NULL, created_at DATETIME(3) NULL, PRIMARY KEY (post_id, user_id)) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
CREATE TABLE IF NOT EXISTS user_follows (
  follower_id BIGINT UNSIGNED NOT NULL, followee_id BIGINT UNSIGNED NOT NULL, created_at DATETIME(3) NULL,
  PRIMARY KEY (follower_id, followee_id), INDEX idx_user_follows_followee (followee_id, created_at)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
CREATE TABLE IF NOT EXISTS notifications (
  id BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY, recipient_id BIGINT UNSIGNED NOT NULL, actor_id BIGINT UNSIGNED NOT NULL,
  type VARCHAR(32) NOT NULL, post_id BIGINT UNSIGNED NOT NULL DEFAULT 0, comment_id BIGINT UNSIGNED NOT NULL DEFAULT 0,
  read_at DATETIME(3) NULL, created_at DATETIME(3) NULL,
  UNIQUE INDEX idx_notification_event (recipient_id, actor_id, type, post_id, comment_id),
  INDEX idx_notifications_recipient_read_created (recipient_id, read_at, created_at), INDEX idx_notifications_actor (actor_id), INDEX idx_notifications_post (post_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

-- Domain C: second-hand products. Business status is separate from review status.
CREATE TABLE IF NOT EXISTS products (
  id BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY, seller_id BIGINT UNSIGNED NOT NULL,
  title VARCHAR(160) NOT NULL DEFAULT '', description TEXT, category VARCHAR(64) NOT NULL DEFAULT '',
  price BIGINT NOT NULL DEFAULT 0, original_price BIGINT NOT NULL DEFAULT 0,
  condition_label VARCHAR(64) NOT NULL DEFAULT '', campus VARCHAR(80) NOT NULL DEFAULT '',
  status VARCHAR(24) NOT NULL DEFAULT 'selling', review_status VARCHAR(24) NOT NULL DEFAULT 'submitted',
  favorite_count BIGINT NOT NULL DEFAULT 0, created_at DATETIME(3) NULL, updated_at DATETIME(3) NULL, deleted_at DATETIME(3) NULL,
  INDEX idx_products_seller_id (seller_id), INDEX idx_products_category (category), INDEX idx_products_status (status),
  INDEX idx_products_review_status (review_status), INDEX idx_products_deleted_at (deleted_at),
  INDEX idx_products_public_status_created (review_status, status, deleted_at, created_at, id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
CREATE TABLE IF NOT EXISTS product_images (
  id BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY, product_id BIGINT UNSIGNED NOT NULL,
  url VARCHAR(512) NOT NULL DEFAULT '', sort BIGINT NOT NULL DEFAULT 0, created_at DATETIME(3) NULL,
  UNIQUE INDEX idx_product_image_sort (product_id, sort)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
CREATE TABLE IF NOT EXISTS product_favorites (product_id BIGINT UNSIGNED NOT NULL, user_id BIGINT UNSIGNED NOT NULL, created_at DATETIME(3) NULL, PRIMARY KEY (product_id, user_id), INDEX idx_product_favorites_user (user_id, created_at)) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
CREATE TABLE IF NOT EXISTS trade_search_outbox (
  id BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY, product_id BIGINT UNSIGNED NOT NULL, operation VARCHAR(16) NOT NULL,
  attempts BIGINT NOT NULL DEFAULT 0, available_at DATETIME(3) NOT NULL, processed_at DATETIME(3) NULL,
  last_error VARCHAR(512) NOT NULL DEFAULT '', created_at DATETIME(3) NULL, updated_at DATETIME(3) NULL,
  INDEX idx_trade_search_outbox_product (product_id),
  INDEX idx_trade_search_outbox_pending (processed_at, available_at, id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

-- Domain D: navigation is category + jump item data only.
CREATE TABLE IF NOT EXISTS navigation_categories (
  id BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY, name VARCHAR(64) NOT NULL DEFAULT '', sort BIGINT NOT NULL DEFAULT 0,
  enabled BOOLEAN NOT NULL DEFAULT TRUE, created_at DATETIME(3) NULL, updated_at DATETIME(3) NULL, INDEX idx_navigation_categories_sort (sort)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
CREATE TABLE IF NOT EXISTS navigation_items (
  id BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY, category_id BIGINT UNSIGNED NOT NULL,
  title VARCHAR(64) NOT NULL DEFAULT '', subtitle VARCHAR(120) NOT NULL DEFAULT '', icon_text VARCHAR(12) NOT NULL DEFAULT '',
  app_id VARCHAR(80) NOT NULL, path VARCHAR(240) NOT NULL, color VARCHAR(24) NOT NULL DEFAULT '',
  sort BIGINT NOT NULL DEFAULT 0, enabled BOOLEAN NOT NULL DEFAULT TRUE, created_at DATETIME(3) NULL, updated_at DATETIME(3) NULL,
  INDEX idx_navigation_items_category_id (category_id), INDEX idx_navigation_items_sort (sort)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

-- Domain E: Tencent IM mappings and relations only. There is deliberately no message table.
CREATE TABLE IF NOT EXISTS im_user_mappings (
  id BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY, user_id BIGINT UNSIGNED NOT NULL UNIQUE,
  im_user_id VARCHAR(64) NOT NULL UNIQUE, created_at DATETIME(3) NULL, updated_at DATETIME(3) NULL
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
CREATE TABLE IF NOT EXISTS im_conversations (
  id BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY, conversation_id VARCHAR(128) NOT NULL,
  buyer_id BIGINT UNSIGNED NOT NULL, seller_id BIGINT UNSIGNED NOT NULL, reference_type VARCHAR(32) NOT NULL, reference_id BIGINT UNSIGNED NOT NULL,
  created_at DATETIME(3) NULL, updated_at DATETIME(3) NULL,
  UNIQUE INDEX idx_im_relation (buyer_id, seller_id, reference_type, reference_id), INDEX idx_conversations_conversation_id (conversation_id), INDEX idx_conversations_buyer_updated (buyer_id, updated_at, id), INDEX idx_conversations_seller_updated (seller_id, updated_at, id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

-- Domain F: one review workflow and report management.
CREATE TABLE IF NOT EXISTS review_tasks (
  id BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY, object_type VARCHAR(32) NOT NULL, object_id BIGINT UNSIGNED NOT NULL,
  submitter_id BIGINT UNSIGNED NOT NULL DEFAULT 0, risk VARCHAR(240) NOT NULL DEFAULT '', status VARCHAR(24) NOT NULL DEFAULT 'submitted',
  reason VARCHAR(240) NOT NULL DEFAULT '', operator_id BIGINT UNSIGNED NOT NULL DEFAULT 0,
  created_at DATETIME(3) NULL, updated_at DATETIME(3) NULL,
  UNIQUE INDEX idx_review_object (object_type, object_id), INDEX idx_review_tasks_status (status), INDEX idx_review_tasks_submitter_id (submitter_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
CREATE TABLE IF NOT EXISTS reports (
  id BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY, reporter_id BIGINT UNSIGNED NOT NULL,
  target_type VARCHAR(32) NOT NULL, target_id BIGINT UNSIGNED NOT NULL, reason VARCHAR(240) NOT NULL, detail TEXT,
  review_status VARCHAR(24) NOT NULL DEFAULT 'submitted', created_at DATETIME(3) NULL, updated_at DATETIME(3) NULL,
  INDEX idx_reports_reporter_id (reporter_id), INDEX idx_reports_target_type (target_type), INDEX idx_reports_target_id (target_id), INDEX idx_reports_review_status (review_status)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
