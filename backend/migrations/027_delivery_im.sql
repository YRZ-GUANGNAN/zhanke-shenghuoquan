-- D-5: delivery-order-scoped Tencent IM work-group assignments.
-- All order/user/media identifiers are logical references; no cross-domain FK.

CREATE TABLE delivery_im_conversations (
  id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  conversation_public_id CHAR(26) NOT NULL,
  delivery_order_public_id CHAR(26) NOT NULL,
  assignment_version BIGINT UNSIGNED NOT NULL,
  merchant_user_id BIGINT UNSIGNED NOT NULL,
  rider_user_id BIGINT UNSIGNED NOT NULL,
  tencent_group_id VARCHAR(64) NOT NULL,
  status VARCHAR(16) NOT NULL,
  next_image_slot SMALLINT UNSIGNED NOT NULL DEFAULT 0,
  row_version BIGINT UNSIGNED NOT NULL,
  attempt_count INT UNSIGNED NOT NULL DEFAULT 0,
  next_attempt_at DATETIME(3) NULL,
  lease_token CHAR(26) NULL,
  lease_expires_at DATETIME(3) NULL,
  last_error VARCHAR(512) NOT NULL DEFAULT '',
  created_at DATETIME(3) NOT NULL,
  updated_at DATETIME(3) NOT NULL,
  activated_at DATETIME(3) NULL,
  read_only_at DATETIME(3) NULL,
  revoked_at DATETIME(3) NULL,
  PRIMARY KEY (id),
  UNIQUE INDEX uq_delivery_im_conversation_public_id (conversation_public_id),
  UNIQUE INDEX uq_delivery_im_order_assignment (delivery_order_public_id, assignment_version),
  UNIQUE INDEX uq_delivery_im_tencent_group_id (tencent_group_id),
  INDEX idx_delivery_im_order_status (delivery_order_public_id, status),
  INDEX idx_delivery_im_participant_status (merchant_user_id, rider_user_id, status),
  INDEX idx_delivery_im_provision_scan (status, next_attempt_at, lease_expires_at, id),
  CONSTRAINT chk_delivery_im_status CHECK (
    status IN ('provisioning','active','read_only','revoking','revoked','failed')
  )
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

ALTER TABLE media_asset_references
  ADD CONSTRAINT chk_media_reference_type_delivery_im_guard
  CHECK (reference_type IN ('post','product','avatar','dish','merchant_logo','delivery_proof','delivery_im'));

ALTER TABLE media_asset_references
  DROP CHECK chk_media_reference_type;

ALTER TABLE media_asset_references
  ADD CONSTRAINT chk_media_reference_type
  CHECK (reference_type IN ('post','product','avatar','dish','merchant_logo','delivery_proof','delivery_im'));

ALTER TABLE media_asset_references
  DROP CHECK chk_media_reference_type_delivery_im_guard;
