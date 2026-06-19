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
	"campus-circle/backend/internal/module/admin/review"
	"campus-circle/backend/internal/module/community"
	"campus-circle/backend/internal/module/im"
	"campus-circle/backend/internal/module/navigation"
	"campus-circle/backend/internal/module/trade"
	"campus-circle/backend/internal/module/user"
	"campus-circle/backend/internal/pkg/media"
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
		&media.Asset{}, &media.AssetAudit{},
	); err != nil {
		log.Fatalf("auto migrate: %v", err)
	}

	// Composition root: repositories are private to their domain; only services cross boundaries.
	reviewService := review.NewService(review.NewRepository(db))
	userService := user.NewService(user.NewRepository(db), cfg.JWTSecret, cfg.WechatAppID, cfg.WechatAppSecret, cfg.AllowMockWechat)
	cosClient, err := media.NewTencentCOS(cfg.COSSecretID, cfg.COSSecretKey, cfg.COSBucket, cfg.COSRegion, cfg.COSAppID)
	if err != nil {
		log.Fatalf("configure Tencent COS: %v", err)
	}
	wechatMediaSafety, err := media.NewWechatSafety(cfg.WechatAppID, cfg.WechatAppSecret, cfg.WechatAPIBase)
	if err != nil {
		log.Fatalf("configure WeChat media safety: %v", err)
	}
	mediaService := media.NewService(media.NewRepository(db), cosClient, cosClient, wechatMediaSafety, reviewService, cfg.COSBucket, cfg.COSRegion, cfg.MediaPublicBaseURL)
	reviewService.RegisterTarget(mediaService)
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
	media.NewHandler(mediaService, cfg.WechatCallbackToken, userService.AuthRequired(), user.UnmutedRequired(), func(c *gin.Context) (uint, string) {
		value := user.CurrentUser(c)
		return value.ID, value.OpenID
	}).Register(api.Group("/media"))
	safety := review.NewSafetyClient(cfg)
	community.NewHandler(communityService, userService, safety).Register(api.Group("/community"))
	trade.NewHandler(tradeService, userService, safety).Register(api.Group("/trade"))
	navigation.NewHandler(navigationService).Register(api.Group("/navigation"))
	im.NewHandler(imService, userService).Register(api.Group("/im"))
	admin.NewHandler(adminService, reviewService, userService, communityService, tradeService, navigationService).Register(api)

	if err := router.Run(cfg.HTTPAddr); err != nil {
		log.Fatalf("run server: %v", err)
	}
}
