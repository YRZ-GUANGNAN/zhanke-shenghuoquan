package notification

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
)

type apiTestStore struct {
	mu       sync.Mutex
	items    []InboxItem
	issued   map[string]Subscription
	results  map[string]string
	credits  int
	issueErr error
}

func (s *apiTestStore) ReadSubscriptionAuthorizationState(_ context.Context, userID uint64, appID, templateKey, _ string, _ time.Time) (SubscriptionAuthorizationState, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	state := SubscriptionAuthorizationState{}
	for nonce, item := range s.issued {
		if item.UserID != userID || item.AppID != appID || item.TemplateKey != templateKey || s.results[nonce] != "accepted" {
			continue
		}
		state.RemainingQuota++
	}
	return state, nil
}

func (s *apiTestStore) ListInbox(_ context.Context, userID uint64, before string, limit int) ([]InboxItem, int64, error) {
	items := []InboxItem{}
	var unread int64
	for _, item := range s.items {
		if item.RecipientUserID != userID || (item.Status != DeliveryDelivered && item.Status != DeliveryRead) {
			continue
		}
		if item.ReadAt == nil {
			unread++
		}
		if len(items) < limit && (before == "" || item.InboxPublicID < before) {
			items = append(items, item)
		}
	}
	return items, unread, nil
}

func (s *apiTestStore) MarkInboxRead(_ context.Context, userID uint64, id string, now time.Time) (bool, error) {
	for i := range s.items {
		item := &s.items[i]
		if item.RecipientUserID == userID && item.InboxPublicID == id && (item.Status == DeliveryDelivered || item.Status == DeliveryRead) {
			item.Status, item.ReadAt = DeliveryRead, &now
			return true, nil
		}
	}
	return false, nil
}

func (s *apiTestStore) IssueSubscriptionAuthorization(_ context.Context, item Subscription, notBefore time.Time) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.issueErr != nil {
		return s.issueErr
	}
	for _, previous := range s.issued {
		if previous.UserID == item.UserID && previous.TemplateKey == item.TemplateKey && previous.AppID == item.AppID && previous.CreatedAt.After(notBefore) {
			return ErrAuthorizationRateLimited
		}
	}
	if s.issued == nil {
		s.issued = make(map[string]Subscription)
		s.results = make(map[string]string)
	}
	s.issued[item.AuthorizationNonce] = item
	return nil
}

func (s *apiTestStore) ReportSubscriptionAuthorization(_ context.Context, input AuthorizationReport) (bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	item, ok := s.issued[input.Nonce]
	if !ok || item.UserID != input.UserID || item.AppID != input.AppID || item.OpenID != input.OpenID || item.TemplateKey != input.TemplateKey {
		return false, ErrAuthorizationNonce
	}
	if previous, ok := s.results[input.Nonce]; ok {
		if previous != input.Status {
			return false, ErrAuthorizationConflict
		}
		return true, nil
	}
	if !input.Now.Before(*item.ExpiresAt) {
		return false, ErrAuthorizationNonce
	}
	s.results[input.Nonce] = input.Status
	if input.Status == "accepted" {
		s.credits++
	}
	return false, nil
}

func apiFixture(t *testing.T) (*gin.Engine, *apiTestStore, *SubscriptionService, *uint64) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	store := &apiTestStore{}
	service := NewSubscriptionService(store, func(_ context.Context, id uint64, appID string) (string, error) {
		if appID != "test-app" {
			t.Fatalf("wrong app: %s", appID)
		}
		return fmt.Sprintf("server-openid-%d", id), nil
	}, SubscriptionConfig{Enabled: true, AppID: "test-app", Templates: map[string]string{"order": "public-template", "private_message": ""}})
	now := time.Date(2026, 11, 1, 1, 0, 0, 0, time.UTC)
	service.now = func() time.Time { return now }
	userID := uint64(7)
	router := gin.New()
	NewHandler(store, service, func(*gin.Context) uint64 { return userID }).Register(router.Group("/api/v1"))
	return router, store, service, &userID
}

func apiRequest(router http.Handler, method, path, body string) *httptest.ResponseRecorder {
	w := httptest.NewRecorder()
	r := httptest.NewRequest(method, "/api/v1/notification"+path, strings.NewReader(body))
	r.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, r)
	return w
}

func challengeNonce(t *testing.T, router http.Handler) string {
	t.Helper()
	w := apiRequest(router, http.MethodPost, "/subscribe/nonce", `{"templateKey":"order"}`)
	if w.Code != http.StatusOK {
		t.Fatalf("issue nonce: %d %s", w.Code, w.Body.String())
	}
	var body struct {
		Data AuthorizationChallenge `json:"data"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil || !validAuthorizationNonce(body.Data.Nonce) {
		t.Fatalf("invalid nonce response: %s", w.Body.String())
	}
	return body.Data.Nonce
}

func authorizationBody(nonce, result string) string {
	return fmt.Sprintf(`{"templateKey":"order","result":%q,"nonce":%q}`, result, nonce)
}

func TestNotificationHTTPRequiresAuthenticationForEveryRoute(t *testing.T) {
	router, store, _, userID := apiFixture(t)
	*userID = 0
	for _, route := range []struct{ method, path string }{
		{http.MethodGet, "/inbox"}, {http.MethodPost, "/inbox/01ARZ3NDEKTSV4RRFFQ69G5FAV/read"},
		{http.MethodGet, "/subscribe/templates"}, {http.MethodPost, "/subscribe/nonce"}, {http.MethodPost, "/subscribe/authorize"},
	} {
		w := apiRequest(router, route.method, route.path, `{}`)
		if w.Code != http.StatusUnauthorized || w.Header().Get("Cache-Control") != "no-store" {
			t.Fatalf("%s %s: %d", route.method, route.path, w.Code)
		}
	}
	if len(store.issued) != 0 {
		t.Fatal("unauthenticated request persisted authorization")
	}
}

func TestNotificationTemplatesDoNotIssueNonceOrExposeIdentity(t *testing.T) {
	router, store, _, _ := apiFixture(t)
	w := apiRequest(router, http.MethodGet, "/subscribe/templates", "")
	if w.Code != http.StatusOK || !strings.Contains(w.Body.String(), `"items"`) || !strings.Contains(w.Body.String(), `"templateKey":"order"`) || !strings.Contains(w.Body.String(), `"templateId":"public-template"`) || !strings.Contains(w.Body.String(), `"remainingQuota":0`) || !strings.Contains(w.Body.String(), `"available":false`) {
		t.Fatalf("templates: %d %s", w.Code, w.Body.String())
	}
	if len(store.issued) != 0 || strings.Contains(w.Body.String(), "openid") || strings.Contains(w.Body.String(), "test-app") {
		t.Fatal("template read persisted authorization or exposed identity")
	}
}

func TestNotificationTemplatesReturnCurrentUserQuotaWithoutIdentity(t *testing.T) {
	router, store, _, userID := apiFixture(t)
	nonce := challengeNonce(t, router)
	if w := apiRequest(router, http.MethodPost, "/subscribe/authorize", authorizationBody(nonce, "accept")); w.Code != http.StatusOK {
		t.Fatalf("authorize: %d %s", w.Code, w.Body.String())
	}
	w := apiRequest(router, http.MethodGet, "/subscribe/templates", "")
	if w.Code != http.StatusOK || !strings.Contains(w.Body.String(), `"remainingQuota":1`) || strings.Contains(w.Body.String(), "openid") || strings.Contains(w.Body.String(), "test-app") {
		t.Fatalf("quota: %d %s", w.Code, w.Body.String())
	}
	*userID = 8
	w = apiRequest(router, http.MethodGet, "/subscribe/templates", "")
	if w.Code != http.StatusOK || !strings.Contains(w.Body.String(), `"remainingQuota":0`) || store.credits != 1 {
		t.Fatalf("cross-user quota: %d %s", w.Code, w.Body.String())
	}
}

func TestNotificationAuthorizationIsBoundToUserNonceAndResult(t *testing.T) {
	router, store, _, userID := apiFixture(t)
	nonce := challengeNonce(t, router)
	item := store.issued[nonce]
	if item.RemainingQuota != 0 || item.Status != "invalid" || item.ExpiresAt.Sub(item.CreatedAt) != 5*time.Minute || item.OpenID != "server-openid-7" {
		t.Fatal("nonce is not a zero-quota server-bound placeholder")
	}
	*userID = 8
	w := apiRequest(router, http.MethodPost, "/subscribe/authorize", authorizationBody(nonce, "accept"))
	if w.Code != http.StatusBadRequest || store.credits != 0 {
		t.Fatal("cross-user nonce accepted")
	}
	*userID = 7
	for i := 0; i < 2; i++ {
		w = apiRequest(router, http.MethodPost, "/subscribe/authorize", authorizationBody(nonce, "accept"))
		if w.Code != http.StatusOK || strings.Contains(w.Body.String(), "openid") || !strings.Contains(w.Body.String(), fmt.Sprintf(`"duplicate":%t`, i == 1)) {
			t.Fatalf("authorization attempt %d: %d %s", i, w.Code, w.Body.String())
		}
	}
	if store.credits != 1 {
		t.Fatal("replay granted more than one quota")
	}
	w = apiRequest(router, http.MethodPost, "/subscribe/authorize", authorizationBody(nonce, "reject"))
	if w.Code != http.StatusConflict || store.credits != 1 {
		t.Fatal("conflicting replay changed quota")
	}
}

func TestNotificationNonceRateLimitExpiryAndTemplateBinding(t *testing.T) {
	router, store, service, _ := apiFixture(t)
	nonce := challengeNonce(t, router)
	w := apiRequest(router, http.MethodPost, "/subscribe/nonce", `{"templateKey":"order"}`)
	if w.Code != http.StatusTooManyRequests || len(store.issued) != 1 {
		t.Fatal("nonce issue did not enforce user/template rate limit")
	}
	service.config.Templates["other"] = "public-template-other"
	w = apiRequest(router, http.MethodPost, "/subscribe/authorize", strings.Replace(authorizationBody(nonce, "accept"), `"order"`, `"other"`, 1))
	if w.Code != http.StatusBadRequest {
		t.Fatal("nonce accepted for another template")
	}
	service.now = func() time.Time { return *store.issued[nonce].ExpiresAt }
	w = apiRequest(router, http.MethodPost, "/subscribe/authorize", authorizationBody(nonce, "accept"))
	if w.Code != http.StatusBadRequest || store.credits != 0 {
		t.Fatal("expired nonce granted quota")
	}
}

func TestNotificationConcurrentAuthorizationReportsGrantOneQuota(t *testing.T) {
	router, store, _, _ := apiFixture(t)
	nonce := challengeNonce(t, router)
	var wg sync.WaitGroup
	statuses := make(chan int, 12)
	for i := 0; i < 12; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			statuses <- apiRequest(router, http.MethodPost, "/subscribe/authorize", authorizationBody(nonce, "accept")).Code
		}()
	}
	wg.Wait()
	close(statuses)
	for code := range statuses {
		if code != http.StatusOK {
			t.Fatalf("concurrent idempotent report returned %d", code)
		}
	}
	if store.credits != 1 || len(store.results) != 1 {
		t.Fatal("concurrent reporting granted multiple quotas")
	}
}

func TestNotificationTemplateRemappingInvalidatesOutstandingNonce(t *testing.T) {
	router, store, service, _ := apiFixture(t)
	nonce := challengeNonce(t, router)
	service.config.Templates["order"] = "replacement-template"
	w := apiRequest(router, http.MethodPost, "/subscribe/authorize", authorizationBody(nonce, "accept"))
	if w.Code != http.StatusBadRequest || store.credits != 0 {
		t.Fatal("old template authorization credited to replacement template")
	}
}

func TestNotificationUnavailableTemplateOrOpenIDCannotPersistAuthorization(t *testing.T) {
	for _, scenario := range []string{"disabled", "empty_template", "missing_openid", "lookup_error"} {
		t.Run(scenario, func(t *testing.T) {
			router, store, service, _ := apiFixture(t)
			switch scenario {
			case "disabled":
				service.config.Enabled = false
			case "empty_template":
				service.config.Templates["order"] = ""
			case "missing_openid":
				service.lookup = func(context.Context, uint64, string) (string, error) { return "", nil }
			case "lookup_error":
				service.lookup = func(context.Context, uint64, string) (string, error) { return "", errors.New("SECRET_OPENID") }
			}
			for _, path := range []string{"/subscribe/nonce", "/subscribe/authorize"} {
				body := `{"templateKey":"order"}`
				if strings.HasSuffix(path, "authorize") {
					body = authorizationBody(strings.Repeat("a", 64), "accept")
				}
				w := apiRequest(router, http.MethodPost, path, body)
				if w.Code != http.StatusServiceUnavailable || strings.Contains(w.Body.String(), "SECRET_OPENID") {
					t.Fatalf("%s: %d %s", path, w.Code, w.Body.String())
				}
			}
			if len(store.issued) != 0 || store.credits != 0 {
				t.Fatal("unavailable subscription was persisted")
			}
		})
	}
}

func TestNotificationRejectAndExceptionNeverGrantQuota(t *testing.T) {
	for _, result := range []string{"reject", "ban", "close", "exception"} {
		t.Run(result, func(t *testing.T) {
			router, store, _, _ := apiFixture(t)
			nonce := challengeNonce(t, router)
			w := apiRequest(router, http.MethodPost, "/subscribe/authorize", authorizationBody(nonce, result))
			if w.Code != http.StatusOK || store.credits != 0 {
				t.Fatalf("%s granted quota", result)
			}
		})
	}
}

func TestNotificationAuthorizationRejectsClientIdentityAndMalformedJSON(t *testing.T) {
	router, store, _, _ := apiFixture(t)
	secretField := "App" + "Secret"
	for _, body := range []string{
		`{"templateKey":"order","openid":"client"}`, fmt.Sprintf(`{"templateKey":"order","%s":"client"}`, secretField),
		`{"templateKey":"order","userId":99}`, `{"templateKey":"order"} {}`, `null`, strings.Repeat("x", 4097),
	} {
		w := apiRequest(router, http.MethodPost, "/subscribe/nonce", body)
		if w.Code != http.StatusBadRequest {
			t.Fatalf("unexpected body accepted: %d", w.Code)
		}
	}
	if len(store.issued) != 0 {
		t.Fatal("malformed request persisted authorization")
	}
}

func TestNotificationInboxIsolationReadAndPagination(t *testing.T) {
	router, store, _, userID := apiFixture(t)
	const owned = "01ARZ3NDEKTSV4RRFFQ69G5FAV"
	const other = "01ARZ3NDEKTSV4RRFFQ69G5FAW"
	store.items = []InboxItem{{InboxPublicID: owned, RecipientUserID: 7, Status: DeliveryDelivered, Purpose: PurposeOrder, Title: "订单提醒"},
		{InboxPublicID: other, RecipientUserID: 8, Status: DeliveryDelivered, Title: "另一用户的通知"}}
	w := apiRequest(router, http.MethodGet, "/inbox?limit=1", "")
	if w.Code != http.StatusOK || !strings.Contains(w.Body.String(), `"read":false`) || strings.Contains(w.Body.String(), "另一用户") || !strings.Contains(w.Body.String(), `"unreadCount":1`) {
		t.Fatalf("inbox isolation: %d %s", w.Code, w.Body.String())
	}
	for i := 0; i < 2; i++ {
		w = apiRequest(router, http.MethodPost, "/inbox/"+owned+"/read", "")
		if w.Code != http.StatusOK {
			t.Fatal("mark-read must be idempotent")
		}
	}
	w = apiRequest(router, http.MethodPost, "/inbox/"+other+"/read", "")
	if w.Code != http.StatusNotFound {
		t.Fatal("cross-user mark-read accepted")
	}
	*userID = 8
	w = apiRequest(router, http.MethodPost, "/inbox/"+owned+"/read", "")
	if w.Code != http.StatusNotFound {
		t.Fatal("account switch retained previous identity")
	}
	for _, path := range []string{"/inbox?limit=0", "/inbox?limit=101", "/inbox?cursor=invalid", "/inbox?limit=abc"} {
		if apiRequest(router, http.MethodGet, path, "").Code != http.StatusBadRequest {
			t.Fatal("invalid pagination accepted")
		}
	}
}

func TestNotificationNilDependenciesFailClosed(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	NewHandler(nil, nil, nil).Register(router.Group("/api/v1"))
	if apiRequest(router, http.MethodGet, "/subscribe/templates", "").Code != http.StatusUnauthorized {
		t.Fatal("missing authentication resolver granted access")
	}
	service := NewSubscriptionService(nil, nil, SubscriptionConfig{})
	if _, err := service.IssueNonce(context.Background(), 7, "order"); !errors.Is(err, ErrSubscriptionEmpty) || len(service.Templates()) != 0 {
		t.Fatal("empty configuration enabled subscriptions")
	}
	var repository *GormRepository
	if _, _, err := repository.ListInbox(context.Background(), 7, "", 20); err == nil {
		t.Fatal("nil DB accepted inbox query")
	}
}
