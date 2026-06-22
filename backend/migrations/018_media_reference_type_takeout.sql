-- Extend only the Media reference-type constraint for takeout image purposes.
-- Migration 017 remains reserved for G-2. This migration deliberately does
-- not modify media_assets.purpose or any column, index, or other constraint.

ALTER TABLE media_asset_references
  ADD CONSTRAINT chk_media_reference_type_takeout_guard
  CHECK (reference_type IN ('post','product','avatar','dish','merchant_logo'));

ALTER TABLE media_asset_references
  DROP CHECK chk_media_reference_type;

ALTER TABLE media_asset_references
  ADD CONSTRAINT chk_media_reference_type
  CHECK (reference_type IN ('post','product','avatar','dish','merchant_logo'));

ALTER TABLE media_asset_references
  DROP CHECK chk_media_reference_type_takeout_guard;
