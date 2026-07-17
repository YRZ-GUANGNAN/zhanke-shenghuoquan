package navigation

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestPublicNavigationHandlerReturnsReadOnlyContract(t *testing.T) {
	gin.SetMode(gin.TestMode)
	repository := newMemoryNavigationRepository()
	systemCategory := repository.seedCategory(Category{
		Name: "外卖商家", SystemKey: stringPointer(SystemKeyExternalTakeoutMerchants), Sort: 20, Enabled: true,
	})
	disabledCategory := repository.seedCategory(Category{Name: "关闭分类", Sort: 30, Enabled: false})
	repository.seedItem(Item{
		CategoryID: systemCategory.ID, Title: "营业商家", Subtitle: "可见", IconText: "餐",
		AppID: "wx1234567890ABCDE6", Path: "pages/store/index", Color: "#778899",
		Sort: 1, Enabled: true, OperatingStatus: OperatingStatusResting,
	})
	repository.seedItem(Item{
		CategoryID: systemCategory.ID, Title: "隐藏商家", IconText: "隐", AppID: "wx1234567890ABCDE7",
		Path: "pages/hidden/index", Color: "#8899AA", Sort: 2, Enabled: false, OperatingStatus: OperatingStatusOpen,
	})
	repository.seedItem(Item{
		CategoryID: disabledCategory.ID, Title: "关闭入口", IconText: "关", AppID: "legacy-disabled",
		Path: "legacy/disabled", Color: "legacy", Sort: 1, Enabled: true, OperatingStatus: OperatingStatusOpen,
	})
	repository.forceUnfilteredTree = true

	router := gin.New()
	NewHandler(NewService(repository)).Register(router.Group("/navigation"))
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/navigation/categories", nil)
	router.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", recorder.Code, recorder.Body.String())
	}

	var envelope struct {
		Code    int              `json:"code"`
		Message string           `json:"message"`
		Data    []map[string]any `json:"data"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &envelope); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if envelope.Code != 0 || envelope.Message != "ok" || len(envelope.Data) != 1 {
		t.Fatalf("unexpected response envelope: %+v", envelope)
	}
	category := envelope.Data[0]
	assertJSONKeys(t, category, "id", "name", "systemKey", "sort", "items")
	items, ok := category["items"].([]any)
	if !ok || len(items) != 1 {
		t.Fatalf("public items = %#v, want one enabled item", category["items"])
	}
	item, ok := items[0].(map[string]any)
	if !ok {
		t.Fatalf("public item has unexpected type: %#v", items[0])
	}
	assertJSONKeys(t, item,
		"id", "title", "subtitle", "iconText", "appId", "path", "color", "sort", "operatingStatus")
	if item["operatingStatus"] != string(OperatingStatusResting) {
		t.Fatalf("operatingStatus = %#v, want resting", item["operatingStatus"])
	}
	for _, forbidden := range []string{"createdAt", "updatedAt", "categoryId", "enabled"} {
		if strings.Contains(recorder.Body.String(), `"`+forbidden+`"`) {
			t.Fatalf("public response leaked %s: %s", forbidden, recorder.Body.String())
		}
	}
}

func TestPublicNavigationHandlerHidesRepositoryError(t *testing.T) {
	gin.SetMode(gin.TestMode)
	repository := newMemoryNavigationRepository()
	repository.failures["tree"] = errors.New("SELECT secret FROM navigation_categories WHERE password='secret'")
	router := gin.New()
	NewHandler(NewService(repository)).Register(router.Group("/navigation"))

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/navigation/categories", nil)
	router.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want 500; body = %s", recorder.Code, recorder.Body.String())
	}
	if recorder.Body.String() != `{"code":500,"message":"list navigation failed"}` {
		t.Fatalf("unexpected fixed error body: %s", recorder.Body.String())
	}
	if strings.Contains(strings.ToLower(recorder.Body.String()), "select") || strings.Contains(recorder.Body.String(), "secret") {
		t.Fatalf("repository error leaked: %s", recorder.Body.String())
	}
}

func assertJSONKeys(t *testing.T, value map[string]any, expected ...string) {
	t.Helper()
	if len(value) != len(expected) {
		t.Fatalf("JSON keys = %#v, want %v", value, expected)
	}
	for _, key := range expected {
		if _, ok := value[key]; !ok {
			t.Fatalf("JSON = %#v, missing key %q", value, key)
		}
	}
}
