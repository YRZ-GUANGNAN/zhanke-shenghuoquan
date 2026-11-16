package studentverification

import (
	"context"
	"database/sql"
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

// TestStudentVerificationWorkflowAgainstMySQL exercises the full loop on a
// real MySQL: submit → approve → cancel request → approve cancel → resubmit →
// reject, including optimistic version checks and asset replacement.
func TestStudentVerificationWorkflowAgainstMySQL(t *testing.T) {
	db := requireStudentVerificationMySQL(t)
	repo := NewRepository(db)
	service := NewService(repo, stubMedia{}, testNow)
	ctx := context.Background()
	const userID, reviewerID = uint64(9_400_001), uint64(9_400_099)

	view, err := service.Submit(ctx, userID, SubmitInput{
		RealName: "张三", StudentNo: "20260001", College: "计算机学院",
		AssetPublicIDs: []string{mustULID()},
	})
	if err != nil || view.Status != StatusPending || view.RowVersion != 1 {
		t.Fatalf("submit view=%+v err=%v", view, err)
	}
	var assetCount int64
	if err := db.Model(&ApplicationAsset{}).Where("verification_id = ?", view.ID).Count(&assetCount).Error; err != nil || assetCount != 1 {
		t.Fatalf("assetCount=%d err=%v", assetCount, err)
	}

	if _, err := service.Submit(ctx, userID, SubmitInput{RealName: "张三", StudentNo: "20260001", AssetPublicIDs: []string{mustULID()}}); !errors.Is(err, ErrApplicationPending) {
		t.Fatalf("expected pending conflict, got %v", err)
	}

	approved, err := service.Approve(ctx, reviewerID, view.ApplicationPublicID, view.RowVersion)
	if err != nil || approved.Status != StatusApproved {
		t.Fatalf("approve view=%+v err=%v", approved, err)
	}
	if _, err := service.Approve(ctx, reviewerID, view.ApplicationPublicID, view.RowVersion); !errors.Is(err, ErrDecisionConflict) {
		t.Fatalf("expected stale decision conflict, got %v", err)
	}
	if _, err := service.Approve(ctx, userID, view.ApplicationPublicID, approved.RowVersion); !errors.Is(err, ErrSelfReview) {
		t.Fatalf("expected self review rejection, got %v", err)
	}
	if _, err := service.Submit(ctx, userID, SubmitInput{RealName: "张三", StudentNo: "20260001", AssetPublicIDs: []string{mustULID()}}); !errors.Is(err, ErrApplicationApproved) {
		t.Fatalf("expected approved conflict, got %v", err)
	}

	requested, err := service.RequestCancel(ctx, userID, approved.RowVersion)
	if err != nil || !requested.CancelRequested {
		t.Fatalf("request cancel view=%+v err=%v", requested, err)
	}
	repeated, err := service.RequestCancel(ctx, userID, requested.RowVersion+7)
	if err != nil || repeated.RowVersion != requested.RowVersion {
		t.Fatalf("idempotent request cancel view=%+v err=%v", repeated, err)
	}

	cancelled, err := service.ApproveCancel(ctx, reviewerID, requested.ApplicationPublicID, requested.RowVersion, "已解除绑定")
	if err != nil || cancelled.Status != StatusCancelled {
		t.Fatalf("approve cancel view=%+v err=%v", cancelled, err)
	}

	newAsset := mustULID()
	resubmitted, err := service.Submit(ctx, userID, SubmitInput{
		RealName: "李四", StudentNo: "20260002", College: "外国语学院",
		AssetPublicIDs: []string{newAsset}, ExpectedRowVersion: cancelled.RowVersion,
	})
	if err != nil || resubmitted.Status != StatusPending || resubmitted.RealName != "李四" {
		t.Fatalf("resubmit view=%+v err=%v", resubmitted, err)
	}
	var assets []ApplicationAsset
	if err := db.Where("verification_id = ?", resubmitted.ID).Find(&assets).Error; err != nil || len(assets) != 1 || assets[0].AssetPublicID != newAsset {
		t.Fatalf("assets=%+v err=%v", assets, err)
	}

	rejected, err := service.Reject(ctx, reviewerID, resubmitted.ApplicationPublicID, resubmitted.RowVersion, "学号不清晰")
	if err != nil || rejected.Status != StatusRejected || rejected.ReviewReason != "学号不清晰" {
		t.Fatalf("reject view=%+v err=%v", rejected, err)
	}

	var eventTypes []EventType
	if err := db.Model(&ApplicationEvent{}).Where("verification_id = ?", resubmitted.ID).Order("created_at ASC, id ASC").Pluck("event_type", &eventTypes).Error; err != nil {
		t.Fatal(err)
	}
	want := []EventType{EventSubmitted, EventApproved, EventCancelRequested, EventCancelApproved, EventResubmitted, EventRejected}
	if fmt.Sprint(eventTypes) != fmt.Sprint(want) {
		t.Fatalf("event trail=%v want %v", eventTypes, want)
	}

	t.Cleanup(func() {
		_ = db.Where("verification_id = ?", resubmitted.ID).Delete(&ApplicationAsset{}).Error
		_ = db.Where("verification_id = ?", resubmitted.ID).Delete(&ApplicationEvent{}).Error
		_ = db.Where("id = ?", resubmitted.ID).Delete(&Application{}).Error
	})
}

func requireStudentVerificationMySQL(t *testing.T) *gorm.DB {
	t.Helper()
	dsn := strings.TrimSpace(os.Getenv("TEST_MYSQL_DSN"))
	if dsn == "" {
		t.Skip("PENDING: set TEST_MYSQL_DSN to run student verification repository tests in an independent temporary MySQL database")
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
		t.Skipf("PENDING: student verification repository contract requires MySQL 8, got %s", version)
	}

	databaseName := fmt.Sprintf("student_verification_repo_%d_%d", os.Getpid(), time.Now().UnixNano())
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
	t.Cleanup(func() {
		if err := databaseSQL.Close(); err != nil {
			t.Errorf("close temporary student verification database: %v", err)
		}
		if _, err := serverDB.Exec("DROP DATABASE IF EXISTS `" + databaseName + "`"); err != nil {
			t.Errorf("drop temporary student verification database: %v", err)
		}
		if err := serverDB.Close(); err != nil {
			t.Errorf("close MySQL server connection: %v", err)
		}
	})
	db, err := gorm.Open(gormMySQL.New(gormMySQL.Config{Conn: databaseSQL}), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		t.Fatalf("open GORM temporary database: %v", err)
	}
	if err := db.AutoMigrate(&Application{}, &ApplicationAsset{}, &ApplicationEvent{}); err != nil {
		t.Fatalf("create student verification test schema: %v", err)
	}
	return db
}
