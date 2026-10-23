package deliveryrider

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"campus-circle/backend/internal/module/delivery"
	"campus-circle/backend/internal/module/takeout"
	"github.com/gin-gonic/gin"
)

type todayStatsIdentityStub struct {
	rider takeout.Rider
	err   error
}

func (s todayStatsIdentityStub) Find(context.Context, uint64) (takeout.Rider, error) {
	return s.rider, s.err
}

func (s todayStatsIdentityStub) RegisterApproved(context.Context, uint64, string, string) (takeout.Rider, error) {
	return s.rider, nil
}

type todayStatsReaderStub struct {
	value   delivery.RiderTodayStats
	err     error
	riderID uint64
	start   time.Time
	end     time.Time
}

func (s *todayStatsReaderStub) ReadRiderTodayStats(_ context.Context, riderID uint64, start, end time.Time) (delivery.RiderTodayStats, error) {
	s.riderID, s.start, s.end = riderID, start, end
	return s.value, s.err
}

func TestTodayStatsUsesShanghaiBusinessDayAndReturnsIncome(t *testing.T) {
	gin.SetMode(gin.TestMode)
	reader := &todayStatsReaderStub{value: delivery.RiderTodayStats{TodayOrders: 3, TodayIncomeCents: 900}}
	identity := todayStatsIdentityStub{rider: takeout.Rider{RiderUserID: 77, Status: takeout.RiderActive}}
	handler := newTodayStatsHandler(identity).
		WithTodayStats(reader).
		WithNow(func() time.Time { return time.Date(2026, 10, 19, 0, 30, 0, 0, time.UTC) })
	router := gin.New()
	handler.Register(router.Group("/rider"))
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/rider/stats/today", nil))
	if recorder.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", recorder.Code, recorder.Body.String())
	}
	wantStart := time.Date(2026, 10, 18, 16, 0, 0, 0, time.UTC)
	wantEnd := time.Date(2026, 10, 19, 16, 0, 0, 0, time.UTC)
	if reader.riderID != 77 || !reader.start.Equal(wantStart) || !reader.end.Equal(wantEnd) {
		t.Fatalf("window rider=%d start=%s end=%s", reader.riderID, reader.start, reader.end)
	}
	body := recorder.Body.String()
	for _, fragment := range []string{`"date":"2026-10-19"`, `"timezone":"Asia/Shanghai"`, `"todayOrders":3`, `"todayIncomeCents":900`, `"definition"`} {
		if !strings.Contains(body, fragment) {
			t.Fatalf("response missing %s: %s", fragment, body)
		}
	}
}

func TestTodayStatsAllowsRestingRiderButRejectsSuspended(t *testing.T) {
	gin.SetMode(gin.TestMode)
	reader := &todayStatsReaderStub{}
	routerFor := func(status takeout.RiderStatus) *gin.Engine {
		handler := newTodayStatsHandler(todayStatsIdentityStub{rider: takeout.Rider{RiderUserID: 77, Status: status}}).WithTodayStats(reader)
		router := gin.New()
		handler.Register(router.Group("/rider"))
		return router
	}
	resting := httptest.NewRecorder()
	routerFor(takeout.RiderResting).ServeHTTP(resting, httptest.NewRequest(http.MethodGet, "/rider/stats/today", nil))
	if resting.Code != http.StatusOK {
		t.Fatalf("resting status=%d body=%s", resting.Code, resting.Body.String())
	}
	suspended := httptest.NewRecorder()
	routerFor(takeout.RiderSuspended).ServeHTTP(suspended, httptest.NewRequest(http.MethodGet, "/rider/stats/today", nil))
	if suspended.Code != http.StatusForbidden {
		t.Fatalf("suspended status=%d body=%s", suspended.Code, suspended.Body.String())
	}
}

func TestTodayStatsDoesNotLeakIdentityAndPropagatesReaderError(t *testing.T) {
	gin.SetMode(gin.TestMode)
	reader := &todayStatsReaderStub{err: errors.New("database unavailable")}
	handler := newTodayStatsHandler(todayStatsIdentityStub{rider: takeout.Rider{RiderUserID: 77, Status: takeout.RiderActive}}).WithTodayStats(reader)
	router := gin.New()
	handler.Register(router.Group("/rider"))
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/rider/stats/today", nil))
	if recorder.Code != http.StatusInternalServerError {
		t.Fatalf("reader error status=%d body=%s", recorder.Code, recorder.Body.String())
	}
	if reader.riderID != 77 {
		t.Fatalf("reader saw unexpected rider id=%d", reader.riderID)
	}
}

func TestTodayStatsReturnsServiceUnavailableWhenReaderMissing(t *testing.T) {
	gin.SetMode(gin.TestMode)
	handler := newTodayStatsHandler(todayStatsIdentityStub{rider: takeout.Rider{RiderUserID: 77, Status: takeout.RiderActive}})
	router := gin.New()
	handler.Register(router.Group("/rider"))
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/rider/stats/today", nil))
	if recorder.Code != http.StatusServiceUnavailable {
		t.Fatalf("missing reader status=%d body=%s", recorder.Code, recorder.Body.String())
	}
}

func newTodayStatsHandler(identity riderIdentity) *Handler {
	handler := NewHandler(nil, nil, nil, func(*gin.Context) uint64 { return 77 })
	handler.identity = identity
	return handler
}

func TestTodayStatsReturnsServiceUnavailableWhenIdentityMissing(t *testing.T) {
	gin.SetMode(gin.TestMode)
	var identity *IdentityAdapter
	handler := NewHandler(nil, identity, nil, func(*gin.Context) uint64 { return 77 }).WithTodayStats(&todayStatsReaderStub{})
	router := gin.New()
	handler.Register(router.Group("/rider"))
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/rider/stats/today", nil))
	if recorder.Code != http.StatusServiceUnavailable {
		t.Fatalf("missing identity status=%d body=%s", recorder.Code, recorder.Body.String())
	}
}
