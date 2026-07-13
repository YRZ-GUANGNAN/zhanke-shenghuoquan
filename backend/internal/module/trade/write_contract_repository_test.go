package trade

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"os"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/go-sql-driver/mysql"
	gormmysql "gorm.io/driver/mysql"
	"gorm.io/gorm"

	"campus-circle/backend/internal/module/admin/review"
	"campus-circle/backend/internal/module/user"
)

func TestWriteContractHelpers(t *testing.T) {
	key, err := newWriteContractULID(time.Date(2026, 7, 13, 0, 0, 0, 0, time.UTC), zeroReader{})
	if err != nil {
		t.Fatal(err)
	}
	if !validWriteContractULID(key) || len(key) != 26 {
		t.Fatalf("generated invalid ULID %q", key)
	}
	hash := sha256.Sum256([]byte("product"))
	if !validCreateCoordinates(&key, hash[:]) {
		t.Fatal("valid create coordinates rejected")
	}
	if validCreateCoordinates(nil, hash[:]) || validCreateCoordinates(&key, hash[:31]) {
		t.Fatal("invalid create coordinates accepted")
	}
	if !isWriteContractDuplicate(&mysql.MySQLError{Number: 1062}) {
		t.Fatal("duplicate key was not recognized")
	}
	if isWriteContractDuplicate(errors.New("other")) {
		t.Fatal("non-duplicate error was misclassified")
	}
	if (WriteOutbox{}).TableName() != "trade_write_outbox" {
		t.Fatal("unexpected outbox table name")
	}
}

func TestTradeCreateOrReplayMySQL(t *testing.T) {
	db := openTradeWriteContractMySQL(t)
	repo := NewRepository(db)
	ctx := context.Background()
	owner := user.User{
		OpenID: fmt.Sprintf("ui21b2a-trade-%d", time.Now().UnixNano()), Nickname: "UI21b2a trade fixture",
		Role: user.RoleUser, Status: user.StatusActive, TokenVersion: 1,
	}
	if err := db.Create(&owner).Error; err != nil {
		t.Fatalf("create owner fixture: %v", err)
	}
	ownerID := owner.ID
	defer func() {
		cleanupTradeWriteContractRows(t, db, ownerID)
		if err := db.Unscoped().Delete(&owner).Error; err != nil {
			t.Errorf("delete owner fixture: %v", err)
		}
	}()

	key, _ := newWriteContractULID(time.Now().UTC(), zeroReader{})
	hash := sha256.Sum256([]byte("same-product"))
	const workers = 12
	var firsts atomic.Int32
	ids := make(chan uint, workers)
	errs := make(chan error, workers)
	var wait sync.WaitGroup
	for index := 0; index < workers; index++ {
		wait.Add(1)
		go func() {
			defer wait.Done()
			value := Product{
				SellerID: ownerID, ClientRequestID: &key, CreateRequestHash: append([]byte(nil), hash[:]...),
				Title: "keyed product", Description: "description", Category: "digital",
				Price: 1500, OriginalPrice: 2000, Condition: "used", Campus: "main",
				Status: StatusSelling, ReviewStatus: review.StatusSubmitted,
				Images: []ProductImage{{URL: "https://example.invalid/product.jpg"}},
			}
			event := WriteOutbox{EventType: "trade_product_created", Payload: `{}`}
			replayed, err := repo.CreateProductOrReplay(ctx, &value, &event)
			if err == nil {
				ids <- value.ID
				if !replayed {
					firsts.Add(1)
				}
			}
			errs <- err
		}()
	}
	wait.Wait()
	close(ids)
	close(errs)
	for err := range errs {
		if err != nil {
			t.Fatalf("concurrent create failed: %v", err)
		}
	}
	if firsts.Load() != 1 {
		t.Fatalf("expected one first create, got %d", firsts.Load())
	}
	var productID uint
	for id := range ids {
		if productID == 0 {
			productID = id
		} else if id != productID {
			t.Fatalf("replay returned different product IDs: %d and %d", productID, id)
		}
	}
	var productCount, outboxCount, imageCount, searchCount int64
	if err := db.Model(&Product{}).Where("seller_id = ? AND client_request_id = ?", ownerID, key).Count(&productCount).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Model(&WriteOutbox{}).Where("owner_user_id = ?", ownerID).Count(&outboxCount).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Model(&ProductImage{}).Where("product_id = ?", productID).Count(&imageCount).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Model(&SearchOutbox{}).Where("product_id = ?", productID).Count(&searchCount).Error; err != nil {
		t.Fatal(err)
	}
	if productCount != 1 || outboxCount != 1 || imageCount != 1 || searchCount != 1 {
		t.Fatalf("unexpected atomic create counts product=%d outbox=%d images=%d search=%d", productCount, outboxCount, imageCount, searchCount)
	}
	foundProduct, found, err := repo.FindProductByRequest(ctx, ownerID, key)
	if err != nil || !found || foundProduct.ID != productID || len(foundProduct.Images) != 1 {
		t.Fatalf("preflight lookup failed: found=%v product=%+v err=%v", found, foundProduct, err)
	}

	conflictHash := sha256.Sum256([]byte("different-product"))
	conflict := Product{SellerID: ownerID, ClientRequestID: &key, CreateRequestHash: conflictHash[:], Title: "other"}
	if _, err := repo.CreateProductOrReplay(ctx, &conflict, &WriteOutbox{EventType: "trade_product_created", Payload: `{}`}); !errors.Is(err, ErrIdempotencyConflict) {
		t.Fatalf("expected idempotency conflict, got %v", err)
	}
}

func openTradeWriteContractMySQL(t *testing.T) *gorm.DB {
	t.Helper()
	dsn := os.Getenv("TEST_MYSQL_DSN")
	if dsn == "" {
		t.Skip("TEST_MYSQL_DSN is not set")
	}
	db, err := gorm.Open(gormmysql.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("open MySQL: %v", err)
	}
	return db
}

func cleanupTradeWriteContractRows(t *testing.T, db *gorm.DB, ownerID uint) {
	t.Helper()
	var productIDs []uint
	if err := db.Unscoped().Model(&Product{}).Where("seller_id = ?", ownerID).Pluck("id", &productIDs).Error; err != nil {
		t.Errorf("load cleanup product IDs: %v", err)
		return
	}
	if len(productIDs) > 0 {
		db.Where("product_id IN ?", productIDs).Delete(&ProductImage{})
		db.Where("product_id IN ?", productIDs).Delete(&SearchOutbox{})
	}
	db.Where("owner_user_id = ?", ownerID).Delete(&WriteOutbox{})
	db.Unscoped().Where("seller_id = ?", ownerID).Delete(&Product{})
}

type zeroReader struct{}

func (zeroReader) Read(value []byte) (int, error) {
	clear(value)
	return len(value), nil
}
