package deliveryproof

import (
	"context"
	"errors"
	"net/http"
	"strconv"
	"time"

	"campus-circle/backend/internal/module/delivery"
	"campus-circle/backend/internal/pkg/response"
	"github.com/gin-gonic/gin"
)

type Handler struct {
	service       proofQueries
	currentUserID func(*gin.Context) uint64
}

type proofQueries interface {
	ListForParticipant(context.Context, uint64, string) ([]delivery.Proof, error)
	ResolveForParticipant(context.Context, uint64, string, delivery.ProofType, uint16) (string, error)
}

func NewHandler(service proofQueries, currentUserID func(*gin.Context) uint64) *Handler {
	return &Handler{service: service, currentUserID: currentUserID}
}

func (h *Handler) Register(group *gin.RouterGroup) {
	group.GET("/:orderId/proofs", h.list)
	group.GET("/:orderId/proofs/:proofType/:slot/resolve", h.resolve)
}

type ProofView struct {
	ProofPublicID string               `json:"proofPublicId"`
	ProofType     delivery.ProofType   `json:"proofType"`
	Slot          uint16               `json:"slot"`
	Status        delivery.ProofStatus `json:"status"`
	CreatedAt     time.Time            `json:"createdAt"`
	ActivatedAt   *time.Time           `json:"activatedAt"`
}

func (h *Handler) list(c *gin.Context) {
	values, err := h.service.ListForParticipant(
		c.Request.Context(), h.currentUserID(c), c.Param("orderId"),
	)
	if err != nil {
		h.writeError(c, err)
		return
	}
	items := make([]ProofView, 0, len(values))
	for _, value := range values {
		items = append(items, ProofView{
			ProofPublicID: value.ProofPublicID, ProofType: value.ProofType,
			Slot: value.Slot, Status: value.Status, CreatedAt: value.CreatedAt,
			ActivatedAt: value.ActivatedAt,
		})
	}
	response.OK(c, gin.H{"items": items})
}

func (h *Handler) resolve(c *gin.Context) {
	slot, err := strconv.ParseUint(c.Param("slot"), 10, 16)
	if err != nil {
		response.Fail(c, http.StatusBadRequest, "invalid delivery proof coordinate")
		return
	}
	value, err := h.service.ResolveForParticipant(
		c.Request.Context(), h.currentUserID(c), c.Param("orderId"),
		delivery.ProofType(c.Param("proofType")), uint16(slot),
	)
	if err != nil {
		h.writeError(c, err)
		return
	}
	c.Header("Cache-Control", "no-store")
	response.OK(c, gin.H{
		"url": value, "expiresInSeconds": int64(delivery.PrivateProofURLTTL.Seconds()),
	})
}

func (h *Handler) writeError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, delivery.ErrForbiddenOrder), errors.Is(err, delivery.ErrNotFound),
		errors.Is(err, delivery.ErrProofNotActive):
		response.Fail(c, http.StatusNotFound, "delivery proof not found")
	case errors.Is(err, delivery.ErrInvalidProof), errors.Is(err, delivery.ErrInvalidULID):
		response.Fail(c, http.StatusBadRequest, "invalid delivery proof request")
	default:
		response.Fail(c, http.StatusInternalServerError, "delivery proof operation failed")
	}
}
