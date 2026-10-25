-- Remove only funding provenance introduced by migration 049.

DROP TABLE IF EXISTS delivery_funding_allocations;
DROP TABLE IF EXISTS delivery_funding_lots;
