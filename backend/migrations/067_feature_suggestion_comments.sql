-- 功能建议对话：用户与维护者在每条建议下的交谈。
CREATE TABLE IF NOT EXISTS feature_suggestion_comments (
  id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  public_id CHAR(26) NOT NULL,
  suggestion_id BIGINT UNSIGNED NOT NULL,
  user_id BIGINT UNSIGNED NOT NULL,
  is_maintainer TINYINT(1) NOT NULL DEFAULT 0,
  content VARCHAR(300) NOT NULL,
  created_at DATETIME(3) NOT NULL,
  PRIMARY KEY (id),
  UNIQUE KEY uq_feature_comment_public_id (public_id),
  KEY idx_feature_comment_suggestion (suggestion_id, id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
