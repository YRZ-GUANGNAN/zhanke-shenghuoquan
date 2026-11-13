-- Support staff must be able to record how a user feedback was handled.
-- Additive only: existing feedback rows keep status=open and no handler.
ALTER TABLE user_feedbacks
  ADD COLUMN admin_note VARCHAR(500) NOT NULL DEFAULT '' AFTER status,
  ADD COLUMN handled_by BIGINT UNSIGNED NULL AFTER admin_note,
  ADD COLUMN handled_at DATETIME(3) NULL AFTER handled_by,
  ADD KEY idx_user_feedbacks_status_created (status, created_at);
