-- Administrator-editable notification channel settings.
-- Row 1 holds the WeCom group webhooks; empty values fall back to env config.
CREATE TABLE IF NOT EXISTS notification_channel_settings (
  id TINYINT UNSIGNED NOT NULL PRIMARY KEY,
  wecom_merchant_webhook VARCHAR(512) NOT NULL DEFAULT '',
  wecom_rider_webhook VARCHAR(512) NOT NULL DEFAULT '',
  row_version BIGINT UNSIGNED NOT NULL DEFAULT 1,
  updated_by BIGINT UNSIGNED NULL,
  updated_at DATETIME(3) NOT NULL
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

INSERT INTO notification_channel_settings (id, updated_at)
VALUES (1, NOW(3))
ON DUPLICATE KEY UPDATE id = id;
