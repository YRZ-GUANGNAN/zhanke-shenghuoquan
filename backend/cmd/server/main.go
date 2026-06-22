package main

import (
	"context"
	"log"
	"time"

	"github.com/gin-gonic/gin"

	"campus-circle/backend/internal/config"
	"campus-circle/backend/internal/database"
	"campus-circle/backend/internal/middleware"
	"campus-circle/backend/internal/module/admin"
	"campus-circle/backend/internal/module/admin/mediaconsole"
	"campus-circle/backend/internal/module/admin/review"
	"campus-circle/backend/internal/module/community"
	"campus-circle/backend/internal/module/im"
	coremedia "campus-circle/backend/internal/module/media"
	"campus-circle/backend/internal/module/media/miniappflow"
	"campus-circle/backend/internal/module/navigation"
	"campus-circle/backend/internal/module/trade"
	"campus-circle/backend/internal/module/user"
	legacymedia "campus-circle/backend/internal/pkg/media"
	"campus-circle/backend/internal/pkg/response"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("load configuration: %v", err)
	}
	db, err := database.Open(cfg)
	if err != nil {
		log.Fatalf("open database: %v", err)
	}
	if err := db.AutoMigrate(
		&user.User{},
		&community.Post{}, &community.PostImage{}, &community.Comment{}, &community.PostLike{}, &community.PostFavorite{}, &community.UserFollow{}, &community.Notification{},
		&trade.Product{}, &trade.ProductImage{}, &trade.ProductFavorite{}, &trade.SearchOutbox{},
		&navigation.Category{}, &navigation.Item{},
		&im.UserMapping{}, &im.Conversation{},
		&review.Task{}, &admin.Report{},
		&legacymedia.Asset{}, &legacymedia.AssetAudit{},
	); err != nil {
		log.Fatalf("auto migrate: %v", err)
	}

	// Composition root: repositories are private to their domain; only services cross boundaries.
	reviewService := review.NewService(review.NewRepository(db))
	userService := user.NewService(user.NewRepository(db), cfg.JWTSecret, cfg.WechatAppID, cfg.WechatAppSecret, cfg.AllowMockWechat)
	cosClient, err := legacymedia.NewTencentCOS(cfg.COSSecretID, cfg.COSSecretKey, cfg.COSBucket, cfg.COSRegion, cfg.COSAppID)
	if err != nil {
		log.Fatalf("configure Tencent COS: %v", err)
	}
	wechatMediaSafety, err := legacymedia.NewWechatSafety(cfg.WechatAppID, cfg.WechatAppSecret, cfg.WechatAPIBase)
	if err != nil {
		log.Fatalf("configure WeChat media safety: %v", err)
	}
	mediaService := legacymedia.NewService(legacymedia.NewRepository(db), cosClient, cosClient, wechatMediaSafety, reviewService, cfg.COSBucket, cfg.COSRegion, cfg.MediaPublicBaseURL)
	reviewService.RegisterTarget(mediaService)
	mediaRepository := coremedia.NewRepository(db)
	uploadAuthorizer, err := coremedia.NewTencentUploadAuthorizer(coremedia.TencentUploadAuthorizerConfig{
		SecretID: cfg.COSSecretID, SecretKey: cfg.COSSecretKey, Bucket: cfg.COSBucket,
		Region: cfg.COSRegion, AppID: cfg.COSAppID, STSTTL: 15 * time.Minute,
	})
	if err != nil {
		log.Fatalf("configure Module 10 upload authorizer: %v", err)
	}
	uploadTickets, err := coremedia.NewUploadTicketService(mediaRepository, uploadAuthorizer, coremedia.UploadTicketServiceConfig{
		Environment: cfg.AppEnv, TicketTTL: 30 * time.Minute, AuthorizationTTL: 2 * time.Minute,
		Quota: coremedia.DefaultQuotaPolicy(),
	})
	if err != nil {
		log.Fatalf("configure Module 10 upload tickets: %v", err)
	}
	completionStore, err := coremedia.NewTencentCompletionStore(coremedia.TencentCompletionStoreConfig{
		SecretID: cfg.COSSecretID, SecretKey: cfg.COSSecretKey, Bucket: cfg.COSBucket, Region: cfg.COSRegion,
	})
	if err != nil {
		log.Fatalf("configure Module 10 completion store: %v", err)
	}
	completionService, err := coremedia.NewCompletionService(mediaRepository, completionStore, coremedia.CompletionServiceConfig{Environment: cfg.AppEnv})
	if err != nil {
		log.Fatalf("configure Module 10 completion service: %v", err)
	}
	projectionService, err := coremedia.NewReviewProjectionService(mediaRepository, nil)
	if err != nil {
		log.Fatalf("configure Module 10 review projection: %v", err)
	}
	mediaConsumer, err := coremedia.NewMediaConsumer(mediaRepository, projectionService, coremedia.MediaConsumerConfig{})
	if err != nil {
		log.Fatalf("configure Module 10 media consumer: %v", err)
	}
	miniappMedia, err := miniappflow.NewService(
		uploadTickets, mediaRepository, completionService, coremedia.NewService(mediaRepository),
		projectionService, mediaRepository, mediaConsumer, miniappflow.NewStatusRepository(db), nil,
	)
	if err != nil {
		log.Fatalf("configure Module 10 miniapp flow: %v", err)
	}
	imImporter, err := im.NewTencentRESTClient(cfg.TencentIMSDKAppID, cfg.TencentIMSecret, cfg.TencentIMAdmin, cfg.TencentIMEndpoint)
	if err != nil {
		log.Fatalf("configure Tencent IM: %v", err)
	}
	imService := im.NewService(im.NewRepository(db), imImporter, cfg.TencentIMSDKAppID, cfg.TencentIMSecret)
	searchIndex, err := trade.NewMeiliSearch(cfg.MeiliHost, cfg.MeiliMasterKey, cfg.MeiliIndex)
	if err != nil {
		log.Fatalf("configure Meilisearch: %v", err)
	}
	searchContext, cancelSearch := context.WithTimeout(context.Background(), 15*time.Second)
	if err := searchIndex.Configure(searchContext); err != nil {
		cancelSearch()
		log.Fatalf("initialize Meilisearch: %v", err)
	}
	cancelSearch()
	communityService := community.NewService(community.NewRepository(db), userService, reviewService, mediaService)
	tradeService := trade.NewService(trade.NewRepository(db), userService, reviewService, imService, searchIndex, mediaService)
	go func() {
		for {
			if err := tradeService.RunSearchWorker(context.Background(), time.Second); err != nil {
				log.Printf("trade search worker stopped; retrying in 5s: %v", err)
			}
			time.Sleep(5 * time.Second)
		}
	}()
	go func() {
		for {
			if err := mediaService.RunMaintenance(context.Background(), time.Minute); err != nil {
				log.Printf("media maintenance worker stopped; retrying in 5s: %v", err)
			}
			time.Sleep(5 * time.Second)
		}
	}()
	navigationService := navigation.NewService(navigation.NewRepository(db))
	adminService := admin.NewService(admin.NewRepository(db), reviewService)
	mediaAdminQuery, err := mediaconsole.NewQueryService(mediaconsole.NewQueryRepository(db))
	if err != nil {
		log.Fatalf("configure media admin console: %v", err)
	}
	for _, target := range community.NewReviewTargets(communityService) {
		reviewService.RegisterTarget(target)
	}
	reviewService.RegisterTarget(tradeService)
	reviewService.RegisterTarget(adminService)

	if cfg.AppEnv == "prod" {
		gin.SetMode(gin.ReleaseMode)
	}

	router := gin.Default()
	router.Use(middleware.CORS())
	router.GET("/healthz", func(c *gin.Context) {
		response.OK(c, gin.H{"status": "ok"})
	})

	api := router.Group("/api/v1")
	user.NewHandler(userService, mediaService).Register(api.Group("/auth"))
	legacymedia.NewHandler(mediaService, cfg.WechatCallbackToken, userService.AuthRequired(), user.UnmutedRequired(), func(c *gin.Context) (uint, string) {
		value := user.CurrentUser(c)
		return value.ID, value.OpenID
	}).Register(api.Group("/media"))
	miniappMediaRoutes := api.Group("/miniapp/media", userService.AuthRequired(), user.UnmutedRequired())
	miniappflow.NewHandler(miniappMedia, func(c *gin.Context) uint64 {
		return uint64(user.CurrentUserID(c))
	}).Register(miniappMediaRoutes)
	safety := review.NewSafetyClient(cfg)
	community.NewHandler(communityService, userService, safety).Register(api.Group("/community"))
	trade.NewHandler(tradeService, userService, safety).Register(api.Group("/trade"))
	navigation.NewHandler(navigationService).Register(api.Group("/navigation"))
	im.NewHandler(imService, userService).Register(api.Group("/im"))
	admin.NewHandler(adminService, reviewService, userService, communityService, tradeService, navigationService).Register(api)
	mediaAdminRoutes := api.Group("/admin/media", userService.AuthRequired(), user.AdminRequired())
	mediaconsole.NewHandler(mediaAdminQuery).Register(mediaAdminRoutes)

	if err := router.Run(cfg.HTTPAddr); err != nil {
		log.Fatalf("run server: %v", err)
	}
}
