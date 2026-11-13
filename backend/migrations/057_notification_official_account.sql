-- Add official-account personal notification delivery and OAuth bindings.
ALTER TABLE delivery_notification_deliveries
  DROP CHECK chk_delivery_notification_delivery_channel;

ALTER TABLE delivery_notification_deliveries
  ADD CONSTRAINT chk_delivery_notification_delivery_channel
  CHECK (channel IN ('in_app', 'wechat_subscribe', 'wechat_official', 'wecom'));

CREATE TABLE IF NOT EXISTS notification_official_account_bindings (
  user_id BIGINT UNSIGNED NOT NULL,
  app_id VARCHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  openid VARCHAR(128) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  follow_state VARCHAR(24) NOT NULL DEFAULT 'unknown',
  row_version BIGINT UNSIGNED NOT NULL DEFAULT 1,
  bound_at DATETIME(3) NOT NULL,
  updated_at DATETIME(3) NOT NULL,
  PRIMARY KEY (user_id),
  UNIQUE INDEX uq_notification_official_openid (app_id, openid),
  CONSTRAINT chk_notification_official_follow_state
    CHECK (follow_state IN ('unknown', 'following', 'not_following')),
  CONSTRAINT chk_notification_official_identity
    CHECK (user_id > 0 AND CHAR_LENGTH(openid) > 0 AND row_version > 0)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
