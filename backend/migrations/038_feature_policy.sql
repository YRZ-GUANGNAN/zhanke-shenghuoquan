CREATE TABLE IF NOT EXISTS feature_policy_configs (
    id TINYINT UNSIGNED NOT NULL,
    forum BOOLEAN NOT NULL DEFAULT TRUE,
    post BOOLEAN NOT NULL DEFAULT TRUE,
    comment BOOLEAN NOT NULL DEFAULT TRUE,
    secondhand BOOLEAN NOT NULL DEFAULT TRUE,
    secondhand_publish BOOLEAN NOT NULL DEFAULT TRUE,
    secondhand_chat BOOLEAN NOT NULL DEFAULT TRUE,
    takeout BOOLEAN NOT NULL DEFAULT TRUE,
    campus_tools BOOLEAN NOT NULL DEFAULT TRUE,
    delivery BOOLEAN NOT NULL DEFAULT TRUE,
    merchant BOOLEAN NOT NULL DEFAULT TRUE,
    rider BOOLEAN NOT NULL DEFAULT TRUE,
    messages BOOLEAN NOT NULL DEFAULT TRUE,
    following BOOLEAN NOT NULL DEFAULT TRUE,
    posting_mode VARCHAR(16) NOT NULL DEFAULT 'admin_only',
    row_version BIGINT UNSIGNED NOT NULL DEFAULT 1,
    updated_by BIGINT UNSIGNED NOT NULL DEFAULT 0,
    updated_at DATETIME(6) NOT NULL,
    created_at DATETIME(6) NOT NULL,
    PRIMARY KEY (id),
    CONSTRAINT chk_feature_policy_singleton CHECK (id = 1),
    CONSTRAINT chk_feature_policy_posting_mode CHECK (posting_mode IN ('admin_only', 'all_users'))
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

CREATE TABLE IF NOT EXISTS feature_policy_audit_logs (
    id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
    actor_id BIGINT UNSIGNED NOT NULL,
    before_json JSON NOT NULL,
    after_json JSON NOT NULL,
    created_at DATETIME(6) NOT NULL,
    PRIMARY KEY (id),
    KEY idx_feature_policy_audit_actor_created (actor_id, created_at)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;
