package trade

import (
	"campus-circle/backend/internal/module/admin/review"
	"campus-circle/backend/internal/module/user"
	"campus-circle/backend/internal/pkg/response"
	"context"
	"errors"
	"github.com/gin-gonic/gin"
	"net/http"
	"strconv"
	"strings"
)

type Handler struct {
	service *Service
	users   *user.Service
	safety  *review.SafetyClient
}

func NewHandler(service *Service, users *user.Service, safety *review.SafetyClient) *Handler {
	return &Handler{service: service, users: users, safety: safety}
}
func (h *Handler) Register(rg *gin.RouterGroup) {
	rg.GET("/products", h.list)
	rg.GET("/products/:id", h.product)
	auth := rg.Group("", h.users.AuthRequired())
	auth.GET("/my-products", h.myProducts)
	auth.GET("/products/:id/viewer-state", h.viewerState)
	auth.POST("/products", user.UnmutedRequired(), h.create)
	auth.PUT("/products/:id", user.UnmutedRequired(), h.update)
	auth.DELETE("/products/:id", h.delete)
	auth.PATCH("/products/:id/status", h.status)
	auth.POST("/products/:id/favorite", h.favorite)
	auth.POST("/products/:id/contact", h.contact)
	auth.PUT("/products/:id/favorite", h.setFavorite(true))
	auth.DELETE("/products/:id/favorite", h.setFavorite(false))
	auth.PUT("/products/:id/contact", h.ensureContact)
}
func (h *Handler) viewerState(c *gin.Context) {
	id, ok := paramID(c)
	if !ok {
		return
	}
	value, err := h.service.ViewerState(c, id, user.CurrentUserID(c))
	writeTargetState(c, value, err)
}
func (h *Handler) myProducts(c *gin.Context) {
	page, size := pagination(c)
	value, err := h.service.MyProducts(c, user.CurrentUserID(c), page, size)
	write(c, value, err)
}

type productRequest struct {
	ClientRequestID string   `json:"clientRequestId"`
	Title           string   `json:"title"`
	Description     string   `json:"description"`
	Category        string   `json:"category"`
	Price           int64    `json:"price"`
	OriginalPrice   int64    `json:"originalPrice"`
	Condition       string   `json:"condition"`
	Campus          string   `json:"campus"`
	ImageAssetIDs   []string `json:"imageAssetIds"`
}

func (h *Handler) list(c *gin.Context) {
	page, size := pagination(c)
	value, err := h.service.List(c, ListQuery{Keyword: c.Query("keyword"), Category: c.Query("category"), Status: ProductStatus(c.Query("status")), Page: page, PageSize: size})
	write(c, value, err)
}
func (h *Handler) product(c *gin.Context) {
	id, ok := paramID(c)
	if !ok {
		return
	}
	value, err := h.service.Product(c, id)
	write(c, value, err)
}
func (h *Handler) create(c *gin.Context) {
	req, ok := bindProduct(c)
	if !ok {
		return
	}
	clientRequestID, err := resolveCreateRequestID(c.GetHeader("Idempotency-Key"), req.ClientRequestID)
	if err != nil {
		response.Fail(c, http.StatusBadRequest, err.Error())
		return
	}
	input := toInput(req, user.CurrentUserID(c))
	risk := h.safety.CheckText(c, "", req.Title+"\n"+req.Description)
	if clientRequestID != "" {
		value, replayed, err := h.service.CreateKeyed(c, input, risk, clientRequestID)
		if err != nil {
			switch {
			case errors.Is(err, ErrIdempotencyConflict):
				response.Fail(c, http.StatusConflict, err.Error())
			case errors.Is(err, ErrWriteContractInvalid):
				response.Fail(c, http.StatusBadRequest, err.Error())
			case errors.Is(err, ErrWriteContractUnavailable):
				response.Fail(c, http.StatusServiceUnavailable, err.Error())
			case errors.Is(err, context.DeadlineExceeded):
				response.Fail(c, http.StatusGatewayTimeout, err.Error())
			default:
				response.Fail(c, http.StatusInternalServerError, err.Error())
			}
			return
		}
		if replayed {
			writeCreateResult(c, value, true)
			return
		}
		writeCreateResult(c, value, false)
		return
	}
	value, err := h.service.Create(c, input, risk)
	if err != nil {
		response.Fail(c, 400, err.Error())
		return
	}
	response.Created(c, value)
}

func writeCreateResult(c *gin.Context, value Product, replayed bool) {
	if replayed {
		c.Header("Idempotency-Replayed", "true")
		response.OK(c, value)
		return
	}
	response.Created(c, value)
}

func resolveCreateRequestID(headerValue, bodyValue string) (string, error) {
	headerValue = strings.TrimSpace(headerValue)
	bodyValue = strings.TrimSpace(bodyValue)
	if headerValue != "" && bodyValue != "" && headerValue != bodyValue {
		return "", errors.New("idempotency key header and body mismatch")
	}
	value := headerValue
	if value == "" {
		value = bodyValue
	}
	if value != "" && !validWriteContractULID(value) {
		return "", errors.New("invalid idempotency key")
	}
	return value, nil
}
func (h *Handler) update(c *gin.Context) {
	id, ok := paramID(c)
	if !ok {
		return
	}
	req, ok := bindProduct(c)
	if !ok {
		return
	}
	input := toInput(req, user.CurrentUserID(c))
	risk := h.safety.CheckText(c, "", req.Title+"\n"+req.Description)
	value, err := h.service.Update(c, id, input, risk)
	write(c, value, err)
}
func (h *Handler) delete(c *gin.Context) {
	id, ok := paramID(c)
	if !ok {
		return
	}
	err := h.service.Delete(c, id, user.CurrentUserID(c))
	write(c, gin.H{"deleted": err == nil}, err)
}
func (h *Handler) status(c *gin.Context) {
	id, ok := paramID(c)
	if !ok {
		return
	}
	var req struct {
		Status ProductStatus `json:"status"`
	}
	if c.ShouldBindJSON(&req) != nil {
		response.Fail(c, 400, "invalid status")
		return
	}
	err := h.service.ChangeStatus(c, id, user.CurrentUserID(c), req.Status)
	write(c, gin.H{"updated": err == nil}, err)
}
func (h *Handler) favorite(c *gin.Context) {
	id, ok := paramID(c)
	if !ok {
		return
	}
	value, err := h.service.ToggleFavorite(c, id, user.CurrentUserID(c))
	write(c, gin.H{"favorited": value}, err)
}
func (h *Handler) setFavorite(desired bool) gin.HandlerFunc {
	return func(c *gin.Context) {
		id, ok := paramID(c)
		if !ok {
			return
		}
		value, err := h.service.SetFavorite(c, id, user.CurrentUserID(c), desired)
		writeTargetState(c, gin.H{"favorited": value}, err)
	}
}
func (h *Handler) contact(c *gin.Context) {
	id, ok := paramID(c)
	if !ok {
		return
	}
	value, err := h.service.ContactSeller(c, id, user.CurrentUserID(c))
	write(c, value, err)
}
func (h *Handler) ensureContact(c *gin.Context) {
	id, ok := paramID(c)
	if !ok {
		return
	}
	value, err := h.service.EnsureSellerContact(c, id, user.CurrentUserID(c))
	writeTargetState(c, value, err)
}
func bindProduct(c *gin.Context) (productRequest, bool) {
	var req productRequest
	if c.ShouldBindJSON(&req) != nil {
		response.Fail(c, 400, "invalid product")
		return req, false
	}
	return req, true
}
func toInput(r productRequest, sellerID uint) ProductInput {
	return ProductInput{SellerID: sellerID, Title: r.Title, Description: r.Description, Category: r.Category, Price: r.Price, OriginalPrice: r.OriginalPrice, Condition: r.Condition, Campus: r.Campus, ImageAssetIDs: r.ImageAssetIDs}
}
func pagination(c *gin.Context) (int, int) {
	p, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	s, _ := strconv.Atoi(c.DefaultQuery("pageSize", "20"))
	return p, s
}
func paramID(c *gin.Context) (uint, bool) {
	v, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil || v == 0 {
		response.Fail(c, 400, "invalid id")
		return 0, false
	}
	return uint(v), true
}
func write(c *gin.Context, data any, err error) {
	if errors.Is(err, ErrNotFound) {
		response.Fail(c, http.StatusNotFound, err.Error())
		return
	}
	if errors.Is(err, ErrForbidden) {
		response.Fail(c, http.StatusForbidden, err.Error())
		return
	}
	if errors.Is(err, ErrProductSold) || errors.Is(err, ErrConflict) {
		response.Fail(c, http.StatusConflict, err.Error())
		return
	}
	if errors.Is(err, ErrCannotContactSelf) {
		response.Fail(c, http.StatusUnprocessableEntity, err.Error())
		return
	}
	if errors.Is(err, ErrContactStateUnavailable) {
		response.Fail(c, http.StatusServiceUnavailable, err.Error())
		return
	}
	if errors.Is(err, ErrSearchUnavailable) {
		response.Fail(c, http.StatusServiceUnavailable, err.Error())
		return
	}
	if errors.Is(err, context.DeadlineExceeded) {
		response.Fail(c, http.StatusGatewayTimeout, err.Error())
		return
	}
	if err != nil {
		response.Fail(c, 400, err.Error())
		return
	}
	response.OK(c, data)
}

func writeTargetState(c *gin.Context, data any, err error) {
	switch {
	case errors.Is(err, ErrNotFound):
		response.Fail(c, http.StatusNotFound, err.Error())
	case errors.Is(err, ErrForbidden):
		response.Fail(c, http.StatusForbidden, err.Error())
	case errors.Is(err, ErrProductSold), errors.Is(err, ErrConflict):
		response.Fail(c, http.StatusConflict, err.Error())
	case errors.Is(err, ErrCannotContactSelf):
		response.Fail(c, http.StatusUnprocessableEntity, err.Error())
	case errors.Is(err, ErrContactStateUnavailable), errors.Is(err, ErrWriteContractUnavailable):
		response.Fail(c, http.StatusServiceUnavailable, err.Error())
	case errors.Is(err, context.DeadlineExceeded):
		response.Fail(c, http.StatusGatewayTimeout, err.Error())
	case err != nil:
		response.Fail(c, http.StatusInternalServerError, err.Error())
	default:
		response.OK(c, data)
	}
}
