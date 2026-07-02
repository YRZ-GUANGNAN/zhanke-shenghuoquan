package main

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/gin-gonic/gin"

	"campus-circle/backend/internal/config"
	"campus-circle/backend/internal/database"
	"campus-circle/backend/internal/middleware"
	"campus-circle/backend/internal/module/admin"
	"campus-circle/backend/internal/module/admin/deliveryim"
	"campus-circle/backend/internal/module/admin/deliverymerchant"
	"campus-circle/backend/internal/module/admin/deliverynotify"
	"campus-circle/backend/internal/module/admin/deliverypayment"
	"campus-circle/backend/internal/module/admin/deliveryproof"
	"campus-circle/backend/internal/module/admin/deliveryrider"
	"campus-circle/backend/internal/module/admin/mediaconsole"
	"campus-circle/backend/internal/module/admin/review"
	"campus-circle/backend/internal/module/admin/takeoutbuyer"
	"campus-circle/backend/internal/module/admin/takeoutconsole"
	takeoutreview "campus-circle/backend/internal/module/admin/takeoutconsole/reviewtarget"
	"campus-circle/backend/internal/module/admin/takeoutmerchant"
	"campus-circle/backend/internal/module/admin/takeoutpayment"
	"campus-circle/backend/internal/module/admin/takeoutrider"
	"campus-circle/backend/internal/module/community"
	"campus-circle/backend/internal/module/delivery"
	"campus-circle/backend/internal/module/im"
	coremedia "campus-circle/backend/internal/module/media"
	"campus-circle/backend/internal/module/media/miniappflow"
	"campus-circle/backend/internal/module/navigation"
	"campus-circle/backend/internal/module/takeout"
	"campus-circle/backend/internal/module/trade"
	"campus-circle/backend/internal/module/user"
	legacymedia "campus-circle/backend/internal/pkg/media"
	"campus-circle/backend/internal/pkg/response"
	"gorm.io/gorm"
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
	// Composition root: repositories are private to their domain; only services cross boundaries.
	reviewService := review.NewService(review.NewRepository(db))
	userService := user.NewService(user.NewRepository(db), cfg.JWTSecret, cfg.WechatAppID, cfg.WechatAppSecret, cfg.AllowMockWechat, user.WithAppEnv(cfg.AppEnv))
	cosClient, err := legacymedia.NewTencentCOS(cfg.COSSecretID, cfg.COSSecretKey, cfg.COSBucket, cfg.COSRegion, cfg.COSAppID)
	if err != nil {
		log.Fatalf("configure Tencent COS: %v", err)
	}
	wechatMediaSafety, err := newWechatMediaSafety(cfg)
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
	coreMediaService := coremedia.NewService(mediaRepository)
	coreMediaGateway, err := coremedia.NewMediaGateway(mediaRepository)
	if err != nil {
		log.Fatalf("configure Module 10 media gateway: %v", err)
	}
	mediaDisplay, err := coremedia.NewMediaDisplayService(coreMediaGateway, mediaRepository, completionStore, coremedia.MediaDisplayURLTTL)
	if err != nil {
		log.Fatalf("configure Module 10 media display: %v", err)
	}
	privateProofDisplay, err := coremedia.NewMediaDisplayService(
		coreMediaGateway, mediaRepository, completionStore, delivery.PrivateProofURLTTL,
	)
	if err != nil {
		log.Fatalf("configure private delivery proof display: %v", err)
	}
	deliveryProofMedia, err := deliveryproof.NewMediaAdapter(coreMediaService, coreMediaGateway, privateProofDisplay)
	if err != nil {
		log.Fatalf("configure delivery proof media adapter: %v", err)
	}
	privateConversationDisplay, err := coremedia.NewMediaDisplayService(
		coreMediaGateway, mediaRepository, completionStore, delivery.PrivateConversationImageURLTTL,
	)
	if err != nil {
		log.Fatalf("configure private delivery conversation display: %v", err)
	}
	deliveryIMMedia, err := deliveryim.NewMediaAdapter(
		coreMediaService, coreMediaGateway, privateConversationDisplay,
	)
	if err != nil {
		log.Fatalf("configure delivery conversation media adapter: %v", err)
	}
	miniappMedia, err := miniappflow.NewService(
		uploadTickets, mediaRepository, completionService, coreMediaService,
		projectionService, mediaRepository, mediaConsumer, miniappflow.NewStatusRepository(db), nil,
		miniappflow.WithLocalMockAutoApproval(localMockWechatMode(cfg)),
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
	legacyTakeout, err := newLegacyTakeoutRuntime(cfg, db, reviewService, coreMediaService)
	if err != nil {
		log.Fatal(err)
	}
	deliveryRuntime, err := newDeliveryRuntime(cfg, db, deliveryProofMedia)
	if err != nil {
		log.Fatal(err)
	}
	if deliveryRuntime != nil {
		groupAdapter, adapterErr := deliveryim.NewGroupAdapter(imService, imImporter)
		if adapterErr != nil {
			log.Fatalf("configure delivery IM group adapter: %v", adapterErr)
		}
		conversations, conversationErr := delivery.NewConversationService(
			delivery.NewRepository(db), deliveryRuntime.orders, groupAdapter, deliveryIMMedia,
			delivery.ConversationServiceConfig{ImagesEnabled: localMockWechatMode(cfg)},
		)
		if conversationErr != nil {
			log.Fatalf("configure delivery conversation service: %v", conversationErr)
		}
		lifecycle, lifecycleErr := deliveryim.NewLifecycle(conversations)
		if lifecycleErr != nil {
			log.Fatalf("configure delivery conversation lifecycle: %v", lifecycleErr)
		}
		imageTokens, tokenErr := deliveryim.NewImageTokenCodec(cfg.JWTSecret)
		if tokenErr != nil {
			log.Fatalf("configure delivery conversation image token: %v", tokenErr)
		}
		deliveryRuntime.conversations = conversations
		deliveryRuntime.conversationLifecycle = lifecycle
		deliveryRuntime.conversationImageTokens = imageTokens
		deliveryRuntime.conversationImagesEnabled = localMockWechatMode(cfg)
		deliveryRuntime.conversationReadReceiptEnabled = cfg.EnableDeliveryIMReadReceipt
	}
	if deliveryRuntime != nil {
		go func() {
			for {
				if err := deliveryRuntime.notificationConsumer.Run(context.Background(), time.Second); err != nil {
					log.Printf("delivery notification worker stopped; retrying in 5s: %v", err)
				}
				time.Sleep(5 * time.Second)
			}
		}()
		if deliveryRuntime.proofs != nil {
			go func() {
				for {
					if err := deliveryRuntime.proofs.Run(context.Background(), time.Second); err != nil {
						log.Printf("delivery proof binding worker stopped; retrying in 5s: %v", err)
					}
					time.Sleep(5 * time.Second)
				}
			}()
		}
		if deliveryRuntime.conversations != nil {
			go func() {
				for {
					if err := deliveryRuntime.conversations.Run(context.Background(), time.Second); err != nil {
						log.Printf("delivery conversation worker stopped; retrying in 5s: %v", err)
					}
					time.Sleep(5 * time.Second)
				}
			}()
		}
	}
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
	}, mediaDisplay).Register(miniappMediaRoutes)
	safety := review.NewSafetyClient(cfg)
	community.NewHandler(communityService, userService, safety).Register(api.Group("/community"))
	trade.NewHandler(tradeService, userService, safety).Register(api.Group("/trade"))
	navigation.NewHandler(navigationService).Register(api.Group("/navigation"))
	im.NewHandler(imService, userService).Register(api.Group("/im"))
	admin.NewHandler(adminService, reviewService, userService, communityService, tradeService, navigationService).Register(api)
	mediaAdminRoutes := api.Group("/admin/media", userService.AuthRequired(), user.AdminRequired())
	mediaconsole.NewHandler(mediaAdminQuery).Register(mediaAdminRoutes)
	registerLegacyTakeoutRoutes(legacyTakeout, api, userService, reviewService, coreMediaService)
	registerDeliveryRoutes(deliveryRuntime, api, userService)

	if err := router.Run(cfg.HTTPAddr); err != nil {
		log.Fatalf("run server: %v", err)
	}
}

type deliveryRuntime struct {
	orders                         *delivery.Service
	dispatch                       *delivery.DispatchService
	funds                          *delivery.FundsService
	paymentProvider                deliverypayment.Provider
	identity                       *deliveryrider.IdentityAdapter
	notificationConsumer           *deliverynotify.Consumer
	proofs                         *delivery.ProofService
	conversations                  *delivery.ConversationService
	conversationLifecycle          *deliveryim.Lifecycle
	conversationImageTokens        *deliveryim.ImageTokenCodec
	conversationImagesEnabled      bool
	conversationReadReceiptEnabled bool
	allowMockConfirm               bool
}

func newDeliveryRuntime(cfg config.Config, db *gorm.DB, proofPorts ...delivery.MediaProofPort) (*deliveryRuntime, error) {
	if !cfg.EnableDelivery {
		return nil, nil
	}
	repository := delivery.NewRepository(db)
	orderService, err := delivery.NewService(repository, nil)
	if err != nil {
		return nil, fmt.Errorf("configure delivery order service: %w", err)
	}
	riderIdentity, err := takeout.NewRiderIdentityService(takeout.NewRepository(db), nil)
	if err != nil {
		return nil, fmt.Errorf("configure delivery rider identity: %w", err)
	}
	identityAdapter, err := deliveryrider.NewIdentityAdapter(riderIdentity)
	if err != nil {
		return nil, fmt.Errorf("configure delivery rider adapter: %w", err)
	}
	dispatchService, err := delivery.NewDispatchServiceWithFeeProvider(
		repository, orderService, identityAdapter, repository, nil,
	)
	if err != nil {
		return nil, fmt.Errorf("configure delivery dispatch service: %w", err)
	}
	sender, err := deliverynotify.NewWeComSender(cfg.WeComRiderWebhookURL)
	if err != nil {
		return nil, fmt.Errorf("configure delivery rider notification: %w", err)
	}
	consumer, err := deliverynotify.NewConsumer(repository, sender, deliverynotify.ConsumerConfig{})
	if err != nil {
		return nil, fmt.Errorf("configure delivery notification consumer: %w", err)
	}
	runtime := &deliveryRuntime{
		orders: orderService, dispatch: dispatchService, identity: identityAdapter,
		notificationConsumer: consumer,
		allowMockConfirm: cfg.AppEnv != "prod" && cfg.EnableDelivery &&
			cfg.AllowMockDeliveryFeeConfirmation,
	}
	if len(proofPorts) > 0 && proofPorts[0] != nil {
		proofService, err := delivery.NewProofService(repository, proofPorts[0], delivery.ProofServiceConfig{})
		if err != nil {
			return nil, fmt.Errorf("configure delivery proof service: %w", err)
		}
		runtime.proofs = proofService
	}
	if cfg.EnableDeliveryPayment {
		funds, err := delivery.NewFundsService(repository, repository, nil)
		if err != nil {
			return nil, fmt.Errorf("configure delivery funds service: %w", err)
		}
		provider, err := deliverypayment.NewProviderFromConfig(cfg)
		if err != nil {
			return nil, fmt.Errorf("configure delivery payment provider: %w", err)
		}
		runtime.funds = funds
		runtime.paymentProvider = provider
	}
	return runtime, nil
}

func registerDeliveryRoutes(runtime *deliveryRuntime, api *gin.RouterGroup, userService *user.Service) {
	if runtime == nil {
		return
	}
	currentUserID := func(c *gin.Context) uint64 {
		return uint64(user.CurrentUserID(c))
	}
	merchantRoutes := api.Group("/delivery/merchant", userService.AuthRequired())
	deliverymerchant.NewHandler(runtime.dispatch, currentUserID, runtime.allowMockConfirm).Register(merchantRoutes)
	var paymentHandler *deliverypayment.Handler
	if runtime.funds != nil {
		paymentHandler = deliverypayment.NewHandler(
			runtime.funds, runtime.paymentProvider, currentUserID,
			func(c *gin.Context) string { return user.CurrentUser(c).OpenID },
		)
		paymentHandler.RegisterMerchant(merchantRoutes)
		paymentHandler.RegisterWebhook(api.Group("/delivery/payments"))
	}
	riderRoutes := api.Group("/delivery/rider", userService.AuthRequired())
	riderHandler := deliveryrider.NewHandler(runtime.dispatch, runtime.identity, userService, currentUserID, runtime.proofs)
	if runtime.conversationLifecycle != nil {
		riderHandler.WithConversationCommands(runtime.conversationLifecycle)
	}
	riderHandler.Register(riderRoutes)
	if runtime.proofs != nil {
		proofRoutes := api.Group("/delivery/orders", userService.AuthRequired())
		deliveryproof.NewHandler(runtime.proofs, currentUserID).Register(proofRoutes)
	}
	if runtime.conversations != nil {
		conversationRoutes := api.Group("/delivery", userService.AuthRequired())
		deliveryim.NewHandler(
			runtime.conversations, currentUserID, runtime.conversationImageTokens,
			runtime.conversationImagesEnabled, runtime.conversationReadReceiptEnabled,
		).Register(conversationRoutes)
	}
	adminRoutes := api.Group("/admin/delivery", userService.AuthRequired(), user.AdminRequired())
	riderHandler.RegisterAdmin(adminRoutes)
	if paymentHandler != nil {
		paymentHandler.RegisterAdmin(adminRoutes)
	}
}

type legacyTakeoutRuntime struct {
	merchantService  *takeout.MerchantService
	merchantIdentity *takeout.MerchantIdentityService
	catalogService   *takeout.CatalogService
	orderService     *takeout.OrderService
	paymentService   *takeout.PaymentService
	paymentProvider  takeoutpayment.Provider
	riderService     *takeout.RiderService
	adminQuery       *takeout.AdminQueryService
	buyerQuery       *takeout.BuyerQueryService
	withdrawService  *takeout.WithdrawService
	riderTransport   *takeoutrider.Service
}

func newLegacyTakeoutRuntime(
	cfg config.Config,
	db *gorm.DB,
	reviewService *review.Service,
	coreMediaService *coremedia.Service,
) (*legacyTakeoutRuntime, error) {
	if !cfg.EnableLegacyTakeout {
		return nil, nil
	}

	takeoutRepository := takeout.NewRepository(db)
	merchantService, err := takeout.NewMerchantService(takeoutRepository, nil)
	if err != nil {
		return nil, fmt.Errorf("configure takeout merchant service: %w", err)
	}
	merchantIdentity, err := takeout.NewMerchantIdentityService(takeoutRepository)
	if err != nil {
		return nil, fmt.Errorf("configure takeout merchant identity service: %w", err)
	}
	catalogService, err := takeout.NewCatalogService(takeoutRepository, nil)
	if err != nil {
		return nil, fmt.Errorf("configure takeout catalog service: %w", err)
	}
	orderService, err := takeout.NewOrderService(takeoutRepository, takeoutRepository, nil)
	if err != nil {
		return nil, fmt.Errorf("configure takeout order service: %w", err)
	}
	paymentService, err := takeout.NewPaymentService(takeoutRepository, orderService, nil)
	if err != nil {
		return nil, fmt.Errorf("configure takeout payment service: %w", err)
	}
	paymentProvider, err := takeoutpayment.NewProviderFromConfig(cfg)
	if err != nil {
		return nil, fmt.Errorf("configure takeout payment provider: %w", err)
	}
	riderOrderOps, err := takeout.NewRiderOrderOps(takeoutRepository, nil)
	if err != nil {
		return nil, fmt.Errorf("configure takeout rider order operations: %w", err)
	}
	riderService, err := takeout.NewRiderService(takeoutRepository, riderOrderOps, nil)
	if err != nil {
		return nil, fmt.Errorf("configure takeout rider service: %w", err)
	}
	adminQuery, err := takeout.NewAdminQueryService(takeoutRepository)
	if err != nil {
		return nil, fmt.Errorf("configure takeout admin query: %w", err)
	}
	buyerQuery, err := takeout.NewBuyerQueryService(takeoutRepository)
	if err != nil {
		return nil, fmt.Errorf("configure takeout buyer query: %w", err)
	}
	withdrawService, err := takeout.NewWithdrawService(takeoutRepository, nil)
	if err != nil {
		return nil, fmt.Errorf("configure takeout withdrawal service: %w", err)
	}
	riderTransport, err := takeoutrider.NewService(riderService, orderService, withdrawService, takeoutrider.NewRepository(db))
	if err != nil {
		return nil, fmt.Errorf("configure takeout rider transport service: %w", err)
	}
	for _, target := range takeoutreview.NewTargets(merchantService, catalogService, coreMediaService) {
		reviewService.RegisterTarget(target)
	}

	return &legacyTakeoutRuntime{
		merchantService: merchantService, merchantIdentity: merchantIdentity,
		catalogService: catalogService, orderService: orderService,
		paymentService: paymentService, paymentProvider: paymentProvider,
		riderService: riderService, adminQuery: adminQuery, buyerQuery: buyerQuery,
		withdrawService: withdrawService, riderTransport: riderTransport,
	}, nil
}

func registerLegacyTakeoutRoutes(
	runtime *legacyTakeoutRuntime,
	api *gin.RouterGroup,
	userService *user.Service,
	reviewService *review.Service,
	coreMediaService *coremedia.Service,
) {
	if runtime == nil {
		return
	}

	takeoutAdminRoutes := api.Group("/admin/takeout", userService.AuthRequired(), user.AdminRequired())
	takeoutconsole.NewHandler(runtime.merchantService, runtime.catalogService, runtime.orderService, runtime.riderService, runtime.adminQuery, runtime.withdrawService).Register(takeoutAdminRoutes)
	takeoutMerchantRoutes := api.Group("/takeout/merchant", userService.AuthRequired())
	takeoutmerchant.NewHandler(runtime.merchantIdentity, runtime.merchantService, runtime.catalogService, runtime.orderService, runtime.adminQuery, reviewService, coreMediaService, func(c *gin.Context) uint64 {
		return uint64(user.CurrentUserID(c))
	}).Register(takeoutMerchantRoutes)
	takeoutBuyerRoutes := api.Group("/takeout/buyer", userService.AuthRequired(), user.UnmutedRequired())
	takeoutbuyer.NewHandler(runtime.buyerQuery, runtime.orderService, func(c *gin.Context) uint64 {
		return uint64(user.CurrentUserID(c))
	}).Register(takeoutBuyerRoutes)
	takeoutpayment.NewHandler(runtime.paymentService, runtime.paymentProvider, func(c *gin.Context) uint64 {
		return uint64(user.CurrentUserID(c))
	}, func(c *gin.Context) string {
		return user.CurrentUser(c).OpenID
	}).RegisterBuyer(takeoutBuyerRoutes)
	takeoutpayment.NewHandler(runtime.paymentService, runtime.paymentProvider, func(c *gin.Context) uint64 {
		return uint64(user.CurrentUserID(c))
	}, func(c *gin.Context) string {
		return user.CurrentUser(c).OpenID
	}).RegisterWebhook(api.Group("/takeout/payments"))
	takeoutRiderRoutes := api.Group("/takeout/rider", userService.AuthRequired())
	takeoutrider.NewHandler(runtime.riderTransport, func(c *gin.Context) uint64 {
		return uint64(user.CurrentUserID(c))
	}).Register(takeoutRiderRoutes)
}

func newWechatMediaSafety(cfg config.Config) (legacymedia.SafetyChecker, error) {
	if localMockWechatMode(cfg) {
		return legacymedia.NewLocalNoopSafety(), nil
	}
	return legacymedia.NewWechatSafety(cfg.WechatAppID, cfg.WechatAppSecret, cfg.WechatAPIBase)
}

func localMockWechatMode(cfg config.Config) bool {
	return cfg.AppEnv != "prod" && cfg.AllowMockWechat && cfg.WechatAppID == "" && cfg.WechatAppSecret == ""
}
