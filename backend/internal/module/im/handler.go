package im

import (
	"errors"
	"net/http"
	"strconv"

	"campus-circle/backend/internal/module/featureflags"
	"campus-circle/backend/internal/module/user"
	"campus-circle/backend/internal/pkg/response"
	"github.com/gin-gonic/gin"
)

type Handler struct {
	service *Service
	users   *user.Service
}

func NewHandler(service *Service, users *user.Service) *Handler {
	return &Handler{service: service, users: users}
}
func (h *Handler) Register(rg *gin.RouterGroup) {
	auth := rg.Group("", h.users.AuthRequired())
	auth.GET("/usersig", h.credentials)
	auth.GET("/conversations", h.conversations)
	auth.GET("/conversations/:relationId", h.conversation)
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
	value, err := h.service.Conversations(c, user.CurrentUserID(c), page, size)
	if err != nil {
		response.Fail(c, 500, "list conversations failed")
		return
	}
	response.OK(c, value)
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
	response.OK(c, value)
}
