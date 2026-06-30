package deliveryrider

import (
	"context"
	"errors"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"campus-circle/backend/internal/module/delivery"
	"campus-circle/backend/internal/module/takeout"
	"campus-circle/backend/internal/module/user"
	"campus-circle/backend/internal/pkg/response"
)

type Handler struct {
	service       *delivery.DispatchService
	identity      *IdentityAdapter
	users         *user.Service
	currentUserID func(*gin.Context) uint64
	proofs        proofCommands
}

type proofCommands interface {
	AdvanceWithProof(context.Context, delivery.ProofTransitionInput) (delivery.Order, delivery.Proof, error)
}

func NewHandler(
	service *delivery.DispatchService,
	identity *IdentityAdapter,
	users *user.Service,
	currentUserID func(*gin.Context) uint64,
	proofs ...proofCommands,
) *Handler {
	handler := &Handler{service: service, identity: identity, users: users, currentUserID: currentUserID}
	if len(proofs) > 0 {
		handler.proofs = proofs[0]
	}
	return handler
}

func (h *Handler) Register(group *gin.RouterGroup) {
	group.GET("/me", h.me)
	group.GET("/orders/claimable", h.claimable)
	group.POST("/orders/:id/claim", h.claim)
	group.GET("/orders", h.list)
	group.GET("/orders/:id", h.detail)
	group.PATCH("/orders/:id/to-pickup", h.toPickup)
	group.PATCH("/orders/:id/picked-up", h.pickedUp)
	group.PATCH("/orders/:id/delivering", h.delivering)
	group.PATCH("/orders/:id/delivered", h.delivered)
	group.PATCH("/orders/:id/release", h.release)
}

func (h *Handler) RegisterAdmin(group *gin.RouterGroup) {
	group.POST("/riders", h.approveRider)
}

func (h *Handler) me(c *gin.Context) {
	value, err := h.identity.Find(c.Request.Context(), h.currentUserID(c))
	if errors.Is(err, takeout.ErrNotFound) {
		response.OK(c, MeView{Registered: false, Message: "请联系管理员开通骑手权限"})
		return
	}
	if err != nil {
		h.write(c, nil, err)
		return
	}
	view := riderView(value)
	response.OK(c, MeView{Registered: true, Rider: &view})
}

func (h *Handler) approveRider(c *gin.Context) {
	var req struct {
		RiderUserID uint64 `json:"riderUserId"`
		RealName    string `json:"realName"`
		Phone       string `json:"phone"`
	}
	if c.ShouldBindJSON(&req) != nil {
		response.Fail(c, http.StatusBadRequest, "invalid request")
		return
	}
	if h.users == nil {
		response.Fail(c, http.StatusInternalServerError, "delivery rider identity unavailable")
		return
	}
	if _, err := h.users.Get(c.Request.Context(), uint(req.RiderUserID)); err != nil {
		if errors.Is(err, user.ErrNotFound) {
			response.Fail(c, http.StatusNotFound, "user not found")
		} else {
			response.Fail(c, http.StatusInternalServerError, "delivery rider identity lookup failed")
		}
		return
	}
	value, err := h.identity.RegisterApproved(c.Request.Context(), req.RiderUserID, req.RealName, req.Phone)
	if err != nil {
		h.write(c, nil, err)
		return
	}
	response.Created(c, riderView(value))
}

func (h *Handler) claimable(c *gin.Context) {
	page, ok := parsePage(c)
	if !ok {
		return
	}
	values, total, err := h.service.ClaimableOrders(c.Request.Context(), h.currentUserID(c), page)
	if err != nil {
		h.write(c, nil, err)
		return
	}
	items := make([]ClaimableOrderView, 0, len(values))
	for _, value := range values {
		items = append(items, claimableView(value))
	}
	response.OK(c, Page[ClaimableOrderView]{Items: items, Page: page.Page, PageSize: page.Size, Total: total})
}

type rowVersionRequest struct {
	ExpectedRowVersion uint64 `json:"expectedRowVersion"`
}

func (h *Handler) claim(c *gin.Context) {
	var req rowVersionRequest
	if c.ShouldBindJSON(&req) != nil {
		response.Fail(c, http.StatusBadRequest, "invalid request")
		return
	}
	value, err := h.service.ClaimOrder(
		c.Request.Context(), h.currentUserID(c), c.Param("id"), req.ExpectedRowVersion,
	)
	h.write(c, assignedView(value), err)
}

func (h *Handler) list(c *gin.Context) {
	page, ok := parsePage(c)
	if !ok {
		return
	}
	values, total, err := h.service.RiderOrders(c.Request.Context(), h.currentUserID(c), page)
	if err != nil {
		h.write(c, nil, err)
		return
	}
	items := make([]AssignedOrderView, 0, len(values))
	for _, value := range values {
		items = append(items, assignedView(value))
	}
	response.OK(c, Page[AssignedOrderView]{Items: items, Page: page.Page, PageSize: page.Size, Total: total})
}

func (h *Handler) detail(c *gin.Context) {
	value, err := h.service.RiderOrder(c.Request.Context(), h.currentUserID(c), c.Param("id"))
	h.write(c, assignedView(value), err)
}

func (h *Handler) toPickup(c *gin.Context) {
	h.transition(c, h.service.StartToPickup)
}

func (h *Handler) pickedUp(c *gin.Context) {
	h.proofTransition(c, delivery.ProofPickup)
}

func (h *Handler) delivering(c *gin.Context) {
	h.transition(c, h.service.StartDelivering)
}

func (h *Handler) delivered(c *gin.Context) {
	h.proofTransition(c, delivery.ProofDelivery)
}

func (h *Handler) proofTransition(c *gin.Context, proofType delivery.ProofType) {
	if h.proofs == nil {
		response.Fail(c, http.StatusServiceUnavailable, "delivery proof service unavailable")
		return
	}
	var req struct {
		ExpectedRowVersion uint64 `json:"expectedRowVersion"`
		ProofAssetPublicID string `json:"proofAssetPublicId"`
	}
	if c.ShouldBindJSON(&req) != nil || req.ProofAssetPublicID == "" {
		response.Fail(c, http.StatusBadRequest, "delivery proof is required")
		return
	}
	value, _, err := h.proofs.AdvanceWithProof(c.Request.Context(), delivery.ProofTransitionInput{
		OrderPublicID: c.Param("id"), RiderUserID: h.currentUserID(c),
		ExpectedRowVersion: req.ExpectedRowVersion, ProofType: proofType,
		AssetPublicID: req.ProofAssetPublicID,
	})
	h.write(c, assignedView(value), err)
}

func (h *Handler) transition(
	c *gin.Context,
	command func(context.Context, uint64, string, uint64) (delivery.Order, error),
) {
	var req rowVersionRequest
	if c.ShouldBindJSON(&req) != nil {
		response.Fail(c, http.StatusBadRequest, "invalid request")
		return
	}
	value, err := command(
		c.Request.Context(), h.currentUserID(c), c.Param("id"), req.ExpectedRowVersion,
	)
	h.write(c, assignedView(value), err)
}

func (h *Handler) release(c *gin.Context) {
	var req struct {
		ExpectedStatus     delivery.Status `json:"expectedStatus"`
		ExpectedRowVersion uint64          `json:"expectedRowVersion"`
	}
	if c.ShouldBindJSON(&req) != nil {
		response.Fail(c, http.StatusBadRequest, "invalid request")
		return
	}
	value, _, err := h.service.ReleaseToPool(
		c.Request.Context(), h.currentUserID(c), c.Param("id"),
		req.ExpectedStatus, req.ExpectedRowVersion,
	)
	h.write(c, assignedView(value), err)
}

func (h *Handler) write(c *gin.Context, value any, err error) {
	switch {
	case err == nil:
		response.OK(c, value)
	case errors.Is(err, delivery.ErrForbiddenOrder), errors.Is(err, delivery.ErrNotFound):
		response.Fail(c, http.StatusNotFound, "delivery order not found")
	case errors.Is(err, delivery.ErrRiderNotRegistered):
		response.Fail(c, http.StatusForbidden, "请联系管理员开通骑手权限")
	case errors.Is(err, delivery.ErrRiderUnavailable):
		response.Fail(c, http.StatusForbidden, err.Error())
	case errors.Is(err, delivery.ErrConcurrentUpdate), errors.Is(err, delivery.ErrRiderMismatch),
		errors.Is(err, delivery.ErrInvalidTransition), errors.Is(err, delivery.ErrIdempotencyConflict),
		errors.Is(err, delivery.ErrProofConflict):
		response.Fail(c, http.StatusConflict, err.Error())
	case errors.Is(err, delivery.ErrInvalidRecord), errors.Is(err, delivery.ErrInvalidULID),
		errors.Is(err, delivery.ErrInvalidProof), errors.Is(err, delivery.ErrProofRequired):
		response.Fail(c, http.StatusBadRequest, err.Error())
	case errors.Is(err, delivery.ErrProofAssetRejected):
		response.Fail(c, http.StatusUnprocessableEntity, "delivery proof asset rejected")
	case errors.Is(err, delivery.ErrProofBindingPending):
		response.Fail(c, http.StatusServiceUnavailable, "delivery proof binding pending")
	default:
		response.Fail(c, http.StatusInternalServerError, "delivery rider operation failed")
	}
}

func parsePage(c *gin.Context) (delivery.Page, bool) {
	page, err := strconv.Atoi(defaultQuery(c, "page", "1"))
	if err != nil {
		response.Fail(c, http.StatusBadRequest, "invalid pagination")
		return delivery.Page{}, false
	}
	size, err := strconv.Atoi(defaultQuery(c, "pageSize", "20"))
	if err != nil {
		response.Fail(c, http.StatusBadRequest, "invalid pagination")
		return delivery.Page{}, false
	}
	value := delivery.Page{Page: page, Size: size}
	if !value.Valid() {
		response.Fail(c, http.StatusBadRequest, "invalid pagination")
		return delivery.Page{}, false
	}
	return value, true
}

func defaultQuery(c *gin.Context, key, fallback string) string {
	if value := c.Query(key); value != "" {
		return value
	}
	return fallback
}
