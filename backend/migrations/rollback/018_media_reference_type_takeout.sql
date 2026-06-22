-- Restore the frozen three-value reference-type constraint. The first ALTER
-- is an intentional fail-closed guard: if dish or merchant_logo rows exist,
-- MySQL rejects it before the expanded constraint is removed. No data is
-- deleted or rewritten by this rollback.

ALTER TABLE media_asset_references
  ADD CONSTRAINT chk_media_reference_type_rollback_guard
  CHECK (reference_type IN ('post','product','avatar'));

ALTER TABLE media_asset_references
  DROP CHECK chk_media_reference_type;

ALTER TABLE media_asset_references
  ADD CONSTRAINT chk_media_reference_type
  CHECK (reference_type IN ('post','product','avatar'));

ALTER TABLE media_asset_references
  DROP CHECK chk_media_reference_type_rollback_guard;
