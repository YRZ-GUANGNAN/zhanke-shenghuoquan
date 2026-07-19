-- Independent rider/merchant identity applications, audit history and grants.
-- Forward-only: approved identities and their review trail are retained when an
-- application release is rolled back. Reapplying the migration is idempotent.

DELIMITER $$
DROP PROCEDURE IF EXISTS migrate_identity_application_review$$
CREATE PROCEDURE migrate_identity_application_review()
SQL SECURITY INVOKER
BEGIN
  DECLARE lock_acquired INT DEFAULT 0;

  DECLARE EXIT HANDLER FOR SQLEXCEPTION
  BEGIN
    IF lock_acquired = 1 THEN
      DO RELEASE_LOCK('campus_circle:migration:034');
    END IF;
    RESIGNAL;
  END;

  SELECT GET_LOCK('campus_circle:migration:034', 10) INTO lock_acquired;
  IF lock_acquired IS NULL OR lock_acquired <> 1 THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT = 'migration 034 lock unavailable';
  END IF;

  IF (SELECT COUNT(*) FROM information_schema.tables
      WHERE table_schema=DATABASE()
        AND table_name IN ('users','media_assets','media_asset_references','takeout_riders','delivery_orders')) <> 5 THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT = 'migration 034 base tables missing';
  END IF;

  IF (SELECT COUNT(*) FROM information_schema.columns
      WHERE table_schema=DATABASE() AND table_name='users' AND (
        (column_name='id' AND column_type='bigint unsigned' AND is_nullable='NO') OR
        (column_name='open_id' AND data_type='varchar' AND is_nullable='NO') OR
        (column_name='status' AND data_type='varchar' AND is_nullable='NO') OR
        (column_name='closed_at' AND data_type='datetime' AND is_nullable='YES') OR
        (column_name='deleted_at' AND data_type='datetime' AND is_nullable='YES')
      )) <> 5 THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT = 'migration 034 users schema mismatch';
  END IF;
  IF (SELECT COUNT(*) FROM information_schema.columns
      WHERE table_schema=DATABASE() AND (
        (table_name='takeout_riders' AND column_name='rider_user_id' AND column_type='bigint unsigned' AND is_nullable='NO') OR
        (table_name='takeout_riders' AND column_name='status' AND data_type='varchar' AND is_nullable='NO') OR
        (table_name='delivery_orders' AND column_name='merchant_user_id' AND column_type='bigint unsigned' AND is_nullable='NO') OR
        (table_name='media_assets' AND column_name='purpose' AND data_type='varchar' AND is_nullable='NO') OR
        (table_name='media_asset_references' AND column_name='reference_type' AND data_type='varchar' AND is_nullable='NO')
      )) <> 5 THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT = 'migration 034 domain schema mismatch';
  END IF;

  IF EXISTS (
    SELECT 1 FROM media_assets
    WHERE CHAR_LENGTH(purpose) > 24 OR purpose NOT IN (
      'post','product','avatar','dish','merchant_logo','delivery_proof','delivery_im','feedback','identity_application'
    )
  ) THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT = 'migration 034 unsupported media purpose';
  END IF;
  IF EXISTS (
    SELECT 1 FROM media_asset_references
    WHERE CHAR_LENGTH(reference_type) > 24 OR reference_type NOT IN (
      'post','product','avatar','dish','merchant_logo','delivery_proof','delivery_im','feedback','identity_application'
    )
  ) THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT = 'migration 034 unsupported media reference type';
  END IF;

  IF EXISTS (
    SELECT 1
    FROM information_schema.table_constraints AS table_constraint
    JOIN information_schema.check_constraints AS check_constraint
      ON check_constraint.constraint_schema=table_constraint.constraint_schema
     AND check_constraint.constraint_name=table_constraint.constraint_name
    WHERE table_constraint.constraint_schema=DATABASE()
      AND table_constraint.table_name='media_assets'
      AND table_constraint.constraint_type='CHECK'
      AND LOCATE('purpose',LOWER(REPLACE(check_constraint.check_clause,'`',''))) > 0
      AND table_constraint.constraint_name NOT IN (
        'chk_media_asset_purpose',
        'chk_media_asset_purpose_identity_application_guard'
      )
  ) THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT = 'migration 034 unexpected media purpose check constraint';
  END IF;
  IF EXISTS (
    SELECT 1
    FROM information_schema.table_constraints AS table_constraint
    JOIN information_schema.check_constraints AS check_constraint
      ON check_constraint.constraint_schema=table_constraint.constraint_schema
     AND check_constraint.constraint_name=table_constraint.constraint_name
    WHERE table_constraint.constraint_schema=DATABASE()
      AND table_constraint.table_name='media_asset_references'
      AND table_constraint.constraint_type='CHECK'
      AND LOCATE('reference_type',LOWER(REPLACE(check_constraint.check_clause,'`',''))) > 0
      AND table_constraint.constraint_name NOT IN (
        'chk_media_reference_type',
        'chk_media_reference_type_identity_application_guard'
      )
  ) THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT = 'migration 034 unexpected media reference type check constraint';
  END IF;
  IF EXISTS (
    SELECT 1 FROM information_schema.table_constraints
    WHERE constraint_schema=DATABASE()
      AND constraint_name IN (
        'chk_media_asset_purpose',
        'chk_media_asset_purpose_identity_application_guard'
      )
      AND (table_name<>'media_assets' OR constraint_type<>'CHECK')
  ) THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT = 'migration 034 conflicting media purpose constraint';
  END IF;
  IF EXISTS (
    SELECT 1 FROM information_schema.table_constraints
    WHERE constraint_schema=DATABASE()
      AND constraint_name IN (
        'chk_media_reference_type',
        'chk_media_reference_type_identity_application_guard'
      )
      AND (table_name<>'media_asset_references' OR constraint_type<>'CHECK')
  ) THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT = 'migration 034 conflicting media reference type constraint';
  END IF;

  IF NOT EXISTS (
    SELECT 1 FROM information_schema.tables
    WHERE table_schema=DATABASE() AND table_name='identity_applications'
  ) THEN
    CREATE TABLE identity_applications (
      id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
      application_public_id CHAR(26) NOT NULL,
      user_id BIGINT UNSIGNED NOT NULL,
      role VARCHAR(16) NOT NULL,
      status VARCHAR(16) NOT NULL DEFAULT 'pending',
      real_name VARCHAR(64) NOT NULL,
      phone VARCHAR(32) NOT NULL,
      merchant_name VARCHAR(120) NOT NULL DEFAULT '',
      merchant_address VARCHAR(240) NOT NULL DEFAULT '',
      review_reason VARCHAR(500) NOT NULL DEFAULT '',
      reviewer_user_id BIGINT UNSIGNED NULL,
      reviewed_at DATETIME(3) NULL,
      submitted_at DATETIME(3) NOT NULL,
      row_version BIGINT UNSIGNED NOT NULL DEFAULT 1,
      created_at DATETIME(3) NOT NULL,
      updated_at DATETIME(3) NOT NULL,
      PRIMARY KEY (id),
      UNIQUE KEY uq_identity_application_public_id (application_public_id),
      UNIQUE KEY uq_identity_application_user_role (user_id, role),
      KEY idx_identity_application_status_submitted (status, submitted_at, id),
      CONSTRAINT chk_identity_application_role CHECK (role IN ('rider','merchant')),
      CONSTRAINT chk_identity_application_status CHECK (status IN ('pending','approved','rejected'))
    ) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
  END IF;

  IF NOT EXISTS (
    SELECT 1 FROM information_schema.tables
    WHERE table_schema=DATABASE() AND table_name='identity_application_assets'
  ) THEN
    CREATE TABLE identity_application_assets (
      id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
      application_id BIGINT UNSIGNED NOT NULL,
      asset_public_id CHAR(26) NOT NULL,
      sort_order SMALLINT UNSIGNED NOT NULL,
      created_at DATETIME(3) NOT NULL,
      PRIMARY KEY (id),
      UNIQUE KEY uq_identity_application_asset_slot (application_id, sort_order),
      UNIQUE KEY uq_identity_application_asset (application_id, asset_public_id)
    ) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
  END IF;

  IF NOT EXISTS (
    SELECT 1 FROM information_schema.tables
    WHERE table_schema=DATABASE() AND table_name='identity_application_events'
  ) THEN
    CREATE TABLE identity_application_events (
      id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
      event_public_id CHAR(26) NOT NULL,
      application_id BIGINT UNSIGNED NOT NULL,
      actor_user_id BIGINT UNSIGNED NOT NULL,
      event_type VARCHAR(16) NOT NULL,
      status VARCHAR(16) NOT NULL,
      reason VARCHAR(500) NOT NULL DEFAULT '',
      snapshot_json JSON NOT NULL,
      created_at DATETIME(3) NOT NULL,
      PRIMARY KEY (id),
      UNIQUE KEY uq_identity_application_event_public_id (event_public_id),
      KEY idx_identity_application_event_application (application_id, created_at, id),
      CONSTRAINT chk_identity_application_event_type CHECK (event_type IN ('submitted','resubmitted','approved','rejected')),
      CONSTRAINT chk_identity_application_event_status CHECK (status IN ('pending','approved','rejected'))
    ) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
  END IF;

  IF NOT EXISTS (
    SELECT 1 FROM information_schema.tables
    WHERE table_schema=DATABASE() AND table_name='user_identity_grants'
  ) THEN
    CREATE TABLE user_identity_grants (
      id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
      user_id BIGINT UNSIGNED NOT NULL,
      role VARCHAR(16) NOT NULL,
      status VARCHAR(16) NOT NULL DEFAULT 'active',
      source_application_id BIGINT UNSIGNED NULL,
      granted_at DATETIME(3) NOT NULL,
      row_version BIGINT UNSIGNED NOT NULL DEFAULT 1,
      created_at DATETIME(3) NOT NULL,
      updated_at DATETIME(3) NOT NULL,
      PRIMARY KEY (id),
      UNIQUE KEY uq_identity_grant_user_role (user_id, role),
      KEY idx_identity_grant_status (status, role, user_id),
      CONSTRAINT chk_identity_grant_role CHECK (role IN ('rider','merchant')),
      CONSTRAINT chk_identity_grant_status CHECK (status IN ('active','suspended'))
    ) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
  END IF;

  IF NOT EXISTS (
    SELECT 1 FROM information_schema.columns
    WHERE table_schema=DATABASE() AND table_name='media_assets' AND column_name='purpose'
      AND data_type='varchar' AND character_maximum_length=24 AND is_nullable='NO'
  ) THEN
    ALTER TABLE media_assets
      MODIFY COLUMN purpose VARCHAR(24) NOT NULL;
  END IF;
  IF NOT EXISTS (
    SELECT 1 FROM information_schema.columns
    WHERE table_schema=DATABASE() AND table_name='media_asset_references' AND column_name='reference_type'
      AND data_type='varchar' AND character_maximum_length=24 AND is_nullable='NO'
  ) THEN
    ALTER TABLE media_asset_references
      MODIFY COLUMN reference_type VARCHAR(24) NOT NULL;
  END IF;

  IF EXISTS (
    SELECT 1 FROM information_schema.table_constraints
    WHERE constraint_schema=DATABASE() AND table_name='media_assets'
      AND constraint_name='chk_media_asset_purpose'
  ) THEN
    IF NOT EXISTS (
      SELECT 1 FROM information_schema.table_constraints
      WHERE constraint_schema=DATABASE() AND table_name='media_assets'
        AND constraint_name='chk_media_asset_purpose_identity_application_guard'
    ) THEN
      ALTER TABLE media_assets
        ADD CONSTRAINT chk_media_asset_purpose_identity_application_guard
        CHECK (purpose IN ('post','product','avatar','dish','merchant_logo','delivery_proof','delivery_im','feedback','identity_application'));
    END IF;
    ALTER TABLE media_assets
      DROP CHECK chk_media_asset_purpose;
  END IF;
  IF NOT EXISTS (
    SELECT 1 FROM information_schema.table_constraints
    WHERE constraint_schema=DATABASE() AND table_name='media_assets'
      AND constraint_name='chk_media_asset_purpose'
  ) THEN
    ALTER TABLE media_assets
      ADD CONSTRAINT chk_media_asset_purpose
      CHECK (purpose IN ('post','product','avatar','dish','merchant_logo','delivery_proof','delivery_im','feedback','identity_application'));
  END IF;
  IF EXISTS (
    SELECT 1 FROM information_schema.table_constraints
    WHERE constraint_schema=DATABASE() AND table_name='media_assets'
      AND constraint_name='chk_media_asset_purpose_identity_application_guard'
  ) THEN
    ALTER TABLE media_assets
      DROP CHECK chk_media_asset_purpose_identity_application_guard;
  END IF;

  IF EXISTS (
    SELECT 1 FROM information_schema.table_constraints
    WHERE constraint_schema=DATABASE() AND table_name='media_asset_references'
      AND constraint_name='chk_media_reference_type'
  ) THEN
    IF NOT EXISTS (
      SELECT 1 FROM information_schema.table_constraints
      WHERE constraint_schema=DATABASE() AND table_name='media_asset_references'
        AND constraint_name='chk_media_reference_type_identity_application_guard'
    ) THEN
      ALTER TABLE media_asset_references
        ADD CONSTRAINT chk_media_reference_type_identity_application_guard
        CHECK (reference_type IN ('post','product','avatar','dish','merchant_logo','delivery_proof','delivery_im','feedback','identity_application'));
    END IF;
    ALTER TABLE media_asset_references
      DROP CHECK chk_media_reference_type;
  END IF;
  IF NOT EXISTS (
    SELECT 1 FROM information_schema.table_constraints
    WHERE constraint_schema=DATABASE() AND table_name='media_asset_references'
      AND constraint_name='chk_media_reference_type'
  ) THEN
    ALTER TABLE media_asset_references
      ADD CONSTRAINT chk_media_reference_type
      CHECK (reference_type IN ('post','product','avatar','dish','merchant_logo','delivery_proof','delivery_im','feedback','identity_application'));
  END IF;
  IF EXISTS (
    SELECT 1 FROM information_schema.table_constraints
    WHERE constraint_schema=DATABASE() AND table_name='media_asset_references'
      AND constraint_name='chk_media_reference_type_identity_application_guard'
  ) THEN
    ALTER TABLE media_asset_references
      DROP CHECK chk_media_reference_type_identity_application_guard;
  END IF;

  -- Recover safely if an earlier interrupted/replaced 034 backfill activated a
  -- legacy grant for an account that is no longer eligible. Application-backed
  -- grants are audit facts and are intentionally outside this migration repair.
  UPDATE user_identity_grants AS identity_grant
  JOIN users ON users.id=identity_grant.user_id
  SET identity_grant.status='suspended',
      identity_grant.row_version=identity_grant.row_version+1,
      identity_grant.updated_at=NOW(3)
  WHERE identity_grant.source_application_id IS NULL
    AND identity_grant.status='active'
    AND (
      users.open_id LIKE 'mock_openid\_%' OR
      LEFT(users.open_id,12)='mock_openid_' OR
      users.deleted_at IS NOT NULL OR
      users.closed_at IS NOT NULL OR
      users.status NOT IN ('active','muted')
    );

  INSERT INTO user_identity_grants
    (user_id,role,status,source_application_id,granted_at,row_version,created_at,updated_at)
  SELECT tr.rider_user_id,'rider','active',NULL,NOW(3),1,NOW(3),NOW(3)
  FROM takeout_riders tr
  JOIN users ON users.id=tr.rider_user_id
  WHERE tr.status='active'
    AND users.open_id NOT LIKE 'mock_openid\_%'
    AND LEFT(users.open_id,12)<>'mock_openid_'
    AND users.deleted_at IS NULL
    AND users.closed_at IS NULL
    AND users.status IN ('active','muted')
  ON DUPLICATE KEY UPDATE user_id=VALUES(user_id);

  INSERT INTO user_identity_grants
    (user_id,role,status,source_application_id,granted_at,row_version,created_at,updated_at)
  SELECT DISTINCT o.merchant_user_id,'merchant','active',NULL,NOW(3),1,NOW(3),NOW(3)
  FROM delivery_orders o
  JOIN users ON users.id=o.merchant_user_id
  WHERE users.open_id NOT LIKE 'mock_openid\_%'
    AND LEFT(users.open_id,12)<>'mock_openid_'
    AND users.deleted_at IS NULL
    AND users.closed_at IS NULL
    AND users.status IN ('active','muted')
  ON DUPLICATE KEY UPDATE user_id=VALUES(user_id);

  DO RELEASE_LOCK('campus_circle:migration:034');
  SET lock_acquired = 0;
END$$
CALL migrate_identity_application_review()$$
DROP PROCEDURE migrate_identity_application_review$$
DELIMITER ;
