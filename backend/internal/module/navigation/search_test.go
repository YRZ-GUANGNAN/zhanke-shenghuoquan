package navigation

import (
	"context"
	"testing"
)

func TestSearchExternalTakeoutUsesPublicVisibleDirectory(t *testing.T) {
	repository := newMemoryNavigationRepository()
	category := repository.seedCategory(Category{
		Name: "外卖商家", SystemKey: stringPointer(SystemKeyExternalTakeoutMerchants), Enabled: true,
	})
	repository.seedItem(Item{CategoryID: category.ID, Title: "奶茶店", Subtitle: "芋泥波波", Enabled: true, Sort: 2, OperatingStatus: OperatingStatusResting})
	repository.seedItem(Item{CategoryID: category.ID, Title: "奶茶铺", Subtitle: "柠檬茶", Enabled: true, Sort: 1, OperatingStatus: OperatingStatusOpen})
	repository.seedItem(Item{CategoryID: category.ID, Title: "隐藏奶茶", Subtitle: "不可见", Enabled: false, Sort: 0, OperatingStatus: OperatingStatusOpen})
	repository.forceUnfilteredTree = true

	page, err := NewService(repository).SearchExternalTakeout(context.Background(), "奶茶", 1, 20)
	if err != nil {
		t.Fatal(err)
	}
	if page.Total != 2 || len(page.Items) != 2 {
		t.Fatalf("page=%+v", page)
	}
	if page.Items[0].Title != "奶茶铺" || page.Items[1].Title != "奶茶店" {
		t.Fatalf("items=%+v", page.Items)
	}
}

func TestSearchExternalTakeoutClosedDirectoryReturnsEmpty(t *testing.T) {
	repository := newMemoryNavigationRepository()
	category := repository.seedCategory(Category{
		Name: "外卖商家", SystemKey: stringPointer(SystemKeyExternalTakeoutMerchants), Enabled: false,
	})
	repository.seedItem(Item{CategoryID: category.ID, Title: "奶茶店", Enabled: true})
	repository.forceUnfilteredTree = true
	page, err := NewService(repository).SearchExternalTakeout(context.Background(), "奶茶", 1, 20)
	if err != nil || page.Total != 0 || len(page.Items) != 0 {
		t.Fatalf("page=%+v err=%v", page, err)
	}
}
