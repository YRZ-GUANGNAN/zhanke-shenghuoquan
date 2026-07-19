package identityapplication

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	mysqlDriver "github.com/go-sql-driver/mysql"
	gormMySQL "gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

const (
	mysqlAssetA = "01ARZ3NDEKTSV4RRFFQ69G5FAV"
	mysqlAssetB = "01ARZ3NDEKTSV4RRFFQ69G5FAW"
	mysqlAssetC = "01ARZ3NDEKTSV4RRFFQ69G5FAX"
	mysqlAssetD = "01ARZ3NDEKTSV4RRFFQ69G5FAY"
)

func TestGormRepositoryMySQLContract(t *testing.T) {
	db := requireIdentityApplicationMySQL(t)
	repository := NewRepository(db)
	ctx := context.Background()
	now := time.Date(2026, 7, 19, 10, 0, 0, 123000000, time.UTC)
	firstSubmittedAt := now
	service := NewService(repository, &submissionMediaStub{}, func() time.Time { return now })

	first, err := service.Submit(ctx, 41, RoleRider, SubmitInput{
		RealName: "张同学", Phone: "13800138000", AssetPublicIDs: []string{mysqlAssetA},
	})
	if err != nil {
		t.Fatalf("first submit: %v", err)
	}
	if first.Status != StatusPending || first.RowVersion != 1 || first.UserID != "41" {
		t.Fatalf("first=%+v", first)
	}
	assertMySQLCount(t, db, 1, &Application{}, "user_id = ? AND role = ?", 41, RoleRider)
	assertMySQLCount(t, db, 1, &ApplicationAsset{}, "application_id = ?", first.ID)
	assertMySQLCount(t, db, 1, &ApplicationEvent{}, "application_id = ? AND event_type = ?", first.ID, EventSubmitted)
	var submittedEvent ApplicationEvent
	if err := db.Where("application_id = ?", first.ID).Take(&submittedEvent).Error; err != nil {
		t.Fatal(err)
	}
	if submittedEvent.ActorUserID != 41 || submittedEvent.Status != StatusPending || len(submittedEvent.SnapshotJSON) == 0 {
		t.Fatalf("submitted event=%+v", submittedEvent)
	}

	reviewerID := uint64(7)
	reviewedAt := now.Add(30 * time.Minute)
	result := db.Model(&Application{}).Where("id = ? AND row_version = ?", first.ID, 1).Updates(map[string]any{
		"status":           StatusRejected,
		"review_reason":    "资料不清晰",
		"reviewer_user_id": reviewerID,
		"reviewed_at":      reviewedAt,
		"row_version":      2,
		"updated_at":       reviewedAt,
	})
	if result.Error != nil || result.RowsAffected != 1 {
		t.Fatalf("mark rejected: rows=%d err=%v", result.RowsAffected, result.Error)
	}

	now = now.Add(time.Hour)
	resubmitted, err := service.Submit(ctx, 41, RoleRider, SubmitInput{
		RealName: "张同学", Phone: "13800138000", AssetPublicIDs: []string{mysqlAssetB, mysqlAssetC}, ExpectedRowVersion: 2,
	})
	if err != nil {
		t.Fatalf("resubmit rejected: %v", err)
	}
	if resubmitted.RowVersion != 3 || resubmitted.Status != StatusPending || resubmitted.ReviewerUserID != nil ||
		resubmitted.ReviewedAt != nil || len(resubmitted.AssetPublicIDs) != 2 {
		t.Fatalf("resubmitted=%+v", resubmitted)
	}
	assertMySQLCount(t, db, 2, &ApplicationAsset{}, "application_id = ?", first.ID)
	assertMySQLCount(t, db, 2, &ApplicationEvent{}, "application_id = ?", first.ID)

	staleBase := now.Add(15 * time.Minute)
	result = db.Model(&Application{}).Where("id = ? AND row_version = ?", first.ID, 3).Updates(map[string]any{
		"status":           StatusRejected,
		"review_reason":    "仍需补充",
		"reviewer_user_id": reviewerID,
		"reviewed_at":      staleBase,
		"row_version":      4,
		"updated_at":       staleBase,
	})
	if result.Error != nil || result.RowsAffected != 1 {
		t.Fatalf("prepare stale CAS: rows=%d err=%v", result.RowsAffected, result.Error)
	}
	stale, found, err := repository.FindCurrent(ctx, 41, RoleRider)
	if err != nil || !found {
		t.Fatalf("load stale candidate: found=%v err=%v", found, err)
	}
	stale.Status = StatusPending
	stale.RowVersion = 4
	stale.SubmittedAt = staleBase.Add(time.Hour)
	stale.UpdatedAt = stale.SubmittedAt
	if err := repository.SaveSubmission(ctx, &stale, []string{mysqlAssetD}, EventResubmitted, 3); !errors.Is(err, ErrVersionConflict) {
		t.Fatalf("stale CAS err=%v", err)
	}
	unchanged, found, err := repository.FindCurrent(ctx, 41, RoleRider)
	if err != nil || !found || unchanged.Status != StatusRejected || unchanged.RowVersion != 4 || unchanged.ReviewReason != "仍需补充" {
		t.Fatalf("stale CAS changed application: found=%v value=%+v err=%v", found, unchanged, err)
	}
	if len(unchanged.Assets) != 2 || unchanged.Assets[0].AssetPublicID != mysqlAssetB || unchanged.Assets[1].AssetPublicID != mysqlAssetC {
		t.Fatalf("stale CAS changed assets: %+v", unchanged.Assets)
	}
	assertMySQLCount(t, db, 2, &ApplicationEvent{}, "application_id = ?", first.ID)

	rollbackTime := now.Add(2 * time.Hour)
	rollbackApplication := Application{
		ApplicationPublicID: "01ARZ3NDEKTSV4RRFFQ69G5FB0",
		UserID:              99,
		Role:                RoleRider,
		Status:              StatusPending,
		RealName:            "回滚用户",
		Phone:               "13900139000",
		SubmittedAt:         rollbackTime,
		RowVersion:          1,
		CreatedAt:           rollbackTime,
		UpdatedAt:           rollbackTime,
	}
	if err := repository.SaveSubmission(ctx, &rollbackApplication, []string{mysqlAssetA, mysqlAssetA}, EventSubmitted, 0); err == nil {
		t.Fatal("duplicate asset transaction unexpectedly succeeded")
	}
	assertMySQLCount(t, db, 0, &Application{}, "user_id = ?", 99)
	assertMySQLCount(t, db, 0, &ApplicationAsset{}, "application_id = ?", rollbackApplication.ID)
	assertMySQLCount(t, db, 0, &ApplicationEvent{}, "actor_user_id = ?", 99)

	now = now.Add(3 * time.Hour)
	submitMySQLApplication(t, service, 42, RoleRider, "骑手甲")
	now = now.Add(time.Minute)
	approved := submitMySQLApplication(t, service, 43, RoleRider, "骑手乙")
	if err := db.Model(&Application{}).Where("id = ?", approved.ID).Updates(map[string]any{
		"status": StatusApproved, "row_version": 2, "updated_at": now,
	}).Error; err != nil {
		t.Fatal(err)
	}
	now = now.Add(time.Minute)
	submitMySQLApplication(t, service, 44, RoleMerchant, "商家甲")
	now = now.Add(time.Minute)
	latestPendingRider := submitMySQLApplication(t, service, 45, RoleRider, "骑手丙")

	pageOne, total, err := repository.ListAdmin(ctx, AdminListFilter{Role: RoleRider, Status: StatusPending, Page: 1, PageSize: 1})
	if err != nil || total != 2 || len(pageOne) != 1 || pageOne[0].ID != latestPendingRider.ID || len(pageOne[0].Assets) != 1 {
		t.Fatalf("rider pending page one: total=%d values=%+v err=%v", total, pageOne, err)
	}
	pageTwo, total, err := repository.ListAdmin(ctx, AdminListFilter{Role: RoleRider, Status: StatusPending, Page: 2, PageSize: 1})
	if err != nil || total != 2 || len(pageTwo) != 1 || pageTwo[0].UserID != 42 {
		t.Fatalf("rider pending page two: total=%d values=%+v err=%v", total, pageTwo, err)
	}
	merchants, total, err := repository.ListAdmin(ctx, AdminListFilter{Role: RoleMerchant, Status: StatusPending, Page: 1, PageSize: 20})
	if err != nil || total != 1 || len(merchants) != 1 || merchants[0].UserID != 44 {
		t.Fatalf("merchant pending filter: total=%d values=%+v err=%v", total, merchants, err)
	}

	// Make physical row order disagree with sort_order, and give the two real
	// submission events the same timestamp. The extra earlier event proves that
	// FindDetail applies created_at first and id as the deterministic tie-breaker.
	if err := db.Model(&ApplicationAsset{}).Where("application_id = ? AND sort_order = ?", first.ID, 0).Update("sort_order", 9).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Model(&ApplicationAsset{}).Where("application_id = ? AND sort_order = ?", first.ID, 1).Update("sort_order", 0).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Model(&ApplicationAsset{}).Where("application_id = ? AND sort_order = ?", first.ID, 9).Update("sort_order", 1).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Model(&ApplicationEvent{}).Where("application_id = ?", first.ID).Update("created_at", firstSubmittedAt).Error; err != nil {
		t.Fatal(err)
	}
	earlyEvent := ApplicationEvent{
		EventPublicID: "01ARZ3NDEKTSV4RRFFQ69G5FB1",
		ApplicationID: first.ID,
		ActorUserID:   7,
		EventType:     EventRejected,
		Status:        StatusRejected,
		Reason:        "排序哨兵",
		SnapshotJSON:  json.RawMessage(`{}`),
		CreatedAt:     firstSubmittedAt.Add(-time.Hour),
	}
	if err := db.Create(&earlyEvent).Error; err != nil {
		t.Fatal(err)
	}

	detail, err := repository.FindDetail(ctx, first.ApplicationPublicID)
	if err != nil {
		t.Fatal(err)
	}
	if detail.UserID != "41" || detail.ReviewerUserID == nil || *detail.ReviewerUserID != "7" {
		t.Fatalf("detail string ids=%+v", detail.ApplicationView)
	}
	if len(detail.AssetPublicIDs) != 2 || detail.AssetPublicIDs[0] != mysqlAssetC || detail.AssetPublicIDs[1] != mysqlAssetB {
		t.Fatalf("detail assets not sorted by sort_order: %+v", detail.AssetPublicIDs)
	}
	if len(detail.Events) != 3 || detail.Events[0].EventType != EventRejected || detail.Events[0].ActorUserID != "7" ||
		detail.Events[1].EventType != EventSubmitted || detail.Events[2].EventType != EventResubmitted ||
		detail.Events[1].ActorUserID != "41" || !detail.Events[1].CreatedAt.Equal(detail.Events[2].CreatedAt) {
		t.Fatalf("detail events not sorted by created_at/id: %+v", detail.Events)
	}
}

func submitMySQLApplication(t *testing.T, service *Service, userID uint64, role Role, name string) ApplicationView {
	t.Helper()
	input := SubmitInput{RealName: name, Phone: "13800138000", AssetPublicIDs: []string{mysqlAssetA}}
	if role == RoleMerchant {
		input.MerchantName = "测试商家"
		input.MerchantAddress = "测试地址"
	}
	value, err := service.Submit(context.Background(), userID, role, input)
	if err != nil {
		t.Fatalf("submit user %d role %s: %v", userID, role, err)
	}
	return value
}

func assertMySQLCount(t *testing.T, db *gorm.DB, want int64, model any, query string, args ...any) {
	t.Helper()
	var count int64
	if err := db.Model(model).Where(query, args...).Count(&count).Error; err != nil {
		t.Fatal(err)
	}
	if count != want {
		t.Fatalf("count=%d want=%d model=%T query=%s", count, want, model, query)
	}
}

func requireIdentityApplicationMySQL(t *testing.T) *gorm.DB {
	t.Helper()
	dsn := strings.TrimSpace(os.Getenv("TEST_MYSQL_DSN"))
	if dsn == "" {
		t.Skip("PENDING: set TEST_MYSQL_DSN to run identity application repository tests in an independent temporary MySQL database")
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
		t.Fatalf("open MySQL server connection: %v", err)
	}
	if err := serverDB.Ping(); err != nil {
		_ = serverDB.Close()
		t.Fatalf("connect TEST_MYSQL_DSN server: %v", err)
	}
	var version string
	if err := serverDB.QueryRow("SELECT VERSION()").Scan(&version); err != nil {
		_ = serverDB.Close()
		t.Fatalf("read MySQL version: %v", err)
	}
	if !strings.HasPrefix(version, "8.") {
		_ = serverDB.Close()
		t.Skipf("PENDING: identity application repository contract requires MySQL 8, got %s", version)
	}

	databaseName := fmt.Sprintf("identity_application_repo_%d_%d", os.Getpid(), time.Now().UnixNano())
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
		t.Fatalf("connect temporary database: %v", err)
	}
	t.Cleanup(func() {
		if err := databaseSQL.Close(); err != nil {
			t.Errorf("close temporary identity application database: %v", err)
		}
		if _, err := serverDB.Exec("DROP DATABASE IF EXISTS `" + databaseName + "`"); err != nil {
			t.Errorf("drop temporary identity application database: %v", err)
		}
		if err := serverDB.Close(); err != nil {
			t.Errorf("close MySQL server connection: %v", err)
		}
	})

	for _, statement := range identityApplicationRepositoryDDL() {
		if _, err := databaseSQL.Exec(statement); err != nil {
			t.Fatalf("create identity application test schema: %v\n%s", err, statement)
		}
	}
	db, err := gorm.Open(gormMySQL.New(gormMySQL.Config{Conn: databaseSQL}), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		t.Fatalf("open GORM temporary database: %v", err)
	}
	return db
}

func identityApplicationRepositoryDDL() []string {
	return []string{
		`CREATE TABLE identity_applications (
			id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
			application_public_id CHAR(26) NOT NULL,
			user_id BIGINT UNSIGNED NOT NULL,
			role VARCHAR(16) NOT NULL,
			status VARCHAR(16) NOT NULL DEFAULT 'pending',
			real_name VARCHAR(64) NOT NULL,
			phone VARCHAR(32) NOT NULL,
			merchant_name VARCHAR(120) NOT NULL DEFAULT '',
			merchant_address VARCHAR(240) NOT NULL DEFAULT '',
			review_reason VARCHAR(500) NOT NULL DEFAULT '',
			reviewer_user_id BIGINT UNSIGNED NULL,
			reviewed_at DATETIME(3) NULL,
			submitted_at DATETIME(3) NOT NULL,
			row_version BIGINT UNSIGNED NOT NULL DEFAULT 1,
			created_at DATETIME(3) NOT NULL,
			updated_at DATETIME(3) NOT NULL,
			PRIMARY KEY (id),
			UNIQUE KEY uq_identity_application_public_id (application_public_id),
			UNIQUE KEY uq_identity_application_user_role (user_id, role),
			KEY idx_identity_application_status_submitted (status, submitted_at, id)
		) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci`,
		`CREATE TABLE identity_application_assets (
			id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
			application_id BIGINT UNSIGNED NOT NULL,
			asset_public_id CHAR(26) NOT NULL,
			sort_order SMALLINT UNSIGNED NOT NULL,
			created_at DATETIME(3) NOT NULL,
			PRIMARY KEY (id),
			UNIQUE KEY uq_identity_application_asset_slot (application_id, sort_order),
			UNIQUE KEY uq_identity_application_asset (application_id, asset_public_id)
		) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci`,
		`CREATE TABLE identity_application_events (
			id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
			event_public_id CHAR(26) NOT NULL,
			application_id BIGINT UNSIGNED NOT NULL,
			actor_user_id BIGINT UNSIGNED NOT NULL,
			event_type VARCHAR(16) NOT NULL,
			status VARCHAR(16) NOT NULL,
			reason VARCHAR(500) NOT NULL DEFAULT '',
			snapshot_json JSON NOT NULL,
			created_at DATETIME(3) NOT NULL,
			PRIMARY KEY (id),
			UNIQUE KEY uq_identity_application_event_public_id (event_public_id),
			KEY idx_identity_application_event_application (application_id, created_at, id)
		) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci`,
	}
}
