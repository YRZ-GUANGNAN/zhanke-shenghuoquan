-- Roll back Domain A's user-facing identity state.

DELIMITER $$
CREATE PROCEDURE rollback_user_current_identity()
BEGIN
  IF EXISTS (
    SELECT 1 FROM information_schema.table_constraints
    WHERE table_schema = DATABASE()
      AND table_name = 'users'
      AND constraint_name = 'chk_users_current_identity'
  ) THEN
    ALTER TABLE users DROP CHECK chk_users_current_identity;
  END IF;

  IF EXISTS (
    SELECT 1 FROM information_schema.columns
    WHERE table_schema = DATABASE()
      AND table_name = 'users'
      AND column_name = 'current_identity'
  ) THEN
    ALTER TABLE users DROP COLUMN current_identity;
  END IF;
END$$
CALL rollback_user_current_identity()$$
DROP PROCEDURE rollback_user_current_identity$$
DELIMITER ;
