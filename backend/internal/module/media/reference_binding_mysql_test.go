package media

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"

	"gorm.io/gorm"
)

const (
	bindingOwnerConcurrent = uint64(99_000_901)
	bindingOwnerLifecycle  = uint64(99_000_902)
	bindingOwnerClassify   = uint64(99_000_903)
	bindingOwnerTakeover   = uint64(99_000_904)
	bindingOwnerReplace    = uint64(99_000_905)
)

type productBindingFixture struct {
	Asset      Asset
	Projection ReviewProjection
}

func TestReferenceBindingOnlyOneConcurrentReservationWinsAgainstMySQL84(t *testing.T) {
	db := openModule6MySQL(t)
	cleanupGatewayOwner(t, db, bindingOwnerConcurrent)
	t.Cleanup(func() { cleanupGatewayOwner(t, db, bindingOwnerConcurrent) })
	first := createApprovedProductBindingFixture(t, db, bindingOwnerConcurrent)
	second := createApprovedProductBindingFixture(t, db, bindingOwnerConcurrent)
	service, err := NewReferenceBindingService(NewReferenceBindingRepository(db), time.Now)
	if err != nil {
		t.Fatal(err)
	}

	start := make(chan struct{})
	results := make(chan error, 2)
	inputs := []struct {
		token string
		ids   []string
	}{
		{"m10_" + testBindingULID1, []string{first.Asset.AssetPublicID, second.Asset.AssetPublicID}},
		{"m10_" + testBindingULID2, []string{second.Asset.AssetPublicID, first.Asset.AssetPublicID}},
	}
	var workers sync.WaitGroup
	for _, input := range inputs {
		workers.Add(1)
		go func(token string, ids []string) {
			defer workers.Done()
			<-start
			_, reserveErr := service.Reserve(context.Background(), BindingReserveInput{
				OwnerID: bindingOwnerConcurrent, Purpose: PurposeProduct,
				OrderedAssetPublicIDs: ids, Token: token, ExpiresAt: time.Now().Add(2 * time.Minute),
			})
			results <- reserveErr
		}(input.token, input.ids)
	}
	close(start)
	workers.Wait()
	close(results)

	succeeded, conflicted := 0, 0
	for result := range results {
		switch {
		case result == nil:
			succeeded++
		case errors.Is(result, ErrBindingConflict):
			conflicted++
		default:
			t.Fatalf("unexpected reservation result: %v", result)
		}
	}
	if succeeded != 1 || conflicted != 1 {
		t.Fatalf("succeeded=%d conflicted=%d", succeeded, conflicted)
	}
}

func TestReferenceBindingLifecycleAndExactReplayAgainstMySQL84(t *testing.T) {
	db := openModule6MySQL(t)
	cleanupGatewayOwner(t, db, bindingOwnerLifecycle)
	t.Cleanup(func() { cleanupGatewayOwner(t, db, bindingOwnerLifecycle) })
	first := createApprovedProductBindingFixture(t, db, bindingOwnerLifecycle)
	second := createApprovedProductBindingFixture(t, db, bindingOwnerLifecycle)
	service, _ := NewReferenceBindingService(NewReferenceBindingRepository(db), time.Now)
	token := "m10_" + testBindingULID1
	ordered := []string{second.Asset.AssetPublicID, first.Asset.AssetPublicID}

	reservation, err := service.Reserve(context.Background(), BindingReserveInput{
		OwnerID: bindingOwnerLifecycle, Purpose: PurposeProduct,
		OrderedAssetPublicIDs: ordered, Token: token, ExpiresAt: time.Now().Add(2 * time.Minute),
	})
	if err != nil || len(reservation.References) != 2 || reservation.References[0].InternalID != second.Asset.ID {
		t.Fatalf("reservation=%+v err=%v", reservation, err)
	}
	commit := BindingCommitInput{
		OwnerID: bindingOwnerLifecycle, Token: token, ReferenceType: PurposeProduct,
		ReferenceID: "9001", OrderedAssetPublicIDs: ordered,
	}
	if err := service.Commit(context.Background(), commit); err != nil {
		t.Fatal(err)
	}
	if err := service.EnsureCommitted(context.Background(), commit); err != nil {
		t.Fatalf("exact replay: %v", err)
	}
	mismatch := commit
	mismatch.OrderedAssetPublicIDs = []string{first.Asset.AssetPublicID, second.Asset.AssetPublicID}
	if err := service.EnsureCommitted(context.Background(), mismatch); !errors.Is(err, ErrBindingMismatch) {
		t.Fatalf("mismatched replay err=%v", err)
	}
	if err := service.Publish(context.Background(), PurposeProduct, "9001"); err != nil {
		t.Fatal(err)
	}
	assertBindingReferenceState(t, db, second.Asset.ID, "9001", 0, ReferenceActive, ParentVisible)
	assertBindingReferenceState(t, db, first.Asset.ID, "9001", 1, ReferenceActive, ParentVisible)
	assertBindingActiveRefCount(t, db, first.Asset.ID, 1)
	assertBindingActiveRefCount(t, db, second.Asset.ID, 1)

	if err := service.Revoke(context.Background(), PurposeProduct, "9001", []uint64{first.Asset.ID}); err != nil {
		t.Fatal(err)
	}
	assertBindingReferenceState(t, db, second.Asset.ID, "9001", 0, ReferenceRevoked, ParentHidden)
	assertBindingReferenceState(t, db, first.Asset.ID, "9001", 1, ReferenceActive, ParentVisible)
	assertBindingActiveRefCount(t, db, first.Asset.ID, 1)
	assertBindingActiveRefCount(t, db, second.Asset.ID, 0)
	var revoked AssetReference
	if err := db.Where("asset_id = ? AND reference_type = ? AND reference_id = ?", second.Asset.ID, PurposeProduct, "9001").First(&revoked).Error; err != nil {
		t.Fatal(err)
	}
	version := revoked.RowVersion
	if revoked.RevocationFence == nil || !ValidULID(*revoked.RevocationFence) {
		t.Fatalf("revocation fence=%v", revoked.RevocationFence)
	}
	if err := service.Revoke(context.Background(), PurposeProduct, "9001", []uint64{first.Asset.ID}); err != nil {
		t.Fatal(err)
	}
	if err := db.First(&revoked, revoked.ID).Error; err != nil || revoked.RowVersion != version {
		t.Fatalf("revoke replay rowVersion=%d want=%d err=%v", revoked.RowVersion, version, err)
	}
}

func TestReferenceBindingClassifiesLegacyAndRejectsMixedAgainstMySQL84(t *testing.T) {
	db := openModule6MySQL(t)
	cleanupGatewayOwner(t, db, bindingOwnerClassify)
	t.Cleanup(func() { cleanupGatewayOwner(t, db, bindingOwnerClassify) })
	core := createApprovedProductBindingFixture(t, db, bindingOwnerClassify)
	legacyID := createLegacyProductBindingFixture(t, db, bindingOwnerClassify)
	service, _ := NewReferenceBindingService(NewReferenceBindingRepository(db), time.Now)

	_, err := service.Reserve(context.Background(), BindingReserveInput{
		OwnerID: bindingOwnerClassify, Purpose: PurposeProduct,
		OrderedAssetPublicIDs: []string{legacyID}, Token: "m10_" + testBindingULID1,
		ExpiresAt: time.Now().Add(time.Minute),
	})
	if !errors.Is(err, ErrBindingLegacySet) {
		t.Fatalf("legacy err=%v", err)
	}
	_, err = service.Reserve(context.Background(), BindingReserveInput{
		OwnerID: bindingOwnerClassify, Purpose: PurposeProduct,
		OrderedAssetPublicIDs: []string{core.Asset.AssetPublicID, legacyID}, Token: "m10_" + testBindingULID2,
		ExpiresAt: time.Now().Add(time.Minute),
	})
	if !errors.Is(err, ErrBindingInvalidAsset) {
		t.Fatalf("mixed err=%v", err)
	}
}

func TestReferenceBindingExpiredReservationCanBeTakenOverAgainstMySQL84(t *testing.T) {
	db := openModule6MySQL(t)
	cleanupGatewayOwner(t, db, bindingOwnerTakeover)
	t.Cleanup(func() { cleanupGatewayOwner(t, db, bindingOwnerTakeover) })
	fixture := createApprovedProductBindingFixture(t, db, bindingOwnerTakeover)
	if err := db.Model(&Asset{}).Where("id = ?", fixture.Asset.ID).Updates(map[string]any{
		"reservation_token": "m10_" + testBindingULID1, "reservation_expires_at": time.Now().Add(-time.Minute),
	}).Error; err != nil {
		t.Fatal(err)
	}
	service, _ := NewReferenceBindingService(NewReferenceBindingRepository(db), time.Now)
	reservation, err := service.Reserve(context.Background(), BindingReserveInput{
		OwnerID: bindingOwnerTakeover, Purpose: PurposeProduct,
		OrderedAssetPublicIDs: []string{fixture.Asset.AssetPublicID}, Token: "m10_" + testBindingULID2,
		ExpiresAt: time.Now().Add(time.Minute),
	})
	if err != nil || reservation.Token != "m10_"+testBindingULID2 {
		t.Fatalf("reservation=%+v err=%v", reservation, err)
	}
	if err := service.Cancel(context.Background(), bindingOwnerTakeover+1, reservation.Token); err != nil {
		t.Fatal(err)
	}
	var asset Asset
	if err := db.First(&asset, fixture.Asset.ID).Error; err != nil {
		t.Fatal(err)
	}
	var token string
	if err := db.Table("media_assets").Select("reservation_token").Where("id = ?", fixture.Asset.ID).Scan(&token).Error; err != nil {
		t.Fatal(err)
	}
	if token != reservation.Token {
		t.Fatalf("unauthorized cancel cleared token=%q", token)
	}
	if err := service.Cancel(context.Background(), bindingOwnerTakeover, reservation.Token); err != nil {
		t.Fatal(err)
	}
}

func TestReferenceBindingUpdateKeepsReplacesAndReordersAgainstMySQL84(t *testing.T) {
	db := openModule6MySQL(t)
	cleanupGatewayOwner(t, db, bindingOwnerReplace)
	t.Cleanup(func() { cleanupGatewayOwner(t, db, bindingOwnerReplace) })
	removed := createApprovedProductBindingFixture(t, db, bindingOwnerReplace)
	kept := createApprovedProductBindingFixture(t, db, bindingOwnerReplace)
	added := createApprovedProductBindingFixture(t, db, bindingOwnerReplace)
	service, _ := NewReferenceBindingService(NewReferenceBindingRepository(db), time.Now)

	initialIDs := []string{removed.Asset.AssetPublicID, kept.Asset.AssetPublicID}
	initialToken := "m10_" + testBindingULID1
	if _, err := service.Reserve(context.Background(), BindingReserveInput{
		OwnerID: bindingOwnerReplace, Purpose: PurposeProduct, OrderedAssetPublicIDs: initialIDs,
		Token: initialToken, ExpiresAt: time.Now().Add(time.Minute),
	}); err != nil {
		t.Fatal(err)
	}
	if err := service.Commit(context.Background(), BindingCommitInput{
		OwnerID: bindingOwnerReplace, Token: initialToken, ReferenceType: PurposeProduct,
		ReferenceID: "9002", OrderedAssetPublicIDs: initialIDs,
	}); err != nil {
		t.Fatal(err)
	}
	if err := service.Publish(context.Background(), PurposeProduct, "9002"); err != nil {
		t.Fatal(err)
	}

	replacementIDs := []string{kept.Asset.AssetPublicID, added.Asset.AssetPublicID}
	replacementToken := "m10_" + testBindingULID2
	reservation, err := service.Reserve(context.Background(), BindingReserveInput{
		OwnerID: bindingOwnerReplace, Purpose: PurposeProduct, OrderedAssetPublicIDs: replacementIDs,
		Token: replacementToken, ExpiresAt: time.Now().Add(time.Minute), ExistingReferenceID: "9002",
	})
	if err != nil || len(reservation.References) != 2 || !reservation.References[0].Kept || reservation.References[1].Kept {
		t.Fatalf("replacement reservation=%+v err=%v", reservation, err)
	}
	if err := service.Commit(context.Background(), BindingCommitInput{
		OwnerID: bindingOwnerReplace, Token: replacementToken, ReferenceType: PurposeProduct,
		ReferenceID: "9002", OrderedAssetPublicIDs: replacementIDs,
	}); err != nil {
		t.Fatal(err)
	}
	if err := service.Revoke(context.Background(), PurposeProduct, "9002", []uint64{kept.Asset.ID, added.Asset.ID}); err != nil {
		t.Fatal(err)
	}
	assertBindingReferenceState(t, db, kept.Asset.ID, "9002", 0, ReferenceActive, ParentVisible)
	assertBindingReferenceState(t, db, added.Asset.ID, "9002", 1, ReferencePending, ParentHidden)
	var removedReference AssetReference
	if err := db.Where("asset_id = ? AND reference_type = ? AND reference_id = ?", removed.Asset.ID, PurposeProduct, "9002").First(&removedReference).Error; err != nil {
		t.Fatal(err)
	}
	if removedReference.Status != ReferenceRevoked || removedReference.ParentVisibility != ParentHidden || removedReference.Slot < maxReferenceBindingAssets {
		t.Fatalf("removed reference=%+v", removedReference)
	}
	if err := service.Publish(context.Background(), PurposeProduct, "9002"); err != nil {
		t.Fatalf("publish replacement set: %v", err)
	}
	assertBindingReferenceState(t, db, kept.Asset.ID, "9002", 0, ReferenceActive, ParentVisible)
	assertBindingReferenceState(t, db, added.Asset.ID, "9002", 1, ReferenceActive, ParentVisible)
	if err := db.First(&removedReference, removedReference.ID).Error; err != nil {
		t.Fatal(err)
	}
	if removedReference.Status != ReferenceRevoked || removedReference.ParentVisibility != ParentHidden || removedReference.RevocationFence == nil {
		t.Fatalf("publish resurrected removed reference=%+v", removedReference)
	}
}

func createApprovedProductBindingFixture(t *testing.T, db *gorm.DB, ownerID uint64) productBindingFixture {
	t.Helper()
	publicID, _ := NewULID()
	projectionID, _ := NewULID()
	now := time.Now().UTC()
	finalKey := fmt.Sprintf("final/trade-binding/%d/%s.webp", ownerID, publicID)
	finalVersion := "fixed-" + publicID
	hash := module5HashA
	asset := Asset{
		AssetPublicID: publicID, OwnerID: ownerID, Purpose: PurposeProduct,
		StorageStage: StorageStageFinal, ReviewStatus: ReviewPending, TechnicalStatus: TechnicalFinalReady,
		TrustedSHA256: &hash, VerifiedMIME: "image/webp", ByteSize: 1024, Width: 800, Height: 600,
		FinalKey: &finalKey, FinalVersionID: &finalVersion, ProjectedDecision: ReviewPending,
		IdempotencyKey: "trade-binding-" + publicID, RowVersion: 1, CreatedAt: now, UpdatedAt: now,
	}
	if err := db.Create(&asset).Error; err != nil {
		t.Fatal(err)
	}
	projection := ReviewProjection{
		ProjectionID: projectionID, AssetID: asset.ID, Status: ProjectionApproved,
		AppliedDecisionVersion: 1, LastDecisionHash: &hash, DecidedAt: &now,
		RowVersion: 1, CreatedAt: now, UpdatedAt: now,
	}
	if err := db.Create(&projection).Error; err != nil {
		t.Fatal(err)
	}
	return productBindingFixture{Asset: asset, Projection: projection}
}

func createLegacyProductBindingFixture(t *testing.T, db *gorm.DB, ownerID uint64) string {
	t.Helper()
	assetPublicID, _ := NewULID()
	legacyID, _ := randomLegacyBindingID()
	now := time.Now().UTC()
	asset := Asset{
		AssetPublicID: assetPublicID, OwnerID: ownerID, Purpose: Purpose("product_image"),
		StorageStage: StorageStageStaging, ReviewStatus: ReviewPending, TechnicalStatus: TechnicalPendingUpload,
		VerifiedMIME: "image/webp", IdempotencyKey: "trade-binding-legacy-" + assetPublicID,
		RowVersion: 1, CreatedAt: now, UpdatedAt: now,
	}
	if err := db.Create(&asset).Error; err != nil {
		t.Fatal(err)
	}
	updates := map[string]any{
		"public_id": legacyID, "owner_user_id": ownerID, "object_key": "legacy/" + legacyID + ".webp",
		"status": "approved", "upload_expires_at": now.Add(time.Hour), "review_available_at": now,
	}
	if err := db.Table("media_assets").Where("id = ?", asset.ID).Updates(updates).Error; err != nil {
		t.Fatal(err)
	}
	return legacyID
}

func randomLegacyBindingID() (string, error) {
	id, err := NewULID()
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%x", []byte(id))[:32], nil
}

func assertBindingReferenceState(t *testing.T, db *gorm.DB, assetID uint64, referenceID string, slot uint16, status ReferenceStatus, visibility ParentVisibility) {
	t.Helper()
	var reference AssetReference
	if err := db.Where("asset_id = ? AND reference_type = ? AND reference_id = ? AND slot = ?", assetID, PurposeProduct, referenceID, slot).First(&reference).Error; err != nil {
		t.Fatal(err)
	}
	if reference.Status != status || reference.ParentVisibility != visibility {
		t.Fatalf("reference=%+v want status=%s visibility=%s", reference, status, visibility)
	}
}

func assertBindingActiveRefCount(t *testing.T, db *gorm.DB, assetID, expected uint64) {
	t.Helper()
	var asset Asset
	if err := db.First(&asset, assetID).Error; err != nil {
		t.Fatal(err)
	}
	if asset.ActiveRefCount != expected {
		t.Fatalf("asset=%d activeRefCount=%d want=%d", assetID, asset.ActiveRefCount, expected)
	}
}

func TestZZReferenceBindingDatabaseResidueAgainstMySQL84(t *testing.T) {
	db := openModule6MySQL(t)
	owners := []uint64{bindingOwnerConcurrent, bindingOwnerLifecycle, bindingOwnerClassify, bindingOwnerTakeover, bindingOwnerReplace}
	for _, ownerID := range owners {
		cleanupGatewayOwner(t, db, ownerID)
	}
	var count int64
	if err := db.Model(&Asset{}).Where("owner_id IN ?", owners).Count(&count).Error; err != nil {
		t.Fatal(err)
	}
	if count != 0 {
		t.Fatalf("reference binding residue assets=%d", count)
	}
}
