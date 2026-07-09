package main

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"

	"campus-circle/backend/internal/module/takeout"
	"campus-circle/backend/internal/module/user"
)

const p0IdentityFence = "p0-identity-switch"

type p0IdentityEnvelope struct {
	Code    int             `json:"code"`
	Message string          `json:"message"`
	Data    json.RawMessage `json:"data"`
}

func TestP0IdentityMigrationApplyRollbackApply(t *testing.T) {
	db := openP0IdentityMySQL(t)
	refuseP0IdentityRollbackWithNonDefaultFacts(t, db)

	executeP0IdentitySQLFile(t, db, "../../migrations/029_user_current_identity.sql")
	assertP0IdentityColumn(t, db, true)
	executeP0IdentitySQLFile(t, db, "../../migrations/rollback/029_user_current_identity.sql")
	assertP0IdentityColumn(t, db, false)
	executeP0IdentitySQLFile(t, db, "../../migrations/029_user_current_identity.sql")
	assertP0IdentityColumn(t, db, true)
	t.Log("migration_029_apply_rollback_apply=PASS users.current_identity foreign_keys=0")
}

func TestP0IdentityRealHTTPMySQLEligibilitySwitch(t *testing.T) {
	db := openP0IdentityMySQL(t)
	executeP0IdentitySQLFile(t, db, "../../migrations/029_user_current_identity.sql")
	cleanupP0Identity(t, db)
	t.Cleanup(func() { cleanupP0Identity(t, db) })

	ctx := context.Background()
	takeoutRepository := takeout.NewRepository(db)
	riders, err := takeout.NewRiderIdentityService(takeoutRepository, nil)
	if err != nil {
		t.Fatal(err)
	}
	merchants, err := takeout.NewMerchantIdentityService(takeoutRepository)
	if err != nil {
		t.Fatal(err)
	}
	auth := user.NewService(
		user.NewRepository(db), "p0-identity-test-secret-that-is-longer-than-thirty-two-characters",
		"", "", true, user.WithAppEnv("dev"),
		user.WithIdentityEligibility(newTakeoutIdentityEligibility(riders, merchants)),
	)

	gin.SetMode(gin.TestMode)
	router := gin.New()
	user.NewHandler(auth).Register(router.Group("/api/v1/auth"))
	server := httptest.NewServer(router)
	defer server.Close()

	status, _ := p0IdentityHTTPJSON(t, server.Client(), http.MethodGet, server.URL+"/api/v1/auth/identity", "", nil)
	if status != http.StatusUnauthorized {
		t.Fatalf("unauthenticated identity status=%d", status)
	}

	login, err := auth.WechatLogin(ctx, user.LoginInput{Code: p0IdentityFence + "-user", Nickname: "P0 Identity"})
	if err != nil {
		t.Fatal(err)
	}
	status, body := p0IdentityHTTPJSON(t, server.Client(), http.MethodGet, server.URL+"/api/v1/auth/identity", login.Token, nil)
	if status != http.StatusOK || !strings.Contains(string(body.Data), `"currentIdentity":"student"`) ||
		!strings.Contains(string(body.Data), `"identity":"rider"`) ||
		!strings.Contains(string(body.Data), `"status":"missing"`) {
		t.Fatalf("default identity status=%d body=%s message=%s", status, body.Data, body.Message)
	}
	status, _ = p0IdentityHTTPJSON(t, server.Client(), http.MethodPost, server.URL+"/api/v1/auth/identity", login.Token, map[string]any{"identity": "rider"})
	if status != http.StatusForbidden {
		t.Fatalf("missing rider switch status=%d", status)
	}

	now := time.Now().UTC()
	if err := db.Create(&takeout.Rider{
		RiderUserID: uint64(login.User.ID), Status: takeout.RiderActive,
		RealName: "Rider", Phone: "13100000000", RowVersion: 1, CreatedAt: now, UpdatedAt: now,
	}).Error; err != nil {
		t.Fatal(err)
	}
	status, body = p0IdentityHTTPJSON(t, server.Client(), http.MethodPost, server.URL+"/api/v1/auth/identity", login.Token, map[string]any{"identity": "rider"})
	if status != http.StatusOK || !strings.Contains(string(body.Data), `"currentIdentity":"rider"`) {
		t.Fatalf("active rider switch status=%d body=%s message=%s", status, body.Data, body.Message)
	}
	if err := db.Model(&takeout.Rider{}).Where("rider_user_id = ?", login.User.ID).Update("status", takeout.RiderResting).Error; err != nil {
		t.Fatal(err)
	}
	status, _ = p0IdentityHTTPJSON(t, server.Client(), http.MethodPost, server.URL+"/api/v1/auth/identity", login.Token, map[string]any{"identity": "student"})
	if status != http.StatusOK {
		t.Fatalf("student switch after rider status=%d", status)
	}
	status, _ = p0IdentityHTTPJSON(t, server.Client(), http.MethodPost, server.URL+"/api/v1/auth/identity", login.Token, map[string]any{"identity": "rider"})
	if status != http.StatusForbidden {
		t.Fatalf("resting rider switch status=%d", status)
	}

	merchantPublicID, err := takeout.NewULID()
	if err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&takeout.Merchant{
		MerchantPublicID: merchantPublicID, OwnerUserID: uint64(login.User.ID),
		Name: "P0 Merchant", ContactPhone: "13100000001", Location: "Campus",
		BusinessHours: "09:00-18:00", Status: takeout.MerchantActive,
		RowVersion: 1, CreatedAt: now, UpdatedAt: now,
	}).Error; err != nil {
		t.Fatal(err)
	}
	status, body = p0IdentityHTTPJSON(t, server.Client(), http.MethodPost, server.URL+"/api/v1/auth/identity", login.Token, map[string]any{"identity": "merchant"})
	if status != http.StatusOK || !strings.Contains(string(body.Data), `"currentIdentity":"merchant"`) {
		t.Fatalf("active merchant switch status=%d body=%s message=%s", status, body.Data, body.Message)
	}
	if err := db.Model(&takeout.Merchant{}).Where("owner_user_id = ?", login.User.ID).Update("status", takeout.MerchantResting).Error; err != nil {
		t.Fatal(err)
	}
	status, _ = p0IdentityHTTPJSON(t, server.Client(), http.MethodPost, server.URL+"/api/v1/auth/identity", login.Token, map[string]any{"identity": "student"})
	if status != http.StatusOK {
		t.Fatalf("student switch after merchant status=%d", status)
	}
	status, _ = p0IdentityHTTPJSON(t, server.Client(), http.MethodPost, server.URL+"/api/v1/auth/identity", login.Token, map[string]any{"identity": "merchant"})
	if status != http.StatusForbidden {
		t.Fatalf("resting merchant switch status=%d", status)
	}

	var loaded user.User
	if err := db.First(&loaded, login.User.ID).Error; err != nil {
		t.Fatal(err)
	}
	if loaded.Role != user.RoleUser {
		t.Fatalf("identity switch changed users.role=%q", loaded.Role)
	}
	t.Log("p0_identity_http=PASS default_student+qualification_403+active_switch+role_unchanged")
}

func openP0IdentityMySQL(t *testing.T) *gorm.DB {
	t.Helper()
	dsn := os.Getenv("P0_IDENTITY_MYSQL_DSN")
	if dsn == "" {
		t.Skip("set P0_IDENTITY_MYSQL_DSN to run P0 identity real HTTP/MySQL acceptance")
	}
	db, err := gorm.Open(mysql.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	return db
}

func executeP0IdentitySQLFile(t *testing.T, db *gorm.DB, relativePath string) {
	t.Helper()
	raw, err := os.ReadFile(filepath.Clean(relativePath))
	if err != nil {
		t.Fatal(err)
	}
	for _, statement := range splitP0IdentitySQL(string(raw)) {
		if strings.TrimSpace(statement) == "" {
			continue
		}
		if err := db.Exec(statement).Error; err != nil {
			t.Fatalf("apply %s: %v\nstatement:\n%s", relativePath, err, statement)
		}
	}
}

func splitP0IdentitySQL(raw string) []string {
	delimiter := ";"
	var statements []string
	var current strings.Builder
	for _, line := range strings.Split(raw, "\n") {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(strings.ToUpper(trimmed), "DELIMITER ") {
			delimiter = strings.TrimSpace(strings.TrimPrefix(trimmed, "DELIMITER "))
			continue
		}
		current.WriteString(line)
		current.WriteString("\n")
		if strings.HasSuffix(strings.TrimSpace(current.String()), delimiter) {
			statement := strings.TrimSpace(current.String())
			statement = strings.TrimSpace(strings.TrimSuffix(statement, delimiter))
			statements = append(statements, statement)
			current.Reset()
		}
	}
	if strings.TrimSpace(current.String()) != "" {
		statements = append(statements, strings.TrimSpace(current.String()))
	}
	return statements
}

func assertP0IdentityColumn(t *testing.T, db *gorm.DB, want bool) {
	t.Helper()
	got := db.Migrator().HasColumn(&user.User{}, "current_identity")
	if got != want {
		t.Fatalf("users.current_identity exists=%v want=%v", got, want)
	}
}

func refuseP0IdentityRollbackWithNonDefaultFacts(t *testing.T, db *gorm.DB) {
	t.Helper()
	if !db.Migrator().HasColumn(&user.User{}, "current_identity") {
		return
	}
	var count int64
	if err := db.Table("users").Where("current_identity <> ?", user.IdentityStudent).Count(&count).Error; err != nil {
		t.Fatal(err)
	}
	if count > 0 {
		t.Skipf("refusing migration rollback with non-student identity facts: %d", count)
	}
}

func cleanupP0Identity(t *testing.T, db *gorm.DB) {
	t.Helper()
	var userIDs []uint
	if db.Migrator().HasTable(&user.User{}) {
		if err := db.Unscoped().Model(&user.User{}).Where("open_id LIKE ?", "mock_openid_"+p0IdentityFence+"%").
			Pluck("id", &userIDs).Error; err != nil {
			t.Errorf("list P0 identity users: %v", err)
		}
	}
	if len(userIDs) == 0 {
		return
	}
	if db.Migrator().HasTable(&takeout.Merchant{}) {
		if err := db.Where("owner_user_id IN ?", userIDs).Delete(&takeout.Merchant{}).Error; err != nil {
			t.Errorf("cleanup P0 identity merchants: %v", err)
		}
	}
	if db.Migrator().HasTable(&takeout.Rider{}) {
		if err := db.Where("rider_user_id IN ?", userIDs).Delete(&takeout.Rider{}).Error; err != nil {
			t.Errorf("cleanup P0 identity riders: %v", err)
		}
	}
	if err := db.Unscoped().Where("id IN ?", userIDs).Delete(&user.User{}).Error; err != nil {
		t.Errorf("cleanup P0 identity users: %v", err)
	}
}

func p0IdentityHTTPJSON(
	t *testing.T,
	client *http.Client,
	method, endpoint, token string,
	payload any,
) (int, p0IdentityEnvelope) {
	t.Helper()
	var body *bytes.Reader
	if payload == nil {
		body = bytes.NewReader(nil)
	} else {
		encoded, err := json.Marshal(payload)
		if err != nil {
			t.Fatal(err)
		}
		body = bytes.NewReader(encoded)
	}
	request, err := http.NewRequest(method, endpoint, body)
	if err != nil {
		t.Fatal(err)
	}
	request.Header.Set("Content-Type", "application/json")
	if token != "" {
		request.Header.Set("Authorization", "Bearer "+token)
	}
	response, err := client.Do(request)
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	var envelope p0IdentityEnvelope
	_ = json.NewDecoder(response.Body).Decode(&envelope)
	return response.StatusCode, envelope
}
