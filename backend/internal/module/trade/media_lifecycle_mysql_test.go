package trade

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/joho/godotenv"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"

	"campus-circle/backend/internal/module/admin/review"
	coremedia "campus-circle/backend/internal/module/media"
	"campus-circle/backend/internal/module/user"
	legacymedia "campus-circle/backend/internal/pkg/media"
)

type tradeMediaLifecycleAsset struct {
	asset      coremedia.Asset
	projection coremedia.ReviewProjection
}

type tradeMediaLifecycleReviews struct{}

func (tradeMediaLifecycleReviews) Submit(_ context.Context, input review.Submission) (review.Task, error) {
	return review.Task{ObjectType: input.ObjectType, ObjectID: input.ObjectID, SubmitterID: input.SubmitterID}, nil
}

func (tradeMediaLifecycleReviews) EnsureSubmitted(_ context.Context, input review.Submission) (review.Task, error) {
	return review.Task{ObjectType: input.ObjectType, ObjectID: input.ObjectID, SubmitterID: input.SubmitterID}, nil
}

type tradeMediaLifecycleLegacyGateway struct{}

func (tradeMediaLifecycleLegacyGateway) Reserve(context.Context, uint, legacymedia.Purpose, []string) (legacymedia.Reservation, error) {
	return legacymedia.Reservation{}, legacymedia.ErrNotFound
}

func (tradeMediaLifecycleLegacyGateway) Commit(context.Context, uint, legacymedia.Reservation, string, uint64) error {
	return nil
}

func (tradeMediaLifecycleLegacyGateway) EnsureCommitted(context.Context, uint, string, string, uint64, []string) error {
	return nil
}

func (tradeMediaLifecycleLegacyGateway) Cancel(context.Context, uint, legacymedia.Reservation) error {
	return nil
}

func (tradeMediaLifecycleLegacyGateway) DeleteReference(context.Context, string, uint64, []uint64) error {
	return nil
}

func TestTradeMediaLifecycleAgainstMySQL84(t *testing.T) {
	db := openTradeMediaLifecycleMySQL(t)
	ctx := context.Background()
	seller := user.User{
		OpenID:   fmt.Sprintf("ui21b2c-media-lifecycle-%d", time.Now().UnixNano()),
		Nickname: "UI-21b-2c media lifecycle", Role: user.RoleUser, Status: user.StatusActive, TokenVersion: 1,
	}
	if err := db.Create(&seller).Error; err != nil {
		t.Fatalf("create seller fixture: %v", err)
	}
	cleanup := func() { cleanupTradeMediaLifecycle(t, db, seller.ID) }
	t.Cleanup(cleanup)

	coreBinding, err := coremedia.NewReferenceBindingService(coremedia.NewReferenceBindingRepository(db), nil)
	if err != nil {
		t.Fatal(err)
	}
	mediaGateway, err := NewHybridMediaGateway(coreBinding, tradeMediaLifecycleLegacyGateway{}, "https://api.example.test")
	if err != nil {
		t.Fatal(err)
	}
	repository := NewRepository(db)
	reviews := tradeMediaLifecycleReviews{}
	service := NewService(repository, nil, reviews, nil, nil, mediaGateway)
	processor := NewWriteOutboxProcessor(repository, mediaGateway, reviews)
	displayRepository := coremedia.NewRepository(db)

	removed := createTradeMediaLifecycleAsset(t, db, uint64(seller.ID))
	kept := createTradeMediaLifecycleAsset(t, db, uint64(seller.ID))
	added := createTradeMediaLifecycleAsset(t, db, uint64(seller.ID))
	requestID, err := coremedia.NewULID()
	if err != nil {
		t.Fatal(err)
	}
	created, replayed, err := service.CreateKeyed(ctx, tradeMediaLifecycleProductInput(
		seller.ID, "keyed lifecycle", removed.asset.AssetPublicID, kept.asset.AssetPublicID,
	), "integration", requestID)
	if err != nil || replayed {
		t.Fatalf("keyed create replayed=%v product=%+v err=%v", replayed, created, err)
	}
	assertTradeMediaReferenceMissing(t, db, removed.asset.ID, created.ID)
	var createdEvent WriteOutbox
	if err := db.Where("aggregate_type = ? AND aggregate_id = ?", "product", created.ID).First(&createdEvent).Error; err != nil {
		t.Fatalf("load keyed create outbox: %v", err)
	}
	if err := processor.deliver(ctx, createdEvent); err != nil {
		t.Fatalf("deliver keyed create outbox: %v", err)
	}
	assertTradeMediaReference(t, db, removed.asset.ID, created.ID, 0, coremedia.ReferencePending, coremedia.ParentHidden)
	assertTradeMediaReference(t, db, kept.asset.ID, created.ID, 1, coremedia.ReferencePending, coremedia.ParentHidden)
	assertTradeMediaInvisible(t, displayRepository, removed.asset.AssetPublicID)

	if err := service.SetReviewStatus(ctx, created.ID, review.StatusApproved); err != nil {
		t.Fatalf("approve keyed product: %v", err)
	}
	assertTradeMediaReference(t, db, removed.asset.ID, created.ID, 0, coremedia.ReferenceActive, coremedia.ParentVisible)
	identity := assertTradeMediaVisible(t, displayRepository, removed.asset.AssetPublicID)
	if identity.FinalKey != *removed.asset.FinalKey || identity.FinalVersionID != *removed.asset.FinalVersionID {
		t.Fatalf("fixed identity=%+v asset key=%q version=%q", identity, *removed.asset.FinalKey, *removed.asset.FinalVersionID)
	}

	updated, err := service.Update(ctx, created.ID, tradeMediaLifecycleProductInput(
		seller.ID, "updated lifecycle", kept.asset.AssetPublicID, added.asset.AssetPublicID,
	), "integration-update")
	if err != nil {
		t.Fatalf("update product media: %v", err)
	}
	if len(updated.Images) != 2 || updated.Images[0].AssetID == nil || *updated.Images[0].AssetID != kept.asset.ID ||
		updated.Images[1].AssetID == nil || *updated.Images[1].AssetID != added.asset.ID {
		t.Fatalf("updated image order=%+v", updated.Images)
	}
	assertTradeMediaInvisible(t, displayRepository, removed.asset.AssetPublicID)
	assertTradeMediaVisible(t, displayRepository, kept.asset.AssetPublicID)
	assertTradeMediaInvisible(t, displayRepository, added.asset.AssetPublicID)
	assertTradeMediaReference(t, db, added.asset.ID, created.ID, 1, coremedia.ReferencePending, coremedia.ParentHidden)
	if err := service.SetReviewStatus(ctx, created.ID, review.StatusApproved); err != nil {
		t.Fatalf("approve updated product: %v", err)
	}
	assertTradeMediaVisible(t, displayRepository, kept.asset.AssetPublicID)
	assertTradeMediaVisible(t, displayRepository, added.asset.AssetPublicID)
	if err := service.SetReviewStatus(ctx, created.ID, review.StatusRejected); err != nil {
		t.Fatalf("reject updated product: %v", err)
	}
	assertTradeMediaInvisible(t, displayRepository, kept.asset.AssetPublicID)
	assertTradeMediaInvisible(t, displayRepository, added.asset.AssetPublicID)

	deleteAsset := createTradeMediaLifecycleAsset(t, db, uint64(seller.ID))
	deleteProduct := createApprovedTradeMediaProduct(t, ctx, service, seller.ID, "delete lifecycle", deleteAsset.asset.AssetPublicID)
	if err := service.Delete(ctx, deleteProduct.ID, seller.ID); err != nil {
		t.Fatalf("delete product: %v", err)
	}
	assertTradeMediaInvisible(t, displayRepository, deleteAsset.asset.AssetPublicID)

	takedownAsset := createTradeMediaLifecycleAsset(t, db, uint64(seller.ID))
	takedownProduct := createApprovedTradeMediaProduct(t, ctx, service, seller.ID, "takedown lifecycle", takedownAsset.asset.AssetPublicID)
	if err := service.AdminTakedown(ctx, takedownProduct.ID, seller.ID, "integration takedown"); err != nil {
		t.Fatalf("admin takedown product: %v", err)
	}
	assertTradeMediaInvisible(t, displayRepository, takedownAsset.asset.AssetPublicID)

	soldAsset := createTradeMediaLifecycleAsset(t, db, uint64(seller.ID))
	soldProduct := createApprovedTradeMediaProduct(t, ctx, service, seller.ID, "sold lifecycle", soldAsset.asset.AssetPublicID)
	if err := service.ChangeStatus(ctx, soldProduct.ID, seller.ID, StatusSold); err != nil {
		t.Fatalf("mark product sold: %v", err)
	}
	assertTradeMediaVisible(t, displayRepository, soldAsset.asset.AssetPublicID)

	contendedAsset := createTradeMediaLifecycleAsset(t, db, uint64(seller.ID))
	start := make(chan struct{})
	results := make(chan error, 2)
	var workers sync.WaitGroup
	for index := 0; index < 2; index++ {
		workers.Add(1)
		go func(index int) {
			defer workers.Done()
			requestID, createErr := coremedia.NewULID()
			if createErr == nil {
				<-start
				_, _, createErr = service.CreateKeyed(ctx, tradeMediaLifecycleProductInput(
					seller.ID, fmt.Sprintf("contended lifecycle %d", index), contendedAsset.asset.AssetPublicID,
				), "integration-concurrency", requestID)
			}
			results <- createErr
		}(index)
	}
	close(start)
	workers.Wait()
	close(results)
	succeeded, conflicted := 0, 0
	for result := range results {
		switch {
		case result == nil:
			succeeded++
		case errors.Is(result, ErrMediaConflict):
			conflicted++
		default:
			t.Fatalf("unexpected concurrent create result: %v", result)
		}
	}
	if succeeded != 1 || conflicted != 1 {
		t.Fatalf("concurrent create succeeded=%d conflicted=%d", succeeded, conflicted)
	}
	var contendedProducts int64
	if err := db.Unscoped().Model(&Product{}).
		Where("seller_id = ? AND title LIKE ?", seller.ID, "contended lifecycle%").Count(&contendedProducts).Error; err != nil {
		t.Fatal(err)
	}
	if contendedProducts != 1 {
		t.Fatalf("contended product rows=%d want=1", contendedProducts)
	}

	cleanup()
	assertTradeMediaLifecycleResidue(t, db, seller.ID)
}

func openTradeMediaLifecycleMySQL(t *testing.T) *gorm.DB {
	t.Helper()
	_ = godotenv.Load("../../../.env")
	dsn := os.Getenv("MYSQL_DSN")
	if dsn == "" {
		dsn = os.Getenv("TEST_MYSQL_DSN")
	}
	if dsn == "" {
		t.Skip("set MYSQL_DSN or TEST_MYSQL_DSN to run trade media lifecycle acceptance")
	}
	db, err := gorm.Open(mysql.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("open MySQL: %v", err)
	}
	for _, table := range []any{&Product{}, &ProductImage{}, &WriteOutbox{}, &coremedia.Asset{}, &coremedia.AssetReference{}, &coremedia.ReviewProjection{}, &user.User{}} {
		if !db.Migrator().HasTable(table) {
			t.Fatalf("required frozen table missing: %T", table)
		}
	}
	return db
}

func createTradeMediaLifecycleAsset(t *testing.T, db *gorm.DB, ownerID uint64) tradeMediaLifecycleAsset {
	t.Helper()
	assetPublicID, err := coremedia.NewULID()
	if err != nil {
		t.Fatal(err)
	}
	projectionID, err := coremedia.NewULID()
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC()
	finalKey := fmt.Sprintf("final/trade-lifecycle/%d/%s.webp", ownerID, assetPublicID)
	finalVersion := "fixed-" + assetPublicID
	asset := coremedia.Asset{
		AssetPublicID: assetPublicID, OwnerID: ownerID, Purpose: coremedia.PurposeProduct,
		StorageStage: coremedia.StorageStageFinal, ReviewStatus: coremedia.ReviewPending,
		TechnicalStatus: coremedia.TechnicalFinalReady, VerifiedMIME: "image/webp",
		ByteSize: 1024, Width: 800, Height: 600, FinalKey: &finalKey, FinalVersionID: &finalVersion,
		ProjectedDecision: coremedia.ReviewPending, IdempotencyKey: "trade-lifecycle-" + assetPublicID,
		RowVersion: 1, CreatedAt: now, UpdatedAt: now,
	}
	if err := db.Create(&asset).Error; err != nil {
		t.Fatalf("create media asset: %v", err)
	}
	projection := coremedia.ReviewProjection{
		ProjectionID: projectionID, AssetID: asset.ID, Status: coremedia.ProjectionApproved,
		AppliedDecisionVersion: 1, DecidedAt: &now, RowVersion: 1, CreatedAt: now, UpdatedAt: now,
	}
	if err := db.Create(&projection).Error; err != nil {
		t.Fatalf("create media review projection: %v", err)
	}
	return tradeMediaLifecycleAsset{asset: asset, projection: projection}
}

func tradeMediaLifecycleProductInput(sellerID uint, title string, assetIDs ...string) ProductInput {
	return ProductInput{
		SellerID: sellerID, Title: title, Description: "real MySQL media lifecycle",
		Category: "digital", Price: 1200, OriginalPrice: 1500, Condition: "used", Campus: "main",
		ImageAssetIDs: append([]string(nil), assetIDs...),
	}
}

func createApprovedTradeMediaProduct(t *testing.T, ctx context.Context, service *Service, sellerID uint, title, assetID string) Product {
	t.Helper()
	product, err := service.Create(ctx, tradeMediaLifecycleProductInput(sellerID, title, assetID), "integration")
	if err != nil {
		t.Fatalf("create %s: %v", title, err)
	}
	if err := service.SetReviewStatus(ctx, product.ID, review.StatusApproved); err != nil {
		t.Fatalf("approve %s: %v", title, err)
	}
	return product
}

func assertTradeMediaReferenceMissing(t *testing.T, db *gorm.DB, assetID uint64, productID uint) {
	t.Helper()
	var count int64
	if err := db.Model(&coremedia.AssetReference{}).
		Where("asset_id = ? AND reference_type = ? AND reference_id = ?", assetID, coremedia.PurposeProduct, strconv.FormatUint(uint64(productID), 10)).
		Count(&count).Error; err != nil {
		t.Fatal(err)
	}
	if count != 0 {
		t.Fatalf("asset=%d product=%d reference rows=%d want=0", assetID, productID, count)
	}
}

func assertTradeMediaReference(t *testing.T, db *gorm.DB, assetID uint64, productID uint, slot uint16, status coremedia.ReferenceStatus, visibility coremedia.ParentVisibility) {
	t.Helper()
	var reference coremedia.AssetReference
	if err := db.Where("asset_id = ? AND reference_type = ? AND reference_id = ?", assetID, coremedia.PurposeProduct, strconv.FormatUint(uint64(productID), 10)).
		First(&reference).Error; err != nil {
		t.Fatal(err)
	}
	if reference.Slot != slot || reference.Status != status || reference.ParentVisibility != visibility {
		t.Fatalf("reference=%+v want slot=%d status=%s visibility=%s", reference, slot, status, visibility)
	}
}

func assertTradeMediaVisible(t *testing.T, repository *coremedia.GormRepository, assetPublicID string) coremedia.DisplayObjectIdentity {
	t.Helper()
	identity, err := repository.FindVisibleDisplayObjectByAsset(context.Background(), assetPublicID)
	if err != nil {
		t.Fatalf("asset %s should be visible: %v", assetPublicID, err)
	}
	return identity
}

func assertTradeMediaInvisible(t *testing.T, repository *coremedia.GormRepository, assetPublicID string) {
	t.Helper()
	if _, err := repository.FindVisibleDisplayObjectByAsset(context.Background(), assetPublicID); !errors.Is(err, coremedia.ErrNotFound) {
		t.Fatalf("asset %s visibility err=%v want ErrNotFound", assetPublicID, err)
	}
}

func cleanupTradeMediaLifecycle(t *testing.T, db *gorm.DB, sellerID uint) {
	t.Helper()
	var productIDs []uint
	if err := db.Unscoped().Model(&Product{}).Where("seller_id = ?", sellerID).Pluck("id", &productIDs).Error; err != nil {
		t.Errorf("load cleanup product IDs: %v", err)
		return
	}
	if len(productIDs) > 0 {
		if err := db.Where("product_id IN ?", productIDs).Delete(&ProductFavorite{}).Error; err != nil {
			t.Errorf("delete product favorites: %v", err)
		}
		if err := db.Where("product_id IN ?", productIDs).Delete(&ProductImage{}).Error; err != nil {
			t.Errorf("delete product images: %v", err)
		}
		if err := db.Where("product_id IN ?", productIDs).Delete(&SearchOutbox{}).Error; err != nil {
			t.Errorf("delete trade search outbox: %v", err)
		}
		assertTradeMediaLifecycleRows(t, db, &ProductImage{}, "product_id IN ?", productIDs, "product images")
	}
	if err := db.Where("owner_user_id = ?", sellerID).Delete(&WriteOutbox{}).Error; err != nil {
		t.Errorf("delete trade write outbox: %v", err)
	}
	if err := db.Unscoped().Where("seller_id = ?", sellerID).Delete(&Product{}).Error; err != nil {
		t.Errorf("delete products: %v", err)
	}
	var assetIDs []uint64
	if err := db.Model(&coremedia.Asset{}).Where("owner_id = ?", sellerID).Pluck("id", &assetIDs).Error; err != nil {
		t.Errorf("load cleanup media asset IDs: %v", err)
		return
	}
	if len(assetIDs) > 0 {
		if err := db.Where("asset_id IN ?", assetIDs).Delete(&coremedia.AssetReference{}).Error; err != nil {
			t.Errorf("delete media references: %v", err)
		}
		if err := db.Where("asset_id IN ?", assetIDs).Delete(&coremedia.ReviewProjection{}).Error; err != nil {
			t.Errorf("delete media projections: %v", err)
		}
		assertTradeMediaLifecycleRows(t, db, &coremedia.AssetReference{}, "asset_id IN ?", assetIDs, "media references")
		assertTradeMediaLifecycleRows(t, db, &coremedia.ReviewProjection{}, "asset_id IN ?", assetIDs, "media projections")
	}
	if err := db.Where("owner_id = ?", sellerID).Delete(&coremedia.Asset{}).Error; err != nil {
		t.Errorf("delete media assets: %v", err)
	}
	if err := db.Unscoped().Where("id = ?", sellerID).Delete(&user.User{}).Error; err != nil {
		t.Errorf("delete seller fixture: %v", err)
	}
}

func assertTradeMediaLifecycleRows(t *testing.T, db *gorm.DB, model any, query string, arg any, name string) {
	t.Helper()
	var count int64
	if err := db.Unscoped().Model(model).Where(query, arg).Count(&count).Error; err != nil {
		t.Errorf("count cleanup %s: %v", name, err)
		return
	}
	if count != 0 {
		t.Errorf("%s residue=%d", name, count)
	}
}

func assertTradeMediaLifecycleResidue(t *testing.T, db *gorm.DB, sellerID uint) {
	t.Helper()
	checks := []struct {
		name  string
		model any
		query string
		arg   any
	}{
		{"products", &Product{}, "seller_id = ?", sellerID},
		{"write outbox", &WriteOutbox{}, "owner_user_id = ?", sellerID},
		{"assets", &coremedia.Asset{}, "owner_id = ?", sellerID},
		{"users", &user.User{}, "id = ?", sellerID},
	}
	for _, check := range checks {
		var count int64
		if err := db.Unscoped().Model(check.model).Where(check.query, check.arg).Count(&count).Error; err != nil {
			t.Fatalf("count residue %s: %v", check.name, err)
		}
		if count != 0 {
			t.Fatalf("%s residue=%d", check.name, count)
		}
	}
}
