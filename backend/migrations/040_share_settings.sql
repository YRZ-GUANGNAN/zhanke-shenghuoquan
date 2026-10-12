CREATE TABLE IF NOT EXISTS share_settings (
    id TINYINT UNSIGNED NOT NULL PRIMARY KEY,
    home_title VARCHAR(240) NOT NULL,
    secondhand_title VARCHAR(240) NOT NULL,
    services_title VARCHAR(240) NOT NULL,
    default_image_url VARCHAR(2048) NOT NULL DEFAULT '',
    detail_title_suffix VARCHAR(240) NOT NULL DEFAULT '',
    prefer_content_image BOOLEAN NOT NULL DEFAULT TRUE,
    row_version BIGINT UNSIGNED NOT NULL DEFAULT 1,
    updated_by BIGINT UNSIGNED NOT NULL DEFAULT 0,
    updated_at DATETIME(6) NOT NULL,
    CONSTRAINT chk_share_settings_singleton CHECK (id = 1)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

CREATE TABLE IF NOT EXISTS share_settings_audit_logs (
    id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT PRIMARY KEY,
    actor_id BIGINT UNSIGNED NOT NULL,
    before_json JSON NOT NULL,
    after_json JSON NOT NULL,
    created_at DATETIME(6) NOT NULL,
    KEY idx_share_settings_audit_actor (actor_id, created_at)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

INSERT INTO share_settings (id, home_title, secondhand_title, services_title, detail_title_suffix, updated_at)
SELECT 1, '湛科生活圈', '校园二手 · 湛科生活圈', '校园服务 · 湛科生活圈', '湛科生活圈', CURRENT_TIMESTAMP(6)
WHERE NOT EXISTS (SELECT 1 FROM share_settings WHERE id = 1);
