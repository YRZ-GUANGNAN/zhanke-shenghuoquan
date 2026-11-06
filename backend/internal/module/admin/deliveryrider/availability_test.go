package deliveryrider

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"

	"campus-circle/backend/internal/module/delivery"
)

type availabilityRepositoryStub struct {
	snapshot delivery.RiderAvailabilitySnapshot
	config   delivery.RiderBroadcastConfig
	state    delivery.RiderDutyState
	version  uint64
	max      uint32
}

func (s *availabilityRepositoryStub) ReadRiderAvailability(context.Context, uint64) (delivery.RiderAvailabilitySnapshot, error) {
	return s.snapshot, nil
}
func (s *availabilityRepositoryStub) SetRiderDuty(_ context.Context, riderID uint64, state delivery.RiderDutyState, version uint64, _ time.Time) (delivery.RiderAvailabilitySnapshot, error) {
	s.state, s.version = state, version
	value := s.snapshot
	value.RiderUserID, value.State, value.RowVersion = riderID, state, version+1
	value.UpdatedAt = time.Now().UTC()
	return value, nil
}
func (s *availabilityRepositoryStub) ReadRiderBroadcastConfig(context.Context) (delivery.RiderBroadcastConfig, error) {
	return s.config, nil
}
func (s *availabilityRepositoryStub) UpdateRiderBroadcastConfig(_ context.Context, max uint32, version uint64, _ time.Time) (delivery.RiderBroadcastConfig, error) {
	s.max, s.version = max, version
	value := s.config
	value.MaxInFlightPerRider, value.RowVersion = max, version+1
	return value, nil
}
func (s *availabilityRepositoryStub) ListOnDutyRiderAvailability(context.Context, time.Time) ([]delivery.RiderAvailabilitySnapshot, error) {
	return nil, nil
}

func TestAvailabilityRoutesAreExplicitlyDisabledWithoutStore(t *testing.T) {
	gin.SetMode(gin.TestMode)
	handler := NewHandler(nil, nil, nil, func(*gin.Context) uint64 { return 77 })
	router := gin.New()
	handler.Register(router.Group("/rider"))
	handler.RegisterAdmin(router.Group("/admin"))

	get := httptest.NewRecorder()
	router.ServeHTTP(get, httptest.NewRequest(http.MethodGet, "/rider/availability", nil))
	if get.Code != http.StatusOK || !strings.Contains(get.Body.String(), `"enabled":false`) {
		t.Fatalf("disabled GET status=%d body=%s", get.Code, get.Body.String())
	}
	post := httptest.NewRecorder()
	router.ServeHTTP(post, httptest.NewRequest(http.MethodPost, "/rider/availability/start", bytes.NewBufferString(`{"expectedRowVersion":0}`)))
	if post.Code != http.StatusServiceUnavailable {
		t.Fatalf("disabled start status=%d body=%s", post.Code, post.Body.String())
	}
	admin := httptest.NewRecorder()
	router.ServeHTTP(admin, httptest.NewRequest(http.MethodGet, "/admin/rider-config", nil))
	if admin.Code != http.StatusOK || !strings.Contains(admin.Body.String(), `"enabled":false`) {
		t.Fatalf("disabled admin GET status=%d body=%s", admin.Code, admin.Body.String())
	}
}

func TestAvailabilityRoutesPreserveZeroInitialVersionAndConfigCAS(t *testing.T) {
	gin.SetMode(gin.TestMode)
	now := time.Now().UTC()
	repository := &availabilityRepositoryStub{
		snapshot: delivery.RiderAvailabilitySnapshot{RiderUserID: 77, State: delivery.RiderOffDuty,
			Scope: delivery.RiderAvailabilityScopeCampus, MaxInFlight: 3},
		config: delivery.RiderBroadcastConfig{ID: 1, ConfigScope: "global", MaxInFlightPerRider: 3,
			RowVersion: 4, CreatedAt: now, UpdatedAt: now},
	}
	handler := NewHandler(nil, nil, nil, func(*gin.Context) uint64 { return 77 }).WithAvailability(repository)
	router := gin.New()
	handler.Register(router.Group("/rider"))
	handler.RegisterAdmin(router.Group("/admin"))
	initial := httptest.NewRecorder()
	router.ServeHTTP(initial, httptest.NewRequest(http.MethodGet, "/rider/availability", nil))
	if initial.Code != http.StatusOK || !strings.Contains(initial.Body.String(), `"inFlight":0`) ||
		!strings.Contains(initial.Body.String(), `"rowVersion":0`) {
		t.Fatalf("initial zero-value contract status=%d body=%s", initial.Code, initial.Body.String())
	}

	start := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/rider/availability/start", bytes.NewBufferString(`{"expectedRowVersion":0}`))
	request.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(start, request)
	if start.Code != http.StatusOK || repository.state != delivery.RiderOnDuty || repository.version != 0 {
		t.Fatalf("initial start status=%d state=%s version=%d body=%s", start.Code, repository.state, repository.version, start.Body.String())
	}

	patch := httptest.NewRecorder()
	request = httptest.NewRequest(http.MethodPatch, "/admin/rider-config", bytes.NewBufferString(`{"expectedRowVersion":4,"maxInFlightPerRider":6}`))
	request.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(patch, request)
	if patch.Code != http.StatusOK || repository.max != 6 || repository.version != 4 {
		t.Fatalf("config PATCH status=%d max=%d version=%d body=%s", patch.Code, repository.max, repository.version, patch.Body.String())
	}
}
