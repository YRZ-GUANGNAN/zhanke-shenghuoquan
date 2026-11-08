-- Source-bound rider withdrawals. A withdrawal may consume several settled
-- earnings, while one earning may be split across several withdrawals.
-- No rows are created by this migration.

CREATE TABLE delivery_rider_withdrawal_allocations (
  id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  allocation_public_id CHAR(26) NOT NULL,
  withdraw_public_id CHAR(26) NOT NULL,
  earning_public_id CHAR(26) NOT NULL,
  delivery_order_public_id CHAR(26) NOT NULL,
  rider_user_id BIGINT UNSIGNED NOT NULL,
  wallet_public_id CHAR(26) NOT NULL,
  amount_cents BIGINT UNSIGNED NOT NULL,
  currency VARCHAR(8) NOT NULL DEFAULT 'CNY',
  status VARCHAR(16) NOT NULL,
  source_policy_version VARCHAR(32) NOT NULL,
  source_evidence_hash CHAR(64) NOT NULL,
  earning_row_version BIGINT UNSIGNED NOT NULL,
  created_at DATETIME(3) NOT NULL,
  updated_at DATETIME(3) NOT NULL,
  PRIMARY KEY (id),
  UNIQUE INDEX uq_delivery_rider_withdrawal_allocation_public_id (allocation_public_id),
  UNIQUE INDEX uq_delivery_rider_withdrawal_allocation_pair (withdraw_public_id, earning_public_id),
  INDEX idx_delivery_rider_withdrawal_allocation_earning_status (earning_public_id, status, id),
  INDEX idx_delivery_rider_withdrawal_allocation_withdraw_status (withdraw_public_id, status, id),
  INDEX idx_delivery_rider_withdrawal_allocation_rider_status (rider_user_id, status, id),
  CONSTRAINT chk_delivery_rider_withdrawal_allocation_amount CHECK (amount_cents > 0 AND amount_cents <= 9007199254740991),
  CONSTRAINT chk_delivery_rider_withdrawal_allocation_currency CHECK (currency = 'CNY'),
  CONSTRAINT chk_delivery_rider_withdrawal_allocation_status CHECK (status IN ('in_flight', 'paid', 'released')),
  CONSTRAINT chk_delivery_rider_withdrawal_allocation_source CHECK (
    CHAR_LENGTH(TRIM(source_policy_version)) > 0 AND CHAR_LENGTH(source_evidence_hash) = 64
  )
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

-- Immutable processing audit. It intentionally stores no payment credentials.
CREATE TABLE delivery_rider_withdrawal_events (
  id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  event_public_id CHAR(26) NOT NULL,
  withdraw_public_id CHAR(26) NOT NULL,
  event_type VARCHAR(24) NOT NULL,
  operator_user_id BIGINT UNSIGNED NOT NULL,
  reason VARCHAR(256) NOT NULL DEFAULT '',
  payment_reference VARCHAR(128) NOT NULL DEFAULT '',
  claim_key VARCHAR(26) NOT NULL DEFAULT '',
  request_hash CHAR(64) NOT NULL,
  idempotency_key CHAR(26) NOT NULL,
  expected_row_version BIGINT UNSIGNED NOT NULL,
  created_at DATETIME(3) NOT NULL,
  PRIMARY KEY (id),
  UNIQUE INDEX uq_delivery_rider_withdrawal_event_public_id (event_public_id),
  UNIQUE INDEX uq_delivery_rider_withdrawal_event_idempotency (idempotency_key),
  INDEX idx_delivery_rider_withdrawal_event_withdraw_created (withdraw_public_id, created_at, id),
  CONSTRAINT chk_delivery_rider_withdrawal_event_type CHECK (event_type IN ('claimed', 'released', 'paid', 'rejected', 'exception')),
  CONSTRAINT chk_delivery_rider_withdrawal_event_operator CHECK (
    operator_user_id > 0 AND expected_row_version > 0 AND CHAR_LENGTH(request_hash) = 64
  )
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE delivery_rider_withdrawal_claims (
  id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  claim_public_id CHAR(26) NOT NULL,
  withdraw_public_id CHAR(26) NOT NULL,
  operator_user_id BIGINT UNSIGNED NOT NULL,
  claim_key CHAR(26) NOT NULL,
  expected_row_version BIGINT UNSIGNED NOT NULL,
  status VARCHAR(16) NOT NULL,
  reason VARCHAR(256) NOT NULL DEFAULT '',
  payment_reference VARCHAR(128) NOT NULL DEFAULT '',
  idempotency_key CHAR(26) NOT NULL,
  created_at DATETIME(3) NOT NULL,
  updated_at DATETIME(3) NOT NULL,
  PRIMARY KEY (id),
  UNIQUE INDEX uq_delivery_rider_withdrawal_claim_public_id (claim_public_id),
  UNIQUE INDEX uq_delivery_rider_withdrawal_claim_withdraw (withdraw_public_id),
  UNIQUE INDEX uq_delivery_rider_withdrawal_claim_key (claim_key),
  UNIQUE INDEX uq_delivery_rider_withdrawal_claim_idempotency (idempotency_key),
  INDEX idx_delivery_rider_withdrawal_claim_operator_status (operator_user_id, status, id),
  CONSTRAINT chk_delivery_rider_withdrawal_claim_status CHECK (status IN ('active', 'released', 'completed', 'exception')),
  CONSTRAINT chk_delivery_rider_withdrawal_claim_operator CHECK (operator_user_id > 0 AND expected_row_version > 0)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

-- A manual payout receipt may belong to exactly one withdrawal.  This is
-- separate from the withdrawal row so a payment reference cannot be reused by
-- another withdrawal during retries or reconciliation.
CREATE TABLE delivery_rider_withdrawal_payment_references (
  id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  payment_reference VARCHAR(128) NOT NULL,
  withdraw_public_id CHAR(26) NOT NULL,
  created_at DATETIME(3) NOT NULL,
  PRIMARY KEY (id),
  UNIQUE INDEX uq_delivery_rider_withdrawal_payment_reference (payment_reference),
  UNIQUE INDEX uq_delivery_rider_withdrawal_payment_withdraw (withdraw_public_id),
  CONSTRAINT chk_delivery_rider_withdrawal_payment_reference CHECK (CHAR_LENGTH(TRIM(payment_reference)) > 0)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
