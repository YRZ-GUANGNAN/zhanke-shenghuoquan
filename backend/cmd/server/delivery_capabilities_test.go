package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"

	"campus-circle/backend/internal/module/admin/deliverypayment"
	"campus-circle/backend/internal/module/delivery"
)

func TestDeliveryCapabilitiesDoNotAdvertiseMockOrUnavailablePayments(t *testing.T) {
	gin.SetMode(gin.TestMode)
	var absentProvider *deliverypayment.WechatProvider
	for _, test := range []struct {
		name    string
		runtime *deliveryRuntime
		payment bool
	}{
		{"disabled", nil, false},
		{"missing", &deliveryRuntime{}, false},
		{"mock", &deliveryRuntime{funds: &delivery.FundsService{}, paymentProvider: deliverypayment.MockProvider{}}, false},
		{"unconfigured", &deliveryRuntime{funds: &delivery.FundsService{}, paymentProvider: deliverypayment.UnconfiguredProvider{}}, false},
		{"typed nil", &deliveryRuntime{funds: &delivery.FundsService{}, paymentProvider: absentProvider}, false},
		{"no funds", &deliveryRuntime{paymentProvider: &deliverypayment.WechatProvider{}}, false},
		{"intake boundary only", &deliveryRuntime{externalIntake: &delivery.ExternalIntake{}}, false},
		{"wechat runtime", &deliveryRuntime{funds: &delivery.FundsService{}, paymentProvider: &deliverypayment.WechatProvider{}}, true},
	} {
		t.Run(test.name, func(t *testing.T) {
			router := gin.New()
			registerDeliveryCapabilities(router.Group("/api/v1"), test.runtime, func(c *gin.Context) {
				if c.GetHeader("Authorization") != "unit-test" {
					c.AbortWithStatus(http.StatusUnauthorized)
				}
			})
			for _, authenticated := range []bool{false, true} {
				request := httptest.NewRequest(http.MethodGet, "/api/v1/delivery/capabilities", nil)
				if authenticated {
					request.Header.Set("Authorization", "unit-test")
				}
				response := httptest.NewRecorder()
				router.ServeHTTP(response, request)
				if !authenticated {
					if response.Code != http.StatusUnauthorized {
						t.Fatal("anonymous capability access accepted")
					}
					continue
				}
				var value struct {
					Data struct {
						WechatPayment  bool `json:"wechatPayment"`
						ExternalOrders bool `json:"externalOrders"`
					} `json:"data"`
				}
				if response.Code != http.StatusOK || json.Unmarshal(response.Body.Bytes(), &value) != nil {
					t.Fatalf("invalid response: %s", response.Body.String())
				}
				if value.Data.WechatPayment != test.payment || value.Data.ExternalOrders {
					t.Fatalf("wrong advertised capabilities: %s", response.Body.String())
				}
			}
		})
	}
}
