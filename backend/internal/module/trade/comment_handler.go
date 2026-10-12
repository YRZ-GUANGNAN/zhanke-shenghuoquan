package trade

import (
	"context"
	"errors"
	"net/http"
	"strconv"
	"strings"

	"campus-circle/backend/internal/module/admin/review"
	"campus-circle/backend/internal/module/featureflags"
	"campus-circle/backend/internal/module/user"
	"campus-circle/backend/internal/pkg/response"
	"github.com/gin-gonic/gin"
)

type ProductCommentHandler struct {
	service *ProductCommentService
	users   *user.Service
	safety  review.TextSafetyChecker
}

func NewProductCommentHandler(service *ProductCommentService, users *user.Service, safety review.TextSafetyChecker) *ProductCommentHandler {
	return &ProductCommentHandler{service: service, users: users, safety: safety}
}

func (h *ProductCommentHandler) Register(rg *gin.RouterGroup) {
	auth := rg.Group("", h.users.AuthRequired())
	auth.GET("/products/:id/comments", h.list)
	auth.POST("/products/:id/comments", user.UnmutedRequired(), h.create)
	auth.POST("/products/:id/comments/:commentId/replies", user.UnmutedRequired(), h.reply)
}

type productCommentRequest struct {
	Content         string `json:"content"`
	ClientRequestID string `json:"clientRequestId"`
}

func (h *ProductCommentHandler) list(c *gin.Context) {
	current, ok := authenticatedProductCommentUser(c)
	if !ok {
		return
	}
	if !featureflags.Can(featureflags.KeySecondhand) {
		response.Fail(c, http.StatusForbidden, "该功能暂未开放")
		return
	}
	productID, ok := paramID(c)
	if !ok {
		return
	}
	page, pageSize := pagination(c)
	value, err := h.service.List(c, current.ID, productID, page, pageSize)
	writeProductCommentResult(c, value, err)
}

func (h *ProductCommentHandler) create(c *gin.Context) {
	current, ok := authenticatedProductCommentUser(c)
	if !ok {
		return
	}
	if !featureflags.Can(featureflags.KeySecondhandComments) {
		response.Fail(c, http.StatusForbidden, "评论功能暂未开放")
		return
	}
	productID, ok := paramID(c)
	if !ok {
		return
	}
	request, ok := bindProductCommentRequest(c)
	if !ok {
		return
	}
	request.ClientRequestID, ok = productCommentRequestID(c, request.ClientRequestID)
	if !ok {
		return
	}
	value, replayed, err := h.service.Create(
		c,
		current.ID,
		productID,
		ProductCommentInput{Content: request.Content, ClientRequestID: request.ClientRequestID},
		h.checkText(c, current.OpenID, request.Content),
	)
	writeProductCommentCreateResult(c, value, replayed, err)
}

func (h *ProductCommentHandler) reply(c *gin.Context) {
	current, ok := authenticatedProductCommentUser(c)
	if !ok {
		return
	}
	if !featureflags.Can(featureflags.KeySecondhandComments) {
		response.Fail(c, http.StatusForbidden, "评论功能暂未开放")
		return
	}
	productID, ok := paramID(c)
	if !ok {
		return
	}
	commentID, ok := productCommentParamID(c, "commentId")
	if !ok {
		return
	}
	request, ok := bindProductCommentRequest(c)
	if !ok {
		return
	}
	request.ClientRequestID, ok = productCommentRequestID(c, request.ClientRequestID)
	if !ok {
		return
	}
	value, replayed, err := h.service.Reply(
		c,
		current.ID,
		productID,
		commentID,
		ProductCommentInput{Content: request.Content, ClientRequestID: request.ClientRequestID},
		h.checkText(c, current.OpenID, request.Content),
	)
	writeProductCommentCreateResult(c, value, replayed, err)
}

func authenticatedProductCommentUser(c *gin.Context) (user.User, bool) {
	current := user.CurrentUser(c)
	if current.ID == 0 {
		response.Fail(c, http.StatusUnauthorized, "authentication required")
		return user.User{}, false
	}
	return current, true
}

func bindProductCommentRequest(c *gin.Context) (productCommentRequest, bool) {
	var request productCommentRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		response.Fail(c, http.StatusBadRequest, "invalid product comment")
		return productCommentRequest{}, false
	}
	return request, true
}

func productCommentRequestID(c *gin.Context, bodyValue string) (string, bool) {
	value, err := resolveCreateRequestID(c.GetHeader("Idempotency-Key"), bodyValue)
	if err != nil || strings.TrimSpace(value) == "" {
		if err == nil {
			err = ErrInvalidProductComment
		}
		response.Fail(c, http.StatusBadRequest, err.Error())
		return "", false
	}
	return value, true
}

func productCommentParamID(c *gin.Context, name string) (uint, bool) {
	value, err := strconv.ParseUint(c.Param(name), 10, 64)
	converted := uint(value)
	if err != nil || value == 0 || uint64(converted) != value {
		response.Fail(c, http.StatusBadRequest, "invalid "+name)
		return 0, false
	}
	return converted, true
}

func (h *ProductCommentHandler) checkText(c *gin.Context, openID, content string) string {
	if h.safety == nil {
		return "manual:wechat_safety_unavailable"
	}
	return h.safety.CheckText(c.Request.Context(), openID, content)
}

func writeProductCommentCreateResult(c *gin.Context, value ProductCommentView, replayed bool, err error) {
	if err != nil {
		writeProductCommentError(c, err)
		return
	}
	if replayed {
		c.Header("Idempotency-Replayed", "true")
		response.OK(c, value)
		return
	}
	response.Created(c, value)
}

func writeProductCommentResult(c *gin.Context, value any, err error) {
	if err != nil {
		writeProductCommentError(c, err)
		return
	}
	response.OK(c, value)
}

func writeProductCommentError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, ErrInvalidProductComment), errors.Is(err, ErrWriteContractInvalid):
		response.Fail(c, http.StatusBadRequest, err.Error())
	case errors.Is(err, ErrNotFound):
		response.Fail(c, http.StatusNotFound, err.Error())
	case errors.Is(err, ErrConflict),
		errors.Is(err, ErrIdempotencyConflict),
		errors.Is(err, ErrProductCommentDuplicate):
		response.Fail(c, http.StatusConflict, err.Error())
	case errors.Is(err, ErrProductCommentRateLimited):
		response.Fail(c, http.StatusTooManyRequests, err.Error())
	case errors.Is(err, ErrWriteContractUnavailable):
		response.Fail(c, http.StatusServiceUnavailable, err.Error())
	case errors.Is(err, context.DeadlineExceeded):
		response.Fail(c, http.StatusGatewayTimeout, err.Error())
	default:
		response.Fail(c, http.StatusInternalServerError, err.Error())
	}
}
