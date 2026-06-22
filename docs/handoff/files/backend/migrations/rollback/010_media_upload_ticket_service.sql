-- Roll back P0-4 Module 3 query indexes only.
ALTER TABLE media_upload_tickets DROP INDEX idx_media_ticket_expiry_scan;
ALTER TABLE media_quota_reservations DROP INDEX idx_media_quota_owner_created;
