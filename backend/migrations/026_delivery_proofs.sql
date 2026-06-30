-- D-4: private delivery proof-photo facts and media reference-type expansion.
-- All order/user/media identifiers are logical references; no cross-domain FK.

CREATE TABLE delivery_proofs (
  id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  proof_public_id CHAR(26) NOT NULL,
  delivery_order_public_id CHAR(26) NOT NULL,
  proof_type VARCHAR(16) NOT NULL,
  slot SMALLINT UNSIGNED NOT NULL DEFAULT 0,
  asset_public_id CHAR(26) NOT NULL,
  uploader_user_id BIGINT UNSIGNED NOT NULL,
  status VARCHAR(16) NOT NULL,
  operation_id CHAR(26) NOT NULL,
  row_version BIGINT UNSIGNED NOT NULL,
  attempt_count INT UNSIGNED NOT NULL DEFAULT 0,
  next_attempt_at DATETIME(3) NULL,
  lease_token CHAR(26) NULL,
  lease_expires_at DATETIME(3) NULL,
  created_at DATETIME(3) NOT NULL,
  updated_at DATETIME(3) NOT NULL,
  activated_at DATETIME(3) NULL,
  revoked_at DATETIME(3) NULL,
  PRIMARY KEY (id),
  UNIQUE INDEX uq_delivery_proof_public_id (proof_public_id),
  UNIQUE INDEX uq_delivery_proof_order_type_slot (delivery_order_public_id, proof_type, slot),
  UNIQUE INDEX uq_delivery_proof_asset (asset_public_id),
  UNIQUE INDEX uq_delivery_proof_operation (operation_id),
  INDEX idx_delivery_proof_order_status (delivery_order_public_id, status),
  INDEX idx_delivery_proof_binding_scan (status, next_attempt_at, lease_expires_at, id),
  CONSTRAINT chk_delivery_proof_type CHECK (proof_type IN ('pickup','delivery')),
  CONSTRAINT chk_delivery_proof_status CHECK (status IN ('binding','active','deadletter','revoked'))
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

ALTER TABLE media_asset_references
  ADD CONSTRAINT chk_media_reference_type_delivery_guard
  CHECK (reference_type IN ('post','product','avatar','dish','merchant_logo','delivery_proof'));

ALTER TABLE media_asset_references
  DROP CHECK chk_media_reference_type;

ALTER TABLE media_asset_references
  ADD CONSTRAINT chk_media_reference_type
  CHECK (reference_type IN ('post','product','avatar','dish','merchant_logo','delivery_proof'));

ALTER TABLE media_asset_references
  DROP CHECK chk_media_reference_type_delivery_guard;
