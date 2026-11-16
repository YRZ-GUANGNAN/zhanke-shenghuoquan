-- 学生认证：绑定式闭环（提交 → 后台审核 → 已认证 / 未通过 → 申请取消 → 后台解除）。
CREATE TABLE IF NOT EXISTS student_verifications (
  id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  public_id CHAR(26) NOT NULL,
  user_id BIGINT UNSIGNED NOT NULL,
  status VARCHAR(16) NOT NULL DEFAULT 'pending',
  real_name VARCHAR(64) NOT NULL,
  student_no VARCHAR(64) NOT NULL,
  college VARCHAR(120) NOT NULL DEFAULT '',
  review_reason VARCHAR(500) NOT NULL DEFAULT '',
  reviewer_user_id BIGINT UNSIGNED NULL,
  reviewed_at DATETIME(3) NULL,
  cancel_requested_at DATETIME(3) NULL,
  submitted_at DATETIME(3) NOT NULL,
  row_version BIGINT UNSIGNED NOT NULL DEFAULT 1,
  created_at DATETIME(3) NOT NULL,
  updated_at DATETIME(3) NOT NULL,
  PRIMARY KEY (id),
  UNIQUE KEY uq_student_verification_public_id (public_id),
  UNIQUE KEY uq_student_verification_user (user_id),
  KEY idx_student_verification_status_submitted (status, submitted_at, id),
  CONSTRAINT chk_student_verification_status CHECK (status IN ('pending','approved','rejected','cancelled'))
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE IF NOT EXISTS student_verification_assets (
  id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  verification_id BIGINT UNSIGNED NOT NULL,
  asset_public_id CHAR(26) NOT NULL,
  sort_order SMALLINT UNSIGNED NOT NULL,
  created_at DATETIME(3) NOT NULL,
  PRIMARY KEY (id),
  UNIQUE KEY uq_student_verification_asset (verification_id, asset_public_id),
  UNIQUE KEY uq_student_verification_asset_slot (verification_id, sort_order)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE IF NOT EXISTS student_verification_events (
  id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  event_public_id CHAR(26) NOT NULL,
  verification_id BIGINT UNSIGNED NOT NULL,
  actor_user_id BIGINT UNSIGNED NOT NULL,
  event_type VARCHAR(24) NOT NULL,
  status VARCHAR(16) NOT NULL,
  reason VARCHAR(500) NOT NULL DEFAULT '',
  snapshot_json JSON NOT NULL,
  created_at DATETIME(3) NOT NULL,
  PRIMARY KEY (id),
  UNIQUE KEY uq_student_verification_event_public_id (event_public_id),
  KEY idx_student_verification_event_verification (verification_id, created_at, id),
  CONSTRAINT chk_student_verification_event_type CHECK (event_type IN ('submitted','resubmitted','approved','rejected','cancel_requested','cancel_approved'))
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

-- 扩展媒体用途约束：允许 purpose/reference_type 使用 student_verification。
ALTER TABLE media_assets
  DROP CHECK chk_media_asset_purpose,
  ADD CONSTRAINT chk_media_asset_purpose
  CHECK (purpose IN ('post','product','avatar','profile_cover','dish','merchant_logo','delivery_proof','delivery_im','feedback','identity_application','student_verification'));

ALTER TABLE media_asset_references
  DROP CHECK chk_media_reference_type,
  ADD CONSTRAINT chk_media_reference_type
  CHECK (reference_type IN ('post','product','avatar','profile_cover','dish','merchant_logo','delivery_proof','delivery_im','feedback','identity_application','student_verification'));
