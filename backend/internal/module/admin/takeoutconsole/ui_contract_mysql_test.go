package takeoutconsole

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"campus-circle/backend/internal/module/takeout"
	"campus-circle/backend/internal/module/user"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

const uiContractPrefix = "G6A UI Contract "

func TestTakeoutAdminUIContractPatchAgainstRealHTTPAndMySQL84(t *testing.T) {
	db := openG6A1HTTPMySQL(t)
	var version string
	if err := db.Raw("SELECT VERSION()").Scan(&version).Error; err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(version, "8.4.") {
		t.Fatalf("mysql version=%s want 8.4.x", version)
	}
	cleanupUIContract(t, db)
	t.Cleanup(func() { cleanupUIContract(t, db) })

	ctx := context.Background()
	now := time.Date(2026, 6, 23, 1, 0, 0, 0, time.UTC)
	repository := takeout.NewRepository(db)
	merchants, _ := takeout.NewMerchantService(repository, func() time.Time { return now })
	catalog, _ := takeout.NewCatalogService(repository, func() time.Time { return now })
	orders, _ := takeout.NewOrderService(repository, repository, func() time.Time { return now })
	orderOps, _ := takeout.NewRiderOrderOps(repository, func() time.Time { return now })
	riders, _ := takeout.NewRiderService(repository, orderOps, func() time.Time { return now })
	queries, _ := takeout.NewAdminQueryService(repository)
	withdraws, _ := takeout.NewWithdrawService(repository, func() time.Time { return now })
	auth := user.NewService(user.NewRepository(db), "g6aui-contract-secret-longer-than-thirty-two-characters", "", "", true)
	administrator, err := auth.WechatLogin(ctx, user.LoginInput{Code: "g6aui-admin", Nickname: "G6A UI admin"})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.Model(&user.User{}).Where("id = ?", administrator.User.ID).Update("role", user.RoleAdmin).Error; err != nil {
		t.Fatal(err)
	}

	merchant, err := merchants.CreateMerchant(ctx, takeout.CreateMerchantInput{OwnerUserID: uint64(administrator.User.ID), Name: uiContractPrefix + "Merchant"})
	if err != nil {
		t.Fatal(err)
	}
	merchant, err = merchants.Approve(ctx, merchant.MerchantPublicID, merchant.RowVersion)
	if err != nil {
		t.Fatal(err)
	}
	categoryA, err := catalog.CreateCategory(ctx, takeout.CreateCategoryInput{MerchantPublicID: merchant.MerchantPublicID, Name: uiContractPrefix + "A"})
	if err != nil {
		t.Fatal(err)
	}
	categoryB, err := catalog.CreateCategory(ctx, takeout.CreateCategoryInput{MerchantPublicID: merchant.MerchantPublicID, Name: uiContractPrefix + "B"})
	if err != nil {
		t.Fatal(err)
	}
	dishA := createUIContractDish(t, ctx, catalog, merchant.MerchantPublicID, categoryA.CategoryPublicID, uiContractPrefix+"Dish A")
	_ = createUIContractDish(t, ctx, catalog, merchant.MerchantPublicID, categoryB.CategoryPublicID, uiContractPrefix+"Dish B")
	order, err := orders.CreateOrder(ctx, takeout.CreateOrderInput{
		BuyerUserID: uint64(administrator.User.ID), MerchantPublicID: merchant.MerchantPublicID,
		Items:   []takeout.OrderItemRequest{{DishPublicID: dishA.DishPublicID, Quantity: 1}},
		Address: takeout.OrderAddress{ContactName: uiContractPrefix + "Buyer", ContactPhone: "13000000000", AddressBuilding: "A", AddressRoom: "101"},
	})
	if err != nil {
		t.Fatal(err)
	}
	order, err = orders.MarkPaid(ctx, order.OrderPublicID, order.RowVersion, "g6aui-pay")
	if err != nil {
		t.Fatal(err)
	}

	gin.SetMode(gin.TestMode)
	router := gin.New()
	routes := router.Group("/api/v1/admin/takeout", auth.AuthRequired(), user.AdminRequired())
	NewHandler(merchants, catalog, orders, riders, queries, withdraws).Register(routes)
	server := httptest.NewServer(router)
	defer server.Close()

	base := server.URL + "/api/v1/admin/takeout"
	status, body := httpJSON(t, server.Client(), http.MethodGet, base+"/dishes?merchantId="+merchant.MerchantPublicID+"&page=1&pageSize=20", administrator.Token, nil)
	unfiltered := decodePage[DishView](t, status, body)
	if unfiltered.Total != 2 || len(unfiltered.Items) != 2 {
		t.Fatalf("omitted categoryId changed behavior: %+v", unfiltered)
	}
	status, body = httpJSON(t, server.Client(), http.MethodGet, base+"/dishes?merchantId="+merchant.MerchantPublicID+"&categoryId="+categoryA.CategoryPublicID+"&page=1&pageSize=20", administrator.Token, nil)
	filtered := decodePage[DishView](t, status, body)
	if filtered.Total != 1 || len(filtered.Items) != 1 || filtered.Items[0].DishPublicID != dishA.DishPublicID {
		t.Fatalf("category filter result=%+v", filtered)
	}
	status, body = httpJSON(t, server.Client(), http.MethodGet, base+"/orders/"+order.OrderPublicID, administrator.Token, nil)
	if status != http.StatusOK || body.Code != 0 {
		t.Fatalf("order detail status=%d body=%+v", status, body)
	}
	var detail OrderDetailView
	if err := json.Unmarshal(body.Data, &detail); err != nil {
		t.Fatal(err)
	}
	if detail.Order.PaidAt == nil || !detail.Order.PaidAt.Equal(*order.PaidAt) {
		t.Fatalf("paidAt missing/mismatch: dto=%v model=%v", detail.Order.PaidAt, order.PaidAt)
	}

	t.Logf("real_mysql_version=%s", version)
	t.Log("legacy_dish_query_without_category=PASS total=2")
	t.Log("category_filter=PASS total=1")
	t.Log("order_stage_timestamps=PASS paidAt returned")
	cleanupUIContract(t, db)
	assertUIContractResidue(t, db)
}

func createUIContractDish(t *testing.T, ctx context.Context, service *takeout.CatalogService, merchantID, categoryID, name string) takeout.Dish {
	t.Helper()
	value, err := service.CreateDish(ctx, takeout.CreateDishInput{MerchantPublicID: merchantID, CategoryPublicID: &categoryID, Name: name, PriceCents: 1000})
	if err != nil {
		t.Fatal(err)
	}
	value, err = service.PutOnShelf(ctx, value.DishPublicID, merchantID, value.Status, value.RowVersion)
	if err != nil {
		t.Fatal(err)
	}
	return value
}

func decodePage[T any](t *testing.T, status int, body apiEnvelope) Page[T] {
	t.Helper()
	if status != http.StatusOK || body.Code != 0 {
		t.Fatalf("page status=%d body=%+v", status, body)
	}
	var page Page[T]
	if err := json.Unmarshal(body.Data, &page); err != nil {
		t.Fatal(err)
	}
	return page
}

func cleanupUIContract(t *testing.T, db *gorm.DB) {
	t.Helper()
	var merchants []takeout.Merchant
	if err := db.Where("name LIKE ?", uiContractPrefix+"%").Find(&merchants).Error; err != nil {
		t.Fatal(err)
	}
	for _, merchant := range merchants {
		orderIDs := db.Model(&takeout.Order{}).Select("order_public_id").Where("merchant_public_id = ?", merchant.MerchantPublicID)
		if err := db.Where("order_public_id IN (?)", orderIDs).Delete(&takeout.OrderItem{}).Error; err != nil {
			t.Fatal(err)
		}
		if err := db.Where("merchant_public_id = ?", merchant.MerchantPublicID).Delete(&takeout.Order{}).Error; err != nil {
			t.Fatal(err)
		}
		if err := db.Where("merchant_public_id = ?", merchant.MerchantPublicID).Delete(&takeout.Dish{}).Error; err != nil {
			t.Fatal(err)
		}
		if err := db.Where("merchant_public_id = ?", merchant.MerchantPublicID).Delete(&takeout.DishCategory{}).Error; err != nil {
			t.Fatal(err)
		}
		if err := db.Delete(&takeout.Merchant{}, merchant.ID).Error; err != nil {
			t.Fatal(err)
		}
	}
	if err := db.Unscoped().Where("open_id = ?", "mock_openid_g6aui-admin").Delete(&user.User{}).Error; err != nil {
		t.Fatal(err)
	}
}

func assertUIContractResidue(t *testing.T, db *gorm.DB) {
	t.Helper()
	checks := []struct {
		name  string
		model any
		query string
		arg   any
	}{
		{"merchants", &takeout.Merchant{}, "name LIKE ?", uiContractPrefix + "%"},
		{"categories", &takeout.DishCategory{}, "name LIKE ?", uiContractPrefix + "%"},
		{"dishes", &takeout.Dish{}, "name LIKE ?", uiContractPrefix + "%"},
		{"orders", &takeout.Order{}, "contact_name LIKE ?", uiContractPrefix + "%"},
		{"users", &user.User{}, "open_id = ?", "mock_openid_g6aui-admin"},
	}
	for _, check := range checks {
		var count int64
		if err := db.Unscoped().Model(check.model).Where(check.query, check.arg).Count(&count).Error; err != nil {
			t.Fatal(err)
		}
		if count != 0 {
			t.Errorf("%s residue=%d", check.name, count)
		}
	}
	t.Log("contract_patch_residue merchants=0 categories=0 dishes=0 orders=0 users=0")
}
