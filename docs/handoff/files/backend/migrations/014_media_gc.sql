-- P0-4 Module 8: durable, version-pinned Media Garbage Collection jobs.
-- The only foreign key remains inside the Media Domain.

CREATE TABLE IF NOT EXISTS media_gc_jobs (
  job_id CHAR(26) PRIMARY KEY,
  asset_id BIGINT UNSIGNED NOT NULL,
  object_stage VARCHAR(16) NOT NULL,
  object_key VARCHAR(512) NOT NULL,
  object_version_id VARCHAR(128) NOT NULL,
  status VARCHAR(24) NOT NULL DEFAULT 'pending',
  lease_token CHAR(26) NULL,
  lease_expires_at DATETIME(3) NULL,
  attempt_count INT UNSIGNED NOT NULL DEFAULT 0,
  next_attempt_at DATETIME(3) NULL,
  last_error_code VARCHAR(64) NOT NULL DEFAULT '',
  provider_request_id VARCHAR(128) NOT NULL DEFAULT '',
  deleted_at DATETIME(3) NULL,
  row_version BIGINT UNSIGNED NOT NULL DEFAULT 1,
  created_at DATETIME(3) NOT NULL,
  updated_at DATETIME(3) NOT NULL,
  CONSTRAINT fk_media_gc_job_asset FOREIGN KEY (asset_id) REFERENCES media_assets(id) ON DELETE RESTRICT ON UPDATE RESTRICT,
  CONSTRAINT chk_media_gc_job_stage CHECK (object_stage IN ('staging','final')),
  CONSTRAINT chk_media_gc_job_status CHECK (status IN ('pending','processing','retry_wait','succeeded','deadletter')),
  CONSTRAINT chk_media_gc_fixed_identity CHECK (CHAR_LENGTH(object_key) > 0 AND CHAR_LENGTH(object_version_id) > 0),
  UNIQUE INDEX uq_media_gc_object_identity (asset_id, object_stage, object_key, object_version_id),
  INDEX idx_media_gc_claim (status, next_attempt_at, lease_expires_at, job_id),
  INDEX idx_media_gc_asset_status (asset_id, status, object_stage)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE IF NOT EXISTS media_gc_auxiliary_jobs (
  job_id CHAR(26) PRIMARY KEY,
  asset_id BIGINT UNSIGNED NOT NULL,
  status VARCHAR(24) NOT NULL DEFAULT 'pending',
  lease_token CHAR(26) NULL,
  lease_expires_at DATETIME(3) NULL,
  attempt_count INT UNSIGNED NOT NULL DEFAULT 0,
  last_error_code VARCHAR(64) NOT NULL DEFAULT '',
  purged_at DATETIME(3) NULL,
  row_version BIGINT UNSIGNED NOT NULL DEFAULT 1,
  created_at DATETIME(3) NOT NULL,
  updated_at DATETIME(3) NOT NULL,
  CONSTRAINT fk_media_gc_aux_job_asset FOREIGN KEY (asset_id) REFERENCES media_assets(id) ON DELETE RESTRICT ON UPDATE RESTRICT,
  CONSTRAINT chk_media_gc_aux_job_status CHECK (status IN ('pending','processing','succeeded','deadletter')),
  UNIQUE INDEX uq_media_gc_aux_asset (asset_id),
  INDEX idx_media_gc_aux_claim (status, lease_expires_at, job_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
