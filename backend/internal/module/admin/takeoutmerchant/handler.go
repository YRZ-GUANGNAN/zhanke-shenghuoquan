package takeoutmerchant

import (
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"

	"campus-circle/backend/internal/module/admin/review"
	takeoutreview "campus-circle/backend/internal/module/admin/takeoutconsole/reviewtarget"
	coremedia "campus-circle/backend/internal/module/media"
	"campus-circle/backend/internal/module/takeout"
	"campus-circle/backend/internal/pkg/response"
)

const merchantContextKey = "domainG.currentMerchant"

var (
	ErrMerchantNotOnboarded = errors.New("merchant onboarding required")
	ErrMerchantNotActive    = errors.New("merchant is not active")
	ErrForbiddenAsset       = errors.New("media asset is not owned by current user or has wrong purpose")
)

type Handler struct {
	identity      *takeout.MerchantIdentityService
	merchants     *takeout.MerchantService
	catalog       *takeout.CatalogService
	orders        *takeout.OrderService
	queries       *takeout.AdminQueryService
	reviews       review.Submitter
	media         *coremedia.Service
	currentUserID func(*gin.Context) uint64
}

func NewHandler(
	identity *takeout.MerchantIdentityService,
	merchants *takeout.MerchantService,
	catalog *takeout.CatalogService,
	orders *takeout.OrderService,
	queries *takeout.AdminQueryService,
	reviews review.Submitter,
	media *coremedia.Service,
	currentUserID func(*gin.Context) uint64,
) *Handler {
	return &Handler{
		identity: identity, merchants: merchants, catalog: catalog, orders: orders,
		queries: queries, reviews: reviews, media: media, currentUserID: currentUserID,
	}
}

func (h *Handler) Register(group *gin.RouterGroup) {
	group.GET("/me", h.me)
	group.POST("/onboarding", h.onboarding)

	merchant := group.Group("", h.requireActiveMerchant())
	merchant.GET("/categories", h.listCategories)
	merchant.POST("/categories", h.createCategory)
	merchant.GET("/categories/:id", h.category)
	merchant.PATCH("/categories/:id", h.updateCategory)
	merchant.GET("/dishes", h.listDishes)
	merchant.POST("/dishes", h.createDish)
	merchant.GET("/dishes/:id", h.dish)
	merchant.PATCH("/dishes/:id", h.updateDish)
	merchant.PATCH("/dishes/:id/on-shelf", h.putDishOnShelf)
	merchant.PATCH("/dishes/:id/sold-out", h.markDishSoldOut)
	merchant.PATCH("/dishes/:id/restock", h.restockDish)
	merchant.PATCH("/dishes/:id/off-shelf", h.takeDishOffShelf)
	merchant.PATCH("/dishes/:id/remove", h.removeDish)
	merchant.GET("/orders", h.listOrders)
	merchant.GET("/orders/:id", h.order)
	merchant.PATCH("/orders/:id/accept", h.acceptOrder)
	merchant.PATCH("/orders/:id/start-preparing", h.startPreparing)
	merchant.PATCH("/orders/:id/mark-ready", h.markReady)
}

func (h *Handler) me(c *gin.Context) {
	value, err := h.identity.FindByOwnerUserID(c.Request.Context(), h.currentUserID(c))
	if errors.Is(err, takeout.ErrNotFound) {
		response.OK(c, MeView{Onboarded: false})
		return
	}
	if err != nil {
		h.write(c, nil, err)
		return
	}
	view := merchantView(value)
	response.OK(c, MeView{Onboarded: true, Merchant: &view})
}

func (h *Handler) onboarding(c *gin.Context) {
	var req struct {
		Name              string  `json:"name"`
		ContactPhone      string  `json:"contactPhone"`
		Location          string  `json:"location"`
		BusinessHours     string  `json:"businessHours"`
		LogoAssetPublicID *string `json:"logoAssetPublicId"`
	}
	if c.ShouldBindJSON(&req) != nil {
		response.Fail(c, http.StatusBadRequest, "invalid request")
		return
	}
	ownerID := h.currentUserID(c)
	if existing, err := h.identity.FindByOwnerUserID(c.Request.Context(), ownerID); err == nil && blocksNewOnboarding(existing) {
		response.Fail(c, http.StatusConflict, "merchant already onboarded")
		return
	} else if err != nil && !errors.Is(err, takeout.ErrNotFound) {
		h.write(c, nil, err)
		return
	}
	logoAsset, err := h.validateMediaAsset(c, ownerID, coremedia.PurposeMerchantLogo, req.LogoAssetPublicID)
	if err != nil {
		h.write(c, nil, err)
		return
	}
	value, err := h.merchants.CreateMerchant(c.Request.Context(), takeout.CreateMerchantInput{
		OwnerUserID: ownerID, Name: req.Name, ContactPhone: req.ContactPhone,
		Location: req.Location, BusinessHours: req.BusinessHours, LogoAssetPublicID: req.LogoAssetPublicID,
	})
	if err != nil {
		h.write(c, nil, err)
		return
	}
	if err := h.bindMediaReference(c, logoAsset, coremedia.PurposeMerchantLogo, value.MerchantPublicID, value.RowVersion); err != nil {
		h.write(c, nil, err)
		return
	}
	task, err := h.reviews.Submit(c.Request.Context(), review.Submission{
		ObjectType:  takeoutreview.MerchantObjectType,
		ObjectID:    uint(value.ID),
		SubmitterID: uint(ownerID),
		Risk:        "takeout merchant onboarding",
	})
	if err != nil {
		h.write(c, nil, err)
		return
	}
	response.Created(c, OnboardingView{Merchant: merchantView(value), Review: reviewView(task)})
}

func (h *Handler) listCategories(c *gin.Context) {
	merchant := currentMerchant(c)
	page, size, ok := pagination(c)
	if !ok {
		return
	}
	values, total, err := h.queries.Categories(c.Request.Context(), merchant.MerchantPublicID, page, size)
	h.write(c, Page[CategoryView]{Items: mapValues(values, categoryView), Page: page, PageSize: size, Total: total}, err)
}

func (h *Handler) createCategory(c *gin.Context) {
	merchant := currentMerchant(c)
	var req struct {
		Name      string `json:"name"`
		SortOrder int32  `json:"sortOrder"`
	}
	if c.ShouldBindJSON(&req) != nil {
		response.Fail(c, http.StatusBadRequest, "invalid request")
		return
	}
	value, err := h.catalog.CreateCategory(c.Request.Context(), takeout.CreateCategoryInput{
		MerchantPublicID: merchant.MerchantPublicID, Name: req.Name, SortOrder: req.SortOrder,
	})
	h.writeCreated(c, categoryView(value), err)
}

func (h *Handler) category(c *gin.Context) {
	value, err := h.catalog.FindCategory(c.Request.Context(), c.Param("id"))
	if err == nil && value.MerchantPublicID != currentMerchant(c).MerchantPublicID {
		err = takeout.ErrNotFound
	}
	h.write(c, categoryView(value), err)
}

func (h *Handler) updateCategory(c *gin.Context) {
	merchant := currentMerchant(c)
	var req struct {
		Name               *string `json:"name"`
		SortOrder          *int32  `json:"sortOrder"`
		ExpectedRowVersion uint64  `json:"expectedRowVersion"`
	}
	if c.ShouldBindJSON(&req) != nil || (req.Name == nil && req.SortOrder == nil) {
		response.Fail(c, http.StatusBadRequest, "invalid request")
		return
	}
	var (
		value takeout.DishCategory
		err   error
	)
	if req.Name != nil {
		value, err = h.catalog.RenameCategory(c.Request.Context(), c.Param("id"), merchant.MerchantPublicID, req.ExpectedRowVersion, *req.Name)
		if err != nil {
			h.write(c, nil, err)
			return
		}
		req.ExpectedRowVersion = value.RowVersion
	}
	if req.SortOrder != nil {
		value, err = h.catalog.ReorderCategory(c.Request.Context(), c.Param("id"), merchant.MerchantPublicID, req.ExpectedRowVersion, *req.SortOrder)
	}
	h.write(c, categoryView(value), err)
}

func (h *Handler) listDishes(c *gin.Context) {
	merchant := currentMerchant(c)
	page, size, ok := pagination(c)
	if !ok {
		return
	}
	status := takeout.DishStatus(c.Query("status"))
	categoryID := strings.TrimSpace(c.Query("categoryId"))
	var (
		values []takeout.Dish
		total  int64
		err    error
	)
	if categoryID != "" {
		values, total, err = h.queries.DishesFiltered(c.Request.Context(), merchant.MerchantPublicID, categoryID, status, page, size)
	} else {
		values, total, err = h.queries.Dishes(c.Request.Context(), merchant.MerchantPublicID, status, page, size)
	}
	h.write(c, Page[DishView]{Items: mapValues(values, dishView), Page: page, PageSize: size, Total: total}, err)
}

func (h *Handler) createDish(c *gin.Context) {
	merchant := currentMerchant(c)
	var req dishMutationRequest
	if c.ShouldBindJSON(&req) != nil {
		response.Fail(c, http.StatusBadRequest, "invalid request")
		return
	}
	imageAsset, err := h.validateMediaAsset(c, merchant.OwnerUserID, coremedia.PurposeDish, req.ImageAssetPublicID)
	if err != nil {
		h.write(c, nil, err)
		return
	}
	value, err := h.catalog.CreateDish(c.Request.Context(), takeout.CreateDishInput{
		MerchantPublicID: merchant.MerchantPublicID, CategoryPublicID: req.CategoryPublicID,
		Name: req.Name, Description: req.Description, PriceCents: req.PriceCents,
		ImageAssetPublicID: req.ImageAssetPublicID, SortOrder: req.SortOrder,
	})
	if err != nil {
		h.write(c, nil, err)
		return
	}
	if err := h.bindMediaReference(c, imageAsset, coremedia.PurposeDish, value.DishPublicID, value.RowVersion); err != nil {
		h.write(c, nil, err)
		return
	}
	h.writeCreated(c, dishView(value), nil)
}

func (h *Handler) dish(c *gin.Context) {
	value, err := h.catalog.FindDish(c.Request.Context(), c.Param("id"))
	if err == nil && value.MerchantPublicID != currentMerchant(c).MerchantPublicID {
		err = takeout.ErrNotFound
	}
	h.write(c, dishView(value), err)
}

func (h *Handler) updateDish(c *gin.Context) {
	merchant := currentMerchant(c)
	var req dishMutationRequest
	if c.ShouldBindJSON(&req) != nil {
		response.Fail(c, http.StatusBadRequest, "invalid request")
		return
	}
	imageAsset, err := h.validateMediaAsset(c, merchant.OwnerUserID, coremedia.PurposeDish, req.ImageAssetPublicID)
	if err != nil {
		h.write(c, nil, err)
		return
	}
	value, err := h.catalog.UpdateDish(c.Request.Context(), takeout.UpdateDishInput{
		DishPublicID: c.Param("id"), MerchantPublicID: merchant.MerchantPublicID,
		ExpectedStatus: req.ExpectedStatus, ExpectedRowVersion: req.ExpectedRowVersion,
		CategoryPublicID: req.CategoryPublicID, Name: req.Name, Description: req.Description,
		PriceCents: req.PriceCents, ImageAssetPublicID: req.ImageAssetPublicID, SortOrder: req.SortOrder,
	})
	if err != nil {
		h.write(c, nil, err)
		return
	}
	if err := h.bindMediaReference(c, imageAsset, coremedia.PurposeDish, value.DishPublicID, value.RowVersion); err != nil {
		h.write(c, nil, err)
		return
	}
	h.write(c, dishView(value), nil)
}

type dishMutationRequest struct {
	CategoryPublicID   *string            `json:"categoryPublicId"`
	Name               string             `json:"name"`
	Description        string             `json:"description"`
	PriceCents         uint64             `json:"priceCents"`
	ImageAssetPublicID *string            `json:"imageAssetPublicId"`
	SortOrder          int32              `json:"sortOrder"`
	ExpectedStatus     takeout.DishStatus `json:"expectedStatus"`
	ExpectedRowVersion uint64             `json:"expectedRowVersion"`
}

func (h *Handler) putDishOnShelf(c *gin.Context) {
	var req statusRequest[takeout.DishStatus]
	if c.ShouldBindJSON(&req) != nil {
		response.Fail(c, http.StatusBadRequest, "invalid request")
		return
	}
	value, err := h.catalog.PutOnShelf(c.Request.Context(), c.Param("id"), currentMerchant(c).MerchantPublicID, req.ExpectedStatus, req.ExpectedRowVersion)
	h.write(c, dishView(value), err)
}

func (h *Handler) markDishSoldOut(c *gin.Context) {
	var req rowVersionRequest
	if c.ShouldBindJSON(&req) != nil {
		response.Fail(c, http.StatusBadRequest, "invalid request")
		return
	}
	value, err := h.catalog.MarkSoldOut(c.Request.Context(), c.Param("id"), currentMerchant(c).MerchantPublicID, req.ExpectedRowVersion)
	h.write(c, dishView(value), err)
}

func (h *Handler) restockDish(c *gin.Context) {
	var req rowVersionRequest
	if c.ShouldBindJSON(&req) != nil {
		response.Fail(c, http.StatusBadRequest, "invalid request")
		return
	}
	value, err := h.catalog.Restock(c.Request.Context(), c.Param("id"), currentMerchant(c).MerchantPublicID, req.ExpectedRowVersion)
	h.write(c, dishView(value), err)
}

func (h *Handler) takeDishOffShelf(c *gin.Context) {
	var req statusRequest[takeout.DishStatus]
	if c.ShouldBindJSON(&req) != nil {
		response.Fail(c, http.StatusBadRequest, "invalid request")
		return
	}
	value, err := h.catalog.TakeOffShelf(c.Request.Context(), c.Param("id"), currentMerchant(c).MerchantPublicID, req.ExpectedStatus, req.ExpectedRowVersion)
	h.write(c, dishView(value), err)
}

func (h *Handler) removeDish(c *gin.Context) {
	var req rowVersionRequest
	if c.ShouldBindJSON(&req) != nil {
		response.Fail(c, http.StatusBadRequest, "invalid request")
		return
	}
	value, err := h.catalog.RemoveDish(c.Request.Context(), c.Param("id"), currentMerchant(c).MerchantPublicID, req.ExpectedRowVersion)
	h.write(c, dishView(value), err)
}

func (h *Handler) listOrders(c *gin.Context) {
	merchant := currentMerchant(c)
	page, size, ok := pagination(c)
	if !ok {
		return
	}
	values, total, err := h.queries.OrdersByMerchant(c.Request.Context(), merchant.MerchantPublicID, takeout.OrderStatus(c.Query("status")), page, size)
	h.write(c, Page[OrderView]{Items: mapValues(values, orderView), Page: page, PageSize: size, Total: total}, err)
}

func (h *Handler) order(c *gin.Context) {
	value, err := h.findOwnedOrder(c)
	if err != nil {
		h.write(c, nil, err)
		return
	}
	items, err := h.orders.ListOrderItems(c.Request.Context(), value.OrderPublicID)
	h.write(c, OrderDetailView{Order: orderView(value), Items: mapValues(items, orderItemView)}, err)
}

func (h *Handler) acceptOrder(c *gin.Context) {
	h.transitionOwnedOrder(c, func(id string, rv uint64) (takeout.Order, error) {
		return h.orders.Accept(c.Request.Context(), id, rv)
	})
}

func (h *Handler) startPreparing(c *gin.Context) {
	h.transitionOwnedOrder(c, func(id string, rv uint64) (takeout.Order, error) {
		return h.orders.StartPreparing(c.Request.Context(), id, rv)
	})
}

func (h *Handler) markReady(c *gin.Context) {
	h.transitionOwnedOrder(c, func(id string, rv uint64) (takeout.Order, error) {
		return h.orders.MarkReady(c.Request.Context(), id, rv)
	})
}

func (h *Handler) transitionOwnedOrder(c *gin.Context, transition func(string, uint64) (takeout.Order, error)) {
	var req rowVersionRequest
	if c.ShouldBindJSON(&req) != nil {
		response.Fail(c, http.StatusBadRequest, "invalid request")
		return
	}
	value, err := h.findOwnedOrder(c)
	if err != nil {
		h.write(c, nil, err)
		return
	}
	value, err = transition(value.OrderPublicID, req.ExpectedRowVersion)
	h.write(c, orderView(value), err)
}

func (h *Handler) findOwnedOrder(c *gin.Context) (takeout.Order, error) {
	value, err := h.orders.FindOrder(c.Request.Context(), c.Param("id"))
	if err != nil {
		return takeout.Order{}, err
	}
	if value.MerchantPublicID != currentMerchant(c).MerchantPublicID {
		return takeout.Order{}, takeout.ErrNotFound
	}
	return value, nil
}

func (h *Handler) requireActiveMerchant() gin.HandlerFunc {
	return func(c *gin.Context) {
		value, err := h.identity.FindByOwnerUserID(c.Request.Context(), h.currentUserID(c))
		switch {
		case errors.Is(err, takeout.ErrNotFound):
			response.Fail(c, http.StatusConflict, ErrMerchantNotOnboarded.Error())
			c.Abort()
			return
		case err != nil:
			h.write(c, nil, err)
			c.Abort()
			return
		case value.Status != takeout.MerchantActive:
			response.Fail(c, http.StatusForbidden, ErrMerchantNotActive.Error())
			c.Abort()
			return
		default:
			c.Set(merchantContextKey, value)
			c.Next()
		}
	}
}

func currentMerchant(c *gin.Context) takeout.Merchant {
	value, _ := c.Get(merchantContextKey)
	merchant, _ := value.(takeout.Merchant)
	return merchant
}

func (h *Handler) validateMediaAsset(c *gin.Context, ownerID uint64, purpose coremedia.Purpose, assetPublicID *string) (*coremedia.Asset, error) {
	if assetPublicID == nil || strings.TrimSpace(*assetPublicID) == "" {
		return nil, nil
	}
	if h.media == nil {
		return nil, coremedia.ErrInvalidRecord
	}
	asset, err := h.media.FindAsset(c.Request.Context(), strings.TrimSpace(*assetPublicID))
	if err != nil {
		return nil, err
	}
	if asset.OwnerID != ownerID || asset.Purpose != purpose {
		return nil, ErrForbiddenAsset
	}
	return &asset, nil
}

func (h *Handler) bindMediaReference(c *gin.Context, asset *coremedia.Asset, purpose coremedia.Purpose, referenceID string, aggregateVersion uint64) error {
	if asset == nil {
		return nil
	}
	operationID, err := coremedia.NewULID()
	if err != nil {
		return err
	}
	_, err = h.media.CreateReferenceRecord(c.Request.Context(), coremedia.AssetReference{
		AssetID: asset.ID, ReferenceType: purpose, ReferenceID: referenceID, Slot: 0,
		Status: coremedia.ReferencePending, ParentVisibility: coremedia.ParentHidden,
		AggregateVersion: aggregateVersion, OperationID: operationID, RowVersion: 1,
	})
	return err
}

type rowVersionRequest struct {
	ExpectedRowVersion uint64 `json:"expectedRowVersion"`
}

type statusRequest[T ~string] struct {
	ExpectedStatus     T      `json:"expectedStatus"`
	ExpectedRowVersion uint64 `json:"expectedRowVersion"`
}

func pagination(c *gin.Context) (int, int, bool) {
	page, err := strconv.Atoi(c.DefaultQuery("page", "1"))
	if err != nil || page < 1 {
		response.Fail(c, http.StatusBadRequest, "invalid pagination")
		return 0, 0, false
	}
	size, err := strconv.Atoi(c.DefaultQuery("pageSize", "20"))
	if err != nil || size < 1 || size > 100 {
		response.Fail(c, http.StatusBadRequest, "invalid pagination")
		return 0, 0, false
	}
	return page, size, true
}

func blocksNewOnboarding(value takeout.Merchant) bool {
	switch value.Status {
	case takeout.MerchantPendingReview, takeout.MerchantActive, takeout.MerchantResting, takeout.MerchantSuspended:
		return true
	default:
		return false
	}
}

func (h *Handler) writeCreated(c *gin.Context, value any, err error) {
	if err != nil {
		h.write(c, nil, err)
		return
	}
	response.Created(c, value)
}

func (h *Handler) write(c *gin.Context, value any, err error) {
	switch {
	case err == nil:
		response.OK(c, value)
	case errors.Is(err, takeout.ErrNotFound):
		response.Fail(c, http.StatusNotFound, "takeout record not found")
	case errors.Is(err, ErrMerchantNotOnboarded):
		response.Fail(c, http.StatusConflict, err.Error())
	case errors.Is(err, ErrMerchantNotActive), errors.Is(err, ErrForbiddenAsset):
		response.Fail(c, http.StatusForbidden, err.Error())
	case errors.Is(err, takeout.ErrConcurrentUpdate), errors.Is(err, takeout.ErrInvalidTransition), errors.Is(err, takeout.ErrInFlightLimit), errors.Is(err, takeout.ErrOrderAlreadyClaimed):
		response.Fail(c, http.StatusConflict, err.Error())
	case errors.Is(err, takeout.ErrInvalidRecord), errors.Is(err, takeout.ErrInvalidULID), errors.Is(err, takeout.ErrAmountOverflow), errors.Is(err, coremedia.ErrInvalidRecord):
		response.Fail(c, http.StatusBadRequest, err.Error())
	default:
		response.Fail(c, http.StatusInternalServerError, "takeout merchant operation failed")
	}
}
