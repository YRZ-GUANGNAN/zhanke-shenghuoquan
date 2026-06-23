package reviewtarget_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"campus-circle/backend/internal/module/admin"
	"campus-circle/backend/internal/module/admin/review"
	"campus-circle/backend/internal/module/admin/takeoutconsole/reviewtarget"
	coremedia "campus-circle/backend/internal/module/media"
	"campus-circle/backend/internal/module/takeout"
	"campus-circle/backend/internal/module/user"
	"github.com/gin-gonic/gin"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
)

const (
	testMerchantPrefix = "G6A2 Review "
	testDishPrefix     = "G6A2 Dish "
)

type apiEnvelope struct {
	Code    int             `json:"code"`
	Message string          `json:"message"`
	Data    json.RawMessage `json:"data"`
}

type racingMerchantService struct {
	service *takeout.MerchantService
	db      *gorm.DB
	raceID  uint64
	mu      sync.Mutex
	reads   int
}

func (s *racingMerchantService) FindMerchantByInternalID(ctx context.Context, id uint64) (takeout.Merchant, error) {
	value, err := s.service.FindMerchantByInternalID(ctx, id)
	if err != nil || id != s.raceID {
		return value, err
	}
	s.mu.Lock()
	s.reads++
	inject := s.reads == 2
	s.mu.Unlock()
	if inject {
		result := s.db.WithContext(ctx).Model(&takeout.Merchant{}).
			Where("id = ? AND row_version = ?", value.ID, value.RowVersion).
			UpdateColumn("row_version", gorm.Expr("row_version + 1"))
		if result.Error != nil {
			return takeout.Merchant{}, result.Error
		}
		if result.RowsAffected != 1 {
			return takeout.Merchant{}, fmt.Errorf("inject merchant review race: affected=%d", result.RowsAffected)
		}
	}
	return value, nil
}

func (s *racingMerchantService) Approve(ctx context.Context, publicID string, rowVersion uint64) (takeout.Merchant, error) {
	return s.service.Approve(ctx, publicID, rowVersion)
}

func (s *racingMerchantService) Reject(ctx context.Context, publicID string, rowVersion uint64) (takeout.Merchant, error) {
	return s.service.Reject(ctx, publicID, rowVersion)
}

func TestTakeoutReviewTargetsAgainstRealHTTPAndMySQL84(t *testing.T) {
	db := openG6A2MySQL(t)
	var mysqlVersion string
	if err := db.Raw("SELECT VERSION()").Scan(&mysqlVersion).Error; err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(mysqlVersion, "8.4.") {
		t.Fatalf("real MySQL version=%s want 8.4.x", mysqlVersion)
	}
	t.Logf("real_mysql_version=%s", mysqlVersion)
	assertG6A2Schema(t, db)
	cleanupG6A2(t, db)
	t.Cleanup(func() { cleanupG6A2(t, db) })

	ctx := context.Background()
	now := time.Date(2026, 6, 22, 23, 45, 0, 0, time.UTC)
	repository := takeout.NewRepository(db)
	merchants, err := takeout.NewMerchantService(repository, func() time.Time { return now })
	if err != nil {
		t.Fatal(err)
	}
	catalog, err := takeout.NewCatalogService(repository, func() time.Time { return now })
	if err != nil {
		t.Fatal(err)
	}
	mediaRepository := coremedia.NewRepository(db)
	mediaService := coremedia.NewService(mediaRepository)
	mediaGateway, err := coremedia.NewMediaGateway(mediaRepository)
	if err != nil {
		t.Fatal(err)
	}

	auth := user.NewService(user.NewRepository(db), "g6a2-test-secret-that-is-longer-than-thirty-two-characters", "", "", true)
	ordinary, err := auth.WechatLogin(ctx, user.LoginInput{Code: "g6a2-ordinary", Nickname: "G6A2 ordinary"})
	if err != nil {
		t.Fatal(err)
	}
	administrator, err := auth.WechatLogin(ctx, user.LoginInput{Code: "g6a2-admin", Nickname: "G6A2 admin"})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.Model(&user.User{}).Where("id = ?", administrator.User.ID).Update("role", user.RoleAdmin).Error; err != nil {
		t.Fatal(err)
	}

	logoAsset := createApprovedMediaAsset(t, ctx, db, mediaService, uint64(administrator.User.ID), coremedia.PurposeMerchantLogo, "g6a2-logo-approved")
	merchantApproved := createMerchant(t, ctx, merchants, uint64(administrator.User.ID), testMerchantPrefix+"Approved", logoAsset.AssetPublicID)
	bindReviewTargetReference(t, ctx, mediaService, logoAsset, coremedia.PurposeMerchantLogo, merchantApproved.MerchantPublicID, merchantApproved.RowVersion)
	assertReviewTargetMediaNotVisible(t, ctx, mediaGateway, coremedia.PurposeMerchantLogo, merchantApproved.MerchantPublicID, logoAsset.AssetPublicID)
	merchantRejected := createMerchant(t, ctx, merchants, uint64(administrator.User.ID), testMerchantPrefix+"Rejected")
	merchantRace := createMerchant(t, ctx, merchants, uint64(administrator.User.ID), testMerchantPrefix+"Race")
	dishApproved := createDish(t, ctx, catalog, merchantApproved.MerchantPublicID, testDishPrefix+"Approved")
	dishRejected := createDish(t, ctx, catalog, merchantApproved.MerchantPublicID, testDishPrefix+"Rejected")

	reviewRepository := review.NewRepository(db)
	reviews := review.NewService(reviewRepository)
	racingMerchants := &racingMerchantService{service: merchants, db: db, raceID: merchantRace.ID}
	for _, target := range reviewtarget.NewTargets(racingMerchants, catalog, mediaService) {
		reviews.RegisterTarget(target)
	}

	tasks := map[string]review.Task{}
	for _, fixture := range []struct {
		name       string
		objectType string
		objectID   uint64
	}{
		{name: "merchant-approved", objectType: reviewtarget.MerchantObjectType, objectID: merchantApproved.ID},
		{name: "merchant-rejected", objectType: reviewtarget.MerchantObjectType, objectID: merchantRejected.ID},
		{name: "dish-approved", objectType: reviewtarget.DishObjectType, objectID: dishApproved.ID},
		{name: "dish-rejected", objectType: reviewtarget.DishObjectType, objectID: dishRejected.ID},
		{name: "merchant-race", objectType: reviewtarget.MerchantObjectType, objectID: merchantRace.ID},
	} {
		task, submitErr := reviews.Submit(ctx, review.Submission{
			ObjectType: fixture.objectType, ObjectID: uint(fixture.objectID),
			SubmitterID: administrator.User.ID, Risk: "g6a2-" + fixture.name,
		})
		if submitErr != nil {
			t.Fatal(submitErr)
		}
		if task.Status != review.StatusSubmitted {
			t.Fatalf("%s status=%s", fixture.name, task.Status)
		}
		tasks[fixture.name] = task
	}

	gin.SetMode(gin.TestMode)
	router := gin.New()
	admin.NewHandler(nil, reviews, auth, nil, nil, nil).Register(router.Group("/api/v1"))
	server := httptest.NewServer(router)
	defer server.Close()

	status, _ := httpJSON(t, server.Client(), http.MethodPost, reviewURL(server.URL, tasks["merchant-approved"], "start"), "", nil)
	if status != http.StatusUnauthorized {
		t.Fatalf("review without token=%d want=401", status)
	}
	status, _ = httpJSON(t, server.Client(), http.MethodPost, reviewURL(server.URL, tasks["merchant-approved"], "start"), ordinary.Token, nil)
	if status != http.StatusForbidden {
		t.Fatalf("ordinary review=%d want=403", status)
	}

	startAndDecide(t, server, administrator.Token, tasks["merchant-approved"], "approve", "merchant approved")
	assertMerchantStatus(t, ctx, merchants, merchantApproved.MerchantPublicID, takeout.MerchantActive)
	assertReviewTask(t, ctx, reviewRepository, tasks["merchant-approved"].ID, review.StatusApproved, "merchant approved")
	assertReviewTargetMediaVisible(t, ctx, mediaGateway, coremedia.PurposeMerchantLogo, merchantApproved.MerchantPublicID, logoAsset.AssetPublicID)
	startAndDecide(t, server, administrator.Token, tasks["merchant-rejected"], "reject", "merchant rejected")
	assertMerchantStatus(t, ctx, merchants, merchantRejected.MerchantPublicID, takeout.MerchantRejected)
	assertReviewTask(t, ctx, reviewRepository, tasks["merchant-rejected"].ID, review.StatusRejected, "merchant rejected")
	startAndDecide(t, server, administrator.Token, tasks["dish-approved"], "approve", "dish approved")
	assertDishStatus(t, ctx, catalog, dishApproved.DishPublicID, takeout.DishOnShelf)
	assertReviewTask(t, ctx, reviewRepository, tasks["dish-approved"].ID, review.StatusApproved, "dish approved")
	startAndDecide(t, server, administrator.Token, tasks["dish-rejected"], "reject", "dish policy rejection")
	assertDishStatus(t, ctx, catalog, dishRejected.DishPublicID, takeout.DishDraft)
	assertReviewTask(t, ctx, reviewRepository, tasks["dish-rejected"].ID, review.StatusRejected, "dish policy rejection")

	startAndExpectOK(t, server, administrator.Token, tasks["merchant-race"])
	status, body := httpJSON(t, server.Client(), http.MethodPost, reviewURL(server.URL, tasks["merchant-race"], "approve"), administrator.Token, map[string]any{"reason": "must expose CAS"})
	if status == http.StatusOK || body.Code == 0 {
		t.Fatalf("CAS race was hidden: status=%d body=%+v", status, body)
	}
	if !strings.Contains(body.Message, takeout.ErrConcurrentUpdate.Error()) {
		t.Fatalf("CAS error not observable: status=%d body=%+v", status, body)
	}
	raceMerchant, err := merchants.FindMerchant(ctx, merchantRace.MerchantPublicID)
	if err != nil {
		t.Fatal(err)
	}
	if raceMerchant.Status != takeout.MerchantPendingReview || raceMerchant.RowVersion != merchantRace.RowVersion+1 {
		t.Fatalf("race merchant=%+v", raceMerchant)
	}
	raceTask, err := reviewRepository.Find(ctx, tasks["merchant-race"].ID)
	if err != nil {
		t.Fatal(err)
	}
	if raceTask.Status != review.StatusReviewing {
		t.Fatalf("F non-atomic residue status=%s want=reviewing", raceTask.Status)
	}

	t.Log("happy_path=PASS merchant approved/rejected; dish approved/rejected(draft retained)")
	t.Logf("F_non_atomicity=EXPOSED http_status=%d target_error=%q task_status=%s merchant_status=%s logo_publish=GATEWAY_VISIBLE", status, body.Message, raceTask.Status, raceMerchant.Status)
	cleanupG6A2(t, db)
	assertG6A2Residue(t, db)
}

func createMerchant(t *testing.T, ctx context.Context, service *takeout.MerchantService, owner uint64, name string, logoAssetPublicID ...string) takeout.Merchant {
	t.Helper()
	input := takeout.CreateMerchantInput{OwnerUserID: owner, Name: name}
	if len(logoAssetPublicID) > 0 {
		input.LogoAssetPublicID = &logoAssetPublicID[0]
	}
	value, err := service.CreateMerchant(ctx, input)
	if err != nil {
		t.Fatal(err)
	}
	return value
}

func createApprovedMediaAsset(t *testing.T, ctx context.Context, db *gorm.DB, service *coremedia.Service, ownerID uint64, purpose coremedia.Purpose, idempotencyKey string) coremedia.Asset {
	t.Helper()
	asset, err := service.CreateAssetRecord(ctx, coremedia.NewAsset{OwnerID: ownerID, Purpose: purpose, IdempotencyKey: idempotencyKey})
	if err != nil {
		t.Fatal(err)
	}
	finalKey := "final/g6a2/" + asset.AssetPublicID + ".webp"
	finalVersion := "g6a2-version-" + asset.AssetPublicID
	if err := db.Model(&coremedia.Asset{}).Where("id = ?", asset.ID).Updates(map[string]any{
		"storage_stage":      coremedia.StorageStageFinal,
		"technical_status":   coremedia.TechnicalFinalReady,
		"verified_mime":      "image/webp",
		"byte_size":          2048,
		"width":              640,
		"height":             480,
		"final_key":          finalKey,
		"final_version_id":   finalVersion,
		"projected_decision": coremedia.ReviewApproved,
	}).Error; err != nil {
		t.Fatal(err)
	}
	projectionID, err := coremedia.NewULID()
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC()
	if err := db.Create(&coremedia.ReviewProjection{
		ProjectionID: projectionID, AssetID: asset.ID, Status: coremedia.ProjectionApproved,
		AppliedDecisionVersion: 1, DecidedAt: &now, RowVersion: 1, CreatedAt: now, UpdatedAt: now,
	}).Error; err != nil {
		t.Fatal(err)
	}
	return asset
}

func bindReviewTargetReference(t *testing.T, ctx context.Context, service *coremedia.Service, asset coremedia.Asset, purpose coremedia.Purpose, referenceID string, aggregateVersion uint64) {
	t.Helper()
	operationID, err := coremedia.NewULID()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := service.CreateReferenceRecord(ctx, coremedia.AssetReference{
		AssetID: asset.ID, ReferenceType: purpose, ReferenceID: referenceID, Slot: 0,
		Status: coremedia.ReferencePending, ParentVisibility: coremedia.ParentHidden,
		AggregateVersion: aggregateVersion, OperationID: operationID, RowVersion: 1,
	}); err != nil {
		t.Fatal(err)
	}
}

func assertReviewTargetMediaNotVisible(t *testing.T, ctx context.Context, gateway *coremedia.MediaGateway, purpose coremedia.Purpose, referenceID, assetPublicID string) {
	t.Helper()
	if _, err := gateway.BuildDisplayView(ctx, purpose, referenceID, 0); !errors.Is(err, coremedia.ErrMediaNotVisible) {
		t.Fatalf("media reference should be hidden before review approval reference=%s err=%v", referenceID, err)
	}
	visibility, err := gateway.CheckVisibility(ctx, assetPublicID)
	if err != nil {
		t.Fatal(err)
	}
	if visibility.Visible {
		t.Fatalf("asset should be hidden before review approval: %+v", visibility)
	}
}

func assertReviewTargetMediaVisible(t *testing.T, ctx context.Context, gateway *coremedia.MediaGateway, purpose coremedia.Purpose, referenceID, assetPublicID string) {
	t.Helper()
	view, err := gateway.BuildDisplayView(ctx, purpose, referenceID, 0)
	if err != nil {
		t.Fatalf("media reference should be visible after review approval reference=%s err=%v", referenceID, err)
	}
	if view.Media.MediaID != assetPublicID {
		t.Fatalf("visible media=%s want=%s", view.Media.MediaID, assetPublicID)
	}
	visibility, err := gateway.CheckVisibility(ctx, assetPublicID)
	if err != nil {
		t.Fatal(err)
	}
	if !visibility.Visible {
		t.Fatalf("asset should be visible after review approval: %+v", visibility)
	}
}

func createDish(t *testing.T, ctx context.Context, service *takeout.CatalogService, merchantID, name string) takeout.Dish {
	t.Helper()
	value, err := service.CreateDish(ctx, takeout.CreateDishInput{MerchantPublicID: merchantID, Name: name, PriceCents: 1000})
	if err != nil {
		t.Fatal(err)
	}
	return value
}

func startAndDecide(t *testing.T, server *httptest.Server, token string, task review.Task, decision, reason string) {
	t.Helper()
	startAndExpectOK(t, server, token, task)
	status, body := httpJSON(t, server.Client(), http.MethodPost, reviewURL(server.URL, task, decision), token, map[string]any{"reason": reason})
	if status != http.StatusOK || body.Code != 0 {
		t.Fatalf("%s task=%d status=%d body=%+v", decision, task.ID, status, body)
	}
}

func startAndExpectOK(t *testing.T, server *httptest.Server, token string, task review.Task) {
	t.Helper()
	status, body := httpJSON(t, server.Client(), http.MethodPost, reviewURL(server.URL, task, "start"), token, nil)
	if status != http.StatusOK || body.Code != 0 {
		t.Fatalf("start task=%d status=%d body=%+v", task.ID, status, body)
	}
}

func reviewURL(base string, task review.Task, action string) string {
	return fmt.Sprintf("%s/api/v1/admin/reviews/%d/%s", base, task.ID, action)
}

func assertMerchantStatus(t *testing.T, ctx context.Context, service *takeout.MerchantService, publicID string, want takeout.MerchantStatus) {
	t.Helper()
	value, err := service.FindMerchant(ctx, publicID)
	if err != nil {
		t.Fatal(err)
	}
	if value.Status != want {
		t.Fatalf("merchant %s status=%s want=%s", publicID, value.Status, want)
	}
}

func assertDishStatus(t *testing.T, ctx context.Context, service *takeout.CatalogService, publicID string, want takeout.DishStatus) {
	t.Helper()
	value, err := service.FindDish(ctx, publicID)
	if err != nil {
		t.Fatal(err)
	}
	if value.Status != want {
		t.Fatalf("dish %s status=%s want=%s", publicID, value.Status, want)
	}
}

func assertReviewTask(t *testing.T, ctx context.Context, repository *review.GormRepository, id uint, want review.Status, reason string) {
	t.Helper()
	value, err := repository.Find(ctx, id)
	if err != nil {
		t.Fatal(err)
	}
	if value.Status != want || value.Reason != reason {
		t.Fatalf("review task %d status=%s reason=%q want=%s/%q", id, value.Status, value.Reason, want, reason)
	}
}

func httpJSON(t *testing.T, client *http.Client, method, url, token string, payload any) (int, apiEnvelope) {
	t.Helper()
	body := bytes.NewReader(nil)
	if payload != nil {
		raw, err := json.Marshal(payload)
		if err != nil {
			t.Fatal(err)
		}
		body = bytes.NewReader(raw)
	}
	request, err := http.NewRequest(method, url, body)
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
	var envelope apiEnvelope
	if err := json.NewDecoder(response.Body).Decode(&envelope); err != nil {
		t.Fatal(err)
	}
	return response.StatusCode, envelope
}

func openG6A2MySQL(t *testing.T) *gorm.DB {
	t.Helper()
	dsn := os.Getenv("TAKEOUT_G6A2_MYSQL_DSN")
	if dsn == "" {
		dsn = os.Getenv("MYSQL_DSN")
	}
	if dsn == "" {
		t.Skip("set TAKEOUT_G6A2_MYSQL_DSN or MYSQL_DSN for G-6a-2 acceptance")
	}
	db, err := gorm.Open(mysql.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	return db
}

func assertG6A2Schema(t *testing.T, db *gorm.DB) {
	t.Helper()
	for _, table := range []any{&user.User{}, &review.Task{}, &takeout.Merchant{}, &takeout.Dish{}} {
		if !db.Migrator().HasTable(table) {
			t.Fatalf("required migrated table missing for %T", table)
		}
	}
}

func cleanupG6A2(t *testing.T, db *gorm.DB) {
	t.Helper()
	var assetIDs []uint64
	if err := db.Model(&coremedia.Asset{}).Where("idempotency_key LIKE ?", "g6a2-%").Pluck("id", &assetIDs).Error; err != nil {
		t.Fatal(err)
	}
	if len(assetIDs) > 0 {
		var projectionIDs []string
		if err := db.Model(&coremedia.ReviewProjection{}).Where("asset_id IN ?", assetIDs).Pluck("projection_id", &projectionIDs).Error; err != nil {
			t.Fatal(err)
		}
		if len(projectionIDs) > 0 {
			if err := db.Where("projection_id IN ?", projectionIDs).Delete(&coremedia.ReviewDecision{}).Error; err != nil {
				t.Fatal(err)
			}
		}
		if err := db.Where("asset_id IN ?", assetIDs).Delete(&coremedia.AssetReference{}).Error; err != nil {
			t.Fatal(err)
		}
		if err := db.Where("asset_id IN ?", assetIDs).Delete(&coremedia.ReviewProjection{}).Error; err != nil {
			t.Fatal(err)
		}
		if err := db.Where("id IN ?", assetIDs).Delete(&coremedia.Asset{}).Error; err != nil {
			t.Fatal(err)
		}
	}
	if err := db.Where("risk LIKE ?", "g6a2-%").Delete(&review.Task{}).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Where("name LIKE ?", testDishPrefix+"%").Delete(&takeout.Dish{}).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Where("name LIKE ?", testMerchantPrefix+"%").Delete(&takeout.Merchant{}).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Unscoped().Where("open_id IN ?", []string{"mock_openid_g6a2-ordinary", "mock_openid_g6a2-admin"}).Delete(&user.User{}).Error; err != nil {
		t.Fatal(err)
	}
}

func assertG6A2Residue(t *testing.T, db *gorm.DB) {
	t.Helper()
	checks := []struct {
		name  string
		model any
		query string
		arg   any
	}{
		{name: "review_tasks", model: &review.Task{}, query: "risk LIKE ?", arg: "g6a2-%"},
		{name: "merchants", model: &takeout.Merchant{}, query: "name LIKE ?", arg: testMerchantPrefix + "%"},
		{name: "dishes", model: &takeout.Dish{}, query: "name LIKE ?", arg: testDishPrefix + "%"},
		{name: "users", model: &user.User{}, query: "open_id IN ?", arg: []string{"mock_openid_g6a2-ordinary", "mock_openid_g6a2-admin"}},
		{name: "media_assets", model: &coremedia.Asset{}, query: "idempotency_key LIKE ?", arg: "g6a2-%"},
	}
	for _, check := range checks {
		var count int64
		if err := db.Unscoped().Model(check.model).Where(check.query, check.arg).Count(&count).Error; err != nil {
			t.Fatal(err)
		}
		if count != 0 {
			t.Errorf("%s residue=%d", check.name, count)
		}
	}
	t.Log("residue review_tasks=0 merchants=0 dishes=0 users=0 media_assets=0")
}
