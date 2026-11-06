package main

import (
	"bytes"
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"campus-circle/backend/internal/config"
	"campus-circle/backend/internal/module/admin/deliveryrider"
	"campus-circle/backend/internal/module/delivery"
	"github.com/gin-gonic/gin"
)

type runtimeActiveRider struct{}

func (runtimeActiveRider) RequireActive(context.Context, uint64) error { return nil }

func newRuntimeTestDispatch(t *testing.T) *delivery.DispatchService {
	t.Helper()
	// A nil DB deliberately cannot service a legacy fallback. The disabled
	// composition path must install its explicit admission gate without SQL.
	repository := delivery.NewRepository(nil)
	orders, err := delivery.NewService(repository, nil)
	if err != nil {
		t.Fatal(err)
	}
	dispatch, err := delivery.NewDispatchService(repository, orders, runtimeActiveRider{},
		delivery.FeePolicy{DeliveryFeeCents: 150, RiderFeeCents: 100, PlatformCutCents: 50}, nil)
	if err != nil {
		t.Fatal(err)
	}
	return dispatch
}

func TestDeliveryRuntimeDisabledBroadcastRejectsNewClaims(t *testing.T) {
	dispatch := newRuntimeTestDispatch(t)
	store, err := configureDeliveryRiderAvailability(config.Config{EnableDelivery: true}, nil, dispatch, nil)
	if err != nil || store != nil {
		t.Fatalf("disabled runtime must not construct a broadcast store: %v", err)
	}
	_, err = dispatch.ClaimOrder(context.Background(), 77, notificationTestOrderID, 2)
	if !errors.Is(err, delivery.ErrRiderAvailabilityUnavailable) {
		t.Fatalf("disabled runtime allowed the legacy claim path: %v", err)
	}
	gin.SetMode(gin.TestMode)
	router := gin.New()
	deliveryrider.NewHandler(dispatch, nil, nil, func(*gin.Context) uint64 { return 77 }).Register(router.Group("/rider"))
	request := httptest.NewRequest(http.MethodPost, "/rider/orders/"+notificationTestOrderID+"/claim", bytes.NewBufferString(`{"expectedRowVersion":2}`))
	request.Header.Set("Content-Type", "application/json")
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)
	if response.Code != http.StatusServiceUnavailable || !strings.Contains(response.Body.String(), "接单状态暂不可用") {
		t.Fatalf("disabled claim HTTP contract changed: status=%d body=%s", response.Code, response.Body.String())
	}
}

func TestDeliveryRuntimeEnabledBroadcastRequiresSchema(t *testing.T) {
	dispatch := newRuntimeTestDispatch(t)
	_, err := configureDeliveryRiderAvailability(config.Config{EnableDeliveryRiderBroadcast: true}, nil, dispatch, nil)
	if err == nil || !strings.Contains(err.Error(), "migration 055") {
		t.Fatal("enabled broadcast accepted a missing database/schema")
	}
	_, err = dispatch.ClaimOrder(context.Background(), 77, notificationTestOrderID, 2)
	if !errors.Is(err, delivery.ErrRiderAvailabilityUnavailable) {
		t.Fatalf("failed runtime construction left the legacy claim path open: %v", err)
	}
}
