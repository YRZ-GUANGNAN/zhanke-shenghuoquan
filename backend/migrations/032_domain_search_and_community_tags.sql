-- Domain-isolated search histories/events plus community post tags.
-- Forward-only: clearing or rolling back an application release must not erase search history.

ALTER TABLE posts
  ADD COLUMN tags JSON NULL AFTER topic;

CREATE TABLE community_search_histories (
  id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  user_id BIGINT UNSIGNED NOT NULL,
  normalized_keyword VARCHAR(100) NOT NULL,
  keyword VARCHAR(100) NOT NULL,
  updated_at DATETIME(3) NOT NULL,
  PRIMARY KEY (id),
  UNIQUE KEY uq_community_search_history_user_keyword (user_id, normalized_keyword),
  KEY idx_community_search_history_user_updated (user_id, updated_at)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

CREATE TABLE community_search_events (
  id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  user_id BIGINT UNSIGNED NOT NULL,
  normalized_keyword VARCHAR(100) NOT NULL,
  keyword VARCHAR(100) NOT NULL,
  created_at DATETIME(3) NOT NULL,
  PRIMARY KEY (id),
  KEY idx_community_search_event_keyword_created (normalized_keyword, created_at),
  KEY idx_community_search_event_user_keyword_created (user_id, normalized_keyword, created_at)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

CREATE TABLE trade_search_histories (
  id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  user_id BIGINT UNSIGNED NOT NULL,
  normalized_keyword VARCHAR(100) NOT NULL,
  keyword VARCHAR(100) NOT NULL,
  updated_at DATETIME(3) NOT NULL,
  PRIMARY KEY (id),
  UNIQUE KEY uq_trade_search_history_user_keyword (user_id, normalized_keyword),
  KEY idx_trade_search_history_user_updated (user_id, updated_at)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

CREATE TABLE trade_search_events (
  id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  user_id BIGINT UNSIGNED NOT NULL,
  normalized_keyword VARCHAR(100) NOT NULL,
  keyword VARCHAR(100) NOT NULL,
  created_at DATETIME(3) NOT NULL,
  PRIMARY KEY (id),
  KEY idx_trade_search_event_keyword_created (normalized_keyword, created_at),
  KEY idx_trade_search_event_user_keyword_created (user_id, normalized_keyword, created_at)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

CREATE TABLE navigation_search_histories (
  id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  user_id BIGINT UNSIGNED NOT NULL,
  normalized_keyword VARCHAR(100) NOT NULL,
  keyword VARCHAR(100) NOT NULL,
  updated_at DATETIME(3) NOT NULL,
  PRIMARY KEY (id),
  UNIQUE KEY uq_navigation_search_history_user_keyword (user_id, normalized_keyword),
  KEY idx_navigation_search_history_user_updated (user_id, updated_at)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

CREATE TABLE navigation_search_events (
  id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  user_id BIGINT UNSIGNED NOT NULL,
  normalized_keyword VARCHAR(100) NOT NULL,
  keyword VARCHAR(100) NOT NULL,
  created_at DATETIME(3) NOT NULL,
  PRIMARY KEY (id),
  KEY idx_navigation_search_event_keyword_created (normalized_keyword, created_at),
  KEY idx_navigation_search_event_user_keyword_created (user_id, normalized_keyword, created_at)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;
