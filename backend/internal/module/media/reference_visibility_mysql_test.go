package media

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestReferenceVisibilityPromotionAgainstMySQL84(t *testing.T) {
	db := openModule6MySQL(t)
	ownerID := uint64(99_000_701)
	cleanupGatewayOwner(t, db, ownerID)
	t.Cleanup(func() { cleanupGatewayOwner(t, db, ownerID) })
	now := time.Date(2026, 6, 23, 10, 0, 0, 0, time.UTC)
	fixture := createGatewayFixture(t, db, ownerID, "visibility-promotion-parent", 0, ProjectionApproved, ReferencePending, ParentHidden, "", now)

	repository := NewRepository(db)
	service := NewService(repository)
	gateway, err := NewMediaGateway(repository)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := gateway.BuildDisplayView(context.Background(), PurposePost, fixture.Reference.ReferenceID, fixture.Reference.Slot); !errors.Is(err, ErrMediaNotVisible) {
		t.Fatalf("pending/hidden reference should be invisible, got %v", err)
	}
	if err := service.TransitionReference(context.Background(), fixture.Reference.ID, fixture.Reference.RowVersion, ReferencePending, ReferenceActive); err != nil {
		t.Fatal(err)
	}
	reference, err := repository.FindReference(context.Background(), fixture.Reference.ID)
	if err != nil {
		t.Fatal(err)
	}
	if reference.Status != ReferenceActive || reference.ParentVisibility != ParentHidden || reference.RowVersion != 2 {
		t.Fatalf("status promotion reference=%#v", reference)
	}
	if _, err := gateway.BuildDisplayView(context.Background(), PurposePost, fixture.Reference.ReferenceID, fixture.Reference.Slot); !errors.Is(err, ErrMediaNotVisible) {
		t.Fatalf("active/hidden reference should still be invisible, got %v", err)
	}
	if err := service.TransitionReferenceVisibility(context.Background(), reference.ID, reference.RowVersion, ParentHidden, ParentVisible); err != nil {
		t.Fatal(err)
	}
	promoted, err := repository.FindReference(context.Background(), fixture.Reference.ID)
	if err != nil {
		t.Fatal(err)
	}
	if promoted.Status != ReferenceActive || promoted.ParentVisibility != ParentVisible || promoted.RowVersion != 3 {
		t.Fatalf("visibility promotion reference=%#v", promoted)
	}
	view, err := gateway.BuildDisplayView(context.Background(), PurposePost, fixture.Reference.ReferenceID, fixture.Reference.Slot)
	if err != nil || view.Media.MediaID != fixture.Asset.AssetPublicID || view.Reference.ReferenceID != fixture.Reference.ReferenceID {
		t.Fatalf("visible view=%#v err=%v", view, err)
	}
	metadata, err := gateway.GetVisibleMetadata(context.Background(), fixture.Asset.AssetPublicID)
	if err != nil || metadata.MediaID != fixture.Asset.AssetPublicID {
		t.Fatalf("visible metadata=%#v err=%v", metadata, err)
	}
	if err := service.TransitionReferenceVisibility(context.Background(), promoted.ID, 1, ParentHidden, ParentVisible); err != nil {
		t.Fatalf("idempotent visible promotion failed: %v", err)
	}
	afterIDempotent, err := repository.FindReference(context.Background(), fixture.Reference.ID)
	if err != nil {
		t.Fatal(err)
	}
	if afterIDempotent.RowVersion != promoted.RowVersion {
		t.Fatalf("idempotent promotion changed row_version before=%d after=%d", promoted.RowVersion, afterIDempotent.RowVersion)
	}
	if err := service.TransitionReferenceVisibility(context.Background(), promoted.ID, promoted.RowVersion, ParentVisible, ParentHidden); !errors.Is(err, ErrInvalidTransition) {
		t.Fatalf("visible->hidden must be fail-closed, got %v", err)
	}
	t.Log("pending_hidden=NOT_VISIBLE active_hidden=NOT_VISIBLE active_visible=GATEWAY_VISIBLE idempotent=PASS visible_to_hidden=FAIL_CLOSED")
}

func TestReferenceVisibilityCASAgainstMySQL84(t *testing.T) {
	db := openModule6MySQL(t)
	ownerID := uint64(99_000_702)
	cleanupGatewayOwner(t, db, ownerID)
	t.Cleanup(func() { cleanupGatewayOwner(t, db, ownerID) })
	now := time.Date(2026, 6, 23, 10, 30, 0, 0, time.UTC)
	fixture := createGatewayFixture(t, db, ownerID, "visibility-cas-parent", 0, ProjectionApproved, ReferencePending, ParentHidden, "", now)

	repository := NewRepository(db)
	service := NewService(repository)
	if err := service.TransitionReference(context.Background(), fixture.Reference.ID, fixture.Reference.RowVersion, ReferencePending, ReferenceActive); err != nil {
		t.Fatal(err)
	}
	if err := service.TransitionReferenceVisibility(context.Background(), fixture.Reference.ID, fixture.Reference.RowVersion, ParentHidden, ParentVisible); !errors.Is(err, ErrConcurrentUpdate) {
		t.Fatalf("stale visibility CAS returned %v", err)
	}
	reference, err := repository.FindReference(context.Background(), fixture.Reference.ID)
	if err != nil {
		t.Fatal(err)
	}
	if reference.ParentVisibility != ParentHidden || reference.RowVersion != 2 {
		t.Fatalf("stale CAS mutated reference=%#v", reference)
	}
	if err := service.TransitionReferenceVisibility(context.Background(), reference.ID, reference.RowVersion, ParentHidden, ParentVisible); err != nil {
		t.Fatal(err)
	}
	t.Log("stale_row_version=ErrConcurrentUpdate visibility_retry=PASS")
}

func TestZZReferenceVisibilityPatchResidueAgainstMySQL84(t *testing.T) {
	db := openModule6MySQL(t)
	owners := []uint64{99_000_701, 99_000_702}
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
		t.Fatalf("reference visibility residue assets=%d projections=%d references=%d decisions=%d", assetCount, projectionCount, referenceCount, decisionCount)
	}
	t.Log("reference_visibility_residue assets=0 projections=0 references=0 decisions=0")
}
