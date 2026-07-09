-- P0 identity switch: current user-facing identity is owned by Domain A.
-- Rider and merchant eligibility remain logical facts in takeout tables.

DELIMITER $$
CREATE PROCEDURE migrate_user_current_identity()
BEGIN
  IF NOT EXISTS (
    SELECT 1 FROM information_schema.columns
    WHERE table_schema = DATABASE()
      AND table_name = 'users'
      AND column_name = 'current_identity'
  ) THEN
    ALTER TABLE users
      ADD COLUMN current_identity VARCHAR(24) NOT NULL DEFAULT 'student' AFTER campus;
  END IF;

  UPDATE users
    SET current_identity = 'student'
    WHERE current_identity NOT IN ('student', 'rider', 'merchant');

  IF NOT EXISTS (
    SELECT 1 FROM information_schema.table_constraints
    WHERE table_schema = DATABASE()
      AND table_name = 'users'
      AND constraint_name = 'chk_users_current_identity'
  ) THEN
    ALTER TABLE users
      ADD CONSTRAINT chk_users_current_identity
      CHECK (current_identity IN ('student', 'rider', 'merchant'));
  END IF;
END$$
CALL migrate_user_current_identity()$$
DROP PROCEDURE migrate_user_current_identity$$
DELIMITER ;
