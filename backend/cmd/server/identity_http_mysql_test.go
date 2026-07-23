package main

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	mysqlDriver "github.com/go-sql-driver/mysql"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"campus-circle/backend/internal/module/identityapplication"
	"campus-circle/backend/internal/module/takeout"
	"campus-circle/backend/internal/module/user"
)

const ui21aIdentityFence = "ui21a-runtime-identity"

type ui21aIdentityEnvelope struct {
	Code    int             `json:"code"`
	Message string          `json:"message"`
	Data    json.RawMessage `json:"data"`
}

func TestUI21AIdentityMigrationApplyRollbackApply(t *testing.T) {
	db := openUI21AIdentityMySQL(t)
	if err := db.Exec(`CREATE TABLE users (
		id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
		campus VARCHAR(80) NOT NULL DEFAULT '',
		PRIMARY KEY (id)
	) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci`).Error; err != nil {
		t.Fatal(err)
	}
	refuseUI21AIdentityRollbackWithNonDefaultFacts(t, db)

	executeUI21AIdentitySQLFile(t, db, "../../migrations/029_user_current_identity.sql")
	assertUI21AIdentityColumn(t, db, true)
	assertUI21AIdentityForeignKeys(t, db, 0)
	executeUI21AIdentitySQLFile(t, db, "../../migrations/rollback/029_user_current_identity.sql")
	assertUI21AIdentityColumn(t, db, false)
	executeUI21AIdentitySQLFile(t, db, "../../migrations/029_user_current_identity.sql")
	assertUI21AIdentityColumn(t, db, true)
	t.Log("migration_029_apply_rollback_apply=PASS users.current_identity foreign_keys=0")
}

func TestUI21AIdentityRealHTTPMySQL(t *testing.T) {
	db := openUI21AIdentityMySQL(t)
	if err := db.AutoMigrate(&user.User{}, &takeout.Rider{}, &identityapplication.IdentityGrant{}); err != nil {
		t.Fatalf("create independent identity HTTP schema: %v", err)
	}
	executeUI21AIdentitySQLFile(t, db, "../../migrations/029_user_current_identity.sql")
	if !db.Migrator().HasTable(&takeout.Rider{}) {
		t.Fatal("takeout_riders is required for the shared rider identity acceptance")
	}
	cleanupUI21AIdentity(t, db)
	t.Cleanup(func() { cleanupUI21AIdentity(t, db) })

	riders, err := takeout.NewRiderIdentityService(takeout.NewRepository(db), nil)
	if err != nil {
		t.Fatal(err)
	}
	grants := identityapplication.NewRepository(db)
	auth := newUI21AIdentityAuth(db, newDeliveryIdentityEligibility(true, riders, grants))
	server := newUI21AIdentityServer(auth)
	defer server.Close()

	status, _ := ui21aIdentityHTTPJSON(t, server.Client(), http.MethodGet, server.URL+"/api/v1/auth/identity", "", nil)
	if status != http.StatusUnauthorized {
		t.Fatalf("unauthenticated identity status=%d", status)
	}

	login, err := auth.WechatLogin(context.Background(), user.LoginInput{
		Code: ui21aIdentityFence + "-user", Nickname: "UI21A Identity",
	})
	if err != nil {
		t.Fatal(err)
	}
	initialTokenVersion := login.User.TokenVersion
	status, body := ui21aIdentityHTTPJSON(t, server.Client(), http.MethodGet, server.URL+"/api/v1/auth/identity", login.Token, nil)
	assertUI21AIdentityResponse(t, status, body, http.StatusOK, `"currentIdentity":"student"`)
	status, _ = ui21aIdentityHTTPJSON(t, server.Client(), http.MethodPost, server.URL+"/api/v1/auth/identity", login.Token, map[string]any{"identity": "admin"})
	if status != http.StatusBadRequest {
		t.Fatalf("invalid admin identity status=%d want=%d", status, http.StatusBadRequest)
	}

	status, _ = ui21aIdentityHTTPJSON(t, server.Client(), http.MethodPost, server.URL+"/api/v1/auth/identity", login.Token, map[string]any{"identity": "rider"})
	if status != http.StatusForbidden {
		t.Fatalf("missing rider switch status=%d", status)
	}
	status, body = ui21aIdentityHTTPJSON(t, server.Client(), http.MethodPost, server.URL+"/api/v1/auth/identity", login.Token, map[string]any{"identity": "merchant"})
	if status != http.StatusForbidden {
		t.Fatalf("missing merchant grant switch status=%d", status)
	}
	now := time.Now().UTC()
	merchantApplicationID := uint64(login.User.ID) + 1000
	if err := db.Create(&identityapplication.IdentityGrant{
		UserID: uint64(login.User.ID), Role: identityapplication.RoleMerchant, Status: identityapplication.GrantActive,
		SourceApplicationID: &merchantApplicationID,
		GrantedAt:           now, RowVersion: 1, CreatedAt: now, UpdatedAt: now,
	}).Error; err != nil {
		t.Fatal(err)
	}
	status, body = ui21aIdentityHTTPJSON(t, server.Client(), http.MethodPost, server.URL+"/api/v1/auth/identity", login.Token, map[string]any{"identity": "merchant"})
	assertUI21AIdentityResponse(t, status, body, http.StatusOK, `"currentIdentity":"merchant"`)
	status, body = ui21aIdentityHTTPJSON(t, server.Client(), http.MethodPost, server.URL+"/api/v1/auth/identity", login.Token, map[string]any{"identity": "merchant"})
	assertUI21AIdentityResponse(t, status, body, http.StatusOK, `"currentIdentity":"merchant"`)

	if err := db.Create(&takeout.Rider{
		RiderUserID: uint64(login.User.ID), Status: takeout.RiderActive,
		RealName: "UI21A Rider", Phone: "13100000000", RowVersion: 1,
		CreatedAt: now, UpdatedAt: now,
	}).Error; err != nil {
		t.Fatal(err)
	}
	riderApplicationID := uint64(login.User.ID) + 2000
	if err := db.Create(&identityapplication.IdentityGrant{
		UserID: uint64(login.User.ID), Role: identityapplication.RoleRider, Status: identityapplication.GrantActive,
		SourceApplicationID: &riderApplicationID,
		GrantedAt:           now, RowVersion: 1, CreatedAt: now, UpdatedAt: now,
	}).Error; err != nil {
		t.Fatal(err)
	}
	status, _ = ui21aIdentityHTTPJSON(t, server.Client(), http.MethodPost, server.URL+"/api/v1/auth/identity", login.Token, map[string]any{"identity": "student"})
	if status != http.StatusOK {
		t.Fatalf("student switch status=%d", status)
	}
	status, body = ui21aIdentityHTTPJSON(t, server.Client(), http.MethodPost, server.URL+"/api/v1/auth/identity", login.Token, map[string]any{"identity": "rider"})
	assertUI21AIdentityResponse(t, status, body, http.StatusOK, `"currentIdentity":"rider"`)

	if err := db.Model(&takeout.Rider{}).Where("rider_user_id = ?", login.User.ID).Update("status", takeout.RiderResting).Error; err != nil {
		t.Fatal(err)
	}
	status, _ = ui21aIdentityHTTPJSON(t, server.Client(), http.MethodPost, server.URL+"/api/v1/auth/identity", login.Token, map[string]any{"identity": "student"})
	if status != http.StatusOK {
		t.Fatalf("student switch after resting status=%d", status)
	}
	status, _ = ui21aIdentityHTTPJSON(t, server.Client(), http.MethodPost, server.URL+"/api/v1/auth/identity", login.Token, map[string]any{"identity": "rider"})
	if status != http.StatusForbidden {
		t.Fatalf("resting rider switch status=%d", status)
	}

	disabledAuth := newUI21AIdentityAuth(db, newDeliveryIdentityEligibility(false, nil, grants))
	disabledServer := newUI21AIdentityServer(disabledAuth)
	defer disabledServer.Close()
	status, _ = ui21aIdentityHTTPJSON(t, disabledServer.Client(), http.MethodPost, disabledServer.URL+"/api/v1/auth/identity", login.Token, map[string]any{"identity": "merchant"})
	if status != http.StatusForbidden {
		t.Fatalf("disabled delivery merchant switch status=%d", status)
	}

	status, _ = ui21aIdentityHTTPJSON(t, server.Client(), http.MethodPost, server.URL+"/api/v1/auth/identity", login.Token, map[string]any{"identity": "merchant"})
	if status != http.StatusOK {
		t.Fatalf("merchant persistence setup status=%d", status)
	}
	restartedAuth := newUI21AIdentityAuth(db, newDeliveryIdentityEligibility(true, riders, grants))
	restartedServer := newUI21AIdentityServer(restartedAuth)
	defer restartedServer.Close()
	status, body = ui21aIdentityHTTPJSON(t, restartedServer.Client(), http.MethodGet, restartedServer.URL+"/api/v1/auth/identity", login.Token, nil)
	assertUI21AIdentityResponse(t, status, body, http.StatusOK, `"currentIdentity":"merchant"`)

	var loaded user.User
	if err := db.First(&loaded, login.User.ID).Error; err != nil {
		t.Fatal(err)
	}
	if loaded.Role != user.RoleUser || loaded.TokenVersion != initialTokenVersion {
		t.Fatalf("identity switch changed authorization facts: role=%q token_version=%d", loaded.Role, loaded.TokenVersion)
	}
	t.Log("ui21a_identity_http=PASS auth_401+rider_403+active_switch+merchant_delivery_policy+disabled_gate+persistence+role_unchanged")
}

func newUI21AIdentityAuth(db *gorm.DB, eligibility user.IdentityEligibility) *user.Service {
	return user.NewService(
		user.NewRepository(db), "ui21a-identity-test-secret-that-is-longer-than-thirty-two-characters",
		"", "", true, user.WithAppEnv("dev"), user.WithIdentityEligibility(eligibility),
	)
}

func newUI21AIdentityServer(auth *user.Service) *httptest.Server {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	user.NewHandler(auth).Register(router.Group("/api/v1/auth"))
	return httptest.NewServer(router)
}

func openUI21AIdentityMySQL(t *testing.T) *gorm.DB {
	t.Helper()
	dsn := strings.TrimSpace(os.Getenv("TEST_MYSQL_DSN"))
	if dsn == "" {
		t.Skip("PENDING: set TEST_MYSQL_DSN to run identity HTTP tests in an independent temporary MySQL database")
	}
	serverConfig, err := mysqlDriver.ParseDSN(dsn)
	if err != nil {
		t.Fatalf("parse TEST_MYSQL_DSN: %v", err)
	}
	serverConfig.DBName = ""
	serverConfig.MultiStatements = false
	serverConfig.ParseTime = true
	serverConfig.Loc = time.UTC
	serverDB, err := sql.Open("mysql", serverConfig.FormatDSN())
	if err != nil {
		t.Fatal(err)
	}
	if err := serverDB.Ping(); err != nil {
		_ = serverDB.Close()
		t.Fatalf("connect TEST_MYSQL_DSN server: %v", err)
	}
	var version string
	if err := serverDB.QueryRow("SELECT VERSION()").Scan(&version); err != nil {
		_ = serverDB.Close()
		t.Fatal(err)
	}
	if !strings.HasPrefix(version, "8.") {
		_ = serverDB.Close()
		t.Skipf("PENDING: identity HTTP contract requires MySQL 8, got %s", version)
	}
	databaseName := fmt.Sprintf("identity_http_%d_%d", os.Getpid(), time.Now().UnixNano())
	if _, err := serverDB.Exec("CREATE DATABASE `" + databaseName + "` CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci"); err != nil {
		_ = serverDB.Close()
		t.Skipf("PENDING: cannot create independent temporary database; shared campus_circle fallback is forbidden: %v", err)
	}
	databaseConfig, err := mysqlDriver.ParseDSN(dsn)
	if err != nil {
		_, _ = serverDB.Exec("DROP DATABASE IF EXISTS `" + databaseName + "`")
		_ = serverDB.Close()
		t.Fatal(err)
	}
	databaseConfig.DBName = databaseName
	databaseConfig.MultiStatements = false
	databaseConfig.ParseTime = true
	databaseConfig.Loc = time.UTC
	databaseSQL, err := sql.Open("mysql", databaseConfig.FormatDSN())
	if err != nil {
		_, _ = serverDB.Exec("DROP DATABASE IF EXISTS `" + databaseName + "`")
		_ = serverDB.Close()
		t.Fatal(err)
	}
	if err := databaseSQL.Ping(); err != nil {
		_ = databaseSQL.Close()
		_, _ = serverDB.Exec("DROP DATABASE IF EXISTS `" + databaseName + "`")
		_ = serverDB.Close()
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := databaseSQL.Close(); err != nil {
			t.Errorf("close identity HTTP database: %v", err)
		}
		if _, err := serverDB.Exec("DROP DATABASE IF EXISTS `" + databaseName + "`"); err != nil {
			t.Errorf("drop identity HTTP database: %v", err)
		}
		if err := serverDB.Close(); err != nil {
			t.Errorf("close MySQL server connection: %v", err)
		}
	})
	db, err := gorm.Open(mysql.New(mysql.Config{Conn: databaseSQL}), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		t.Fatal(err)
	}
	return db
}

func executeUI21AIdentitySQLFile(t *testing.T, db *gorm.DB, relativePath string) {
	t.Helper()
	raw, err := os.ReadFile(filepath.Clean(relativePath))
	if err != nil {
		t.Fatal(err)
	}
	for _, statement := range splitUI21AIdentitySQL(string(raw)) {
		if strings.TrimSpace(statement) == "" {
			continue
		}
		if err := db.Exec(statement).Error; err != nil {
			t.Fatalf("execute %s: %v\n%s", relativePath, err, statement)
		}
	}
}

func splitUI21AIdentitySQL(raw string) []string {
	delimiter := ";"
	var statements []string
	var current strings.Builder
	for _, line := range strings.Split(strings.ReplaceAll(raw, "\r\n", "\n"), "\n") {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(strings.ToUpper(trimmed), "DELIMITER ") {
			delimiter = strings.TrimSpace(trimmed[len("DELIMITER "):])
			continue
		}
		current.WriteString(line)
		current.WriteByte('\n')
		candidate := strings.TrimSpace(current.String())
		if strings.HasSuffix(candidate, delimiter) {
			candidate = strings.TrimSpace(strings.TrimSuffix(candidate, delimiter))
			if candidate != "" {
				statements = append(statements, candidate)
			}
			current.Reset()
		}
	}
	if tail := strings.TrimSpace(current.String()); tail != "" {
		statements = append(statements, tail)
	}
	return statements
}

func assertUI21AIdentityColumn(t *testing.T, db *gorm.DB, want bool) {
	t.Helper()
	if got := db.Migrator().HasColumn(&user.User{}, "current_identity"); got != want {
		t.Fatalf("users.current_identity exists=%v want=%v", got, want)
	}
}

func assertUI21AIdentityForeignKeys(t *testing.T, db *gorm.DB, want int64) {
	t.Helper()
	var count int64
	if err := db.Raw(`SELECT COUNT(*) FROM information_schema.table_constraints WHERE table_schema = DATABASE() AND table_name = 'users' AND constraint_type = 'FOREIGN KEY'`).Scan(&count).Error; err != nil {
		t.Fatal(err)
	}
	if count != want {
		t.Fatalf("users foreign keys=%d want=%d", count, want)
	}
}

func refuseUI21AIdentityRollbackWithNonDefaultFacts(t *testing.T, db *gorm.DB) {
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

func cleanupUI21AIdentity(t *testing.T, db *gorm.DB) {
	t.Helper()
	var userIDs []uint
	if db.Migrator().HasTable(&user.User{}) {
		if err := db.Unscoped().Model(&user.User{}).Where("open_id LIKE ?", "mock_openid_"+ui21aIdentityFence+"%").Pluck("id", &userIDs).Error; err != nil {
			t.Errorf("list UI-21a identity users: %v", err)
		}
	}
	if len(userIDs) == 0 {
		return
	}
	if db.Migrator().HasTable(&identityapplication.IdentityGrant{}) {
		if err := db.Where("user_id IN ?", userIDs).Delete(&identityapplication.IdentityGrant{}).Error; err != nil {
			t.Errorf("cleanup UI-21a identity grants: %v", err)
		}
	}
	if db.Migrator().HasTable(&takeout.Rider{}) {
		if err := db.Where("rider_user_id IN ?", userIDs).Delete(&takeout.Rider{}).Error; err != nil {
			t.Errorf("cleanup UI-21a riders: %v", err)
		}
	}
	if err := db.Unscoped().Where("id IN ?", userIDs).Delete(&user.User{}).Error; err != nil {
		t.Errorf("cleanup UI-21a users: %v", err)
	}
}

func ui21aIdentityHTTPJSON(t *testing.T, client *http.Client, method, endpoint, token string, payload any) (int, ui21aIdentityEnvelope) {
	t.Helper()
	var body []byte
	if payload != nil {
		var err error
		body, err = json.Marshal(payload)
		if err != nil {
			t.Fatal(err)
		}
	}
	req, err := http.NewRequest(method, endpoint, bytes.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Content-Type", "application/json")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	resp, err := client.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	var envelope ui21aIdentityEnvelope
	if err := json.NewDecoder(resp.Body).Decode(&envelope); err != nil {
		t.Fatal(err)
	}
	return resp.StatusCode, envelope
}

func assertUI21AIdentityResponse(t *testing.T, status int, body ui21aIdentityEnvelope, wantStatus int, contains string) {
	t.Helper()
	if status != wantStatus || !strings.Contains(string(body.Data), contains) {
		t.Fatalf("status=%d want=%d data=%s message=%s", status, wantStatus, body.Data, body.Message)
	}
}
