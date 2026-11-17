-- 联系设置：单行平台配置（众创申请弹窗的微信号与文案），公开读取、后台维护。
CREATE TABLE IF NOT EXISTS contact_settings (
  id TINYINT UNSIGNED NOT NULL,
  maker_wechat VARCHAR(64) NOT NULL DEFAULT '',
  maker_intro VARCHAR(500) NOT NULL DEFAULT '',
  row_version BIGINT UNSIGNED NOT NULL DEFAULT 1,
  updated_at DATETIME(3) NOT NULL,
  PRIMARY KEY (id),
  CONSTRAINT chk_contact_settings_single_row CHECK (id = 1)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

INSERT INTO contact_settings (id, maker_wechat, maker_intro, row_version, updated_at)
SELECT 1, 'zhanke-zhongchuang', '众创空间入驻与合作，请添加微信，添加时请备注「众创申请」。', 1, NOW(3)
WHERE NOT EXISTS (SELECT 1 FROM contact_settings WHERE id = 1);
