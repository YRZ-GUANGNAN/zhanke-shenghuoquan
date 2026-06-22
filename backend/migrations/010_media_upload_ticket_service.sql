-- P0-4 Module 3: query indexes for atomic quota reservation and expiry scans.
-- No table outside the Media Domain is referenced or modified.

DELIMITER $$
DROP PROCEDURE IF EXISTS migrate_media_upload_ticket_service$$
CREATE PROCEDURE migrate_media_upload_ticket_service()
BEGIN
  IF NOT EXISTS (SELECT 1 FROM information_schema.statistics WHERE table_schema = DATABASE() AND table_name = 'media_quota_reservations' AND index_name = 'idx_media_quota_owner_created') THEN
    ALTER TABLE media_quota_reservations ADD INDEX idx_media_quota_owner_created (owner_id, created_at, status);
  END IF;
  IF NOT EXISTS (SELECT 1 FROM information_schema.statistics WHERE table_schema = DATABASE() AND table_name = 'media_upload_tickets' AND index_name = 'idx_media_ticket_expiry_scan') THEN
    ALTER TABLE media_upload_tickets ADD INDEX idx_media_ticket_expiry_scan (status, expires_at, ticket_id);
  END IF;
END$$
CALL migrate_media_upload_ticket_service()$$
DROP PROCEDURE migrate_media_upload_ticket_service$$
DELIMITER ;
