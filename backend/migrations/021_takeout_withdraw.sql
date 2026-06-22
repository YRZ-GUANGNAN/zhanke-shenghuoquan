-- G-6a-1: rider withdrawal requests. Domain G keeps logical references only.
CREATE TABLE takeout_withdraw_requests (
  id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  withdraw_public_id CHAR(26) NOT NULL,
  rider_user_id BIGINT UNSIGNED NOT NULL,
  amount_cents BIGINT UNSIGNED NOT NULL,
  status VARCHAR(24) NOT NULL,
  note VARCHAR(256) NOT NULL DEFAULT '',
  paid_at DATETIME(3) NULL,
  row_version BIGINT UNSIGNED NOT NULL,
  created_at DATETIME(3) NOT NULL,
  updated_at DATETIME(3) NOT NULL,
  PRIMARY KEY (id),
  UNIQUE INDEX uq_takeout_withdraw_public_id (withdraw_public_id),
  INDEX idx_takeout_withdraw_rider (rider_user_id),
  INDEX idx_takeout_withdraw_status (status),
  CONSTRAINT chk_takeout_withdraw_status CHECK (status IN ('pending','paid','rejected'))
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
