package trade

import (
	"context"
	"errors"
	"fmt"
	"os"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"campus-circle/backend/internal/module/admin/review"
	meilisearch "github.com/meilisearch/meilisearch-go"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
)

func TestMeilisearchTitleDescriptionAndDeleteAgainstConfiguredInstance(t *testing.T) {
	host, key := os.Getenv("TEST_MEILI_HOST"), os.Getenv("TEST_MEILI_MASTER_KEY")
	if host == "" || key == "" {
		t.Skip("real Meilisearch test instance is not configured")
	}
	name := fmt.Sprintf("p03_products_%d", time.Now().UnixNano())
	search, err := NewMeiliSearch(host, key, name)
	if err != nil {
		t.Fatal(err)
	}
	defer search.Close()
	defer func() {
		task, deleteErr := search.client.DeleteIndex(name)
		if deleteErr == nil {
			_, _ = search.client.WaitForTaskWithContext(context.Background(), task.TaskUID, 25*time.Millisecond)
		}
	}()
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if err := search.Configure(ctx); err != nil {
		t.Fatal(err)
	}
	values := []Product{{ID: 101, Title: "高等数学教材", Description: "几乎全新", Status: StatusSelling, ReviewStatus: review.StatusApproved, CreatedAt: time.Now(), UpdatedAt: time.Now()}, {ID: 102, Title: "桌面收纳", Description: "适合宿舍书桌", Status: StatusSold, ReviewStatus: review.StatusApproved, CreatedAt: time.Now(), UpdatedAt: time.Now()}}
	for _, value := range values {
		if err := search.Upsert(ctx, value); err != nil {
			t.Fatal(err)
		}
	}
	ids, _, err := search.Search(ctx, "数学", ListQuery{Status: StatusSelling, Page: 1, PageSize: 20})
	if err != nil || len(ids) != 1 || ids[0] != 101 {
		raw, rawErr := search.index.SearchWithContext(ctx, "数学", &meilisearch.SearchRequest{Limit: 20})
		t.Fatalf("title search failed ids=%v err=%v raw=%+v rawErr=%v", ids, err, raw, rawErr)
	}
	ids, _, err = search.Search(ctx, "宿舍", ListQuery{Status: StatusSold, Page: 1, PageSize: 20})
	if err != nil || len(ids) != 1 || ids[0] != 102 {
		t.Fatalf("description search failed ids=%v err=%v", ids, err)
	}
	if err := search.Delete(ctx, 101); err != nil {
		t.Fatal(err)
	}
	ids, _, err = search.Search(ctx, "数学", ListQuery{Status: StatusSelling, Page: 1, PageSize: 20})
	if err != nil || len(ids) != 0 {
		t.Fatalf("delete sync failed ids=%v err=%v", ids, err)
	}
}

func TestMySQLFavoriteAndStatusConcurrency(t *testing.T) {
	dsn := os.Getenv("TEST_MYSQL_DSN")
	if dsn == "" {
		t.Skip("dedicated MySQL integration DSN is not configured")
	}
	db, err := gorm.Open(mysql.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&Product{}, &ProductImage{}, &ProductFavorite{}, &SearchOutbox{}); err != nil {
		t.Fatal(err)
	}
	stamp := time.Now().UnixNano()
	seller := integrationUser{OpenID: fmt.Sprintf("p03_seller_%d", stamp), Nickname: "p03 seller", Role: "user", Status: "active", TokenVersion: 1}
	buyer := integrationUser{OpenID: fmt.Sprintf("p03_buyer_%d", stamp), Nickname: "p03 buyer", Role: "user", Status: "active", TokenVersion: 1}
	if err := db.Create(&seller).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&buyer).Error; err != nil {
		db.Unscoped().Delete(&integrationUser{}, seller.ID)
		t.Fatal(err)
	}
	value := Product{SellerID: seller.ID, Title: "p03 concurrency", Price: 100, Status: StatusSelling, ReviewStatus: review.StatusApproved}
	if err := db.Create(&value).Error; err != nil {
		t.Fatal(err)
	}
	defer func() {
		db.Where("product_id = ?", value.ID).Delete(&ProductFavorite{})
		db.Where("product_id = ?", value.ID).Delete(&SearchOutbox{})
		db.Unscoped().Delete(&Product{}, value.ID)
		db.Unscoped().Delete(&integrationUser{}, []uint{seller.ID, buyer.ID})
	}()
	repo := NewRepository(db)
	var wg sync.WaitGroup
	errCh := make(chan error, 10)
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, toggleErr := repo.ToggleFavorite(context.Background(), value.ID, buyer.ID, true)
			errCh <- toggleErr
		}()
	}
	wg.Wait()
	close(errCh)
	for toggleErr := range errCh {
		if toggleErr != nil {
			t.Fatal(toggleErr)
		}
	}
	var favoriteRows int64
	db.Model(&ProductFavorite{}).Where("product_id = ?", value.ID).Count(&favoriteRows)
	var stored Product
	if err := db.First(&stored, value.ID).Error; err != nil {
		t.Fatal(err)
	}
	if favoriteRows != 0 || stored.FavoriteCount != 0 {
		t.Fatalf("favorite drift rows=%d count=%d", favoriteRows, stored.FavoriteCount)
	}
	var successes atomic.Int32
	statusErrors := make(chan error, 2)
	for _, next := range []ProductStatus{StatusReserved, StatusSold} {
		wg.Add(1)
		go func(target ProductStatus) {
			defer wg.Done()
			statusErrors <- repo.SetStatus(context.Background(), value.ID, value.SellerID, StatusSelling, target)
		}(next)
	}
	wg.Wait()
	close(statusErrors)
	for statusErr := range statusErrors {
		if statusErr == nil {
			successes.Add(1)
		} else if !errors.Is(statusErr, ErrConflict) {
			t.Fatal(statusErr)
		}
	}
	if successes.Load() != 1 {
		t.Fatalf("CAS allowed %d concurrent transitions", successes.Load())
	}
}

type integrationUser struct {
	ID           uint
	OpenID       string
	Nickname     string
	Role         string
	Status       string
	TokenVersion uint
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

func (integrationUser) TableName() string { return "users" }
