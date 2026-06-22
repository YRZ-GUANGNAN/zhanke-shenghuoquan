package media

import (
	"context"
	"errors"
	"fmt"
	"os"
	"reflect"
	"testing"
	"time"

	"github.com/joho/godotenv"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
)

type gatewayFixture struct {
	Asset      Asset
	Projection ReviewProjection
	Reference  AssetReference
}

func TestMediaGatewayRepositoryAndServiceAgainstMySQL84(t *testing.T) {
	db := openModule6MySQL(t)
	assertMySQL84(t, db)
	ownerID := uint64(99_000_601)
	cleanupGatewayOwner(t, db, ownerID)
	t.Cleanup(func() { cleanupGatewayOwner(t, db, ownerID) })
	now := time.Date(2026, 6, 21, 20, 0, 0, 0, time.UTC)
	fixture := createGatewayFixture(t, db, ownerID, "visible", 0, ProjectionApproved, ReferenceActive, ParentVisible, "", now)
	var assetBefore Asset
	var projectionBefore ReviewProjection
	var referenceBefore AssetReference
	if err := db.First(&assetBefore, fixture.Asset.ID).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Where("projection_id = ?", fixture.Projection.ProjectionID).First(&projectionBefore).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.First(&referenceBefore, fixture.Reference.ID).Error; err != nil {
		t.Fatal(err)
	}

	repository := NewRepository(db)
	gateway, err := NewMediaGateway(repository)
	if err != nil {
		t.Fatal(err)
	}
	visibility, err := gateway.CheckVisibility(context.Background(), fixture.Asset.AssetPublicID)
	if err != nil || !visibility.Visible || visibility.Reason != VisibilityAllowed {
		t.Fatalf("visibility=%#v err=%v", visibility, err)
	}
	metadata, err := gateway.GetVisibleMetadata(context.Background(), fixture.Asset.AssetPublicID)
	if err != nil || metadata.MediaID != fixture.Asset.AssetPublicID || metadata.MIMEType != "image/webp" || metadata.ReviewStatus != ProjectionApproved {
		t.Fatalf("metadata=%#v err=%v", metadata, err)
	}
	projection, err := gateway.GetReviewProjection(context.Background(), fixture.Asset.AssetPublicID)
	if err != nil || projection.Status != ProjectionApproved || projection.AppliedDecisionVersion != 1 {
		t.Fatalf("projection=%#v err=%v", projection, err)
	}
	view, err := gateway.BuildDisplayView(context.Background(), PurposePost, fixture.Reference.ReferenceID, fixture.Reference.Slot)
	if err != nil || view.Media.MediaID != fixture.Asset.AssetPublicID || view.Reference.ReferenceID != fixture.Reference.ReferenceID {
		t.Fatalf("view=%#v err=%v", view, err)
	}
	views, err := gateway.ListDisplayViews(context.Background(), PurposePost, fixture.Reference.ReferenceID)
	if err != nil || len(views) != 1 {
		t.Fatalf("views=%#v err=%v", views, err)
	}

	var assetAfter Asset
	var projectionAfter ReviewProjection
	var referenceAfter AssetReference
	if err := db.First(&assetAfter, fixture.Asset.ID).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Where("projection_id = ?", fixture.Projection.ProjectionID).First(&projectionAfter).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.First(&referenceAfter, fixture.Reference.ID).Error; err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(assetBefore, assetAfter) || !reflect.DeepEqual(projectionBefore, projectionAfter) || !reflect.DeepEqual(referenceBefore, referenceAfter) {
		t.Fatalf("Gateway mutated persisted state\nasset=%#v\nprojection=%#v\nreference=%#v", assetAfter, projectionAfter, referenceAfter)
	}
}

func TestMediaGatewayFiltersUnapprovedHiddenAndRevokedAgainstMySQL84(t *testing.T) {
	db := openModule6MySQL(t)
	ownerID := uint64(99_000_602)
	cleanupGatewayOwner(t, db, ownerID)
	t.Cleanup(func() { cleanupGatewayOwner(t, db, ownerID) })
	now := time.Date(2026, 6, 21, 20, 30, 0, 0, time.UTC)
	parentID := "post-gateway-filter"
	approved := createGatewayFixture(t, db, ownerID, parentID, 0, ProjectionApproved, ReferenceActive, ParentVisible, "", now)
	rejected := createGatewayFixture(t, db, ownerID, parentID, 1, ProjectionRejected, ReferenceActive, ParentVisible, "", now)
	hidden := createGatewayFixture(t, db, ownerID, parentID, 2, ProjectionApproved, ReferenceActive, ParentHidden, "", now)
	fence, _ := NewULID()
	revoked := createGatewayFixture(t, db, ownerID, parentID, 3, ProjectionApproved, ReferenceActive, ParentVisible, fence, now)

	gateway, _ := NewMediaGateway(NewRepository(db))
	views, err := gateway.ListDisplayViews(context.Background(), PurposePost, parentID)
	if err != nil || len(views) != 1 || views[0].Media.MediaID != approved.Asset.AssetPublicID {
		t.Fatalf("fail-closed list=%#v err=%v", views, err)
	}
	for _, fixture := range []gatewayFixture{rejected, hidden, revoked} {
		if _, err := gateway.GetVisibleMetadata(context.Background(), fixture.Asset.AssetPublicID); !errors.Is(err, ErrMediaNotVisible) {
			t.Fatalf("non-visible asset %s returned %v", fixture.Asset.AssetPublicID, err)
		}
	}
	projection, err := gateway.GetReviewProjection(context.Background(), rejected.Asset.AssetPublicID)
	if err != nil || projection.Status != ProjectionRejected {
		t.Fatalf("review projection read failed: %#v err=%v", projection, err)
	}
}

func openModule6MySQL(t *testing.T) *gorm.DB {
	t.Helper()
	_ = godotenv.Load("../../../.env")
	dsn := os.Getenv("MEDIA_MODULE6_MYSQL_DSN")
	if dsn == "" {
		dsn = os.Getenv("MYSQL_DSN")
	}
	if dsn == "" {
		t.Skip("set MEDIA_MODULE6_MYSQL_DSN or MYSQL_DSN to run Module 6 MySQL 8.4 acceptance")
	}
	db, err := gorm.Open(mysql.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	for _, table := range []any{&Asset{}, &AssetReference{}, &ReviewProjection{}} {
		if !db.Migrator().HasTable(table) {
			t.Fatalf("required frozen media table missing: %T", table)
		}
	}
	return db
}

func createGatewayFixture(t *testing.T, db *gorm.DB, ownerID uint64, referenceID string, slot uint16, projectionStatus ProjectionStatus, referenceStatus ReferenceStatus, visibility ParentVisibility, fence string, now time.Time) gatewayFixture {
	t.Helper()
	publicID, _ := NewULID()
	projectionID, _ := NewULID()
	operationID, _ := NewULID()
	finalKey := fmt.Sprintf("final/acceptance/%d/%s.webp", ownerID, publicID)
	finalVersion := "module6-fixed-" + publicID
	hash := module5HashA
	asset := Asset{
		AssetPublicID: publicID, OwnerID: ownerID, Purpose: PurposePost,
		StorageStage: StorageStageFinal, ReviewStatus: ReviewPending, TechnicalStatus: TechnicalFinalReady,
		TrustedSHA256: &hash, VerifiedMIME: "image/webp", ByteSize: 2048, Width: 1280, Height: 720,
		FinalKey: &finalKey, FinalVersionID: &finalVersion, ProjectedDecision: ReviewPending,
		IdempotencyKey: "module6-" + publicID, RowVersion: 1, CreatedAt: now, UpdatedAt: now,
	}
	if err := db.Create(&asset).Error; err != nil {
		t.Fatal(err)
	}
	projection := ReviewProjection{
		ProjectionID: projectionID, AssetID: asset.ID, Status: projectionStatus,
		AppliedDecisionVersion: 1, LastDecisionHash: &hash, RowVersion: 1,
		CreatedAt: now, UpdatedAt: now,
	}
	if projectionStatus.Terminal() {
		projection.DecidedAt = &now
	}
	if err := db.Create(&projection).Error; err != nil {
		t.Fatal(err)
	}
	reference := AssetReference{
		AssetID: asset.ID, ReferenceType: PurposePost, ReferenceID: referenceID, Slot: slot,
		Status: referenceStatus, ParentVisibility: visibility, AggregateVersion: 1,
		OperationID: operationID, RowVersion: 1, CreatedAt: now, UpdatedAt: now,
	}
	if fence != "" {
		reference.RevocationFence = &fence
	}
	if err := db.Create(&reference).Error; err != nil {
		t.Fatal(err)
	}
	return gatewayFixture{Asset: asset, Projection: projection, Reference: reference}
}

func cleanupGatewayOwner(t *testing.T, db *gorm.DB, ownerID uint64) {
	t.Helper()
	var assetIDs []uint64
	if err := db.Model(&Asset{}).Where("owner_id = ?", ownerID).Pluck("id", &assetIDs).Error; err != nil {
		t.Errorf("find Module 6 assets: %v", err)
		return
	}
	if len(assetIDs) == 0 {
		return
	}
	var projectionIDs []string
	if err := db.Model(&ReviewProjection{}).Where("asset_id IN ?", assetIDs).Pluck("projection_id", &projectionIDs).Error; err != nil {
		t.Errorf("find Module 6 projections: %v", err)
	}
	if len(projectionIDs) > 0 {
		if err := db.Where("projection_id IN ?", projectionIDs).Delete(&ReviewDecision{}).Error; err != nil {
			t.Errorf("cleanup Module 6 decisions: %v", err)
		}
	}
	if err := db.Where("asset_id IN ?", assetIDs).Delete(&AssetReference{}).Error; err != nil {
		t.Errorf("cleanup Module 6 references: %v", err)
	}
	if err := db.Where("asset_id IN ?", assetIDs).Delete(&ReviewProjection{}).Error; err != nil {
		t.Errorf("cleanup Module 6 projections: %v", err)
	}
	if err := db.Where("id IN ?", assetIDs).Delete(&Asset{}).Error; err != nil {
		t.Errorf("cleanup Module 6 assets: %v", err)
	}
}

func TestZZMediaGatewayDatabaseResidueAgainstMySQL84(t *testing.T) {
	db := openModule6MySQL(t)
	owners := []uint64{99_000_601, 99_000_602}
	var assetCount int64
	if err := db.Model(&Asset{}).Where("owner_id IN ?", owners).Count(&assetCount).Error; err != nil {
		t.Fatal(err)
	}
	assetSubquery := db.Model(&Asset{}).Select("id").Where("owner_id IN ?", owners)
	var projectionCount, referenceCount, decisionCount int64
	if err := db.Model(&ReviewProjection{}).Where("asset_id IN (?)", assetSubquery).Count(&projectionCount).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Model(&AssetReference{}).Where("asset_id IN (?)", assetSubquery).Count(&referenceCount).Error; err != nil {
		t.Fatal(err)
	}
	projectionSubquery := db.Model(&ReviewProjection{}).Select("projection_id").Where("asset_id IN (?)", assetSubquery)
	if err := db.Model(&ReviewDecision{}).Where("projection_id IN (?)", projectionSubquery).Count(&decisionCount).Error; err != nil {
		t.Fatal(err)
	}
	if assetCount != 0 || projectionCount != 0 || referenceCount != 0 || decisionCount != 0 {
		t.Fatalf("Module 6 residue assets=%d projections=%d references=%d decisions=%d", assetCount, projectionCount, referenceCount, decisionCount)
	}
}
