package deliveryim

import (
	"context"
	"errors"
	"net/http"
	"strconv"
	"time"

	"campus-circle/backend/internal/module/delivery"
	coremedia "campus-circle/backend/internal/module/media"
	"campus-circle/backend/internal/pkg/response"
	"github.com/gin-gonic/gin"
)

type conversationQueries interface {
	Current(context.Context, uint64, string) (delivery.Conversation, error)
	FindForParticipant(context.Context, uint64, string) (delivery.Conversation, delivery.Order, error)
	ListForParticipant(context.Context, uint64, delivery.Page) ([]delivery.Conversation, int64, error)
	PrepareImage(context.Context, uint64, string, string) (uint16, delivery.Conversation, error)
	ResolveImage(context.Context, uint64, string, uint16) (string, error)
}

type Handler struct {
	service            conversationQueries
	currentUserID      func(*gin.Context) uint64
	imageTokens        *ImageTokenCodec
	imagesEnabled      bool
	readReceiptEnabled bool
	credentials        func(context.Context, uint64) (any, error)
}

func NewHandler(
	service conversationQueries,
	currentUserID func(*gin.Context) uint64,
	imageTokens *ImageTokenCodec,
	imagesEnabled bool,
	readReceiptEnabled bool,
) *Handler {
	return &Handler{
		service: service, currentUserID: currentUserID,
		imageTokens: imageTokens, imagesEnabled: imagesEnabled,
		readReceiptEnabled: readReceiptEnabled,
	}
}

func (h *Handler) Register(group *gin.RouterGroup) {
	if h.credentials != nil {
		group.GET("/im-credentials", h.imCredentials)
	}
	group.GET("/orders/:orderId/conversation", h.current)
	group.GET("/conversations", h.list)
	group.POST("/conversations/:id/images/prepare", h.prepareImage)
	group.GET("/conversations/:id/images/:imageId/resolve", h.resolveImage)
}

func (h *Handler) WithCredentials(issuer func(context.Context, uint64) (any, error)) *Handler {
	h.credentials = issuer
	return h
}

func (h *Handler) imCredentials(c *gin.Context) {
	id := h.currentUserID(c)
	items, _, err := h.service.ListForParticipant(c.Request.Context(), id, delivery.Page{Page: 1, Size: 1})
	if err != nil {
		h.writeError(c, err)
		return
	}
	if len(items) == 0 {
		response.Fail(c, http.StatusForbidden, "没有可访问的订单会话")
		return
	}
	value, err := h.credentials(c.Request.Context(), id)
	if err != nil {
		response.Fail(c, http.StatusServiceUnavailable, "订单沟通暂时不可用")
		return
	}
	c.Header("Cache-Control", "no-store")
	response.OK(c, value)
}

type ConversationView struct {
	ConversationPublicID  string                      `json:"conversationPublicId"`
	DeliveryOrderPublicID string                      `json:"deliveryOrderPublicId"`
	AssignmentVersion     uint64                      `json:"assignmentVersion"`
	TencentConversationID string                      `json:"tencentConversationId"`
	Status                delivery.ConversationStatus `json:"status"`
	CanSend               bool                        `json:"canSend"`
	ImagesEnabled         bool                        `json:"imagesEnabled"`
	ReadReceiptEnabled    bool                        `json:"readReceiptEnabled"`
	CreatedAt             time.Time                   `json:"createdAt"`
	UpdatedAt             time.Time                   `json:"updatedAt"`
}

func (h *Handler) current(c *gin.Context) {
	value, err := h.service.Current(
		c.Request.Context(), h.currentUserID(c), c.Param("orderId"),
	)
	if err != nil {
		h.writeError(c, err)
		return
	}
	response.OK(c, h.view(value))
}

func (h *Handler) list(c *gin.Context) {
	page, err := strconv.Atoi(c.DefaultQuery("page", "1"))
	if err != nil {
		response.Fail(c, http.StatusBadRequest, "invalid pagination")
		return
	}
	size, err := strconv.Atoi(c.DefaultQuery("pageSize", "20"))
	if err != nil {
		response.Fail(c, http.StatusBadRequest, "invalid pagination")
		return
	}
	values, total, err := h.service.ListForParticipant(
		c.Request.Context(), h.currentUserID(c), delivery.Page{Page: page, Size: size},
	)
	if err != nil {
		h.writeError(c, err)
		return
	}
	items := make([]ConversationView, 0, len(values))
	for _, value := range values {
		items = append(items, h.view(value))
	}
	response.OK(c, gin.H{
		"items": items, "page": page, "pageSize": size, "total": total,
	})
}

func (h *Handler) prepareImage(c *gin.Context) {
	if !h.imagesEnabled || h.imageTokens == nil {
		response.Fail(c, http.StatusNotFound, "delivery conversation image not found")
		return
	}
	var request struct {
		AssetPublicID string `json:"assetPublicId"`
	}
	if c.ShouldBindJSON(&request) != nil {
		response.Fail(c, http.StatusBadRequest, "invalid delivery conversation image request")
		return
	}
	slot, value, err := h.service.PrepareImage(
		c.Request.Context(), h.currentUserID(c), c.Param("id"), request.AssetPublicID,
	)
	if err != nil {
		h.writeError(c, err)
		return
	}
	imageID := h.imageTokens.Encode(value.ConversationPublicID, slot)
	response.Created(c, gin.H{
		"imageId": imageID,
		"customPayload": gin.H{
			"type": "delivery_im_image", "conversationId": value.ConversationPublicID, "imageId": imageID,
		},
	})
}

func (h *Handler) resolveImage(c *gin.Context) {
	if h.imageTokens == nil {
		response.Fail(c, http.StatusNotFound, "delivery conversation image not found")
		return
	}
	slot, err := h.imageTokens.Decode(c.Param("imageId"), c.Param("id"))
	if err != nil {
		response.Fail(c, http.StatusNotFound, "delivery conversation image not found")
		return
	}
	value, err := h.service.ResolveImage(
		c.Request.Context(), h.currentUserID(c), c.Param("id"), slot,
	)
	if err != nil {
		h.writeError(c, err)
		return
	}
	c.Header("Cache-Control", "no-store")
	response.OK(c, gin.H{
		"url": value, "expiresInSeconds": int64(delivery.PrivateConversationImageURLTTL.Seconds()),
	})
}

func (h *Handler) view(value delivery.Conversation) ConversationView {
	return ConversationView{
		ConversationPublicID:  value.ConversationPublicID,
		DeliveryOrderPublicID: value.DeliveryOrderPublicID,
		AssignmentVersion:     value.AssignmentVersion,
		TencentConversationID: "GROUP" + value.TencentGroupID,
		Status:                value.Status, CanSend: value.Status == delivery.ConversationActive,
		ImagesEnabled: h.imagesEnabled, ReadReceiptEnabled: h.readReceiptEnabled,
		CreatedAt: value.CreatedAt, UpdatedAt: value.UpdatedAt,
	}
}

func (h *Handler) writeError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, delivery.ErrConversationNotFound),
		errors.Is(err, delivery.ErrConversationForbidden),
		errors.Is(err, delivery.ErrNotFound),
		errors.Is(err, coremedia.ErrNotFound),
		errors.Is(err, coremedia.ErrMediaNotVisible):
		response.Fail(c, http.StatusNotFound, "delivery conversation not found")
	case errors.Is(err, delivery.ErrInvalidRecord), errors.Is(err, delivery.ErrInvalidULID):
		response.Fail(c, http.StatusBadRequest, "invalid delivery conversation request")
	case errors.Is(err, delivery.ErrConversationReadOnly):
		response.Fail(c, http.StatusConflict, "delivery conversation is read-only")
	case errors.Is(err, delivery.ErrConversationConcurrent):
		response.Fail(c, http.StatusConflict, "delivery conversation changed; refresh and retry")
	case errors.Is(err, delivery.ErrConversationImageDenied):
		response.Fail(c, http.StatusUnprocessableEntity, "delivery conversation image rejected")
	case errors.Is(err, delivery.ErrConversationUnavailable),
		errors.Is(err, delivery.ErrConversationLease),
		errors.Is(err, delivery.ErrConversationImagePending):
		response.Fail(c, http.StatusServiceUnavailable, "delivery conversation unavailable")
	default:
		response.Fail(c, http.StatusInternalServerError, "delivery conversation operation failed")
	}
}
