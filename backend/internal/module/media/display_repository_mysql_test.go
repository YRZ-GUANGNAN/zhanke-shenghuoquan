package media

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestMediaDisplayRepositoryReturnsOnlyVisibleFinalIdentityAgainstMySQL84(t *testing.T) {
	db := openModule6MySQL(t)
	ownerID := uint64(99_000_701)
	cleanupGatewayOwner(t, db, ownerID)
	t.Cleanup(func() { cleanupGatewayOwner(t, db, ownerID) })
	now := time.Date(2026, 6, 24, 15, 0, 0, 0, time.UTC)
	visible := createGatewayFixture(t, db, ownerID, "display-reference-visible", 0, ProjectionApproved, ReferenceActive, ParentVisible, "", now)
	hidden := createGatewayFixture(t, db, ownerID, "display-reference-hidden", 0, ProjectionApproved, ReferenceActive, ParentHidden, "", now)

	repository := NewRepository(db)
	value, err := repository.FindVisibleDisplayObjectByReference(context.Background(), PurposePost, visible.Reference.ReferenceID, visible.Reference.Slot)
	if err != nil {
		t.Fatalf("find visible identity err=%v", err)
	}
	if value.MediaID != visible.Asset.AssetPublicID || value.FinalKey == "" || value.FinalVersionID == "" {
		t.Fatalf("identity=%#v visible=%#v", value, visible.Asset)
	}
	byAsset, err := repository.FindVisibleDisplayObjectByAsset(context.Background(), visible.Asset.AssetPublicID)
	if err != nil || byAsset != value {
		t.Fatalf("asset identity=%#v reference identity=%#v err=%v", byAsset, value, err)
	}
	if _, err := repository.FindVisibleDisplayObjectByReference(context.Background(), PurposePost, hidden.Reference.ReferenceID, hidden.Reference.Slot); !errors.Is(err, ErrNotFound) {
		t.Fatalf("hidden err=%v want ErrNotFound", err)
	}
	if _, err := repository.FindVisibleDisplayObjectByAsset(context.Background(), hidden.Asset.AssetPublicID); !errors.Is(err, ErrNotFound) {
		t.Fatalf("hidden asset err=%v want ErrNotFound", err)
	}
}

func TestZZMediaDisplayDatabaseResidueAgainstMySQL84(t *testing.T) {
	db := openModule6MySQL(t)
	ownerID := uint64(99_000_701)
	var assetCount int64
	if err := db.Model(&Asset{}).Where("owner_id = ?", ownerID).Count(&assetCount).Error; err != nil {
		t.Fatal(err)
	}
	if assetCount != 0 {
		t.Fatalf("display repository residue assets=%d", assetCount)
	}
}
