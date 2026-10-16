package deliverymerchant

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"campus-circle/backend/internal/module/delivery"
	"github.com/gin-gonic/gin"
)

func TestOrderIntakeReportsOnlyManualCapability(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	NewHandler(nil, func(*gin.Context) uint64 { return 1 }, false).Register(router.Group("/delivery/merchant"))
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/delivery/merchant/order-intake", nil))
	var result struct {
		Data OrderIntakeView `json:"data"`
	}
	if recorder.Code != http.StatusOK || json.Unmarshal(recorder.Body.Bytes(), &result) != nil || !result.Data.ManualAvailable || result.Data.ExternalAvailable || len(result.Data.Platforms) != 3 {
		t.Fatalf("unexpected capability: %s", recorder.Body.String())
	}
	for _, platform := range result.Data.Platforms {
		if platform.Connected {
			t.Fatal("reserved provider falsely reported connected")
		}
	}
}

func TestMerchantOrderViewIncludesSourceMetadata(t *testing.T) {
	orderID, shopID := "001-AbC", "Store-AbC"
	view := orderView(delivery.Order{SourcePlatform: delivery.SourceTaobao, PickupCode: "0022", SourceOrderID: &orderID, SourceShopID: &shopID})
	if view.SourcePlatform != delivery.SourceTaobao || view.PickupCode != "0022" || view.SourceOrderID != &orderID || view.SourceShopID != &shopID {
		t.Fatal("merchant source projection lost data")
	}
	if orderView(delivery.Order{}).SourcePlatform != delivery.SourceLocal {
		t.Fatal("legacy order default is not local")
	}
}
