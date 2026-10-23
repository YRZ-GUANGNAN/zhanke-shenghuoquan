package sharesettings

import (
	"context"
	"encoding/json"
	"errors"
	"net/http/httptest"
	"strings"
	"testing"

	"campus-circle/backend/internal/module/user"
	"github.com/gin-gonic/gin"
)

type memoryStore struct {
	value  Settings
	writes int
	err    error
}

func (s *memoryStore) Load(context.Context) (Settings, error) { return s.value, s.err }
func (s *memoryStore) Save(_ context.Context, _ uint, input UpdateInput) (Settings, error) {
	if s.err != nil {
		return Settings{}, s.err
	}
	if input.ExpectedRowVersion != s.value.RowVersion {
		return Settings{}, ErrConflict
	}
	s.writes++
	s.value = Settings{Cards: input.Cards, RowVersion: s.value.RowVersion + 1}
	return s.value, nil
}

func TestCardValidation(t *testing.T) {
	for _, invalid := range []string{"http://zhanduiwang.top/a.png", "https://evil.example/a.png", "https://zhanduiwang.top/a.svg", "https://u:p@zhanduiwang.top/a.png", "https://zhanduiwang.top/a.png?secret=1", "https://zhanduiwang.top:443/a.png", "https://zhanduiwang.top/a.png#x"} {
		value := Defaults()
		value.DefaultImageURL = invalid
		if value.Validate() == nil {
			t.Fatalf("accepted %q", invalid)
		}
	}
	for _, valid := range []string{"", "https://zhanduiwang.top/media/a.png", "https://bucket-123.cos.ap-guangzhou.myqcloud.com/a.jpg"} {
		value := Defaults()
		value.DefaultImageURL = valid
		if err := value.Validate(); err != nil {
			t.Fatalf("rejected %q: %v", valid, err)
		}
	}
	for _, invalid := range []string{"  ", strings.Repeat("湛", 61), "标题\n换行"} {
		value := Defaults()
		value.HomeTitle = invalid
		if value.Validate() == nil {
			t.Fatalf("accepted invalid title %q", invalid)
		}
	}
}

func TestHTTPPermissionsStrictInputConflictAndPublicProjection(t *testing.T) {
	gin.SetMode(gin.TestMode)
	store := &memoryStore{value: Settings{Cards: Defaults(), RowVersion: 1}}
	router := gin.New()
	NewHandler(store).Register(router.Group("/api/v1"), func(c *gin.Context) {
		role := c.GetHeader("Test-Role")
		if role == "" {
			c.AbortWithStatus(401)
			return
		}
		user.SetCurrentUser(c, user.User{ID: 42, Role: user.Role(role), Status: user.StatusActive})
	})
	input, _ := json.Marshal(UpdateInput{Cards: Defaults(), ExpectedRowVersion: 1})
	request := func(method, path, role, body string, expected int) string {
		t.Helper()
		r := httptest.NewRequest(method, path, strings.NewReader(body))
		r.Header.Set("Test-Role", role)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, r)
		if w.Code != expected {
			t.Fatalf("%s %s: got %d want %d: %s", method, path, w.Code, expected, w.Body.String())
		}
		return w.Body.String()
	}
	public := request("GET", "/api/v1/share-settings", "", "", 200)
	if strings.Contains(public, "updatedBy") || strings.Contains(public, "actorId") {
		t.Fatal("admin identity leaked")
	}
	request("GET", "/api/v1/admin/share-settings", "", "", 401)
	request("PUT", "/api/v1/admin/share-settings", "user", string(input), 403)
	request("PUT", "/api/v1/admin/share-settings", "admin", strings.TrimSuffix(string(input), "}")+`,"unknown":true}`, 400)
	request("PUT", "/api/v1/admin/share-settings", "admin", string(input)+`{}`, 400)
	request("PUT", "/api/v1/admin/share-settings", "admin", `null`, 400)
	if store.writes != 0 {
		t.Fatal("invalid request wrote configuration")
	}
	request("PUT", "/api/v1/admin/share-settings", "admin", string(input), 200)
	request("PUT", "/api/v1/admin/share-settings", "superadmin", string(input), 409)
	if store.writes != 1 {
		t.Fatal("conflicting request wrote configuration")
	}
	store.err = errors.New("internal database details")
	failed := request("GET", "/api/v1/share-settings", "", "", 503)
	if strings.Contains(failed, "internal database") {
		t.Fatal("storage details leaked")
	}
}
