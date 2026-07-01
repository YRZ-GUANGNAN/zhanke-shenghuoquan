-- D-5 rollback is fail-closed while any delivery IM fact or media binding exists.
DELIMITER $$
DROP PROCEDURE IF EXISTS rollback_027_delivery_im$$
CREATE PROCEDURE rollback_027_delivery_im()
BEGIN
  IF EXISTS (SELECT 1 FROM delivery_im_conversations LIMIT 1) THEN
    SIGNAL SQLSTATE '45000'
      SET MESSAGE_TEXT = 'rollback 027 refused: delivery_im_conversations still contains facts';
  END IF;
  IF EXISTS (
    SELECT 1 FROM media_asset_references
    WHERE reference_type = 'delivery_im'
    LIMIT 1
  ) THEN
    SIGNAL SQLSTATE '45000'
      SET MESSAGE_TEXT = 'rollback 027 refused: delivery_im media references still exist';
  END IF;
END$$
CALL rollback_027_delivery_im()$$
DROP PROCEDURE rollback_027_delivery_im$$
DELIMITER ;

ALTER TABLE media_asset_references
  ADD CONSTRAINT chk_media_reference_type_delivery_im_rollback_guard
  CHECK (reference_type IN ('post','product','avatar','dish','merchant_logo','delivery_proof'));

ALTER TABLE media_asset_references
  DROP CHECK chk_media_reference_type;

ALTER TABLE media_asset_references
  ADD CONSTRAINT chk_media_reference_type
  CHECK (reference_type IN ('post','product','avatar','dish','merchant_logo','delivery_proof'));

ALTER TABLE media_asset_references
  DROP CHECK chk_media_reference_type_delivery_im_rollback_guard;

DROP TABLE delivery_im_conversations;
