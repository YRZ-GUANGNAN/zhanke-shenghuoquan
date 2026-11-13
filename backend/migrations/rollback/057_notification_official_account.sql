DROP TABLE IF EXISTS notification_official_account_bindings;

ALTER TABLE delivery_notification_deliveries
  DROP CHECK chk_delivery_notification_delivery_channel;

ALTER TABLE delivery_notification_deliveries
  ADD CONSTRAINT chk_delivery_notification_delivery_channel
  CHECK (channel IN ('in_app', 'wechat_subscribe', 'wecom'));
