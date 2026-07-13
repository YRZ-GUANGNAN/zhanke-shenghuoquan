package im

import (
	"context"
	"fmt"
	"os"
	"sync"
	"testing"
	"time"

	"gorm.io/driver/mysql"
	"gorm.io/gorm"
)

func TestStableProductConversationAgainstMySQL(t *testing.T) {
	dsn := os.Getenv("TEST_MYSQL_DSN")
	if dsn == "" {
		t.Skip("TEST_MYSQL_DSN is not configured")
	}
	db, err := gorm.Open(mysql.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if !db.Migrator().HasTable(&UserMapping{}) || !db.Migrator().HasTable(&Conversation{}) {
		t.Fatal("real MySQL schema is missing im_user_mappings or im_conversations")
	}

	seed := uint(time.Now().UnixNano()%300_000_000 + 1_500_000_000)
	buyerID, sellerID, productID := seed, seed+1, seed+2
	cleanup := func() {
		_ = db.Where("buyer_id = ? AND seller_id = ? AND reference_type = ? AND reference_id IN ?", buyerID, sellerID, "product", []uint{productID, productID + 1}).Delete(&Conversation{}).Error
		_ = db.Where("user_id IN ?", []uint{buyerID, sellerID}).Delete(&UserMapping{}).Error
	}
	cleanup()
	t.Cleanup(cleanup)

	repo := NewRepository(db)
	importer := &importerFake{}
	service := NewService(repo, importer, "1400000000", "secret")
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	first, err := service.EnsureProductConversationStable(ctx, productID, buyerID, sellerID)
	if err != nil {
		t.Fatal(err)
	}
	var persistedFirst Conversation
	if err := db.Where("buyer_id = ? AND seller_id = ? AND reference_type = ? AND reference_id = ?", buyerID, sellerID, "product", productID).First(&persistedFirst).Error; err != nil {
		t.Fatal(err)
	}
	time.Sleep(10 * time.Millisecond)
	second, err := service.EnsureProductConversationStable(ctx, productID, buyerID, sellerID)
	if err != nil {
		t.Fatal(err)
	}
	var persistedSecond Conversation
	if err := db.Where("buyer_id = ? AND seller_id = ? AND reference_type = ? AND reference_id = ?", buyerID, sellerID, "product", productID).First(&persistedSecond).Error; err != nil {
		t.Fatal(err)
	}
	if first != second || persistedFirst.ID != persistedSecond.ID || !persistedFirst.CreatedAt.Equal(persistedSecond.CreatedAt) || !persistedFirst.UpdatedAt.Equal(persistedSecond.UpdatedAt) {
		t.Fatalf("stable replay mutated canonical row: first=%+v second=%+v dbFirst=%+v dbSecond=%+v", first, second, persistedFirst, persistedSecond)
	}
	if len(importer.users) != 2 {
		t.Fatalf("stable replay imported accounts again: %v", importer.users)
	}
	if _, found, err := service.FindProductConversation(ctx, productID+99, buyerID, sellerID); err != nil || found {
		t.Fatalf("missing read-only lookup: found=%v err=%v", found, err)
	}

	values := make([]Conversation, 2)
	for i := range values {
		values[i] = Conversation{
			ConversationID: fmt.Sprintf("C2Ccampus_%d", sellerID), BuyerID: buyerID, SellerID: sellerID,
			ReferenceType: "product", ReferenceID: productID + 1,
		}
	}
	errs := make(chan error, len(values))
	var wg sync.WaitGroup
	for i := range values {
		wg.Add(1)
		go func(index int) {
			defer wg.Done()
			errs <- repo.EnsureConversationStable(ctx, &values[index])
		}(i)
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		if err != nil {
			t.Fatal(err)
		}
	}
	if values[0].ID == 0 || values[0].ID != values[1].ID {
		t.Fatalf("concurrent stable ensure returned different rows: %+v", values)
	}
	var count int64
	if err := db.Model(&Conversation{}).Where("buyer_id = ? AND seller_id = ? AND reference_type = ? AND reference_id = ?", buyerID, sellerID, "product", productID+1).Count(&count).Error; err != nil {
		t.Fatal(err)
	}
	if count != 1 {
		t.Fatalf("concurrent stable ensure persisted %d rows", count)
	}
}
