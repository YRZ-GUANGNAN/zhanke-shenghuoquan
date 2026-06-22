package media

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/joho/godotenv"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
)

const purposeTakeoutOwnerBase = uint64(99_001_818)

func Test00MediaReferenceTypeTakeoutMigrationAgainstRealMySQL84(t *testing.T) {
	db := openPurposeTakeoutMySQL(t)
	assertPurposeTakeoutMySQL84(t, db)
	cleanupPurposeTakeoutOwners(t, db)
	t.Cleanup(func() {
		cleanupPurposeTakeoutOwners(t, db)
		ensureExpandedPurposeConstraint(t, db)
	})

	if countNewPurposeReferences(t, db) != 0 {
		t.Fatal("refusing migration rollback test while dish/merchant_logo references exist")
	}
	assertNoPurposeGuardConstraint(t, db)
	if purposeConstraintExpanded(t, db) {
		applyPurposeMigration(t, db, "../../../migrations/rollback/018_media_reference_type_takeout.sql")
	}
	assertPurposeConstraint(t, db, false)
	baseline := readPurposeSchemaBoundary(t, db)

	applyPurposeMigration(t, db, "../../../migrations/018_media_reference_type_takeout.sql")
	assertPurposeConstraint(t, db, true)
	assertPurposeSchemaBoundaryUnchanged(t, baseline, readPurposeSchemaBoundary(t, db))

	service := NewService(NewRepository(db))
	asset, err := service.CreateAssetRecord(context.Background(), NewAsset{
		OwnerID: purposeTakeoutOwnerBase, Purpose: PurposeDish,
		IdempotencyKey: "purpose-rollback-guard",
	})
	if err != nil {
		t.Fatal(err)
	}
	operationID, _ := NewULID()
	if _, err := service.CreateReferenceRecord(context.Background(), AssetReference{
		AssetID: asset.ID, ReferenceType: PurposeDish, ReferenceID: "rollback-guard-dish", Slot: 0,
		Status: ReferencePending, ParentVisibility: ParentHidden, AggregateVersion: 1,
		OperationID: operationID, RowVersion: 1,
	}); err != nil {
		t.Fatal(err)
	}
	if err := executePurposeMigration(db, "../../../migrations/rollback/018_media_reference_type_takeout.sql"); err == nil {
		t.Fatal("rollback accepted a dish reference instead of failing closed")
	} else {
		t.Logf("rollback_with_new_reference=FAIL_CLOSED error=%s", sanitizePurposeError(err))
	}
	assertPurposeConstraint(t, db, true)
	assertNoPurposeGuardConstraint(t, db)

	cleanupPurposeTakeoutOwners(t, db)
	applyPurposeMigration(t, db, "../../../migrations/rollback/018_media_reference_type_takeout.sql")
	assertPurposeConstraint(t, db, false)
	assertPurposeSchemaBoundaryUnchanged(t, baseline, readPurposeSchemaBoundary(t, db))
	applyPurposeMigration(t, db, "../../../migrations/018_media_reference_type_takeout.sql")
	assertPurposeConstraint(t, db, true)
	assertPurposeSchemaBoundaryUnchanged(t, baseline, readPurposeSchemaBoundary(t, db))
	t.Logf("migration_apply_rollback_apply=PASS purpose_column_unchanged=%s reference_type_column=%s", baseline.PurposeColumn, baseline.ReferenceTypeColumn)
	t.Logf("final_constraint=%s temporary_guard_constraints=0", purposeConstraintClause(t, db, "chk_media_reference_type"))
}

func Test01TakeoutPurposesCreateBindAndGatewayVisibilityAgainstRealMySQL84(t *testing.T) {
	db := openPurposeTakeoutMySQL(t)
	assertPurposeTakeoutMySQL84(t, db)
	ensureExpandedPurposeConstraint(t, db)
	cleanupPurposeTakeoutOwners(t, db)
	t.Cleanup(func() { cleanupPurposeTakeoutOwners(t, db) })

	repository := NewRepository(db)
	service := NewService(repository)
	reviewService, err := NewReviewProjectionService(repository, func() time.Time {
		return time.Date(2026, 6, 22, 16, 0, 0, 0, time.UTC)
	})
	if err != nil {
		t.Fatal(err)
	}
	gateway, err := NewMediaGateway(repository)
	if err != nil {
		t.Fatal(err)
	}

	for index, purpose := range []Purpose{PurposeDish, PurposeMerchantLogo} {
		ownerID := purposeTakeoutOwnerBase + 2 + uint64(index)
		asset, err := service.CreateAssetRecord(context.Background(), NewAsset{
			OwnerID: ownerID, Purpose: purpose, IdempotencyKey: fmt.Sprintf("purpose-takeout-%s", purpose),
		})
		if err != nil {
			t.Fatalf("create %s asset: %v", purpose, err)
		}
		referenceID := fmt.Sprintf("takeout-%s-parent", purpose)
		operationID, _ := NewULID()
		reference, err := service.CreateReferenceRecord(context.Background(), AssetReference{
			AssetID: asset.ID, ReferenceType: purpose, ReferenceID: referenceID, Slot: 0,
			Status: ReferencePending, ParentVisibility: ParentVisible, AggregateVersion: 1,
			OperationID: operationID, RowVersion: 1,
		})
		if err != nil {
			t.Fatalf("bind %s reference: %v", purpose, err)
		}

		version := asset.RowVersion
		for _, transition := range []struct{ from, to TechnicalStatus }{
			{TechnicalPendingUpload, TechnicalUploaded},
			{TechnicalUploaded, TechnicalValidating},
			{TechnicalValidating, TechnicalPromoting},
			{TechnicalPromoting, TechnicalFinalReady},
		} {
			if err := service.TransitionTechnical(context.Background(), asset.ID, version, transition.from, transition.to); err != nil {
				t.Fatalf("%s technical transition %s->%s: %v", purpose, transition.from, transition.to, err)
			}
			version++
		}
		if err := service.TransitionStorage(context.Background(), asset.ID, version, StorageStageStaging, StorageStageFinal); err != nil {
			t.Fatalf("%s storage transition: %v", purpose, err)
		}
		version++
		finalKey := fmt.Sprintf("final/purpose-acceptance/%d/%s.webp", ownerID, asset.AssetPublicID)
		finalVersion := "purpose-fixed-" + asset.AssetPublicID
		trustedHash := strings.Repeat(fmt.Sprintf("%x", index+10), 64)[:64]
		if err := db.Model(&Asset{}).Where("id = ? AND row_version = ?", asset.ID, version).Updates(map[string]any{
			"final_key": finalKey, "final_version_id": finalVersion, "trusted_sha256": trustedHash,
			"verified_mime": "image/webp", "byte_size": 2048, "width": 800, "height": 600,
		}).Error; err != nil {
			t.Fatalf("prepare %s fixed final identity: %v", purpose, err)
		}
		if err := service.TransitionReference(context.Background(), reference.ID, reference.RowVersion, ReferencePending, ReferenceActive); err != nil {
			t.Fatalf("activate %s reference: %v", purpose, err)
		}

		projection, created, err := reviewService.EnsureProjection(context.Background(), asset.AssetPublicID)
		if err != nil || !created {
			t.Fatalf("ensure %s projection created=%v err=%v", purpose, created, err)
		}
		taskID := uint64(9_900_018_000 + index)
		projection, err = reviewService.Submit(context.Background(), projection.ProjectionID, taskID, projection.RowVersion)
		if err != nil {
			t.Fatal(err)
		}
		projection, err = reviewService.StartReview(context.Background(), projection.ProjectionID, projection.RowVersion)
		if err != nil {
			t.Fatal(err)
		}
		projection, err = reviewService.RecordDecision(context.Background(), ApplyReviewDecisionInput{
			ProjectionID: projection.ProjectionID, ReviewTaskID: taskID, DecisionVersion: 1,
			ExpectedStatus: ProjectionReviewing, ExpectedRowVersion: projection.RowVersion,
			Decision: ProjectionApproved, PayloadHash: trustedHash,
		})
		if err != nil || projection.Status != ProjectionApproved {
			t.Fatalf("approve %s projection=%#v err=%v", purpose, projection, err)
		}

		view, err := gateway.BuildDisplayView(context.Background(), purpose, referenceID, 0)
		if err != nil || view.Media.MediaID != asset.AssetPublicID || view.Media.Purpose != purpose ||
			view.Reference.ReferenceType != purpose {
			t.Fatalf("%s gateway view=%#v err=%v", purpose, view, err)
		}
		metadata, err := gateway.GetVisibleMetadata(context.Background(), asset.AssetPublicID)
		if err != nil || metadata.Purpose != purpose || metadata.ReviewStatus != ProjectionApproved {
			t.Fatalf("%s gateway metadata=%#v err=%v", purpose, metadata, err)
		}
		t.Logf("purpose=%s asset_create=PASS reference_bind=PASS projection=approved gateway_visible=PASS", purpose)
	}
}

func TestZZTakeoutPurposeAcceptanceResidueAgainstRealMySQL84(t *testing.T) {
	db := openPurposeTakeoutMySQL(t)
	cleanupPurposeTakeoutOwners(t, db)
	var assets, references, projections, decisions int64
	assetIDs := db.Model(&Asset{}).Select("id").Where("owner_id BETWEEN ? AND ?", purposeTakeoutOwnerBase, purposeTakeoutOwnerBase+99)
	projectionIDs := db.Model(&ReviewProjection{}).Select("projection_id").Where("asset_id IN (?)", assetIDs)
	if err := db.Model(&Asset{}).Where("owner_id BETWEEN ? AND ?", purposeTakeoutOwnerBase, purposeTakeoutOwnerBase+99).Count(&assets).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Model(&AssetReference{}).Where("asset_id IN (?)", assetIDs).Count(&references).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Model(&ReviewProjection{}).Where("asset_id IN (?)", assetIDs).Count(&projections).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Model(&ReviewDecision{}).Where("projection_id IN (?)", projectionIDs).Count(&decisions).Error; err != nil {
		t.Fatal(err)
	}
	if assets != 0 || references != 0 || projections != 0 || decisions != 0 {
		t.Fatalf("purpose acceptance residue assets=%d references=%d projections=%d decisions=%d", assets, references, projections, decisions)
	}
	t.Log("purpose_acceptance_residue assets=0 references=0 projections=0 decisions=0")
}

type purposeSchemaBoundary struct {
	PurposeColumn       string
	ReferenceTypeColumn string
	IndexSignature      string
	OtherCheckSignature string
}

func openPurposeTakeoutMySQL(t *testing.T) *gorm.DB {
	t.Helper()
	_ = godotenv.Load("../../../.env")
	dsn := os.Getenv("MEDIA_PURPOSE_TAKEOUT_MYSQL_DSN")
	if dsn == "" {
		dsn = os.Getenv("MYSQL_DSN")
	}
	if dsn == "" {
		t.Skip("set MEDIA_PURPOSE_TAKEOUT_MYSQL_DSN or MYSQL_DSN for real MySQL 8.4 acceptance")
	}
	db, err := gorm.Open(mysql.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	return db
}

func assertPurposeTakeoutMySQL84(t *testing.T, db *gorm.DB) {
	t.Helper()
	var version string
	if err := db.Raw("SELECT VERSION()").Scan(&version).Error; err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(version, "8.4.") {
		t.Fatalf("expected MySQL 8.4, got %s", version)
	}
	t.Logf("real_mysql_version=%s", version)
}

func applyPurposeMigration(t *testing.T, db *gorm.DB, path string) {
	t.Helper()
	if err := executePurposeMigration(db, path); err != nil {
		t.Fatalf("execute %s: %v", path, err)
	}
}

func executePurposeMigration(db *gorm.DB, path string) error {
	raw, err := os.ReadFile(filepath.Clean(path))
	if err != nil {
		return err
	}
	for _, statement := range strings.Split(string(raw), ";") {
		statement = strings.TrimSpace(statement)
		if statement == "" {
			continue
		}
		if err := db.Exec(statement).Error; err != nil {
			return err
		}
	}
	return nil
}

func ensureExpandedPurposeConstraint(t *testing.T, db *gorm.DB) {
	t.Helper()
	if purposeConstraintExpanded(t, db) {
		return
	}
	applyPurposeMigration(t, db, "../../../migrations/018_media_reference_type_takeout.sql")
}

func purposeConstraintExpanded(t *testing.T, db *gorm.DB) bool {
	t.Helper()
	clause := purposeConstraintClause(t, db, "chk_media_reference_type")
	return strings.Contains(clause, "dish") && strings.Contains(clause, "merchant_logo")
}

func assertPurposeConstraint(t *testing.T, db *gorm.DB, expanded bool) {
	t.Helper()
	clause := purposeConstraintClause(t, db, "chk_media_reference_type")
	for _, required := range []string{"post", "product", "avatar"} {
		if !strings.Contains(clause, required) {
			t.Errorf("constraint missing %s: %s", required, clause)
		}
	}
	hasDish := strings.Contains(clause, "dish")
	hasLogo := strings.Contains(clause, "merchant_logo")
	if hasDish != expanded || hasLogo != expanded {
		t.Fatalf("expanded=%v clause=%s", expanded, clause)
	}
}

func purposeConstraintClause(t *testing.T, db *gorm.DB, constraint string) string {
	t.Helper()
	var clause string
	if err := db.Raw(`SELECT cc.check_clause FROM information_schema.table_constraints tc
		JOIN information_schema.check_constraints cc
		ON cc.constraint_schema=tc.constraint_schema AND cc.constraint_name=tc.constraint_name
		WHERE tc.table_schema=DATABASE() AND tc.table_name='media_asset_references'
		AND tc.constraint_type='CHECK' AND tc.constraint_name=?`, constraint).Scan(&clause).Error; err != nil {
		t.Fatal(err)
	}
	if clause == "" {
		t.Fatalf("missing constraint %s", constraint)
	}
	return clause
}

func assertNoPurposeGuardConstraint(t *testing.T, db *gorm.DB) {
	t.Helper()
	var count int64
	if err := db.Raw(`SELECT COUNT(*) FROM information_schema.table_constraints
		WHERE table_schema=DATABASE() AND table_name='media_asset_references'
		AND constraint_name IN ('chk_media_reference_type_takeout_guard','chk_media_reference_type_rollback_guard')`).Scan(&count).Error; err != nil {
		t.Fatal(err)
	}
	if count != 0 {
		t.Fatalf("temporary purpose constraint residue=%d", count)
	}
}

func readPurposeSchemaBoundary(t *testing.T, db *gorm.DB) purposeSchemaBoundary {
	t.Helper()
	var result purposeSchemaBoundary
	if err := db.Raw(`SELECT column_type FROM information_schema.columns
		WHERE table_schema=DATABASE() AND table_name='media_assets' AND column_name='purpose'`).Scan(&result.PurposeColumn).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Raw(`SELECT column_type FROM information_schema.columns
		WHERE table_schema=DATABASE() AND table_name='media_asset_references' AND column_name='reference_type'`).Scan(&result.ReferenceTypeColumn).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Raw(`SELECT COALESCE(GROUP_CONCAT(CONCAT(index_name,':',non_unique,':',seq_in_index,':',column_name)
		ORDER BY index_name,seq_in_index SEPARATOR '|'),'') FROM information_schema.statistics
		WHERE table_schema=DATABASE() AND table_name='media_asset_references'`).Scan(&result.IndexSignature).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Raw(`SELECT COALESCE(GROUP_CONCAT(CONCAT(tc.constraint_name,':',cc.check_clause)
		ORDER BY tc.constraint_name SEPARATOR '|'),'') FROM information_schema.table_constraints tc
		JOIN information_schema.check_constraints cc
		ON cc.constraint_schema=tc.constraint_schema AND cc.constraint_name=tc.constraint_name
		WHERE tc.table_schema=DATABASE() AND tc.table_name='media_asset_references'
		AND tc.constraint_type='CHECK' AND tc.constraint_name NOT IN
		('chk_media_reference_type','chk_media_reference_type_takeout_guard','chk_media_reference_type_rollback_guard')`).Scan(&result.OtherCheckSignature).Error; err != nil {
		t.Fatal(err)
	}
	return result
}

func assertPurposeSchemaBoundaryUnchanged(t *testing.T, before, after purposeSchemaBoundary) {
	t.Helper()
	if before != after {
		t.Fatalf("migration changed out-of-scope schema before=%#v after=%#v", before, after)
	}
	if after.ReferenceTypeColumn != "varchar(16)" {
		t.Fatalf("reference_type column=%s want varchar(16)", after.ReferenceTypeColumn)
	}
}

func countNewPurposeReferences(t *testing.T, db *gorm.DB) int64 {
	t.Helper()
	var count int64
	if err := db.Model(&AssetReference{}).Where("reference_type IN ?", []Purpose{PurposeDish, PurposeMerchantLogo}).Count(&count).Error; err != nil {
		t.Fatal(err)
	}
	return count
}

func cleanupPurposeTakeoutOwners(t *testing.T, db *gorm.DB) {
	t.Helper()
	assetIDs := db.Model(&Asset{}).Select("id").Where("owner_id BETWEEN ? AND ?", purposeTakeoutOwnerBase, purposeTakeoutOwnerBase+99)
	projectionIDs := db.Model(&ReviewProjection{}).Select("projection_id").Where("asset_id IN (?)", assetIDs)
	for _, deletion := range []struct {
		label string
		query *gorm.DB
	}{
		{"decisions", db.Where("projection_id IN (?)", projectionIDs).Delete(&ReviewDecision{})},
		{"references", db.Where("asset_id IN (?)", assetIDs).Delete(&AssetReference{})},
		{"projections", db.Where("asset_id IN (?)", assetIDs).Delete(&ReviewProjection{})},
		{"assets", db.Where("owner_id BETWEEN ? AND ?", purposeTakeoutOwnerBase, purposeTakeoutOwnerBase+99).Delete(&Asset{})},
	} {
		if deletion.query.Error != nil {
			t.Errorf("cleanup purpose %s: %v", deletion.label, deletion.query.Error)
		}
	}
}

func sanitizePurposeError(err error) string {
	if err == nil {
		return ""
	}
	message := err.Error()
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return "record not found"
	}
	if len(message) > 180 {
		return message[:180]
	}
	return message
}
