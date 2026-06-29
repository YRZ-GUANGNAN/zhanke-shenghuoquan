DROP TABLE IF EXISTS delivery_outbox;

ALTER TABLE delivery_orders
  DROP INDEX uq_delivery_order_merchant_request,
  DROP COLUMN merchant_request_id;
