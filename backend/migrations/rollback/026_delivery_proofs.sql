-- D-4 rollback is fail-closed. It refuses to remove the private-proof schema
-- while any proof fact or delivery_proof media reference still exists.

DELIMITER //
CREATE PROCEDURE rollback_026_delivery_proofs()
BEGIN
  IF EXISTS (SELECT 1 FROM delivery_proofs LIMIT 1) THEN
    SIGNAL SQLSTATE '45000'
      SET MESSAGE_TEXT = 'rollback 026 refused: delivery_proofs still contains facts';
  END IF;
  IF EXISTS (
    SELECT 1 FROM media_asset_references
    WHERE reference_type = 'delivery_proof'
    LIMIT 1
  ) THEN
    SIGNAL SQLSTATE '45000'
      SET MESSAGE_TEXT = 'rollback 026 refused: delivery_proof media references still exist';
  END IF;
END//
DELIMITER ;

CALL rollback_026_delivery_proofs();
DROP PROCEDURE rollback_026_delivery_proofs;

ALTER TABLE media_asset_references
  ADD CONSTRAINT chk_media_reference_type_delivery_rollback_guard
  CHECK (reference_type IN ('post','product','avatar','dish','merchant_logo'));

ALTER TABLE media_asset_references
  DROP CHECK chk_media_reference_type;

ALTER TABLE media_asset_references
  ADD CONSTRAINT chk_media_reference_type
  CHECK (reference_type IN ('post','product','avatar','dish','merchant_logo'));

ALTER TABLE media_asset_references
  DROP CHECK chk_media_reference_type_delivery_rollback_guard;

DROP TABLE delivery_proofs;
