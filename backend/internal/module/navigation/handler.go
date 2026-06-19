package navigation

import (
	"campus-circle/backend/internal/pkg/response"
	"github.com/gin-gonic/gin"
	"net/http"
)

type Handler struct{ service *Service }

func NewHandler(service *Service) *Handler { return &Handler{service: service} }
func (h *Handler) Register(rg *gin.RouterGroup) {
	rg.GET("/categories", func(c *gin.Context) {
		value, err := h.service.PublicTree(c)
		if err != nil {
			response.Fail(c, http.StatusInternalServerError, "list navigation failed")
			return
		}
		response.OK(c, value)
	})
}
