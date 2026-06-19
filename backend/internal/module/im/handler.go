package im

import (
	"campus-circle/backend/internal/module/user"
	"campus-circle/backend/internal/pkg/response"
	"github.com/gin-gonic/gin"
	"net/http"
	"strconv"
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
}
func (h *Handler) credentials(c *gin.Context) {
	value, err := h.service.Credentials(c, user.CurrentUserID(c))
	if err != nil {
		response.Fail(c, http.StatusServiceUnavailable, err.Error())
		return
	}
	response.OK(c, value)
}
func (h *Handler) conversations(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	size, _ := strconv.Atoi(c.DefaultQuery("pageSize", "20"))
	value, err := h.service.Conversations(c, user.CurrentUserID(c), page, size)
	if err != nil {
		response.Fail(c, 500, "list conversations failed")
		return
	}
	response.OK(c, value)
}
