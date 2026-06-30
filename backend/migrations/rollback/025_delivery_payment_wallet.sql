-- D-3 rollback. Operators must archive or explicitly clear money facts before
-- invoking this destructive schema rollback.
DROP TABLE IF EXISTS delivery_refunds;
DROP TABLE IF EXISTS delivery_fee_settlements;
DROP TABLE IF EXISTS delivery_payment_notifications;
DROP TABLE IF EXISTS delivery_payment_attempts;
DROP TABLE IF EXISTS delivery_wallet_ledger;
DROP TABLE IF EXISTS delivery_merchant_wallets;
