package miniappflow

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"campus-circle/backend/internal/module/user"
	"campus-circle/backend/internal/pkg/response"
	"github.com/gin-gonic/gin"
	"github.com/joho/godotenv"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"

	coremedia "campus-circle/backend/internal/module/media"
)

func TestResolveContentURLAgainstRealHTTPMySQLAndCOS(t *testing.T) {
	if os.Getenv("MEDIA_RESOLVE_RUN_REAL_E2E") != "1" {
		t.Skip("set MEDIA_RESOLVE_RUN_REAL_E2E=1 for real resolve URL HTTP/MySQL/COS acceptance")
	}
	_ = godotenv.Load("../../../../.env")
	db := openMediaResolveMySQL(t)
	secretID := module10Env("MEDIA_RESOLVE_COS_SECRET_ID", "MEDIA_MODULE10_COS_SECRET_ID", "MEDIA_MODULE4_COS_SECRET_ID", "TENCENT_COS_SECRET_ID", "COS_SECRET_ID")
	secretKey := module10Env("MEDIA_RESOLVE_COS_SECRET_KEY", "MEDIA_MODULE10_COS_SECRET_KEY", "MEDIA_MODULE4_COS_SECRET_KEY", "TENCENT_COS_SECRET_KEY", "COS_SECRET_KEY")
	bucket := module10Env("MEDIA_RESOLVE_COS_BUCKET", "MEDIA_MODULE10_COS_BUCKET", "MEDIA_MODULE4_COS_BUCKET", "TENCENT_COS_BUCKET", "COS_BUCKET")
	region := module10Env("MEDIA_RESOLVE_COS_REGION", "MEDIA_MODULE10_COS_REGION", "MEDIA_MODULE4_COS_REGION", "TENCENT_COS_REGION", "COS_REGION")
	appID := module10Env("MEDIA_RESOLVE_COS_APP_ID", "MEDIA_MODULE10_COS_APP_ID", "MEDIA_MODULE4_COS_APP_ID", "COS_APP_ID")
	if appID == "" && strings.Contains(bucket, "-") {
		appID = bucket[strings.LastIndex(bucket, "-")+1:]
	}
	if secretID == "" || secretKey == "" || bucket == "" || region == "" || appID == "" {
		t.Fatal("real media resolve COS credentials are incomplete")
	}
	bucket = resolveModule10Bucket(t, secretID, secretKey, bucket, region, appID)
	store, err := coremedia.NewTencentCompletionStore(coremedia.TencentCompletionStoreConfig{
		SecretID: secretID, SecretKey: secretKey, Bucket: bucket, Region: region,
	})
	if err != nil {
		t.Fatal(err)
	}
	cosClient := module10COSClient(t, secretID, secretKey, bucket, region)
	ownerID := uint64(99_007_001)
	cleanupMediaResolve(t, db, ownerID)
	t.Cleanup(func() { cleanupMediaResolve(t, db, ownerID) })

	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	finalKey := fmt.Sprintf("final/resolve-acceptance/%d/%d.txt", ownerID, time.Now().UnixNano())
	payload := []byte("media-resolve-url-acceptance")
	t.Cleanup(func() { cleanupModule10COSKey(t, cosClient, finalKey) })
	putResponse, err := cosClient.Object.Put(ctx, finalKey, bytes.NewReader(payload), nil)
	if err != nil {
		t.Fatalf("real COS PUT resolve fixture: %v", err)
	}
	if putResponse != nil && putResponse.Body != nil {
		defer putResponse.Body.Close()
	}
	finalVersionID := putResponse.Header.Get("x-cos-version-id")
	if finalVersionID == "" {
		t.Fatal("real COS bucket versioning is required for resolve URL fixed-version GET")
	}

	visibleReferenceID := "resolve-dish-visible"
	hiddenReferenceID := "resolve-dish-hidden"
	seedMediaResolveReference(t, db, ownerID, finalKey, finalVersionID, visibleReferenceID, coremedia.ParentVisible)
	seedMediaResolveReference(t, db, ownerID, finalKey+".hidden", finalVersionID+"-hidden", hiddenReferenceID, coremedia.ParentHidden)

	repository := coremedia.NewRepository(db)
	gateway, err := coremedia.NewMediaGateway(repository)
	if err != nil {
		t.Fatal(err)
	}
	display, err := coremedia.NewMediaDisplayService(gateway, repository, store, coremedia.MediaDisplayURLTTL)
	if err != nil {
		t.Fatal(err)
	}
	users := user.NewService(user.NewRepository(db), "media-resolve-test-secret-that-is-longer-than-thirty-two-characters", "", "", true)
	login, err := users.WechatLogin(ctx, user.LoginInput{Code: "media-resolve-user", Nickname: "Media Resolve User"})
	if err != nil {
		t.Fatal(err)
	}
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.GET("/healthz", func(c *gin.Context) { response.OK(c, gin.H{"status": "ok"}) })
	group := router.Group("/api/v1/miniapp/media", users.AuthRequired(), user.UnmutedRequired())
	NewHandler(nil, func(c *gin.Context) uint64 { return uint64(user.CurrentUserID(c)) }, display).Register(group)
	server := httptest.NewServer(router)
	defer server.Close()

	noTokenStatus, _ := mediaResolveJSON(t, server.Client(), server.URL+"/api/v1/miniapp/media/resolve/dish/"+visibleReferenceID+"/0", "")
	if noTokenStatus != http.StatusUnauthorized {
		t.Fatalf("no token status=%d want=401", noTokenStatus)
	}
	invalidStatus, _ := mediaResolveJSON(t, server.Client(), server.URL+"/api/v1/miniapp/media/resolve/invalid/"+visibleReferenceID+"/0", login.Token)
	if invalidStatus != http.StatusBadRequest {
		t.Fatalf("invalid reference status=%d want=400", invalidStatus)
	}
	hiddenStatus, _ := mediaResolveJSON(t, server.Client(), server.URL+"/api/v1/miniapp/media/resolve/dish/"+hiddenReferenceID+"/0", login.Token)
	if hiddenStatus != http.StatusNotFound {
		t.Fatalf("hidden reference status=%d want=404", hiddenStatus)
	}
	missingStatus, _ := mediaResolveJSON(t, server.Client(), server.URL+"/api/v1/miniapp/media/resolve/dish/resolve-dish-missing/0", login.Token)
	if missingStatus != http.StatusNotFound {
		t.Fatalf("missing reference status=%d want=404", missingStatus)
	}

	status, envelope := mediaResolveJSON(t, server.Client(), server.URL+"/api/v1/miniapp/media/resolve/dish/"+visibleReferenceID+"/0", login.Token)
	if status != http.StatusOK || envelope.Code != 0 || envelope.Data.URL == "" || envelope.Data.ExpiresInSeconds != int64(coremedia.MediaDisplayURLTTL.Seconds()) {
		t.Fatalf("resolve status=%d envelope=%+v", status, envelope)
	}
	if !strings.Contains(envelope.Data.URL, "q-signature=") || !strings.Contains(envelope.Data.URL, "versionId=") {
		t.Fatalf("resolved URL is not a fixed-version COS signed URL: %s", redactResolveURL(envelope.Data.URL))
	}
	if envelope.Data.FinalKey != "" || envelope.Data.FinalVersionID != "" {
		t.Fatalf("storage identity leaked: %+v", envelope.Data)
	}
	body := fetchResolvedURL(t, ctx, envelope.Data.URL)
	if string(body) != string(payload) {
		t.Fatalf("resolved URL body=%q want=%q", string(body), string(payload))
	}

	request, err := http.NewRequest(http.MethodGet, server.URL+"/api/v1/miniapp/media/display/dish/"+visibleReferenceID+"/0/content", nil)
	if err != nil {
		t.Fatal(err)
	}
	request.Header.Set("Authorization", "Bearer "+login.Token)
	noRedirectClient := server.Client()
	noRedirectClient.CheckRedirect = func(*http.Request, []*http.Request) error {
		return http.ErrUseLastResponse
	}
	redirectResponse, err := noRedirectClient.Do(request)
	if err != nil {
		t.Fatal(err)
	}
	defer redirectResponse.Body.Close()
	if redirectResponse.StatusCode != http.StatusFound || !strings.Contains(redirectResponse.Header.Get("Location"), "q-signature=") {
		t.Fatalf("display redirect status=%d location=%s", redirectResponse.StatusCode, redactResolveURL(redirectResponse.Header.Get("Location")))
	}
	cleanupMediaResolve(t, db, ownerID)
	assertMediaResolveResidue(t, db, ownerID)
	t.Log("real_http=PASS auth=401 visible_resolve=200 hidden=404 invalid=400 signed_cos_get=PASS display_302_unchanged=PASS db_residue=0")
}

type mediaResolveEnvelope struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Data    struct {
		URL              string `json:"url"`
		ExpiresInSeconds int64  `json:"expiresInSeconds"`
		FinalKey         string `json:"finalKey"`
		FinalVersionID   string `json:"finalVersionId"`
	} `json:"data"`
}

func mediaResolveJSON(t *testing.T, client *http.Client, url, token string) (int, mediaResolveEnvelope) {
	t.Helper()
	request, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		t.Fatal(err)
	}
	if token != "" {
		request.Header.Set("Authorization", "Bearer "+token)
	}
	response, err := client.Do(request)
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	var envelope mediaResolveEnvelope
	_ = json.NewDecoder(response.Body).Decode(&envelope)
	return response.StatusCode, envelope
}

func fetchResolvedURL(t *testing.T, ctx context.Context, url string) []byte {
	t.Helper()
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		t.Fatal(err)
	}
	response, err := (&http.Client{Timeout: 30 * time.Second}).Do(request)
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(response.Body, 1024))
	if response.StatusCode != http.StatusOK {
		t.Fatalf("signed URL status=%d body=%q", response.StatusCode, strings.TrimSpace(string(body)))
	}
	return body
}

func openMediaResolveMySQL(t *testing.T) *gorm.DB {
	t.Helper()
	dsn := strings.TrimSpace(os.Getenv("MEDIA_RESOLVE_MYSQL_DSN"))
	if dsn == "" {
		dsn = strings.TrimSpace(os.Getenv("MYSQL_DSN"))
	}
	if dsn == "" {
		t.Fatal("set MEDIA_RESOLVE_MYSQL_DSN or MYSQL_DSN for real media resolve acceptance")
	}
	db, err := gorm.Open(mysql.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	for _, table := range []any{&coremedia.Asset{}, &coremedia.AssetReference{}, &coremedia.ReviewProjection{}, &user.User{}} {
		if !db.Migrator().HasTable(table) {
			t.Fatalf("required table missing for %T", table)
		}
	}
	return db
}

func seedMediaResolveReference(t *testing.T, db *gorm.DB, ownerID uint64, finalKey, finalVersionID, referenceID string, visibility coremedia.ParentVisibility) {
	t.Helper()
	now := time.Now().UTC()
	assetPublicID, _ := coremedia.NewULID()
	projectionID, _ := coremedia.NewULID()
	operationID, _ := coremedia.NewULID()
	hash := strings.Repeat("b", 64)
	asset := coremedia.Asset{
		AssetPublicID: assetPublicID, OwnerID: ownerID, Purpose: coremedia.PurposeDish,
		StorageStage: coremedia.StorageStageFinal, ReviewStatus: coremedia.ReviewPending, TechnicalStatus: coremedia.TechnicalFinalReady,
		TrustedSHA256: &hash, VerifiedMIME: "image/webp", ByteSize: 32, Width: 1, Height: 1,
		FinalKey: &finalKey, FinalVersionID: &finalVersionID, ProjectedDecision: coremedia.ReviewPending,
		IdempotencyKey: "media-resolve-" + assetPublicID, RowVersion: 1, CreatedAt: now, UpdatedAt: now,
	}
	if err := db.Create(&asset).Error; err != nil {
		t.Fatal(err)
	}
	projection := coremedia.ReviewProjection{
		ProjectionID: projectionID, AssetID: asset.ID, Status: coremedia.ProjectionApproved,
		AppliedDecisionVersion: 1, LastDecisionHash: &hash, DecidedAt: &now,
		RowVersion: 1, CreatedAt: now, UpdatedAt: now,
	}
	if err := db.Create(&projection).Error; err != nil {
		t.Fatal(err)
	}
	reference := coremedia.AssetReference{
		AssetID: asset.ID, ReferenceType: coremedia.PurposeDish, ReferenceID: referenceID, Slot: 0,
		Status: coremedia.ReferenceActive, ParentVisibility: visibility, AggregateVersion: 1,
		OperationID: operationID, RowVersion: 1, CreatedAt: now, UpdatedAt: now,
	}
	if err := db.Create(&reference).Error; err != nil {
		t.Fatal(err)
	}
}

func cleanupMediaResolve(t *testing.T, db *gorm.DB, ownerID uint64) {
	t.Helper()
	var assetIDs []uint64
	if err := db.Model(&coremedia.Asset{}).Where("owner_id = ?", ownerID).Pluck("id", &assetIDs).Error; err != nil {
		t.Errorf("find media resolve assets: %v", err)
	}
	if len(assetIDs) > 0 {
		projectionSubquery := db.Model(&coremedia.ReviewProjection{}).Select("projection_id").Where("asset_id IN ?", assetIDs)
		if err := db.Where("projection_id IN (?)", projectionSubquery).Delete(&coremedia.ReviewDecision{}).Error; err != nil {
			t.Errorf("cleanup media resolve decisions: %v", err)
		}
		if err := db.Where("asset_id IN ?", assetIDs).Delete(&coremedia.AssetReference{}).Error; err != nil {
			t.Errorf("cleanup media resolve references: %v", err)
		}
		if err := db.Where("asset_id IN ?", assetIDs).Delete(&coremedia.ReviewProjection{}).Error; err != nil {
			t.Errorf("cleanup media resolve projections: %v", err)
		}
		if err := db.Where("id IN ?", assetIDs).Delete(&coremedia.Asset{}).Error; err != nil {
			t.Errorf("cleanup media resolve assets: %v", err)
		}
	}
	db.Unscoped().Where("open_id = ?", "mock_openid_media-resolve-user").Delete(&user.User{})
}

func assertMediaResolveResidue(t *testing.T, db *gorm.DB, ownerID uint64) {
	t.Helper()
	var assetCount, referenceCount, projectionCount, userCount int64
	assetSubquery := db.Model(&coremedia.Asset{}).Select("id").Where("owner_id = ?", ownerID)
	if err := db.Model(&coremedia.Asset{}).Where("owner_id = ?", ownerID).Count(&assetCount).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Model(&coremedia.AssetReference{}).Where("asset_id IN (?)", assetSubquery).Count(&referenceCount).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Model(&coremedia.ReviewProjection{}).Where("asset_id IN (?)", assetSubquery).Count(&projectionCount).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Unscoped().Model(&user.User{}).Where("open_id = ?", "mock_openid_media-resolve-user").Count(&userCount).Error; err != nil {
		t.Fatal(err)
	}
	if assetCount != 0 || referenceCount != 0 || projectionCount != 0 || userCount != 0 {
		t.Fatalf("media resolve DB residue assets=%d references=%d projections=%d users=%d", assetCount, referenceCount, projectionCount, userCount)
	}
}

func redactResolveURL(value string) string {
	if value == "" {
		return ""
	}
	if index := strings.Index(value, "?"); index >= 0 {
		return value[:index] + "?<redacted>"
	}
	return value
}
