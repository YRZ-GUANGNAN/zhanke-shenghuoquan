package takeout

import (
	"context"
	"testing"
	"time"
)

type catalogRepositoryStub struct {
	createdCategory *DishCategory
	createdDish     *Dish
	categoryWrites  int
	dishWrites      int
	lastMerchantID  string
	lastDishFrom    DishStatus
	lastDishTo      DishStatus
}

func (r *catalogRepositoryStub) CreateCategory(_ context.Context, value *DishCategory) error {
	copy := *value
	r.createdCategory = &copy
	value.ID = 11
	return nil
}

func (*catalogRepositoryStub) FindCategory(context.Context, string) (DishCategory, error) {
	return DishCategory{ID: 11}, nil
}

func (r *catalogRepositoryStub) RenameCategory(_ context.Context, publicID, merchantID string, version uint64, name string, at time.Time) (DishCategory, error) {
	r.categoryWrites++
	r.lastMerchantID = merchantID
	return DishCategory{ID: 11, CategoryPublicID: publicID, MerchantPublicID: merchantID, Name: name, RowVersion: version + 1, UpdatedAt: at}, nil
}

func (r *catalogRepositoryStub) ReorderCategory(_ context.Context, publicID, merchantID string, version uint64, sortOrder int32, at time.Time) (DishCategory, error) {
	r.categoryWrites++
	r.lastMerchantID = merchantID
	return DishCategory{ID: 11, CategoryPublicID: publicID, MerchantPublicID: merchantID, SortOrder: sortOrder, RowVersion: version + 1, UpdatedAt: at}, nil
}

func (r *catalogRepositoryStub) CreateDish(_ context.Context, value *Dish) error {
	copy := *value
	r.createdDish = &copy
	value.ID = 21
	return nil
}

func (*catalogRepositoryStub) FindDish(context.Context, string) (Dish, error) {
	return Dish{ID: 21}, nil
}

func (r *catalogRepositoryStub) UpdateDish(_ context.Context, publicID, merchantID string, version uint64, status DishStatus, details dishDetails, at time.Time) (Dish, error) {
	r.dishWrites++
	r.lastMerchantID = merchantID
	value := Dish{ID: 21, DishPublicID: publicID, MerchantPublicID: merchantID, Status: status, RowVersion: version + 1, UpdatedAt: at}
	applyDishDetails(&value, details)
	return value, nil
}

func (r *catalogRepositoryStub) TransitionDishStatus(_ context.Context, publicID, merchantID string, version uint64, from, to DishStatus, at time.Time) (Dish, error) {
	r.dishWrites++
	r.lastMerchantID, r.lastDishFrom, r.lastDishTo = merchantID, from, to
	return Dish{ID: 21, DishPublicID: publicID, MerchantPublicID: merchantID, Status: to, RowVersion: version + 1, UpdatedAt: at}, nil
}

func TestCatalogServiceCreatesFrozenInitialRecordsInUTC(t *testing.T) {
	repository := &catalogRepositoryStub{}
	zone := time.FixedZone("UTC+8", 8*60*60)
	now := time.Date(2026, 6, 22, 20, 0, 0, 0, zone)
	service, err := NewCatalogService(repository, func() time.Time { return now })
	if err != nil {
		t.Fatal(err)
	}
	merchantID, _ := NewULID()
	category, err := service.CreateCategory(context.Background(), CreateCategoryInput{MerchantPublicID: merchantID, Name: "  Meals  ", SortOrder: 3})
	if err != nil || category.ID != 11 || category.RowVersion != 1 || category.Name != "Meals" || category.CreatedAt.Location() != time.UTC {
		t.Fatalf("category=%#v err=%v", category, err)
	}
	imageID, _ := NewULID()
	dish, err := service.CreateDish(context.Background(), CreateDishInput{
		MerchantPublicID: merchantID, CategoryPublicID: &category.CategoryPublicID,
		Name: "  Noodles  ", Description: "  warm  ", PriceCents: 1599,
		ImageAssetPublicID: &imageID, SortOrder: 5,
	})
	if err != nil || dish.ID != 21 || dish.Status != DishDraft || dish.RowVersion != 1 || dish.Name != "Noodles" || dish.PriceCents != 1599 {
		t.Fatalf("dish=%#v err=%v", dish, err)
	}
}

func TestCatalogServiceRejectsInvalidInputBeforeRepository(t *testing.T) {
	repository := &catalogRepositoryStub{}
	service, _ := NewCatalogService(repository, time.Now)
	merchantID, _ := NewULID()
	dishID, _ := NewULID()
	if _, err := service.CreateDish(context.Background(), CreateDishInput{MerchantPublicID: merchantID, Name: "Free", PriceCents: 0}); err != ErrInvalidRecord {
		t.Fatalf("zero price returned %v", err)
	}
	if _, err := service.TakeOffShelf(context.Background(), dishID, merchantID, DishDraft, 1); err != ErrInvalidTransition {
		t.Fatalf("illegal transition returned %v", err)
	}
	if _, err := service.UpdateDish(context.Background(), UpdateDishInput{
		DishPublicID: dishID, MerchantPublicID: merchantID, ExpectedStatus: DishRemoved,
		ExpectedRowVersion: 1, Name: "Removed", PriceCents: 1,
	}); err != ErrInvalidTransition {
		t.Fatalf("terminal update returned %v", err)
	}
	if repository.dishWrites != 0 || repository.categoryWrites != 0 || repository.createdDish != nil {
		t.Fatal("repository called for invalid input")
	}
}

func TestCatalogServiceDelegatesOwnerFencesAndExpectedStatuses(t *testing.T) {
	repository := &catalogRepositoryStub{}
	service, _ := NewCatalogService(repository, time.Now)
	merchantID, _ := NewULID()
	categoryID, _ := NewULID()
	dishID, _ := NewULID()
	if _, err := service.RenameCategory(context.Background(), categoryID, merchantID, 2, "Renamed"); err != nil {
		t.Fatal(err)
	}
	if _, err := service.TakeOffShelf(context.Background(), dishID, merchantID, DishSoldOut, 4); err != nil {
		t.Fatal(err)
	}
	if repository.lastMerchantID != merchantID || repository.lastDishFrom != DishSoldOut || repository.lastDishTo != DishOffShelf {
		t.Fatalf("owner/status fence missing repo=%#v", repository)
	}
}

func TestNewCatalogServiceRejectsNilRepository(t *testing.T) {
	if _, err := NewCatalogService(nil, time.Now); err != ErrInvalidRecord {
		t.Fatalf("nil repository returned %v", err)
	}
}
