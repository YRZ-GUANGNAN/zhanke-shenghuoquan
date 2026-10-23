-- Apply before the comment-thread API. All existing comments remain root comments.
ALTER TABLE comments
  ADD COLUMN parent_comment_id BIGINT UNSIGNED NOT NULL DEFAULT 0,
  ADD COLUMN reply_to_comment_id BIGINT UNSIGNED NOT NULL DEFAULT 0,
  ADD COLUMN reply_to_user_id BIGINT UNSIGNED NOT NULL DEFAULT 0,
  ADD COLUMN liked_count BIGINT NOT NULL DEFAULT 0,
  ADD KEY idx_comment_thread_page (post_id, parent_comment_id, created_at, id),
  ADD CONSTRAINT chk_comment_liked_count CHECK (liked_count >= 0);

CREATE TABLE comment_likes (
  comment_id BIGINT UNSIGNED NOT NULL,
  user_id BIGINT UNSIGNED NOT NULL,
  created_at DATETIME(3) NOT NULL,
  PRIMARY KEY (comment_id, user_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
