-- 骑手状态变更审计：记录操作人、前后状态与原因。
CREATE TABLE IF NOT EXISTS delivery_rider_admin_actions (
  id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  rider_user_id BIGINT UNSIGNED NOT NULL,
  actor_user_id BIGINT UNSIGNED NOT NULL,
  from_status VARCHAR(24) NOT NULL,
  to_status VARCHAR(24) NOT NULL,
  reason VARCHAR(255) NOT NULL DEFAULT '',
  created_at DATETIME(3) NOT NULL,
  PRIMARY KEY (id),
  KEY idx_delivery_rider_admin_actions_rider (rider_user_id, created_at, id),
  KEY idx_delivery_rider_admin_actions_actor (actor_user_id, created_at, id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
