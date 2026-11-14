-- Administrator order commands: cancellation reason and an immutable audit trail.
-- Additive only: existing orders keep running with an empty cancel reason.
ALTER TABLE delivery_orders
  ADD COLUMN cancel_reason VARCHAR(255) NOT NULL DEFAULT '' AFTER cancelled_at;

CREATE TABLE IF NOT EXISTS delivery_order_admin_actions (
  id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  delivery_order_public_id VARCHAR(26) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  action VARCHAR(24) NOT NULL,
  actor_user_id BIGINT UNSIGNED NOT NULL,
  from_rider_user_id BIGINT UNSIGNED NULL,
  to_rider_user_id BIGINT UNSIGNED NULL,
  from_status VARCHAR(24) NOT NULL,
  reason VARCHAR(255) NOT NULL DEFAULT '',
  created_at DATETIME(3) NOT NULL,
  PRIMARY KEY (id),
  KEY idx_delivery_admin_actions_order (delivery_order_public_id, created_at, id),
  KEY idx_delivery_admin_actions_actor (actor_user_id, created_at, id),
  CONSTRAINT chk_delivery_admin_actions_action CHECK (action IN ('reassign', 'cancel'))
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
