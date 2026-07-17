package navigation

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"
	"testing"
	"time"
)

func TestExternalTakeoutServiceLifecycle(t *testing.T) {
	ctx := context.Background()
	repository := newMemoryNavigationRepository()
	systemCategory := repository.seedCategory(Category{
		Name: "外卖商家", SystemKey: stringPointer(SystemKeyExternalTakeoutMerchants), Sort: 20, Enabled: false,
	})
	ordinaryCategory := repository.seedCategory(Category{Name: "普通导航", Sort: 30, Enabled: true})
	service := NewService(repository)

	input := validExternalTakeoutInput()
	input.Title = "  校园餐厅  "
	input.Path = "/pages/store/index?b=2&a=1"
	created, err := service.CreateExternalTakeoutMerchant(ctx, input)
	if err != nil {
		t.Fatalf("create external takeout merchant: %v", err)
	}
	if created.ID == 0 || created.Title != "校园餐厅" || created.Path != "pages/store/index?a=1&b=2" ||
		!created.Enabled || created.OperatingStatus != OperatingStatusOpen {
		t.Fatalf("unexpected created merchant: %+v", created)
	}
	stored := repository.items[created.ID]
	if stored.CategoryID != systemCategory.ID || !stored.Enabled || stored.OperatingStatus != OperatingStatusOpen {
		t.Fatalf("merchant was not forced into system category/open/visible: %+v", stored)
	}
	if len(repository.calls) < 3 || repository.calls[0] != "find_category_by_system_key" ||
		repository.calls[1] != "duplicate_target" || repository.calls[2] != "create_item" {
		t.Fatalf("unexpected create call order: %v", repository.calls)
	}

	if _, err := service.CreateExternalTakeoutMerchant(ctx, input); !errors.Is(err, ErrDuplicateTarget) {
		t.Fatalf("duplicate create error = %v, want ErrDuplicateTarget", err)
	}

	stored.Enabled = false
	stored.OperatingStatus = OperatingStatusResting
	repository.items[stored.ID] = stored
	disabledDirectory, err := service.ExternalTakeoutDirectory(ctx)
	if err != nil {
		t.Fatalf("load disabled external takeout directory: %v", err)
	}
	if disabledDirectory.CategoryEnabled || len(disabledDirectory.Items) != 1 ||
		disabledDirectory.Items[0].Enabled || disabledDirectory.Items[0].OperatingStatus != OperatingStatusResting {
		t.Fatalf("disabled directory did not return hidden/resting item: %+v", disabledDirectory)
	}
	repository.calls = nil
	updateInput := ExternalTakeoutMerchantInput{
		Title: "新标题", Subtitle: "新副标题", IconText: "新", AppID: "wx1234567890ABCDE1",
		Path: "pages/store/detail?id=2", Color: "#123ABC", Sort: 0,
	}
	updated, err := service.UpdateExternalTakeoutMerchant(ctx, created.ID, updateInput)
	if err != nil {
		t.Fatalf("update external takeout merchant: %v", err)
	}
	if updated.Title != updateInput.Title || updated.Sort != 0 || updated.Enabled ||
		updated.OperatingStatus != OperatingStatusResting {
		t.Fatalf("profile update changed state or missed zero value: %+v", updated)
	}
	assertChangeKeys(t, repository.lastScopedItemUpdate,
		"title", "subtitle", "icon_text", "app_id", "path", "color", "sort")
	collision := repository.seedItem(Item{
		CategoryID: systemCategory.ID, Title: "碰撞商家", IconText: "碰", AppID: "wx1234567890ABCDE8",
		Path: "pages/collision/index", Color: "#AABBCC", Sort: 99, Enabled: true, OperatingStatus: OperatingStatusOpen,
	})
	duplicateUpdate := updateInput
	duplicateUpdate.AppID = collision.AppID
	duplicateUpdate.Path = collision.Path
	beforeDuplicate := repository.items[created.ID]
	if _, err := service.UpdateExternalTakeoutMerchant(ctx, created.ID, duplicateUpdate); !errors.Is(err, ErrDuplicateTarget) {
		t.Fatalf("duplicate update error = %v, want ErrDuplicateTarget", err)
	}
	if repository.items[created.ID] != beforeDuplicate {
		t.Fatalf("duplicate update mutated item: before=%+v after=%+v", beforeDuplicate, repository.items[created.ID])
	}
	delete(repository.items, collision.ID)

	if err := service.SetExternalTakeoutVisibility(ctx, created.ID, true); err != nil {
		t.Fatalf("restore merchant visibility: %v", err)
	}
	stored = repository.items[created.ID]
	if !stored.Enabled || stored.OperatingStatus != OperatingStatusResting {
		t.Fatalf("visibility update changed operating status: %+v", stored)
	}
	assertChangeKeys(t, repository.lastScopedItemUpdate, "enabled")

	if err := service.SetExternalTakeoutOperatingStatus(ctx, created.ID, OperatingStatusOpen); err != nil {
		t.Fatalf("set merchant operating status: %v", err)
	}
	stored = repository.items[created.ID]
	if !stored.Enabled || stored.OperatingStatus != OperatingStatusOpen {
		t.Fatalf("operating status update changed visibility: %+v", stored)
	}
	assertChangeKeys(t, repository.lastScopedItemUpdate, "operating_status")

	if err := service.SetExternalTakeoutCategoryVisibility(ctx, true); err != nil {
		t.Fatalf("enable external takeout category: %v", err)
	}
	if !repository.categories[systemCategory.ID].Enabled {
		t.Fatal("system category visibility was not persisted")
	}
	assertChangeKeys(t, repository.lastCategoryUpdate, "enabled")

	if err := service.SetExternalTakeoutVisibility(ctx, created.ID, false); err != nil {
		t.Fatalf("hide merchant: %v", err)
	}
	directory, err := service.ExternalTakeoutDirectory(ctx)
	if err != nil {
		t.Fatalf("load external takeout directory: %v", err)
	}
	if !directory.CategoryEnabled || len(directory.Items) != 1 || directory.Items[0].ID != created.ID || directory.Items[0].Enabled {
		t.Fatalf("directory must include hidden merchant and category switch: %+v", directory)
	}

	wrongScopeItem := repository.seedItem(Item{
		CategoryID: ordinaryCategory.ID, Title: "普通项", Subtitle: "", IconText: "普",
		AppID: "wx1234567890ABCDE2", Path: "pages/ordinary/index", Color: "#112233",
		Sort: 1, Enabled: true, OperatingStatus: OperatingStatusOpen,
	})
	for name, operation := range map[string]func() error{
		"update": func() error {
			_, err := service.UpdateExternalTakeoutMerchant(ctx, wrongScopeItem.ID, updateInput)
			return err
		},
		"delete": func() error { return service.DeleteExternalTakeoutMerchant(ctx, wrongScopeItem.ID) },
		"status": func() error {
			return service.SetExternalTakeoutOperatingStatus(ctx, wrongScopeItem.ID, OperatingStatusResting)
		},
		"visibility": func() error { return service.SetExternalTakeoutVisibility(ctx, wrongScopeItem.ID, false) },
	} {
		if err := operation(); !errors.Is(err, ErrNotFound) {
			t.Fatalf("wrong-scope %s error = %v, want ErrNotFound", name, err)
		}
	}

	if err := service.DeleteExternalTakeoutMerchant(ctx, created.ID); err != nil {
		t.Fatalf("delete external takeout merchant: %v", err)
	}
	if _, exists := repository.items[created.ID]; exists {
		t.Fatal("external takeout merchant was not deleted")
	}
	if err := service.DeleteExternalTakeoutMerchant(ctx, created.ID); !errors.Is(err, ErrNotFound) {
		t.Fatalf("second delete error = %v, want ErrNotFound", err)
	}
}

func TestExternalTakeoutServiceErrors(t *testing.T) {
	ctx := context.Background()
	repository := newMemoryNavigationRepository()
	systemCategory := repository.seedCategory(Category{
		Name: "外卖商家", SystemKey: stringPointer(SystemKeyExternalTakeoutMerchants), Sort: 20, Enabled: true,
	})
	service := NewService(repository)

	invalid := validExternalTakeoutInput()
	invalid.AppID = "not-a-mini-program"
	if _, err := service.CreateExternalTakeoutMerchant(ctx, invalid); !errors.Is(err, ErrInvalidExternalTakeoutInput) {
		t.Fatalf("invalid merchant error = %v, want ErrInvalidExternalTakeoutInput", err)
	}
	if err := service.SetExternalTakeoutOperatingStatus(ctx, 1, OperatingStatus("OPEN")); !errors.Is(err, ErrInvalidExternalTakeoutInput) {
		t.Fatalf("invalid status error = %v, want ErrInvalidExternalTakeoutInput", err)
	}
	if _, err := service.UpdateExternalTakeoutMerchant(ctx, 0, validExternalTakeoutInput()); !errors.Is(err, ErrInvalidExternalTakeoutInput) {
		t.Fatalf("zero id update error = %v, want ErrInvalidExternalTakeoutInput", err)
	}
	for name, operation := range map[string]func() error{
		"delete":     func() error { return service.DeleteExternalTakeoutMerchant(ctx, 0) },
		"status":     func() error { return service.SetExternalTakeoutOperatingStatus(ctx, 0, OperatingStatusOpen) },
		"visibility": func() error { return service.SetExternalTakeoutVisibility(ctx, 0, true) },
	} {
		if err := operation(); !errors.Is(err, ErrInvalidExternalTakeoutInput) {
			t.Fatalf("zero id %s error = %v, want ErrInvalidExternalTakeoutInput", name, err)
		}
	}
	secretPath := validExternalTakeoutInput()
	secretPath.Path = "pages/store/index?token=must-not-leak"
	if _, err := service.CreateExternalTakeoutMerchant(ctx, secretPath); !errors.Is(err, ErrInvalidExternalTakeoutInput) ||
		strings.Contains(err.Error(), "must-not-leak") || strings.Contains(err.Error(), secretPath.AppID) {
		t.Fatalf("invalid path error leaked input or lost sentinel: %v", err)
	}

	forced := errors.New("SELECT secret FROM navigation_items")
	repository.failures["list_items_by_category"] = forced
	if _, err := service.ExternalTakeoutDirectory(ctx); err != forced {
		t.Fatalf("directory repository error = %v, want original error", err)
	}
	delete(repository.failures, "list_items_by_category")
	repository.failures["duplicate_target"] = forced
	if _, err := service.CreateExternalTakeoutMerchant(ctx, validExternalTakeoutInput()); err != forced {
		t.Fatalf("duplicate precheck error = %v, want original error", err)
	}
	delete(repository.failures, "duplicate_target")
	repository.failures["create_item"] = forced
	if _, err := service.CreateExternalTakeoutMerchant(ctx, validExternalTakeoutInput()); err != forced {
		t.Fatalf("create repository error = %v, want original error", err)
	}
	repository.failures["create_item"] = ErrDuplicateTarget
	if _, err := service.CreateExternalTakeoutMerchant(ctx, validExternalTakeoutInput()); !errors.Is(err, ErrDuplicateTarget) {
		t.Fatalf("racing create duplicate error = %v, want ErrDuplicateTarget", err)
	}
	delete(repository.failures, "create_item")

	item := repository.seedItem(Item{
		CategoryID: systemCategory.ID, Title: "失败测试商家", IconText: "测", AppID: "wx1234567890ABCDE9",
		Path: "pages/failure/index", Color: "#ABC123", Enabled: true, OperatingStatus: OperatingStatusOpen,
	})
	repository.failures["find_item"] = forced
	if err := service.SetExternalTakeoutVisibility(ctx, item.ID, false); err != forced {
		t.Fatalf("find item error = %v, want original error", err)
	}
	delete(repository.failures, "find_item")
	repository.failures["update_item_in_category"] = forced
	if err := service.SetExternalTakeoutVisibility(ctx, item.ID, false); err != forced {
		t.Fatalf("scoped update error = %v, want original error", err)
	}
	delete(repository.failures, "update_item_in_category")
	repository.failures["delete_item_in_category"] = forced
	if err := service.DeleteExternalTakeoutMerchant(ctx, item.ID); err != forced {
		t.Fatalf("scoped delete error = %v, want original error", err)
	}
	delete(repository.failures, "delete_item_in_category")
	repository.failures["update_item_in_category"] = ErrDuplicateTarget
	if _, err := service.UpdateExternalTakeoutMerchant(ctx, item.ID, ExternalTakeoutMerchantInput{
		Title: "失败测试商家", IconText: "测", AppID: "wx1234567890ABCDE0",
		Path: "pages/failure-updated/index", Color: "#ABC123",
	}); !errors.Is(err, ErrDuplicateTarget) {
		t.Fatalf("racing update duplicate error = %v, want ErrDuplicateTarget", err)
	}
	delete(repository.failures, "update_item_in_category")

	emptyRepository := newMemoryNavigationRepository()
	if _, err := NewService(emptyRepository).ExternalTakeoutDirectory(ctx); !errors.Is(err, ErrNotFound) {
		t.Fatalf("missing system category error = %v, want ErrNotFound", err)
	}
	malformedRepository := newMemoryNavigationRepository()
	malformedRepository.systemCategoryLookupOverride = &Category{
		ID: 99, Name: "伪造系统分类", SystemKey: stringPointer("wrong_key"), Enabled: true,
	}
	if _, err := NewService(malformedRepository).ExternalTakeoutDirectory(ctx); !errors.Is(err, ErrNotFound) {
		t.Fatalf("wrong-key system category error = %v, want ErrNotFound", err)
	}
	malformedRepository.systemCategoryLookupOverride = &Category{
		ID: 0, Name: "零 ID 系统分类", SystemKey: stringPointer(SystemKeyExternalTakeoutMerchants), Enabled: true,
	}
	if _, err := NewService(malformedRepository).ExternalTakeoutDirectory(ctx); !errors.Is(err, ErrNotFound) {
		t.Fatalf("zero-id system category error = %v, want ErrNotFound", err)
	}
}

func TestSystemCategoryGuardAndLegacyCompatibility(t *testing.T) {
	ctx := context.Background()
	repository := newMemoryNavigationRepository()
	systemCategory := repository.seedCategory(Category{
		Name: "外卖商家", SystemKey: stringPointer(SystemKeyExternalTakeoutMerchants), Sort: 20, Enabled: true,
	})
	ordinaryCategory := repository.seedCategory(Category{Name: "普通导航", Sort: 30, Enabled: true})
	otherOrdinaryCategory := repository.seedCategory(Category{Name: "普通导航二", Sort: 31, Enabled: true})
	sameNameOrdinaryCategory := repository.seedCategory(Category{Name: "外卖商家", Sort: 32, Enabled: true})
	futureSystemCategory := repository.seedCategory(Category{
		Name: "未来系统分类", SystemKey: stringPointer("future_system_category"), Sort: 33, Enabled: true,
	})
	systemItem := repository.seedItem(Item{
		CategoryID: systemCategory.ID, Title: "系统商家", IconText: "餐", AppID: "wx1234567890ABCDE3",
		Path: "pages/system/index", Color: "#334455", Sort: 1, Enabled: true, OperatingStatus: OperatingStatusOpen,
	})
	service := NewService(repository)

	forgedKey := SystemKeyExternalTakeoutMerchants
	createdCategory, err := service.CreateCategory(ctx, Category{
		ID: 999, Name: "客户端普通分类", SystemKey: &forgedKey, Sort: 50, Enabled: true,
		Items: []Item{{
			Title: "嵌套绕行项", AppID: "", Path: "", Enabled: true, OperatingStatus: OperatingStatusResting,
		}},
	})
	if err != nil {
		t.Fatalf("create ordinary category: %v", err)
	}
	if createdCategory.ID == 0 || createdCategory.ID == 999 || createdCategory.SystemKey != nil {
		t.Fatalf("generic category create retained identity/system key: %+v", createdCategory)
	}
	if len(createdCategory.Items) != 0 || len(repository.items) != 1 {
		t.Fatalf("generic category create retained nested items: value=%+v items=%+v", createdCategory, repository.items)
	}
	createdCategory.Name = "客户端普通分类更新"
	createdCategory.SystemKey = &forgedKey
	createdCategory.Sort = 0
	createdCategory.Enabled = false
	if err := service.UpdateCategory(ctx, createdCategory); err != nil {
		t.Fatalf("update ordinary category: %v", err)
	}
	storedCategory := repository.categories[createdCategory.ID]
	if storedCategory.SystemKey != nil || storedCategory.Sort != 0 || storedCategory.Enabled ||
		storedCategory.Name != createdCategory.Name {
		t.Fatalf("generic category update bypassed whitelist: %+v", storedCategory)
	}
	assertChangeKeys(t, repository.lastCategoryUpdate, "name", "sort", "enabled")

	if err := service.UpdateCategory(ctx, Category{ID: systemCategory.ID, Name: "重命名系统分类", Enabled: true}); !errors.Is(err, ErrSystemManaged) {
		t.Fatalf("system category update error = %v, want ErrSystemManaged", err)
	}
	if err := service.DeleteCategory(ctx, systemCategory.ID); !errors.Is(err, ErrSystemManaged) {
		t.Fatalf("system category delete error = %v, want ErrSystemManaged", err)
	}
	if err := service.UpdateCategory(ctx, Category{ID: futureSystemCategory.ID, Name: "未来系统分类更新", Enabled: true}); !errors.Is(err, ErrSystemManaged) {
		t.Fatalf("future system category update error = %v, want ErrSystemManaged", err)
	}
	if err := service.UpdateCategory(ctx, Category{ID: sameNameOrdinaryCategory.ID, Name: "同名普通分类更新", Enabled: true}); err != nil {
		t.Fatalf("same-name ordinary category must remain manageable: %v", err)
	}
	if _, err := service.CreateItem(ctx, Item{
		CategoryID: systemCategory.ID, Title: "绕行创建", AppID: "legacy-app", Path: "legacy/path",
	}); !errors.Is(err, ErrSystemManaged) {
		t.Fatalf("generic create in system category error = %v, want ErrSystemManaged", err)
	}

	oldJSON := fmt.Sprintf(`{
		"id":999,"categoryId":%d,"title":"旧普通导航","subtitle":"旧副标题","iconText":"旧",
		"appId":"legacy-app-id","path":"legacy/path","color":"legacy-color","sort":8,"enabled":true
	}`, ordinaryCategory.ID)
	var oldItem Item
	if err := json.Unmarshal([]byte(oldJSON), &oldItem); err != nil {
		t.Fatalf("decode old navigation JSON: %v", err)
	}
	createdItem, err := service.CreateItem(ctx, oldItem)
	if err != nil {
		t.Fatalf("create old navigation JSON: %v", err)
	}
	if createdItem.ID == 0 || createdItem.ID == 999 || createdItem.OperatingStatus != OperatingStatusOpen {
		t.Fatalf("old navigation create did not get open/server id: %+v", createdItem)
	}

	forgedUpdate := createdItem
	forgedUpdate.CategoryID = otherOrdinaryCategory.ID
	forgedUpdate.Title = "旧普通导航更新"
	forgedUpdate.Subtitle = ""
	forgedUpdate.Sort = 0
	forgedUpdate.Enabled = false
	forgedUpdate.OperatingStatus = OperatingStatusResting
	if err := service.UpdateItem(ctx, forgedUpdate); err != nil {
		t.Fatalf("update old navigation JSON: %v", err)
	}
	storedItem := repository.items[createdItem.ID]
	if storedItem.CategoryID != otherOrdinaryCategory.ID || storedItem.Title != forgedUpdate.Title ||
		storedItem.Subtitle != "" || storedItem.Sort != 0 || storedItem.Enabled ||
		storedItem.OperatingStatus != OperatingStatusOpen {
		t.Fatalf("generic update bypassed status guard or missed zero values: %+v", storedItem)
	}
	if _, ok := repository.lastItemUpdate["operating_status"]; ok {
		t.Fatalf("generic update forwarded operating_status: %#v", repository.lastItemUpdate)
	}

	moveIntoSystem := storedItem
	moveIntoSystem.CategoryID = systemCategory.ID
	if err := service.UpdateItem(ctx, moveIntoSystem); !errors.Is(err, ErrSystemManaged) {
		t.Fatalf("move into system category error = %v, want ErrSystemManaged", err)
	}
	moveOutOfSystem := systemItem
	moveOutOfSystem.CategoryID = ordinaryCategory.ID
	if err := service.UpdateItem(ctx, moveOutOfSystem); !errors.Is(err, ErrSystemManaged) {
		t.Fatalf("move out of system category error = %v, want ErrSystemManaged", err)
	}
	if err := service.DeleteItem(ctx, systemItem.ID); !errors.Is(err, ErrSystemManaged) {
		t.Fatalf("generic system item delete error = %v, want ErrSystemManaged", err)
	}

	invalidCases := []struct {
		name string
		run  func() error
	}{
		{name: "create category", run: func() error { _, err := service.CreateCategory(ctx, Category{}); return err }},
		{name: "update category", run: func() error { return service.UpdateCategory(ctx, Category{}) }},
		{name: "delete category", run: func() error { return service.DeleteCategory(ctx, 0) }},
		{name: "create item", run: func() error { _, err := service.CreateItem(ctx, Item{}); return err }},
		{name: "update item", run: func() error { return service.UpdateItem(ctx, Item{}) }},
		{name: "delete item", run: func() error { return service.DeleteItem(ctx, 0) }},
	}
	for _, testCase := range invalidCases {
		t.Run(testCase.name, func(t *testing.T) {
			if err := testCase.run(); !errors.Is(err, ErrInvalidNavigationInput) {
				t.Fatalf("error = %v, want ErrInvalidNavigationInput", err)
			}
		})
	}

	forced := errors.New("repository unavailable")
	repository.failures["find_category"] = forced
	if err := service.UpdateCategory(ctx, Category{ID: ordinaryCategory.ID, Name: "合法", Enabled: true}); err != forced {
		t.Fatalf("generic repository error = %v, want original error", err)
	}
}

func TestPublicTreeReturnsReadOnlyEnabledDTOs(t *testing.T) {
	ctx := context.Background()
	repository := newMemoryNavigationRepository()
	systemCategory := repository.seedCategory(Category{
		Name: "外卖商家", SystemKey: stringPointer(SystemKeyExternalTakeoutMerchants), Sort: 20, Enabled: true,
		CreatedAt: time.Now(), UpdatedAt: time.Now(),
	})
	ordinaryCategory := repository.seedCategory(Category{Name: "历史导航", Sort: 30, Enabled: true})
	disabledCategory := repository.seedCategory(Category{Name: "关闭分类", Sort: 40, Enabled: false})
	repository.seedItem(Item{
		CategoryID: systemCategory.ID, Title: "营业商家", Subtitle: "可见", IconText: "餐",
		AppID: "wx1234567890ABCDE4", Path: "pages/store/index", Color: "#556677",
		Sort: 1, Enabled: true, OperatingStatus: OperatingStatusResting, CreatedAt: time.Now(), UpdatedAt: time.Now(),
	})
	repository.seedItem(Item{
		CategoryID: systemCategory.ID, Title: "隐藏商家", IconText: "隐", AppID: "wx1234567890ABCDE5",
		Path: "pages/hidden/index", Color: "#667788", Sort: 2, Enabled: false, OperatingStatus: OperatingStatusOpen,
	})
	repository.seedItem(Item{
		CategoryID: ordinaryCategory.ID, Title: "旧入口", IconText: "旧", AppID: "legacy-app",
		Path: "legacy/path", Color: "legacy", Sort: 1, Enabled: true, OperatingStatus: OperatingStatusOpen,
	})
	repository.seedItem(Item{
		CategoryID: disabledCategory.ID, Title: "关闭入口", IconText: "关", AppID: "legacy-disabled",
		Path: "legacy/disabled", Color: "legacy", Sort: 1, Enabled: true, OperatingStatus: OperatingStatusOpen,
	})
	repository.forceUnfilteredTree = true

	values, err := NewService(repository).PublicTree(ctx)
	if err != nil {
		t.Fatalf("public tree: %v", err)
	}
	if len(values) != 2 {
		t.Fatalf("public tree categories = %+v, want 2 enabled categories", values)
	}
	if values[0].ID != systemCategory.ID || values[0].SystemKey == nil ||
		*values[0].SystemKey != SystemKeyExternalTakeoutMerchants || len(values[0].Items) != 1 {
		t.Fatalf("unexpected system public category: %+v", values[0])
	}
	if values[0].Items[0].OperatingStatus != OperatingStatusResting {
		t.Fatalf("public item lost operating status: %+v", values[0].Items[0])
	}
	if values[1].ID != ordinaryCategory.ID || len(values[1].Items) != 1 || values[1].Items[0].AppID != "legacy-app" {
		t.Fatalf("legacy navigation was not preserved: %+v", values[1])
	}

	forced := errors.New("SELECT secret FROM navigation_categories")
	repository.failures["tree"] = forced
	if _, err := NewService(repository).PublicTree(ctx); err != forced {
		t.Fatalf("public tree repository error = %v, want original error", err)
	}
}

func validExternalTakeoutInput() ExternalTakeoutMerchantInput {
	return ExternalTakeoutMerchantInput{
		Title: "校园餐厅", Subtitle: "校内配送", IconText: "餐", AppID: "wx1234567890ABCDEF",
		Path: "pages/store/index", Color: "#12AB34", Sort: 10,
	}
}

func assertChangeKeys(t *testing.T, changes map[string]any, expected ...string) {
	t.Helper()
	if len(changes) != len(expected) {
		t.Fatalf("changes = %#v, want keys %v", changes, expected)
	}
	for _, key := range expected {
		if _, ok := changes[key]; !ok {
			t.Fatalf("changes = %#v, missing key %q", changes, key)
		}
	}
}

func stringPointer(value string) *string { return &value }

type memoryNavigationRepository struct {
	categories                   map[uint]Category
	items                        map[uint]Item
	nextCategoryID               uint
	nextItemID                   uint
	failures                     map[string]error
	calls                        []string
	lastCategoryUpdate           map[string]any
	lastItemUpdate               map[string]any
	lastScopedItemUpdate         map[string]any
	forceUnfilteredTree          bool
	systemCategoryLookupOverride *Category
}

func newMemoryNavigationRepository() *memoryNavigationRepository {
	return &memoryNavigationRepository{
		categories:     make(map[uint]Category),
		items:          make(map[uint]Item),
		nextCategoryID: 1,
		nextItemID:     1,
		failures:       make(map[string]error),
	}
}

func (r *memoryNavigationRepository) record(operation string) error {
	r.calls = append(r.calls, operation)
	return r.failures[operation]
}

func (r *memoryNavigationRepository) seedCategory(value Category) Category {
	if value.ID == 0 {
		value.ID = r.nextCategoryID
		r.nextCategoryID++
	} else if value.ID >= r.nextCategoryID {
		r.nextCategoryID = value.ID + 1
	}
	value.Items = nil
	r.categories[value.ID] = cloneMemoryCategory(value)
	return cloneMemoryCategory(value)
}

func (r *memoryNavigationRepository) seedItem(value Item) Item {
	if value.ID == 0 {
		value.ID = r.nextItemID
		r.nextItemID++
	} else if value.ID >= r.nextItemID {
		r.nextItemID = value.ID + 1
	}
	r.items[value.ID] = value
	return value
}

func (r *memoryNavigationRepository) Tree(_ context.Context, enabledOnly bool) ([]Category, error) {
	if err := r.record("tree"); err != nil {
		return nil, err
	}
	values := make([]Category, 0, len(r.categories))
	for _, category := range r.categories {
		if enabledOnly && !r.forceUnfilteredTree && !category.Enabled {
			continue
		}
		category = cloneMemoryCategory(category)
		category.Items = make([]Item, 0)
		for _, item := range r.items {
			if item.CategoryID != category.ID || (enabledOnly && !r.forceUnfilteredTree && !item.Enabled) {
				continue
			}
			category.Items = append(category.Items, item)
		}
		sort.Slice(category.Items, func(i, j int) bool {
			if category.Items[i].Sort == category.Items[j].Sort {
				return category.Items[i].ID < category.Items[j].ID
			}
			return category.Items[i].Sort < category.Items[j].Sort
		})
		values = append(values, category)
	}
	sort.Slice(values, func(i, j int) bool {
		if values[i].Sort == values[j].Sort {
			return values[i].ID < values[j].ID
		}
		return values[i].Sort < values[j].Sort
	})
	return values, nil
}

func (r *memoryNavigationRepository) FindCategory(_ context.Context, id uint) (Category, bool, error) {
	if err := r.record("find_category"); err != nil {
		return Category{}, false, err
	}
	value, ok := r.categories[id]
	return cloneMemoryCategory(value), ok, nil
}

func (r *memoryNavigationRepository) FindCategoryBySystemKey(_ context.Context, systemKey string) (Category, bool, error) {
	if err := r.record("find_category_by_system_key"); err != nil {
		return Category{}, false, err
	}
	if r.systemCategoryLookupOverride != nil {
		return cloneMemoryCategory(*r.systemCategoryLookupOverride), true, nil
	}
	for _, value := range r.categories {
		if value.SystemKey != nil && *value.SystemKey == systemKey {
			return cloneMemoryCategory(value), true, nil
		}
	}
	return Category{}, false, nil
}

func (r *memoryNavigationRepository) FindItem(_ context.Context, id uint) (Item, bool, error) {
	if err := r.record("find_item"); err != nil {
		return Item{}, false, err
	}
	value, ok := r.items[id]
	return value, ok, nil
}

func (r *memoryNavigationRepository) ListItemsByCategory(_ context.Context, categoryID uint) ([]Item, error) {
	if err := r.record("list_items_by_category"); err != nil {
		return nil, err
	}
	values := make([]Item, 0)
	for _, value := range r.items {
		if value.CategoryID == categoryID {
			values = append(values, value)
		}
	}
	sort.Slice(values, func(i, j int) bool {
		if values[i].Sort == values[j].Sort {
			return values[i].ID < values[j].ID
		}
		return values[i].Sort < values[j].Sort
	})
	return values, nil
}

func (r *memoryNavigationRepository) DuplicateTarget(_ context.Context, categoryID uint, appID, path string, excludeID uint) (bool, error) {
	if err := r.record("duplicate_target"); err != nil {
		return false, err
	}
	for _, value := range r.items {
		if value.ID != excludeID && value.CategoryID == categoryID && value.AppID == appID && value.Path == path {
			return true, nil
		}
	}
	return false, nil
}

func (r *memoryNavigationRepository) CreateCategory(_ context.Context, value *Category) error {
	if err := r.record("create_category"); err != nil {
		return err
	}
	nestedItems := append([]Item(nil), value.Items...)
	*value = r.seedCategory(*value)
	for _, item := range nestedItems {
		item.CategoryID = value.ID
		r.seedItem(item)
	}
	return nil
}

func (r *memoryNavigationRepository) UpdateCategory(_ context.Context, id uint, changes map[string]any) error {
	if err := r.record("update_category"); err != nil {
		return err
	}
	value, ok := r.categories[id]
	if !ok {
		return ErrNotFound
	}
	r.lastCategoryUpdate = cloneChanges(changes)
	if change, ok := changes["name"].(string); ok {
		value.Name = change
	}
	if change, ok := changes["sort"].(int); ok {
		value.Sort = change
	}
	if change, ok := changes["enabled"].(bool); ok {
		value.Enabled = change
	}
	r.categories[id] = value
	return nil
}

func (r *memoryNavigationRepository) DeleteCategory(_ context.Context, id uint) error {
	if err := r.record("delete_category"); err != nil {
		return err
	}
	if _, ok := r.categories[id]; !ok {
		return ErrNotFound
	}
	delete(r.categories, id)
	for itemID, item := range r.items {
		if item.CategoryID == id {
			delete(r.items, itemID)
		}
	}
	return nil
}

func (r *memoryNavigationRepository) CreateItem(_ context.Context, value *Item) error {
	if err := r.record("create_item"); err != nil {
		return err
	}
	*value = r.seedItem(*value)
	return nil
}

func (r *memoryNavigationRepository) UpdateItem(_ context.Context, id uint, changes map[string]any) error {
	if err := r.record("update_item"); err != nil {
		return err
	}
	value, ok := r.items[id]
	if !ok {
		return ErrNotFound
	}
	r.lastItemUpdate = cloneChanges(changes)
	applyMemoryItemChanges(&value, changes)
	r.items[id] = value
	return nil
}

func (r *memoryNavigationRepository) DeleteItem(_ context.Context, id uint) error {
	if err := r.record("delete_item"); err != nil {
		return err
	}
	if _, ok := r.items[id]; !ok {
		return ErrNotFound
	}
	delete(r.items, id)
	return nil
}

func (r *memoryNavigationRepository) UpdateItemInCategory(_ context.Context, id, categoryID uint, changes map[string]any) error {
	if err := r.record("update_item_in_category"); err != nil {
		return err
	}
	value, ok := r.items[id]
	if !ok || value.CategoryID != categoryID {
		return ErrNotFound
	}
	r.lastScopedItemUpdate = cloneChanges(changes)
	applyMemoryItemChanges(&value, changes)
	r.items[id] = value
	return nil
}

func (r *memoryNavigationRepository) DeleteItemInCategory(_ context.Context, id, categoryID uint) error {
	if err := r.record("delete_item_in_category"); err != nil {
		return err
	}
	value, ok := r.items[id]
	if !ok || value.CategoryID != categoryID {
		return ErrNotFound
	}
	delete(r.items, id)
	return nil
}

func applyMemoryItemChanges(value *Item, changes map[string]any) {
	if change, ok := changes["category_id"].(uint); ok {
		value.CategoryID = change
	}
	if change, ok := changes["title"].(string); ok {
		value.Title = change
	}
	if change, ok := changes["subtitle"].(string); ok {
		value.Subtitle = change
	}
	if change, ok := changes["icon_text"].(string); ok {
		value.IconText = change
	}
	if change, ok := changes["app_id"].(string); ok {
		value.AppID = change
	}
	if change, ok := changes["path"].(string); ok {
		value.Path = change
	}
	if change, ok := changes["color"].(string); ok {
		value.Color = change
	}
	if change, ok := changes["sort"].(int); ok {
		value.Sort = change
	}
	if change, ok := changes["enabled"].(bool); ok {
		value.Enabled = change
	}
	if change, ok := changes["operating_status"].(OperatingStatus); ok {
		value.OperatingStatus = change
	}
}

func cloneMemoryCategory(value Category) Category {
	if value.SystemKey != nil {
		key := *value.SystemKey
		value.SystemKey = &key
	}
	if value.Items != nil {
		value.Items = append([]Item(nil), value.Items...)
	}
	return value
}

func cloneChanges(values map[string]any) map[string]any {
	cloned := make(map[string]any, len(values))
	for key, value := range values {
		cloned[key] = value
	}
	return cloned
}

var _ Repository = (*memoryNavigationRepository)(nil)
