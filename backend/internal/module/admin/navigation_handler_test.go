package admin

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sort"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"

	"campus-circle/backend/internal/module/navigation"
	"campus-circle/backend/internal/module/user"
)

func TestExternalTakeoutNavigationAdmin(t *testing.T) {
	t.Run("all dedicated routes require admin", testExternalTakeoutNavigationPermissions)
	t.Run("lifecycle keeps status fields isolated", testExternalTakeoutNavigationLifecycle)
	t.Run("strict request bodies fail closed", testExternalTakeoutNavigationStrictBodies)
	t.Run("generic guards and errors are safely mapped", testExternalTakeoutNavigationGuardsAndErrors)
}

func testExternalTakeoutNavigationPermissions(t *testing.T) {
	fixture := newAdminNavigationFixture(t)
	validMerchant := string(mustJSON(t, validAdminMerchantPayload()))
	routes := []struct {
		method string
		path   string
		body   string
	}{
		{method: http.MethodGet, path: "/api/v1/admin/navigation/external-takeout-merchants"},
		{method: http.MethodPost, path: "/api/v1/admin/navigation/external-takeout-merchants", body: validMerchant},
		{method: http.MethodPatch, path: "/api/v1/admin/navigation/external-takeout-merchants/category-visibility", body: `{"enabled":false}`},
		{method: http.MethodPut, path: "/api/v1/admin/navigation/external-takeout-merchants/1", body: validMerchant},
		{method: http.MethodDelete, path: "/api/v1/admin/navigation/external-takeout-merchants/1"},
		{method: http.MethodPatch, path: "/api/v1/admin/navigation/external-takeout-merchants/1/operating-status", body: `{"operatingStatus":"resting"}`},
		{method: http.MethodPatch, path: "/api/v1/admin/navigation/external-takeout-merchants/1/visibility", body: `{"enabled":false}`},
	}
	for _, route := range routes {
		name := route.method + " " + route.path
		t.Run(name+" unauthorized", func(t *testing.T) {
			response := fixture.perform(t, route.method, route.path, "", route.body)
			if response.Code != http.StatusUnauthorized {
				t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
			}
		})
		t.Run(name+" ordinary forbidden", func(t *testing.T) {
			response := fixture.perform(t, route.method, route.path, fixture.tokens["ordinary"], route.body)
			if response.Code != http.StatusForbidden {
				t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
			}
		})
	}
	for _, role := range []string{"admin", "superadmin"} {
		response := fixture.perform(t, http.MethodGet,
			"/api/v1/admin/navigation/external-takeout-merchants", fixture.tokens[role], "")
		if response.Code != http.StatusOK {
			t.Fatalf("%s GET status=%d body=%s", role, response.Code, response.Body.String())
		}
	}
	if len(fixture.navigation.items) != 0 || !fixture.navigation.categories[fixture.systemCategoryID].Enabled {
		t.Fatalf("permission checks mutated navigation state: items=%+v category=%+v",
			fixture.navigation.items, fixture.navigation.categories[fixture.systemCategoryID])
	}
}

func testExternalTakeoutNavigationLifecycle(t *testing.T) {
	fixture := newAdminNavigationFixture(t)
	base := "/api/v1/admin/navigation/external-takeout-merchants"
	createResponse := fixture.perform(t, http.MethodPost, base, fixture.tokens["admin"], string(mustJSON(t, validAdminMerchantPayload())))
	if createResponse.Code != http.StatusCreated {
		t.Fatalf("create status=%d body=%s", createResponse.Code, createResponse.Body.String())
	}
	created := decodeAdminNavigationData[navigation.ExternalTakeoutMerchant](t, createResponse)
	if created.ID == 0 || !created.Enabled || created.OperatingStatus != navigation.OperatingStatusOpen {
		t.Fatalf("unexpected created merchant: %+v", created)
	}

	statusURL := fmt.Sprintf("%s/%d/operating-status", base, created.ID)
	response := fixture.perform(t, http.MethodPatch, statusURL, fixture.tokens["admin"], `{"operatingStatus":"resting"}`)
	if response.Code != http.StatusOK {
		t.Fatalf("status patch=%d body=%s", response.Code, response.Body.String())
	}
	stored := fixture.navigation.items[created.ID]
	if !stored.Enabled || stored.OperatingStatus != navigation.OperatingStatusResting {
		t.Fatalf("status patch changed visibility: %+v", stored)
	}

	visibilityURL := fmt.Sprintf("%s/%d/visibility", base, created.ID)
	response = fixture.perform(t, http.MethodPatch, visibilityURL, fixture.tokens["admin"], `{"enabled":false}`)
	if response.Code != http.StatusOK {
		t.Fatalf("visibility patch=%d body=%s", response.Code, response.Body.String())
	}
	stored = fixture.navigation.items[created.ID]
	if stored.Enabled || stored.OperatingStatus != navigation.OperatingStatusResting {
		t.Fatalf("visibility patch changed operating status: %+v", stored)
	}

	updatePayload := validAdminMerchantPayload()
	updatePayload["title"] = "更新后的餐厅"
	updatePayload["subtitle"] = "新副标题"
	updatePayload["sort"] = 0
	response = fixture.perform(t, http.MethodPut, fmt.Sprintf("%s/%d", base, created.ID), fixture.tokens["admin"], string(mustJSON(t, updatePayload)))
	if response.Code != http.StatusOK {
		t.Fatalf("update status=%d body=%s", response.Code, response.Body.String())
	}
	updated := decodeAdminNavigationData[navigation.ExternalTakeoutMerchant](t, response)
	if updated.Title != "更新后的餐厅" || updated.Sort != 0 || updated.Enabled ||
		updated.OperatingStatus != navigation.OperatingStatusResting {
		t.Fatalf("profile update changed state: %+v", updated)
	}

	response = fixture.perform(t, http.MethodPatch, visibilityURL, fixture.tokens["admin"], `{"enabled":true}`)
	if response.Code != http.StatusOK || !fixture.navigation.items[created.ID].Enabled ||
		fixture.navigation.items[created.ID].OperatingStatus != navigation.OperatingStatusResting {
		t.Fatalf("restore visibility failed: status=%d item=%+v", response.Code, fixture.navigation.items[created.ID])
	}
	response = fixture.perform(t, http.MethodPatch, statusURL, fixture.tokens["admin"], `{"operatingStatus":"open"}`)
	if response.Code != http.StatusOK || !fixture.navigation.items[created.ID].Enabled ||
		fixture.navigation.items[created.ID].OperatingStatus != navigation.OperatingStatusOpen {
		t.Fatalf("restore status failed: status=%d item=%+v", response.Code, fixture.navigation.items[created.ID])
	}

	categoryURL := base + "/category-visibility"
	response = fixture.perform(t, http.MethodPatch, categoryURL, fixture.tokens["admin"], `{"enabled":false}`)
	if response.Code != http.StatusOK || fixture.navigation.categories[fixture.systemCategoryID].Enabled {
		t.Fatalf("category visibility status=%d category=%+v body=%s", response.Code,
			fixture.navigation.categories[fixture.systemCategoryID], response.Body.String())
	}
	response = fixture.perform(t, http.MethodGet, base, fixture.tokens["admin"], "")
	if response.Code != http.StatusOK {
		t.Fatalf("directory GET status=%d body=%s", response.Code, response.Body.String())
	}
	directory := decodeAdminNavigationData[navigation.ExternalTakeoutDirectory](t, response)
	if directory.CategoryEnabled || len(directory.Items) != 1 || directory.Items[0].ID != created.ID {
		t.Fatalf("disabled directory response: %+v", directory)
	}

	response = fixture.perform(t, http.MethodDelete, fmt.Sprintf("%s/%d", base, created.ID), fixture.tokens["admin"], "")
	if response.Code != http.StatusOK {
		t.Fatalf("delete status=%d body=%s", response.Code, response.Body.String())
	}
	if _, exists := fixture.navigation.items[created.ID]; exists {
		t.Fatal("merchant was not deleted")
	}
}

func testExternalTakeoutNavigationStrictBodies(t *testing.T) {
	fixture := newAdminNavigationFixture(t)
	base := "/api/v1/admin/navigation/external-takeout-merchants"
	unknownFields := map[string]any{
		"id": uint(1), "categoryId": uint(1), "enabled": true,
		"operatingStatus": "resting", "systemKey": navigation.SystemKeyExternalTakeoutMerchants,
		"createdAt": "2026-07-17T00:00:00Z", "updatedAt": "2026-07-17T00:00:00Z",
		"items": []any{}, "extraData": map[string]any{"secret": true},
	}
	for field, value := range unknownFields {
		t.Run("unknown "+field, func(t *testing.T) {
			payload := validAdminMerchantPayload()
			payload[field] = value
			response := fixture.perform(t, http.MethodPost, base, fixture.tokens["admin"], string(mustJSON(t, payload)))
			if response.Code != http.StatusBadRequest {
				t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
			}
		})
	}
	if len(fixture.navigation.items) != 0 {
		t.Fatalf("unknown-field requests mutated repository: %+v", fixture.navigation.items)
	}

	valid := string(mustJSON(t, validAdminMerchantPayload()))
	invalidBodies := map[string]string{
		"empty":           "",
		"multiple":        valid + valid,
		"array":           "[]",
		"null":            "null",
		"oversized":       `{"title":"` + strings.Repeat("x", 33*1024) + `","subtitle":"","iconText":"餐","appId":"wx1234567890ABCDEF","path":"pages/store/index","color":"#12AB34","sort":1}`,
		"trailing object": valid + ` {"enabled":false}`,
	}
	for name, body := range invalidBodies {
		t.Run(name, func(t *testing.T) {
			response := fixture.perform(t, http.MethodPost, base, fixture.tokens["admin"], body)
			if response.Code != http.StatusBadRequest {
				t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
			}
		})
	}
	if len(fixture.navigation.items) != 0 {
		t.Fatalf("invalid bodies mutated repository: %+v", fixture.navigation.items)
	}
	if calls := fixture.navigation.totalCalls(); calls != 0 {
		t.Fatalf("invalid request bodies reached repository %d times: %+v", calls, fixture.navigation.calls)
	}

	createResponse := fixture.perform(t, http.MethodPost, base, fixture.tokens["admin"], valid)
	if createResponse.Code != http.StatusCreated {
		t.Fatalf("seed merchant status=%d body=%s", createResponse.Code, createResponse.Body.String())
	}
	created := decodeAdminNavigationData[navigation.ExternalTakeoutMerchant](t, createResponse)
	original := fixture.navigation.items[created.ID]
	callsBeforeStrictUpdates := fixture.navigation.totalCalls()
	for field, value := range unknownFields {
		t.Run("update unknown "+field, func(t *testing.T) {
			payload := validAdminMerchantPayload()
			payload[field] = value
			response := fixture.perform(t, http.MethodPut, fmt.Sprintf("%s/%d", base, created.ID),
				fixture.tokens["admin"], string(mustJSON(t, payload)))
			if response.Code != http.StatusBadRequest {
				t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
			}
		})
	}

	strictPatchRequests := []struct {
		name string
		path string
		body string
	}{
		{name: "visibility missing", path: fmt.Sprintf("%s/%d/visibility", base, created.ID), body: `{}`},
		{name: "visibility null", path: fmt.Sprintf("%s/%d/visibility", base, created.ID), body: `{"enabled":null}`},
		{name: "visibility cross field", path: fmt.Sprintf("%s/%d/visibility", base, created.ID), body: `{"enabled":false,"operatingStatus":"resting"}`},
		{name: "status missing", path: fmt.Sprintf("%s/%d/operating-status", base, created.ID), body: `{}`},
		{name: "status null", path: fmt.Sprintf("%s/%d/operating-status", base, created.ID), body: `{"operatingStatus":null}`},
		{name: "status cross field", path: fmt.Sprintf("%s/%d/operating-status", base, created.ID), body: `{"operatingStatus":"resting","enabled":false}`},
		{name: "category missing", path: base + "/category-visibility", body: `{}`},
		{name: "category null", path: base + "/category-visibility", body: `{"enabled":null}`},
		{name: "category extra data", path: base + "/category-visibility", body: `{"enabled":false,"extraData":{}}`},
	}
	for _, request := range strictPatchRequests {
		t.Run(request.name, func(t *testing.T) {
			response := fixture.perform(t, http.MethodPatch, request.path, fixture.tokens["admin"], request.body)
			if response.Code != http.StatusBadRequest {
				t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
			}
		})
	}
	for _, id := range []string{"0", "not-a-number"} {
		response := fixture.perform(t, http.MethodDelete, base+"/"+id, fixture.tokens["admin"], "")
		if response.Code != http.StatusBadRequest {
			t.Fatalf("invalid id %q status=%d body=%s", id, response.Code, response.Body.String())
		}
	}
	if len(fixture.navigation.items) != 1 || fixture.navigation.items[created.ID] != original ||
		!fixture.navigation.categories[fixture.systemCategoryID].Enabled {
		t.Fatalf("strict requests mutated state: items=%+v category=%+v",
			fixture.navigation.items, fixture.navigation.categories[fixture.systemCategoryID])
	}
	if calls := fixture.navigation.totalCalls(); calls != callsBeforeStrictUpdates {
		t.Fatalf("strict update requests reached repository: before=%d after=%d calls=%+v",
			callsBeforeStrictUpdates, calls, fixture.navigation.calls)
	}
}

func testExternalTakeoutNavigationGuardsAndErrors(t *testing.T) {
	fixture := newAdminNavigationFixture(t)
	adminToken := fixture.tokens["admin"]
	systemID := fixture.systemCategoryID
	ordinaryID := fixture.ordinaryCategoryID
	base := "/api/v1/admin/navigation/external-takeout-merchants"

	systemItem := fixture.navigation.seedItem(navigation.Item{
		CategoryID: systemID, Title: "系统商家", Subtitle: "", IconText: "餐",
		AppID: "wx1234567890ABCDE0", Path: "pages/system/index", Color: "#112233",
		Sort: 1, Enabled: true, OperatingStatus: navigation.OperatingStatusOpen,
	})
	ordinaryItem := fixture.navigation.seedItem(navigation.Item{
		CategoryID: ordinaryID, Title: "ordinary entry", Subtitle: "", IconText: "shop",
		AppID: "wx1234567890ABCDE1", Path: "pages/ordinary/index", Color: "#223344",
		Sort: 2, Enabled: true, OperatingStatus: navigation.OperatingStatusOpen,
	})
	guardRequests := []struct {
		method string
		path   string
		body   string
	}{
		{method: http.MethodPut, path: fmt.Sprintf("/api/v1/admin/navigation/categories/%d", systemID), body: `{"name":"绕行改名","sort":1,"enabled":true}`},
		{method: http.MethodDelete, path: fmt.Sprintf("/api/v1/admin/navigation/categories/%d", systemID)},
		{method: http.MethodPost, path: "/api/v1/admin/navigation/items", body: fmt.Sprintf(`{"categoryId":%d,"title":"绕行创建","appId":"legacy","path":"legacy/path"}`, systemID)},
		{method: http.MethodPut, path: fmt.Sprintf("/api/v1/admin/navigation/items/%d", systemItem.ID), body: fmt.Sprintf(`{"categoryId":%d,"title":"绕行移出","appId":"legacy","path":"legacy/path"}`, ordinaryID)},
		{method: http.MethodPut, path: fmt.Sprintf("/api/v1/admin/navigation/items/%d", ordinaryItem.ID), body: fmt.Sprintf(`{"categoryId":%d,"title":"move into system","appId":"legacy","path":"legacy/path"}`, systemID)},
		{method: http.MethodDelete, path: fmt.Sprintf("/api/v1/admin/navigation/items/%d", systemItem.ID)},
	}
	for _, request := range guardRequests {
		response := fixture.perform(t, request.method, request.path, adminToken, request.body)
		if response.Code != http.StatusConflict {
			t.Fatalf("%s %s status=%d body=%s", request.method, request.path, response.Code, response.Body.String())
		}
	}

	unknown := errors.New("INSERT failed appId=wxSECRET path=pages/secret SELECT password")
	fixture.navigation.failures["create_item"] = unknown
	response := fixture.perform(t, http.MethodPost, base, adminToken, string(mustJSON(t, validAdminMerchantPayload())))
	assertSafeNavigationFailure(t, response, http.StatusInternalServerError)
	delete(fixture.navigation.failures, "create_item")

	fixture.navigation.failures["create_item"] = unknown
	genericBody := fmt.Sprintf(`{"categoryId":%d,"title":"普通入口","appId":"legacy-app","path":"legacy/path"}`, ordinaryID)
	response = fixture.perform(t, http.MethodPost, "/api/v1/admin/navigation/items", adminToken, genericBody)
	assertSafeNavigationFailure(t, response, http.StatusInternalServerError)
	delete(fixture.navigation.failures, "create_item")

	fixture.navigation.failures["create_category"] = unknown
	response = fixture.perform(t, http.MethodPost, "/api/v1/admin/navigation/categories", adminToken,
		`{"name":"ordinary category","sort":50,"enabled":true}`)
	assertSafeNavigationFailure(t, response, http.StatusInternalServerError)
	delete(fixture.navigation.failures, "create_category")

	fixture.navigation.failures["tree"] = unknown
	response = fixture.perform(t, http.MethodGet, "/api/v1/admin/navigation", adminToken, "")
	assertSafeNavigationFailure(t, response, http.StatusInternalServerError)
	delete(fixture.navigation.failures, "tree")

	response = fixture.perform(t, http.MethodPost, "/api/v1/admin/navigation/categories", adminToken,
		fmt.Sprintf(`{"name":"forged system category","systemKey":%q,"sort":51,"enabled":true}`,
			navigation.SystemKeyExternalTakeoutMerchants))
	if response.Code != http.StatusCreated {
		t.Fatalf("forged category status=%d body=%s", response.Code, response.Body.String())
	}
	createdCategory := decodeAdminNavigationData[navigation.Category](t, response)
	if createdCategory.SystemKey != nil || fixture.navigation.categories[createdCategory.ID].SystemKey != nil {
		t.Fatalf("client forged system category: response=%+v stored=%+v", createdCategory,
			fixture.navigation.categories[createdCategory.ID])
	}

	response = fixture.perform(t, http.MethodDelete, base+"/999999", adminToken, "")
	if response.Code != http.StatusNotFound {
		t.Fatalf("missing merchant status=%d body=%s", response.Code, response.Body.String())
	}
	invalidPayload := validAdminMerchantPayload()
	invalidPayload["appId"] = "invalid-appid"
	response = fixture.perform(t, http.MethodPost, base, adminToken, string(mustJSON(t, invalidPayload)))
	if response.Code != http.StatusBadRequest || !strings.Contains(response.Body.String(), "invalid external takeout merchant appid") {
		t.Fatalf("invalid detail status=%d body=%s", response.Code, response.Body.String())
	}
	fixture.navigation.failures["create_item"] = navigation.ErrDuplicateTarget
	response = fixture.perform(t, http.MethodPost, base, adminToken, string(mustJSON(t, validAdminMerchantPayload())))
	if response.Code != http.StatusConflict {
		t.Fatalf("duplicate status=%d body=%s", response.Code, response.Body.String())
	}
}

func assertSafeNavigationFailure(t *testing.T, response *httptest.ResponseRecorder, status int) {
	t.Helper()
	if response.Code != status {
		t.Fatalf("status=%d want=%d body=%s", response.Code, status, response.Body.String())
	}
	body := strings.ToLower(response.Body.String())
	for _, secret := range []string{"select", "insert", "password", "wxsecret", "pages/secret"} {
		if strings.Contains(body, secret) {
			t.Fatalf("response leaked %q: %s", secret, response.Body.String())
		}
	}
	if !strings.Contains(response.Body.String(), "navigation operation failed") {
		t.Fatalf("unexpected fixed error body: %s", response.Body.String())
	}
}

func validAdminMerchantPayload() map[string]any {
	return map[string]any{
		"title": "校园餐厅", "subtitle": "校内配送", "iconText": "餐",
		"appId": "wx1234567890ABCDEF", "path": "pages/store/index", "color": "#12AB34", "sort": 10,
	}
}

type adminNavigationEnvelope struct {
	Code    int             `json:"code"`
	Message string          `json:"message"`
	Data    json.RawMessage `json:"data"`
}

func decodeAdminNavigationData[T any](t *testing.T, response *httptest.ResponseRecorder) T {
	t.Helper()
	var envelope adminNavigationEnvelope
	if err := json.Unmarshal(response.Body.Bytes(), &envelope); err != nil {
		t.Fatalf("decode response envelope: %v; body=%s", err, response.Body.String())
	}
	var value T
	if err := json.Unmarshal(envelope.Data, &value); err != nil {
		t.Fatalf("decode response data: %v; data=%s", err, string(envelope.Data))
	}
	return value
}

func mustJSON(t *testing.T, value any) []byte {
	t.Helper()
	encoded, err := json.Marshal(value)
	if err != nil {
		t.Fatalf("marshal JSON: %v", err)
	}
	return encoded
}

type adminNavigationFixture struct {
	router             *gin.Engine
	tokens             map[string]string
	navigation         *adminNavigationRepository
	systemCategoryID   uint
	ordinaryCategoryID uint
}

func newAdminNavigationFixture(t *testing.T) *adminNavigationFixture {
	t.Helper()
	gin.SetMode(gin.TestMode)
	userRepository := newAdminNavigationUserRepository(
		user.User{ID: 1, OpenID: "mock_openid_ordinary", Nickname: "ordinary", Role: user.RoleUser, Status: user.StatusActive, TokenVersion: 1},
		user.User{ID: 2, OpenID: "mock_openid_admin", Nickname: "admin", Role: user.RoleAdmin, Status: user.StatusActive, TokenVersion: 1},
		user.User{ID: 3, OpenID: "mock_openid_superadmin", Nickname: "superadmin", Role: user.RoleSuperAdmin, Status: user.StatusActive, TokenVersion: 1},
	)
	users := user.NewService(userRepository, "task5-secret-that-is-longer-than-thirty-two-characters", "", "", true)
	tokens := make(map[string]string, 3)
	for _, code := range []string{"ordinary", "admin", "superadmin"} {
		result, err := users.WechatLogin(context.Background(), user.LoginInput{Code: code})
		if err != nil {
			t.Fatalf("login %s: %v", code, err)
		}
		tokens[code] = result.Token
	}

	navigationRepository := newAdminNavigationRepository()
	systemKey := navigation.SystemKeyExternalTakeoutMerchants
	systemCategory := navigationRepository.seedCategory(navigation.Category{
		Name: "外卖商家", SystemKey: &systemKey, Sort: 20, Enabled: true,
	})
	ordinaryCategory := navigationRepository.seedCategory(navigation.Category{Name: "普通导航", Sort: 30, Enabled: true})
	navigationService := navigation.NewService(navigationRepository)
	router := gin.New()
	NewHandler(nil, nil, users, nil, nil, navigationService).Register(router.Group("/api/v1"))
	return &adminNavigationFixture{
		router: router, tokens: tokens, navigation: navigationRepository,
		systemCategoryID: systemCategory.ID, ordinaryCategoryID: ordinaryCategory.ID,
	}
}

func (f *adminNavigationFixture) perform(t *testing.T, method, path, token, body string) *httptest.ResponseRecorder {
	t.Helper()
	var request *http.Request
	if body == "" {
		request = httptest.NewRequest(method, path, nil)
	} else {
		request = httptest.NewRequest(method, path, strings.NewReader(body))
		request.Header.Set("Content-Type", "application/json")
	}
	if token != "" {
		request.Header.Set("Authorization", "Bearer "+token)
	}
	response := httptest.NewRecorder()
	f.router.ServeHTTP(response, request)
	return response
}

type adminNavigationRepository struct {
	categories     map[uint]navigation.Category
	items          map[uint]navigation.Item
	nextCategoryID uint
	nextItemID     uint
	failures       map[string]error
	calls          map[string]int
}

func newAdminNavigationRepository() *adminNavigationRepository {
	return &adminNavigationRepository{
		categories: make(map[uint]navigation.Category), items: make(map[uint]navigation.Item),
		nextCategoryID: 1, nextItemID: 1, failures: make(map[string]error), calls: make(map[string]int),
	}
}

func (r *adminNavigationRepository) fail(operation string) error {
	r.calls[operation]++
	return r.failures[operation]
}

func (r *adminNavigationRepository) totalCalls() int {
	total := 0
	for _, count := range r.calls {
		total += count
	}
	return total
}

func (r *adminNavigationRepository) seedCategory(value navigation.Category) navigation.Category {
	if value.ID == 0 {
		value.ID = r.nextCategoryID
		r.nextCategoryID++
	}
	value.Items = nil
	r.categories[value.ID] = value
	return value
}

func (r *adminNavigationRepository) seedItem(value navigation.Item) navigation.Item {
	if value.ID == 0 {
		value.ID = r.nextItemID
		r.nextItemID++
	}
	r.items[value.ID] = value
	return value
}

func (r *adminNavigationRepository) Tree(_ context.Context, enabledOnly bool) ([]navigation.Category, error) {
	if err := r.fail("tree"); err != nil {
		return nil, err
	}
	values := make([]navigation.Category, 0)
	for _, category := range r.categories {
		if enabledOnly && !category.Enabled {
			continue
		}
		category.Items = make([]navigation.Item, 0)
		for _, item := range r.items {
			if item.CategoryID == category.ID && (!enabledOnly || item.Enabled) {
				category.Items = append(category.Items, item)
			}
		}
		sort.Slice(category.Items, func(i, j int) bool { return category.Items[i].ID < category.Items[j].ID })
		values = append(values, category)
	}
	sort.Slice(values, func(i, j int) bool { return values[i].ID < values[j].ID })
	return values, nil
}

func (r *adminNavigationRepository) FindCategory(_ context.Context, id uint) (navigation.Category, bool, error) {
	if err := r.fail("find_category"); err != nil {
		return navigation.Category{}, false, err
	}
	value, ok := r.categories[id]
	return value, ok, nil
}

func (r *adminNavigationRepository) FindCategoryBySystemKey(_ context.Context, key string) (navigation.Category, bool, error) {
	if err := r.fail("find_category_by_system_key"); err != nil {
		return navigation.Category{}, false, err
	}
	for _, value := range r.categories {
		if value.SystemKey != nil && *value.SystemKey == key {
			return value, true, nil
		}
	}
	return navigation.Category{}, false, nil
}

func (r *adminNavigationRepository) FindItem(_ context.Context, id uint) (navigation.Item, bool, error) {
	if err := r.fail("find_item"); err != nil {
		return navigation.Item{}, false, err
	}
	value, ok := r.items[id]
	return value, ok, nil
}

func (r *adminNavigationRepository) ListItemsByCategory(_ context.Context, categoryID uint) ([]navigation.Item, error) {
	if err := r.fail("list_items_by_category"); err != nil {
		return nil, err
	}
	values := make([]navigation.Item, 0)
	for _, item := range r.items {
		if item.CategoryID == categoryID {
			values = append(values, item)
		}
	}
	sort.Slice(values, func(i, j int) bool { return values[i].ID < values[j].ID })
	return values, nil
}

func (r *adminNavigationRepository) DuplicateTarget(_ context.Context, categoryID uint, appID, path string, excludeID uint) (bool, error) {
	if err := r.fail("duplicate_target"); err != nil {
		return false, err
	}
	for _, item := range r.items {
		if item.ID != excludeID && item.CategoryID == categoryID && item.AppID == appID && item.Path == path {
			return true, nil
		}
	}
	return false, nil
}

func (r *adminNavigationRepository) CreateCategory(_ context.Context, value *navigation.Category) error {
	if err := r.fail("create_category"); err != nil {
		return err
	}
	*value = r.seedCategory(*value)
	return nil
}

func (r *adminNavigationRepository) UpdateCategory(_ context.Context, id uint, changes map[string]any) error {
	if err := r.fail("update_category"); err != nil {
		return err
	}
	value, ok := r.categories[id]
	if !ok {
		return navigation.ErrNotFound
	}
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

func (r *adminNavigationRepository) DeleteCategory(_ context.Context, id uint) error {
	if err := r.fail("delete_category"); err != nil {
		return err
	}
	if _, ok := r.categories[id]; !ok {
		return navigation.ErrNotFound
	}
	delete(r.categories, id)
	return nil
}

func (r *adminNavigationRepository) CreateItem(_ context.Context, value *navigation.Item) error {
	if err := r.fail("create_item"); err != nil {
		return err
	}
	*value = r.seedItem(*value)
	return nil
}

func (r *adminNavigationRepository) UpdateItem(_ context.Context, id uint, changes map[string]any) error {
	if err := r.fail("update_item"); err != nil {
		return err
	}
	value, ok := r.items[id]
	if !ok {
		return navigation.ErrNotFound
	}
	applyAdminNavigationItemChanges(&value, changes)
	r.items[id] = value
	return nil
}

func (r *adminNavigationRepository) DeleteItem(_ context.Context, id uint) error {
	if err := r.fail("delete_item"); err != nil {
		return err
	}
	if _, ok := r.items[id]; !ok {
		return navigation.ErrNotFound
	}
	delete(r.items, id)
	return nil
}

func (r *adminNavigationRepository) UpdateItemInCategory(_ context.Context, id, categoryID uint, changes map[string]any) error {
	if err := r.fail("update_item_in_category"); err != nil {
		return err
	}
	value, ok := r.items[id]
	if !ok || value.CategoryID != categoryID {
		return navigation.ErrNotFound
	}
	applyAdminNavigationItemChanges(&value, changes)
	r.items[id] = value
	return nil
}

func (r *adminNavigationRepository) DeleteItemInCategory(_ context.Context, id, categoryID uint) error {
	if err := r.fail("delete_item_in_category"); err != nil {
		return err
	}
	value, ok := r.items[id]
	if !ok || value.CategoryID != categoryID {
		return navigation.ErrNotFound
	}
	delete(r.items, id)
	return nil
}

func applyAdminNavigationItemChanges(value *navigation.Item, changes map[string]any) {
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
	if change, ok := changes["operating_status"].(navigation.OperatingStatus); ok {
		value.OperatingStatus = change
	}
}

var _ navigation.Repository = (*adminNavigationRepository)(nil)

type adminNavigationUserRepository struct{ users map[uint]user.User }

func newAdminNavigationUserRepository(values ...user.User) *adminNavigationUserRepository {
	repository := &adminNavigationUserRepository{users: make(map[uint]user.User, len(values))}
	for _, value := range values {
		repository.users[value.ID] = value
	}
	return repository
}

func (r *adminNavigationUserRepository) FindByOpenID(_ context.Context, openID string) (user.User, error) {
	for _, value := range r.users {
		if value.OpenID == openID {
			return value, nil
		}
	}
	return user.User{}, user.ErrNotFound
}

func (r *adminNavigationUserRepository) FindByID(_ context.Context, id uint) (user.User, error) {
	value, ok := r.users[id]
	if !ok {
		return user.User{}, user.ErrNotFound
	}
	return value, nil
}

func (r *adminNavigationUserRepository) FindProfiles(_ context.Context, ids []uint) (map[uint]user.Profile, error) {
	values := make(map[uint]user.Profile, len(ids))
	for _, id := range ids {
		if value, ok := r.users[id]; ok {
			values[id] = value.Profile()
		}
	}
	return values, nil
}

func (r *adminNavigationUserRepository) Create(_ context.Context, value *user.User) error {
	if value.ID == 0 {
		value.ID = uint(len(r.users) + 1)
	}
	r.users[value.ID] = *value
	return nil
}

func (r *adminNavigationUserRepository) UpdateProfile(_ context.Context, id uint, profile user.Profile) error {
	value, ok := r.users[id]
	if !ok {
		return user.ErrNotFound
	}
	value.Nickname, value.Avatar, value.AvatarAssetID = profile.Nickname, profile.Avatar, profile.AvatarAssetID
	value.School, value.Campus = profile.School, profile.Campus
	r.users[id] = value
	return nil
}

func (r *adminNavigationUserRepository) UpdateCurrentIdentity(_ context.Context, id uint, identity user.Identity) error {
	value, ok := r.users[id]
	if !ok {
		return user.ErrNotFound
	}
	value.CurrentIdentity = identity
	r.users[id] = value
	return nil
}

func (r *adminNavigationUserRepository) List(_ context.Context, _, _ int) ([]user.User, int64, error) {
	values := make([]user.User, 0, len(r.users))
	for _, value := range r.users {
		values = append(values, value)
	}
	return values, int64(len(values)), nil
}

func (r *adminNavigationUserRepository) UpdateStatusAndRevoke(_ context.Context, id uint, status user.Status, _ bool) error {
	value, ok := r.users[id]
	if !ok {
		return user.ErrNotFound
	}
	value.Status = status
	value.TokenVersion++
	r.users[id] = value
	return nil
}

func (r *adminNavigationUserRepository) UpdateRoleAndRevoke(_ context.Context, id uint, role user.Role, _ bool) error {
	value, ok := r.users[id]
	if !ok {
		return user.ErrNotFound
	}
	value.Role = role
	value.TokenVersion++
	r.users[id] = value
	return nil
}

func (r *adminNavigationUserRepository) Count(context.Context) (int64, int64, error) {
	return int64(len(r.users)), 0, nil
}

var _ user.Repository = (*adminNavigationUserRepository)(nil)
