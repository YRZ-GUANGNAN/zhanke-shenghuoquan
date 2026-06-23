package media

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestPublishReferenceAgainstMySQL84(t *testing.T) {
	db := openModule6MySQL(t)
	ownerID := uint64(99_000_801)
	cleanupGatewayOwner(t, db, ownerID)
	t.Cleanup(func() { cleanupGatewayOwner(t, db, ownerID) })
	now := time.Date(2026, 6, 23, 14, 0, 0, 0, time.UTC)
	fixture := createGatewayFixture(t, db, ownerID, "publish-reference-parent", 0, ProjectionApproved, ReferencePending, ParentHidden, "", now)

	repository := NewRepository(db)
	service := NewService(repository)
	gateway, err := NewMediaGateway(repository)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := gateway.BuildDisplayView(context.Background(), PurposePost, fixture.Reference.ReferenceID, fixture.Reference.Slot); !errors.Is(err, ErrMediaNotVisible) {
		t.Fatalf("pending/hidden reference should be invisible before publish, got %v", err)
	}
	if err := service.PublishReference(context.Background(), fixture.Asset.AssetPublicID, PurposePost, fixture.Reference.ReferenceID, fixture.Reference.Slot); err != nil {
		t.Fatal(err)
	}
	published, err := repository.FindReference(context.Background(), fixture.Reference.ID)
	if err != nil {
		t.Fatal(err)
	}
	if published.Status != ReferenceActive || published.ParentVisibility != ParentVisible || published.RowVersion != 3 {
		t.Fatalf("published reference=%#v", published)
	}
	view, err := gateway.BuildDisplayView(context.Background(), PurposePost, fixture.Reference.ReferenceID, fixture.Reference.Slot)
	if err != nil || view.Media.MediaID != fixture.Asset.AssetPublicID {
		t.Fatalf("visible view=%#v err=%v", view, err)
	}
	if err := service.PublishReference(context.Background(), fixture.Asset.AssetPublicID, PurposePost, fixture.Reference.ReferenceID, fixture.Reference.Slot); err != nil {
		t.Fatalf("idempotent publish failed: %v", err)
	}
	afterIDempotent, err := repository.FindReference(context.Background(), fixture.Reference.ID)
	if err != nil {
		t.Fatal(err)
	}
	if afterIDempotent.RowVersion != published.RowVersion || afterIDempotent.Status != ReferenceActive || afterIDempotent.ParentVisibility != ParentVisible {
		t.Fatalf("idempotent publish mutated reference before=%#v after=%#v", published, afterIDempotent)
	}
	t.Log("pending_hidden=NOT_VISIBLE publish_once=GATEWAY_VISIBLE row_version=3 idempotent=PASS")
}

func TestPublishReferenceCompletesActiveHiddenAgainstMySQL84(t *testing.T) {
	db := openModule6MySQL(t)
	ownerID := uint64(99_000_802)
	cleanupGatewayOwner(t, db, ownerID)
	t.Cleanup(func() { cleanupGatewayOwner(t, db, ownerID) })
	now := time.Date(2026, 6, 23, 14, 30, 0, 0, time.UTC)
	fixture := createGatewayFixture(t, db, ownerID, "publish-reference-active-hidden", 0, ProjectionApproved, ReferenceActive, ParentHidden, "", now)

	repository := NewRepository(db)
	service := NewService(repository)
	gateway, err := NewMediaGateway(repository)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := gateway.BuildDisplayView(context.Background(), PurposePost, fixture.Reference.ReferenceID, fixture.Reference.Slot); !errors.Is(err, ErrMediaNotVisible) {
		t.Fatalf("active/hidden reference should be invisible before publish, got %v", err)
	}
	if err := service.PublishReference(context.Background(), fixture.Asset.AssetPublicID, PurposePost, fixture.Reference.ReferenceID, fixture.Reference.Slot); err != nil {
		t.Fatal(err)
	}
	published, err := repository.FindReference(context.Background(), fixture.Reference.ID)
	if err != nil {
		t.Fatal(err)
	}
	if published.Status != ReferenceActive || published.ParentVisibility != ParentVisible || published.RowVersion != 2 {
		t.Fatalf("active/hidden publish reference=%#v", published)
	}
	if _, err := gateway.BuildDisplayView(context.Background(), PurposePost, fixture.Reference.ReferenceID, fixture.Reference.Slot); err != nil {
		t.Fatalf("active/visible gateway err=%v", err)
	}
	t.Log("active_hidden=NOT_VISIBLE publish_completes_visibility row_version=2")
}

func TestPublishReferenceValidatesAssetCoordinatesAgainstMySQL84(t *testing.T) {
	db := openModule6MySQL(t)
	ownerID := uint64(99_000_803)
	cleanupGatewayOwner(t, db, ownerID)
	t.Cleanup(func() { cleanupGatewayOwner(t, db, ownerID) })
	now := time.Date(2026, 6, 23, 15, 0, 0, 0, time.UTC)
	target := createGatewayFixture(t, db, ownerID, "publish-reference-target", 0, ProjectionApproved, ReferencePending, ParentHidden, "", now)
	wrongAsset := createGatewayFixture(t, db, ownerID, "publish-reference-wrong-asset", 0, ProjectionApproved, ReferencePending, ParentHidden, "", now)

	repository := NewRepository(db)
	service := NewService(repository)
	if err := service.PublishReference(context.Background(), wrongAsset.Asset.AssetPublicID, PurposePost, target.Reference.ReferenceID, target.Reference.Slot); !errors.Is(err, ErrNotFound) {
		t.Fatalf("wrong asset coordinate err=%v want ErrNotFound", err)
	}
	missingAssetPublicID, err := NewULID()
	if err != nil {
		t.Fatal(err)
	}
	if err := service.PublishReference(context.Background(), missingAssetPublicID, PurposePost, target.Reference.ReferenceID, target.Reference.Slot); !errors.Is(err, ErrNotFound) {
		t.Fatalf("missing asset err=%v want ErrNotFound", err)
	}
	if err := service.PublishReference(context.Background(), target.Asset.AssetPublicID, PurposePost, "publish-reference-missing", 0); !errors.Is(err, ErrNotFound) {
		t.Fatalf("missing reference err=%v want ErrNotFound", err)
	}
	unchanged, err := repository.FindReference(context.Background(), target.Reference.ID)
	if err != nil {
		t.Fatal(err)
	}
	if unchanged.Status != ReferencePending || unchanged.ParentVisibility != ParentHidden || unchanged.RowVersion != 1 {
		t.Fatalf("failed coordinate validation mutated target=%#v", unchanged)
	}
	t.Log("wrong_asset=ErrNotFound missing_asset=ErrNotFound missing_reference=ErrNotFound no_mutation=PASS")
}

func TestZZPublishReferenceResidueAgainstMySQL84(t *testing.T) {
	db := openModule6MySQL(t)
	owners := []uint64{99_000_801, 99_000_802, 99_000_803}
	for _, ownerID := range owners {
		cleanupGatewayOwner(t, db, ownerID)
	}
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
		t.Fatalf("PublishReference residue assets=%d projections=%d references=%d decisions=%d", assetCount, projectionCount, referenceCount, decisionCount)
	}
	t.Log("publish_reference_residue=0")
}
