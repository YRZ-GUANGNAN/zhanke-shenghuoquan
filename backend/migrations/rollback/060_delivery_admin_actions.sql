DROP TABLE IF EXISTS delivery_order_admin_actions;

ALTER TABLE delivery_orders
  DROP COLUMN cancel_reason;
