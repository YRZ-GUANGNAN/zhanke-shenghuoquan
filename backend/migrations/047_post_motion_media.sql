-- Additive schema for verified post video/live-photo media.
-- No historical post or image row is rewritten by this migration.
DELIMITER $$
DROP PROCEDURE IF EXISTS migrate_post_motion_media$$
CREATE PROCEDURE migrate_post_motion_media()
SQL SECURITY INVOKER
BEGIN
  DECLARE column_count INT DEFAULT 0;
  SELECT COUNT(*) INTO column_count FROM information_schema.columns
    WHERE table_schema = DATABASE() AND table_name = 'media_assets' AND column_name = 'duration_ms';
  IF column_count = 0 THEN
    ALTER TABLE media_assets ADD COLUMN duration_ms BIGINT UNSIGNED NOT NULL DEFAULT 0 AFTER height;
  END IF;
  SELECT COUNT(*) INTO column_count FROM information_schema.columns
    WHERE table_schema = DATABASE() AND table_name = 'media_upload_completions' AND column_name = 'verified_duration_ms';
  IF column_count = 0 THEN
    ALTER TABLE media_upload_completions ADD COLUMN verified_duration_ms BIGINT UNSIGNED NULL AFTER verified_height;
  END IF;
  SELECT COUNT(*) INTO column_count FROM information_schema.columns
    WHERE table_schema = DATABASE() AND table_name = 'posts' AND column_name = 'motion';
  IF column_count = 0 THEN
    ALTER TABLE posts ADD COLUMN motion JSON NULL AFTER content;
  END IF;
END$$
CALL migrate_post_motion_media()$$
DROP PROCEDURE migrate_post_motion_media$$
DELIMITER ;

CREATE TABLE IF NOT EXISTS media_video_review_jobs (
  asset_public_id CHAR(26) NOT NULL PRIMARY KEY,
  job_id VARCHAR(128) NOT NULL DEFAULT '',
  decision VARCHAR(24) NOT NULL DEFAULT 'pending',
  failure_code VARCHAR(64) NOT NULL DEFAULT '',
  next_poll_at DATETIME(3) NOT NULL,
  created_at DATETIME(3) NOT NULL,
  updated_at DATETIME(3) NOT NULL
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_bin;
