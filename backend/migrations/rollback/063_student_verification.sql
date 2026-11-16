-- 回滚 063：恢复媒体用途约束并删除学生认证相关表。
ALTER TABLE media_assets
  DROP CHECK chk_media_asset_purpose,
  ADD CONSTRAINT chk_media_asset_purpose
  CHECK (purpose IN ('post','product','avatar','profile_cover','dish','merchant_logo','delivery_proof','delivery_im','feedback','identity_application'));

ALTER TABLE media_asset_references
  DROP CHECK chk_media_reference_type,
  ADD CONSTRAINT chk_media_reference_type
  CHECK (reference_type IN ('post','product','avatar','profile_cover','dish','merchant_logo','delivery_proof','delivery_im','feedback','identity_application'));

DROP TABLE IF EXISTS student_verification_events;
DROP TABLE IF EXISTS student_verification_assets;
DROP TABLE IF EXISTS student_verifications;
