package takeout

import (
	"reflect"
	"strings"
	"testing"
	"time"
)

func TestCatalogModelsValidateFrozenInvariants(t *testing.T) {
	now := time.Now().UTC()
	merchantID, _ := NewULID()
	categoryID, _ := NewULID()
	dishID, _ := NewULID()
	imageID, _ := NewULID()
	category := DishCategory{
		CategoryPublicID: categoryID, MerchantPublicID: merchantID, Name: "Hot Meals",
		RowVersion: 1, CreatedAt: now, UpdatedAt: now,
	}
	if err := category.ValidateNew(); err != nil {
		t.Fatal(err)
	}
	dish := Dish{
		DishPublicID: dishID, MerchantPublicID: merchantID, CategoryPublicID: &categoryID,
		Name: "Rice", Description: "Campus meal", PriceCents: 1299, ImageAssetPublicID: &imageID,
		Status: DishDraft, RowVersion: 1, CreatedAt: now, UpdatedAt: now,
	}
	if err := dish.ValidateNew(); err != nil {
		t.Fatal(err)
	}

	invalidPrice := dish
	invalidPrice.PriceCents = 0
	if err := invalidPrice.ValidateNew(); err != ErrInvalidRecord {
		t.Fatalf("zero price returned %v", err)
	}
	invalidImage := dish
	badImage := "not-a-ulid"
	invalidImage.ImageAssetPublicID = &badImage
	if err := invalidImage.ValidateNew(); err != ErrInvalidRecord {
		t.Fatalf("invalid image returned %v", err)
	}
	invalidCategory := category
	invalidCategory.Name = strings.Repeat("菜", 65)
	if err := invalidCategory.ValidateNew(); err != ErrInvalidRecord {
		t.Fatalf("long category name returned %v", err)
	}
}

func TestDishPriceUsesUnsignedIntegerCents(t *testing.T) {
	field, ok := reflect.TypeOf(Dish{}).FieldByName("PriceCents")
	if !ok || field.Type.Kind() != reflect.Uint64 {
		t.Fatalf("PriceCents type=%v want uint64", field.Type)
	}
}
