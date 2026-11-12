package notification

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strconv"
	"time"

	"campus-circle/backend/internal/pkg/response"
	"github.com/gin-gonic/gin"
)

// CurrentUserID must read an identity verified by the application's existing
// authentication middleware. A missing resolver or zero identity denies access.
type CurrentUserID func(*gin.Context) uint64

type Handler struct {
	inbox         InboxRepository
	subscriptions *SubscriptionService
	currentUserID CurrentUserID
}

func NewHandler(inbox InboxRepository, subscriptions *SubscriptionService, currentUserID CurrentUserID) *Handler {
	return &Handler{inbox: inbox, subscriptions: subscriptions, currentUserID: currentUserID}
}

// Register is called only by the composition root on its authenticated group.
func (h *Handler) Register(api *gin.RouterGroup) {
	group := api.Group("/notification", h.authenticate)
	group.GET("/inbox", h.listInbox)
	group.POST("/inbox/:id/read", h.readInbox)
	group.GET("/subscribe/templates", h.templates)
	group.POST("/subscribe/nonce", h.issueNonce)
	group.POST("/subscribe/authorize", h.authorize)
}

const authenticatedRecipientKey = "notification.authenticated_recipient"

func (h *Handler) authenticate(c *gin.Context) {
	c.Header("Cache-Control", "no-store")
	if h.currentUserID == nil {
		response.Fail(c, http.StatusUnauthorized, "请先登录")
		c.Abort()
		return
	}
	userID := h.currentUserID(c)
	if userID == 0 {
		response.Fail(c, http.StatusUnauthorized, "请先登录")
		c.Abort()
		return
	}
	c.Set(authenticatedRecipientKey, userID)
	c.Next()
}

func recipient(c *gin.Context) uint64 {
	value, _ := c.Get(authenticatedRecipientKey)
	userID, _ := value.(uint64)
	return userID
}

type inboxView struct {
	ID                string              `json:"id"`
	Purpose           NotificationPurpose `json:"purpose"`
	Title             string              `json:"title"`
	Summary           string              `json:"summary"`
	AggregateType     string              `json:"aggregateType"`
	AggregatePublicID string              `json:"aggregatePublicId"`
	TargetPath        string              `json:"targetPath"`
	Read              bool                `json:"read"`
	ReadAt            *time.Time          `json:"readAt"`
	CreatedAt         time.Time           `json:"createdAt"`
}

func (h *Handler) listInbox(c *gin.Context) {
	limit := 20
	if raw, present := c.GetQuery("limit"); present {
		value, err := strconv.Atoi(raw)
		if err != nil || value < 1 || value > 100 {
			response.Fail(c, http.StatusBadRequest, "通知分页参数无效")
			return
		}
		limit = value
	}
	cursor := c.Query("cursor")
	if cursor != "" && !validID(cursor) {
		response.Fail(c, http.StatusBadRequest, "通知分页参数无效")
		return
	}
	if h.inbox == nil {
		response.Fail(c, http.StatusServiceUnavailable, "暂时无法读取通知")
		return
	}
	items, unread, err := h.inbox.ListInbox(c.Request.Context(), recipient(c), cursor, limit)
	if err != nil {
		response.Fail(c, http.StatusServiceUnavailable, "暂时无法读取通知")
		return
	}
	views := make([]inboxView, 0, len(items))
	for _, item := range items {
		// Keep the HTTP boundary closed even if an adapter returns a wrong row.
		if item.RecipientUserID != recipient(c) || (item.Status != DeliveryDelivered && item.Status != DeliveryRead) {
			response.Fail(c, http.StatusServiceUnavailable, "暂时无法读取通知")
			return
		}
		views = append(views, inboxView{ID: item.InboxPublicID, Purpose: item.Purpose, Title: item.Title, Summary: item.Summary,
			AggregateType: item.AggregateType, AggregatePublicID: item.AggregatePublicID, TargetPath: item.TargetPath, Read: item.Status == DeliveryRead, ReadAt: item.ReadAt, CreatedAt: item.CreatedAt})
	}
	next := ""
	if len(items) == limit {
		next = items[len(items)-1].InboxPublicID
	}
	response.OK(c, gin.H{"items": views, "unreadCount": unread, "nextCursor": next})
}

func (h *Handler) readInbox(c *gin.Context) {
	id := c.Param("id")
	if !validID(id) {
		response.Fail(c, http.StatusBadRequest, "通知编号无效")
		return
	}
	if h.inbox == nil {
		response.Fail(c, http.StatusServiceUnavailable, "暂时无法更新通知")
		return
	}
	found, err := h.inbox.MarkInboxRead(c.Request.Context(), recipient(c), id, time.Now().UTC())
	if err != nil {
		response.Fail(c, http.StatusServiceUnavailable, "暂时无法更新通知")
		return
	}
	if !found {
		response.Fail(c, http.StatusNotFound, "通知不存在")
		return
	}
	response.OK(c, gin.H{"id": id, "read": true})
}

func (h *Handler) templates(c *gin.Context) {
	if h.subscriptions == nil {
		notificationError(c, ErrSubscriptionEmpty)
		return
	}
	items, err := h.subscriptions.TemplatesForUser(c.Request.Context(), recipient(c))
	if err != nil {
		notificationError(c, err)
		return
	}
	response.OK(c, gin.H{"items": items})
}

type nonceInput struct {
	TemplateKey string `json:"templateKey"`
}

type authorizationInput struct {
	TemplateKey string `json:"templateKey"`
	Result      string `json:"result"`
	Nonce       string `json:"nonce"`
}

func readNotificationJSON(c *gin.Context, target any) bool {
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, 4096)
	decoder := json.NewDecoder(c.Request.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		response.Fail(c, http.StatusBadRequest, "订阅参数格式无效")
		return false
	}
	if err := decoder.Decode(new(any)); err != io.EOF {
		response.Fail(c, http.StatusBadRequest, "订阅参数格式无效")
		return false
	}
	return true
}

func (h *Handler) issueNonce(c *gin.Context) {
	var input nonceInput
	if !readNotificationJSON(c, &input) {
		return
	}
	if h.subscriptions == nil {
		notificationError(c, ErrSubscriptionEmpty)
		return
	}
	value, err := h.subscriptions.IssueNonce(c.Request.Context(), recipient(c), input.TemplateKey)
	if err != nil {
		notificationError(c, err)
		return
	}
	response.OK(c, value)
}

func (h *Handler) authorize(c *gin.Context) {
	var input authorizationInput
	if !readNotificationJSON(c, &input) {
		return
	}
	if h.subscriptions == nil {
		notificationError(c, ErrSubscriptionEmpty)
		return
	}
	duplicate, err := h.subscriptions.Authorize(c.Request.Context(), recipient(c), input.TemplateKey, input.Result, input.Nonce)
	if err != nil {
		notificationError(c, err)
		return
	}
	response.OK(c, gin.H{"recorded": true, "duplicate": duplicate})
}

func notificationError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, ErrInvalidRecord), errors.Is(err, ErrAuthorizationNonce):
		response.Fail(c, http.StatusBadRequest, "订阅参数无效或授权请求已过期")
	case errors.Is(err, ErrAuthorizationConflict):
		response.Fail(c, http.StatusConflict, "授权结果已上报，请重新发起订阅")
	case errors.Is(err, ErrAuthorizationRateLimited):
		response.Fail(c, http.StatusTooManyRequests, "操作过于频繁，请稍后重试")
	default:
		response.Fail(c, http.StatusServiceUnavailable, "订阅暂不可用，站内通知不受影响")
	}
}
