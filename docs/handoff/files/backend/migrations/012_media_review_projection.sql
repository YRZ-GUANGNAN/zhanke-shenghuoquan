-- P0-4 Module 5: review projection and immutable review decisions.
-- The unified review system remains the fact source. These tables only keep
-- the Media Domain projection and contain no foreign keys to A/B/C domains.

CREATE TABLE IF NOT EXISTS media_review_projections (
  projection_id CHAR(26) PRIMARY KEY,
  asset_id BIGINT UNSIGNED NOT NULL,
  review_task_id BIGINT UNSIGNED NULL,
  status VARCHAR(24) NOT NULL DEFAULT 'pending',
  applied_decision_version BIGINT UNSIGNED NOT NULL DEFAULT 0,
  last_decision_hash CHAR(64) NULL,
  failure_code VARCHAR(64) NOT NULL DEFAULT '',
  submitted_at DATETIME(3) NULL,
  reviewing_at DATETIME(3) NULL,
  decided_at DATETIME(3) NULL,
  row_version BIGINT UNSIGNED NOT NULL DEFAULT 1,
  created_at DATETIME(3) NOT NULL,
  updated_at DATETIME(3) NOT NULL,
  CONSTRAINT fk_media_review_projection_asset FOREIGN KEY (asset_id) REFERENCES media_assets(id) ON DELETE RESTRICT ON UPDATE RESTRICT,
  CONSTRAINT chk_media_review_projection_status CHECK (status IN ('pending','submitted','reviewing','approved','rejected','review_failed')),
  UNIQUE INDEX uq_media_review_projection_asset (asset_id),
  UNIQUE INDEX uq_media_review_projection_task (review_task_id),
  INDEX idx_media_review_projection_status (status, updated_at, projection_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE IF NOT EXISTS media_review_decisions (
  id BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY,
  projection_id CHAR(26) NOT NULL,
  review_task_id BIGINT UNSIGNED NOT NULL,
  decision_version BIGINT UNSIGNED NOT NULL,
  decision VARCHAR(24) NOT NULL,
  payload_hash CHAR(64) NOT NULL,
  result_code VARCHAR(64) NOT NULL DEFAULT '',
  received_at DATETIME(3) NOT NULL,
  CONSTRAINT fk_media_review_decision_projection FOREIGN KEY (projection_id) REFERENCES media_review_projections(projection_id) ON DELETE RESTRICT ON UPDATE RESTRICT,
  CONSTRAINT chk_media_review_decision_status CHECK (decision IN ('approved','rejected','review_failed')),
  UNIQUE INDEX uq_media_review_decision_task_version (review_task_id, decision_version),
  UNIQUE INDEX uq_media_review_decision_projection_version (projection_id, decision_version),
  INDEX idx_media_review_decision_received (received_at, id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
