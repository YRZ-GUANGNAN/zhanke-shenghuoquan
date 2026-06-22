-- P0-4 Module 7: Media Events Consumer inbox and aggregate checkpoints.
-- All foreign keys remain inside the Media Domain.

CREATE TABLE IF NOT EXISTS media_event_inbox (
  event_id CHAR(26) PRIMARY KEY,
  aggregate_id CHAR(26) NOT NULL,
  aggregate_version BIGINT UNSIGNED NOT NULL,
  operation VARCHAR(48) NOT NULL,
  payload_hash CHAR(64) NOT NULL,
  status VARCHAR(24) NOT NULL DEFAULT 'pending',
  lease_token CHAR(26) NULL,
  lease_expires_at DATETIME(3) NULL,
  attempt_count INT UNSIGNED NOT NULL DEFAULT 0,
  next_attempt_at DATETIME(3) NULL,
  last_error_code VARCHAR(64) NOT NULL DEFAULT '',
  processed_at DATETIME(3) NULL,
  row_version BIGINT UNSIGNED NOT NULL DEFAULT 1,
  created_at DATETIME(3) NOT NULL,
  updated_at DATETIME(3) NOT NULL,
  CONSTRAINT fk_media_event_inbox_outbox FOREIGN KEY (event_id) REFERENCES media_outbox(event_id) ON DELETE RESTRICT ON UPDATE RESTRICT,
  CONSTRAINT chk_media_event_inbox_status CHECK (status IN ('pending','processing','retry_wait','succeeded','deadletter')),
  INDEX idx_media_event_inbox_claim (status, next_attempt_at, lease_expires_at, event_id),
  INDEX idx_media_event_inbox_aggregate (aggregate_id, aggregate_version, event_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE IF NOT EXISTS media_consumer_checkpoints (
  aggregate_id CHAR(26) PRIMARY KEY,
  last_event_id CHAR(26) NOT NULL DEFAULT '',
  last_aggregate_version BIGINT UNSIGNED NOT NULL,
  last_operation VARCHAR(48) NOT NULL DEFAULT '',
  last_payload_hash CHAR(64) NOT NULL DEFAULT '',
  active_event_id CHAR(26) NULL,
  lease_token CHAR(26) NULL,
  lease_expires_at DATETIME(3) NULL,
  row_version BIGINT UNSIGNED NOT NULL DEFAULT 1,
  created_at DATETIME(3) NOT NULL,
  updated_at DATETIME(3) NOT NULL,
  INDEX idx_media_consumer_checkpoint_event (last_event_id),
  INDEX idx_media_consumer_checkpoint_lease (lease_expires_at, active_event_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
