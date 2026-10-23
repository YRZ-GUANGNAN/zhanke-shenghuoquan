package im

import (
	"context"
	"errors"
	"net/http"
	"strconv"

	"campus-circle/backend/internal/module/featureflags"
	"campus-circle/backend/internal/module/user"
	"campus-circle/backend/internal/pkg/response"
	"github.com/gin-gonic/gin"
)

type Handler struct {
	service        *Service
	users          *user.Service
	profiles       func(context.Context, []uint) (map[uint]user.Profile, error)
	contactAllowed func(context.Context, uint, uint) error
}

func NewHandler(service *Service, users *user.Service) *Handler {
	handler := &Handler{service: service, users: users}
	if users != nil {
		handler.profiles = users.Profiles
		handler.contactAllowed = users.ValidatePrivateContact
	}
	return handler
}
func (h *Handler) Register(rg *gin.RouterGroup) {
	auth := rg.Group("", h.users.AuthRequired())
	auth.GET("/usersig", h.credentials)
	auth.GET("/conversations", h.conversations)
	auth.GET("/conversations/:relationId", h.conversation)
	auth.PUT("/users/:userId/conversation", user.UnmutedRequired(), h.contactUser)
}
func (h *Handler) credentials(c *gin.Context) {
	if !featureflags.Can(featureflags.KeyMessages) {
		response.Fail(c, http.StatusForbidden, "私信功能暂未开放")
		return
	}
	value, err := h.service.Credentials(c, user.CurrentUserID(c))
	if err != nil {
		response.Fail(c, http.StatusServiceUnavailable, err.Error())
		return
	}
	response.OK(c, value)
}
func (h *Handler) conversations(c *gin.Context) {
	if !featureflags.Can(featureflags.KeyMessages) {
		response.Fail(c, http.StatusForbidden, "私信功能暂未开放")
		return
	}
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	size, _ := strconv.Atoi(c.DefaultQuery("pageSize", "20"))
	value, err := h.service.Conversations(c, user.CurrentUserID(c), page, size, featureflags.Can(featureflags.KeySecondhandChat))
	if err != nil {
		response.Fail(c, 500, "list conversations failed")
		return
	}
	response.OK(c, conversationPageView{
		Items: h.conversationViews(c, value.Items, user.CurrentUserID(c)),
		Page:  value.Page, PageSize: value.PageSize, Total: value.Total,
	})
}

func (h *Handler) conversation(c *gin.Context) {
	if !featureflags.Can(featureflags.KeyMessages) {
		response.Fail(c, http.StatusForbidden, "私信功能暂未开放")
		return
	}
	relationID, err := strconv.ParseUint(c.Param("relationId"), 10, 64)
	if err != nil || relationID == 0 {
		response.Fail(c, http.StatusBadRequest, "invalid relation id")
		return
	}
	value, err := h.service.Conversation(c, uint(relationID), user.CurrentUserID(c))
	if errors.Is(err, ErrConversationNotFound) {
		response.Fail(c, http.StatusNotFound, "conversation not found or access denied")
		return
	}
	if err != nil {
		response.Fail(c, http.StatusInternalServerError, "get conversation failed")
		return
	}
	if value.ReferenceType == "product" && !featureflags.Can(featureflags.KeySecondhandChat) {
		response.Fail(c, http.StatusForbidden, "二手私信功能暂未开放")
		return
	}
	if value.ReferenceType == "user" && !h.allowUserContact(c, peerIDForViewer(value, user.CurrentUserID(c))) {
		return
	}
	response.OK(c, h.conversationViews(c, []ConversationContext{value}, user.CurrentUserID(c))[0])
}

func (h *Handler) allowUserContact(c *gin.Context, peerID uint) bool {
	if h.contactAllowed == nil {
		response.Fail(c, http.StatusServiceUnavailable, "私信服务暂不可用")
		return false
	}
	err := h.contactAllowed(c, user.CurrentUserID(c), peerID)
	if errors.Is(err, user.ErrPrivateContactUnavailable) {
		response.Fail(c, http.StatusForbidden, "当前无法与该用户私信")
		return false
	}
	if err != nil {
		response.Fail(c, http.StatusServiceUnavailable, "私信服务暂不可用，请稍后重试")
		return false
	}
	return true
}

func (h *Handler) contactUser(c *gin.Context) {
	if !featureflags.Can(featureflags.KeyMessages) {
		response.Fail(c, http.StatusForbidden, "私信功能暂未开放")
		return
	}
	peerID, err := strconv.ParseUint(c.Param("userId"), 10, 64)
	if err != nil || peerID == 0 || uint(peerID) == user.CurrentUserID(c) {
		response.Fail(c, http.StatusBadRequest, "请选择其他用户发起私信")
		return
	}
	if !h.allowUserContact(c, uint(peerID)) {
		return
	}
	value, err := h.service.EnsureUserConversation(c, user.CurrentUserID(c), uint(peerID))
	if err != nil {
		response.Fail(c, http.StatusServiceUnavailable, "私信连接暂不可用，请稍后重试")
		return
	}
	response.OK(c, h.conversationViews(c, []ConversationContext{value}, user.CurrentUserID(c))[0])
}
