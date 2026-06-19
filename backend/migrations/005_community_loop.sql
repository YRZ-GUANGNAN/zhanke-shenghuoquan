-- P0-2: follow relations and durable in-app community notifications.
CREATE TABLE IF NOT EXISTS user_follows (
  follower_id BIGINT UNSIGNED NOT NULL,
  followee_id BIGINT UNSIGNED NOT NULL,
  created_at DATETIME(3) NULL,
  PRIMARY KEY (follower_id, followee_id),
  INDEX idx_user_follows_followee (followee_id, created_at)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE IF NOT EXISTS notifications (
  id BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY,
  recipient_id BIGINT UNSIGNED NOT NULL,
  actor_id BIGINT UNSIGNED NOT NULL,
  type VARCHAR(32) NOT NULL,
  post_id BIGINT UNSIGNED NOT NULL DEFAULT 0,
  comment_id BIGINT UNSIGNED NOT NULL DEFAULT 0,
  read_at DATETIME(3) NULL,
  created_at DATETIME(3) NULL,
  UNIQUE INDEX idx_notification_event (recipient_id, actor_id, type, post_id, comment_id),
  INDEX idx_notifications_recipient_read_created (recipient_id, read_at, created_at),
  INDEX idx_notifications_actor (actor_id),
  INDEX idx_notifications_post (post_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
