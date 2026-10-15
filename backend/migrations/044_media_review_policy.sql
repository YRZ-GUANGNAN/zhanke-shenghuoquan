-- Apply before the API that writes not_required projections.
-- This records an explicit purpose policy, not an automatic cloud approval.
-- Existing pending/rejected projections and all review decisions are preserved.
ALTER TABLE media_review_projections
  DROP CHECK chk_media_review_projection_status,
  ADD CONSTRAINT chk_media_review_projection_status
    CHECK (status IN ('pending','submitted','reviewing','approved','rejected','review_failed','not_required'));
