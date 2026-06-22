package miniappflow

import (
	"context"
	"net/http"
	"os"
	"testing"
	"time"

	"campus-circle/backend/internal/config"
	"campus-circle/backend/internal/database"
	coremedia "campus-circle/backend/internal/module/media"
	"campus-circle/backend/internal/module/user"
	"campus-circle/backend/internal/pkg/response"
	"github.com/gin-gonic/gin"
	"github.com/joho/godotenv"
)

// TestServeModule10ForWeChat is an acceptance-only composition root. It runs
// real MySQL, Tencent STS/COS and the frozen domain services without requiring
// unrelated trade/search infrastructure during WeChat Developer Tools testing.
func TestServeModule10ForWeChat(t *testing.T) {
	if os.Getenv("MEDIA_MODULE10_SERVE_WECHAT") != "1" {
		t.Skip("set MEDIA_MODULE10_SERVE_WECHAT=1 for the real WeChat acceptance server")
	}
	_ = godotenv.Load("../../../../.env")
	_ = os.Setenv("COS_SECRET_ID", module10Env("MEDIA_MODULE10_COS_SECRET_ID", "MEDIA_MODULE3_COS_SECRET_ID", "TENCENT_COS_SECRET_ID"))
	_ = os.Setenv("COS_SECRET_KEY", module10Env("MEDIA_MODULE10_COS_SECRET_KEY", "MEDIA_MODULE3_COS_SECRET_KEY", "TENCENT_COS_SECRET_KEY"))
	configuredBucket := module10Env("MEDIA_MODULE10_COS_BUCKET", "MEDIA_MODULE3_COS_BUCKET", "TENCENT_COS_BUCKET")
	region := module10Env("MEDIA_MODULE10_COS_REGION", "MEDIA_MODULE3_COS_REGION", "TENCENT_COS_REGION")
	appID := module10Env("MEDIA_MODULE10_COS_APP_ID", "MEDIA_MODULE3_COS_APP_ID", "COS_APP_ID")
	resolvedBucket := resolveModule10Bucket(t, os.Getenv("COS_SECRET_ID"), os.Getenv("COS_SECRET_KEY"), configuredBucket, region, appID)
	_ = os.Setenv("COS_BUCKET", resolvedBucket)
	_ = os.Setenv("COS_REGION", region)
	_ = os.Setenv("COS_APP_ID", appID)

	cfg, err := config.Load()
	if err != nil {
		t.Fatal(err)
	}
	db, err := database.Open(cfg)
	if err != nil {
		t.Fatal(err)
	}
	repository := coremedia.NewRepository(db)
	authorizer, err := coremedia.NewTencentUploadAuthorizer(coremedia.TencentUploadAuthorizerConfig{
		SecretID: cfg.COSSecretID, SecretKey: cfg.COSSecretKey, Bucket: cfg.COSBucket,
		Region: cfg.COSRegion, AppID: cfg.COSAppID, STSTTL: 15 * time.Minute,
	})
	if err != nil {
		t.Fatal(err)
	}
	tickets, err := coremedia.NewUploadTicketService(repository, authorizer, coremedia.UploadTicketServiceConfig{
		Environment: cfg.AppEnv, TicketTTL: 30 * time.Minute, AuthorizationTTL: 2 * time.Minute,
		Quota: coremedia.DefaultQuotaPolicy(),
	})
	if err != nil {
		t.Fatal(err)
	}
	store, err := coremedia.NewTencentCompletionStore(coremedia.TencentCompletionStoreConfig{
		SecretID: cfg.COSSecretID, SecretKey: cfg.COSSecretKey, Bucket: cfg.COSBucket, Region: cfg.COSRegion,
	})
	if err != nil {
		t.Fatal(err)
	}
	completion, err := coremedia.NewCompletionService(repository, store, coremedia.CompletionServiceConfig{Environment: cfg.AppEnv})
	if err != nil {
		t.Fatal(err)
	}
	projections, err := coremedia.NewReviewProjectionService(repository, nil)
	if err != nil {
		t.Fatal(err)
	}
	consumer, err := coremedia.NewMediaConsumer(repository, projections, coremedia.MediaConsumerConfig{})
	if err != nil {
		t.Fatal(err)
	}
	application, err := NewService(
		tickets, repository, completion, coremedia.NewService(repository), projections,
		repository, consumer, NewStatusRepository(db), nil,
	)
	if err != nil {
		t.Fatal(err)
	}
	users := user.NewService(user.NewRepository(db), cfg.JWTSecret, cfg.WechatAppID, cfg.WechatAppSecret, cfg.AllowMockWechat)
	gin.SetMode(gin.ReleaseMode)
	router := gin.New()
	router.Use(gin.Recovery())
	router.GET("/healthz", func(c *gin.Context) { response.OK(c, gin.H{"status": "ok", "scope": "module10-acceptance"}) })
	api := router.Group("/api/v1")
	user.NewHandler(users).Register(api.Group("/auth"))
	routes := api.Group("/miniapp/media", users.AuthRequired(), user.UnmutedRequired())
	NewHandler(application, func(c *gin.Context) uint64 { return uint64(user.CurrentUserID(c)) }).Register(routes)

	server := &http.Server{Addr: cfg.HTTPAddr, Handler: router, ReadHeaderTimeout: 5 * time.Second}
	done := make(chan error, 1)
	go func() { done <- server.ListenAndServe() }()
	t.Logf("Module 10 real WeChat acceptance server listening on %s", cfg.HTTPAddr)
	select {
	case err := <-done:
		if err != nil && err != http.ErrServerClosed {
			t.Fatal(err)
		}
	case <-time.After(12 * time.Minute):
		shutdown, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := server.Shutdown(shutdown); err != nil {
			t.Fatal(err)
		}
	}
}
