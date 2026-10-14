-- Personal covers are independent of avatars; apply before deploying the API.
-- Forward-only. Existing accounts retain their original, empty background.
ALTER TABLE users
  ADD COLUMN cover VARCHAR(512) NOT NULL DEFAULT '' AFTER avatar_asset_id,
  ADD COLUMN cover_asset_id BIGINT UNSIGNED NULL AFTER cover,
  ADD KEY idx_users_cover_asset_id (cover_asset_id);

-- MySQL 8 atomic DDL keeps the allowlist enforced throughout each change.
ALTER TABLE media_assets
  DROP CHECK chk_media_asset_purpose,
  ADD CONSTRAINT chk_media_asset_purpose
    CHECK (purpose IN ('post','product','avatar','profile_cover','dish','merchant_logo','delivery_proof','delivery_im','feedback','identity_application'));

ALTER TABLE media_asset_references
  DROP CHECK chk_media_reference_type,
  ADD CONSTRAINT chk_media_reference_type
    CHECK (reference_type IN ('post','product','avatar','profile_cover','dish','merchant_logo','delivery_proof','delivery_im','feedback','identity_application'));
